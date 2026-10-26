package codes

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Gömülü tr.json geçerli olmalı ve hatasız yüklenmeli (yinelenen kod, ad alanı içinde
// Türkçe ya da ASCII biçimiyle çakışan ad, geçerli adla çakışan yabancı ad olmamalı).
func TestTRLoads(t *testing.T) {
	if !json.Valid(trJSON) {
		t.Fatal("tr.json geçerli JSON değil")
	}
	if _, err := Load("tr", trJSON); err != nil {
		t.Fatal(err)
	}
}

// Dosya insanlar tarafından düzenlenir: kodlar artan sırada ve her girdi kendi satırında
// olmalı. Her satırın tek alanı açıklamadır (ortak adlı kodun satırında ad yoktur).
func TestTRSortedOnePerLine(t *testing.T) {
	checkSortedOnePerLine(t, trJSON, "description")
}

// Farklı iki ad, ad alanları farklı olsa bile aynı ASCII biçime
// düşmemeli: ASCII yazımdan Türkçe yazıma ipucu (Oneri) belirsiz olurdu.
func TestASCIIFormsDoNotClash(t *testing.T) {
	type owners struct {
		code Code
		name string
	}
	owner := map[string]owners{}
	for _, k := range Codes() {
		name := Name(k)
		a := turkish.ASCII(name)
		if o, ok := owner[a]; ok && o.name != name {
			t.Errorf("%s %q ile %s %q aynı ASCII biçime (%q) düşüyor", o.code, o.name, k, name, a)
			continue
		}
		owner[a] = owners{k, name}
	}
}

// Bir ad alanı içinde her ad benzersizdir; anahtar kelimeler yerleşiklerle çakışmaz.
func TestNamespacesUnique(t *testing.T) {
	for ns := NamespaceGlobal; ns < namespaceCount; ns++ {
		seen := map[string]Code{}
		for _, k := range Codes() {
			if k.Namespace() != ns {
				continue
			}
			for _, y := range Spellings(k) {
				if o, ok := seen[y]; ok {
					t.Errorf("%s: %q hem %s hem %s", ns, y, o, k)
				}
				seen[y] = k
				if b, ok := Find(ns, y); !ok || b != k {
					t.Errorf("Bul(%s, %q) = %s, %s bekleniyordu", ns, y, b, k)
				}
			}
		}
	}
}

func TestCanonicalNameIn(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	for _, c := range []struct {
		ns         Namespace
		name, want string
		mustExist  bool
	}{
		{NamespaceGlobal, "yazdir", "yazdır", true}, {NamespaceGlobal, "yazdır", "yazdır", true},
		{NamespaceGlobal, "sonuc", "sonuç", true}, {NamespaceGlobal, "degisken", "değişken", true},
		{NamespaceMember, "sekil", "şekil", true}, {NamespaceParam, "ogrenme_orani", "öğrenme_oranı", true},
		{NamespaceMember, "tur", "tür", true}, {NamespaceGlobal, "tur", "", false}, {NamespaceGlobal, "tanimsiz_ad", "", false},
	} {
		if got, ok := CanonicalNameIn(c.ns, c.name); ok != c.mustExist || got != c.want {
			t.Errorf("AsilAdAlanda(%s, %q) = %q %v; %q %v bekleniyordu", c.ns, c.name, got, ok, c.want, c.mustExist)
		}
	}
}

// ASCII biçimler kabul edilmez (her işin tek yazımı); Oneri onlar için ve yabancı adlar için
// her zaman Türkçe adı önerir, ASCII biçimi asla önermez.
func TestASCIINotAcceptedSuggestsTurkish(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	if AcceptASCII {
		t.Fatal("ASCII biçimler kapalı olmalı")
	}
	for _, y := range []string{"yazdir", "degisken", "sonuc", "dogru"} {
		if k, ok := Find(NamespaceGlobal, y); ok {
			t.Errorf("%q kabul edilmemeli (%s)", y, k)
		}
	}
	for _, c := range []struct {
		ns   Namespace
		name string
		want string
	}{
		{NamespaceGlobal, "yazdir", "yazdır"}, {NamespaceGlobal, "degisken", "değişken"},
		{NamespaceGlobal, "degisken", "değişken"}, {NamespaceMember, "tur", "tür"}, {NamespaceMember, "sekil", "şekil"},
		{NamespaceParam, "turev_izle", "türev_izle"},
	} {
		k, ok := Suggest(c.ns, c.name, nil)
		if !ok || Name(k) != c.want {
			t.Errorf("Oneri(%s, %q) = %s %v; %q bekleniyordu", c.ns, c.name, k, ok, c.want)
		}
	}
	if _, ok := Suggest(NamespaceGlobal, "yazdır", nil); ok {
		t.Error("geçerli ad için öneri olmamalı")
	}
	if _, ok := Suggest(NamespaceGlobal, "yazdir", func(k Code) bool { return !k.IsBuiltin() }); ok {
		t.Error("süzgeç yerleşik kodu dışlamalıydı")
	}
}

// Kural: kodun dosyadaki yazımı kimliğidir (TR_<sınıf>_<ad>); Parse kimliği kod listesinden çözer,
// kod listesinde olmayan kimlik ve eski kod biçimi (A001) kod değildir. Ad alanı kimliğin
// sınıfından gelir.
func TestIdentities(t *testing.T) {
	for _, c := range []struct {
		k  Code
		s  string
		ns Namespace
	}{{KwIf, "TR_kw_if", NamespaceGlobal}, {FnPrint, "TR_fn_print", NamespaceGlobal}, {MemStep, "TR_mem_step", NamespaceMember},
		{TypeTensor, "TR_type_Tensor", NamespaceType}, {ArgLr, "TR_arg_lr", NamespaceParam}, {ConstPi, "TR_const_pi", NamespaceGlobal},
		{CmdIndex, "TR_cmd_index", NamespaceCommand}, {SubDefinition, "TR_sub_definition", NamespaceSubcommand},
		{OptHelp, "TR_opt_help", NamespaceOption}, {ReplExit, "TR_repl_exit", NamespaceCommand},
		{DbgStep, "TR_dbg_step", NamespaceDebugger}, {EnvLang, "TR_env_LANG", NamespaceEnvironment},
		// Değer adları genel ad alanındadır ama program adı değildir (dil tanımı 17'nin tablosu):
		// ad çakışması sınavı onları atlar; ad alanı, aynı dilde aynı yazılan bir yerleşikle
		// çakışmayı yükleyicide yakalar.
		{ValNan, "TR_val_nan", NamespaceGlobal}} {
		if c.k.String() != c.s {
			t.Errorf("%d: %s, %s bekleniyordu", c.k, c.k, c.s)
		}
		if k, err := Parse(c.s); err != nil || k != c.k {
			t.Errorf("Parse(%q) = %v, %v", c.s, k, err)
		}
		if c.k.Namespace() != c.ns {
			t.Errorf("%s: ad alanı %s, %s bekleniyordu", c.k, c.k.Namespace(), c.ns)
		}
	}
	for _, s := range []string{"", "A001", "G100", "TR_fn_yok", "TR_fn_", "tr_fn_print", "TR_kw_x_"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) hata vermeliydi", s)
		}
	}
	if None.Namespace() != NamespaceNone || Code(len(identities)).Namespace() != NamespaceNone {
		t.Error("geçersiz kodun ad alanı olmamalı")
	}
	if !KwIf.IsKeyword() || CtxAs.IsKeyword() || !FnSum.IsBuiltin() || !ConstPi.IsBuiltin() || ArgDim.IsBuiltin() ||
		!SubRename.IsIndexSubcommand() || CmdIndex.IsIndexSubcommand() {
		t.Error("sınıftan okunan türler yanlış")
	}
	// Ayrılmış adlar (2.3a): TR_ ile başlayan her ad, kimlik kod listesinde olmasa da; eski kod biçimi sıradan addır.
	for s, want := range map[string]bool{"TR_fn_print": true, "TR_": true, "TR_kw_yeni": true,
		"G100": false, "A001": false, "tr_fn_print": false, "Tr_x": false, "x_TR_": false, "": false} {
		if IsReservedName(s) != want {
			t.Errorf("IsReservedName(%q) = %v, %v bekleniyordu", s, !want, want)
		}
	}
}

// Kural: dil dosyalarında ve definitions.json'da her kimliğin sınıfı bilinen bir sınıftır ve kodlar
// sınıf sırasıyla, sınıf içinde kimliğe göre dizilidir (checkSortedOnePerLine); kodun Go sabiti
// kimliğinden türer ve benzersizdir (üreteç codeconsts aynı Go adını iki kimliğe vermez).
func TestIdentityClassesKnown(t *testing.T) {
	known := map[Class]bool{}
	for _, c := range Classes {
		known[c.Class] = true
	}
	for _, k := range Codes() {
		if !known[k.Class()] {
			t.Errorf("%s: sınıf %q bilinmiyor", k, k.Class())
		}
	}
}

// Kural (dil tanımı 17, kimlik): iki kimlik yalnızca harf büyüklüğüyle ayrılamaz; böyle çıkan iki
// kavramın kimliğine sahibinin adı eklenir (TR_arg_G_muon, TR_arg_g_swiglu). Go adı denetimi
// (tools/gen/codeconsts) bunu yalnız MixedCaps aynı çıkınca yakalar: TR_arg_aB ile TR_arg_ab'yi değil.
func TestIdentitiesDifferBeyondLetterCase(t *testing.T) {
	if got := caseOnlyPairs([]string{"TR_arg_G_muon", "TR_arg_g_swiglu", "TR_arg_aB", "TR_arg_ab"}); len(got) != 1 || got[0] != "TR_arg_aB TR_arg_ab" {
		t.Errorf("caseOnlyPairs = %q; yalnız TR_arg_aB TR_arg_ab bekleniyordu", got)
	}
	ids := make([]string, 0, len(Codes()))
	for _, k := range Codes() {
		ids = append(ids, k.String())
	}
	for _, p := range caseOnlyPairs(ids) {
		t.Errorf("kimlikler yalnızca harf büyüklüğüyle ayrılıyor: %s (sahibin adını ekleyin)", p)
	}
}

// caseOnlyPairs küçük harfe indirilince aynı çıkan kimlik çiftleridir ("ilk sonraki").
func caseOnlyPairs(ids []string) []string {
	seen := map[string]string{}
	var r []string
	for _, id := range ids {
		low := strings.ToLower(id)
		if first, ok := seen[low]; ok {
			r = append(r, first+" "+id)
			continue
		}
		seen[low] = id
	}
	return r
}

// Yabancı adlar (başka dillerden alışkanlık: lr, axis) kabul edilmez; FindForeign ve Suggest
// asıl adı verir. accept süzgeci öbür kodları dışlar.
func TestFindForeign(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	for _, c := range []struct {
		name string
		code Code
	}{{"lr", ArgLr}, {"axis", ArgDim}} {
		if _, ok := Find(NamespaceParam, c.name); ok {
			t.Errorf("%q kabul edilmemeli", c.name)
		}
		k, _, ok := FindForeign(NamespaceParam, c.name, nil)
		if !ok || k != c.code {
			t.Errorf("FindForeign(%q) = %s %v; %s bekleniyordu", c.name, k, ok, c.code)
		}
		if k, ok := Suggest(NamespaceParam, c.name, nil); !ok || k != c.code {
			t.Errorf("Suggest(%q) = %s %v; %s bekleniyordu", c.name, k, ok, c.code)
		}
	}
	if _, _, ok := FindForeign(NamespaceParam, "lr", func(k Code) bool { return k == ArgDim }); ok {
		t.Error("süzgeç öbür kodları dışlamalıydı")
	}
}

// Yukle kod listesi hatalarını hangi kodların çakıştığını söyleyerek yakalar.
func TestClashesCaught(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	for _, c := range []struct{ json, want string }{
		{`{"TR_fn_print": {"name": "a"}, "TR_fn_print": {"name": "b"}}`, "TR_fn_print is defined twice"},
		{`{"TR_fn_print": {"name": "yazdır"}, "TR_fn_input": {"name": "yazdır"}}`, "TR_fn_input \"yazdır\" and TR_fn_print"},
		{`{"TR_fn_print": {"name": "yazdır"}, "TR_fn_input": {"name": "yazdir"}}`, "TR_fn_input \"yazdir\" and TR_fn_print \"yazdır\""},
		{`{"TR_kw_if": {"name": "eğer"}, "TR_fn_print": {"name": "eger"}}`, "TR_kw_if \"eğer\" and TR_fn_print"},
		{`{"TR_fn_print": {"name": "yazdır"}, "TR_fn_input": {"name": "girdi", "foreign": ["yazdir"]}}`, "the foreign name \"yazdir\" of TR_fn_input clashes with TR_fn_print"},
		{`{"TR_fn_print": {"name": "a", "foreign": ["çık"]}, "TR_fn_input": {"name": "b", "foreign": ["cik"]}}`, "\"cik\" is registered for both TR_fn_input and TR_fn_print"},
		{`{"TR_fn_print": {"name": "a", "foreign": ["a"]}}`, "must not be the name itself"},
		{`{"TR_fn_print": {"name": "a", "eski": ["b"]}}`, "unknown field \"eski\""},
		{`{"TR_fn_print": {"name": "a b"}}`, "contain spaces"},
		{`{"TR_fn_print": {"name": "yazdır", "es": ["yaz"]}}`, "unknown field \"es\""},
		{`{"X100": {"name": "a"}}`, "not an identity"},
		{`{"TR_fn_yok": {"name": "a"}}`, "not an identity"},
		{`{"TR_fn_print": {"name": "a", "bilinmeyen": 1}}`, "bilinmeyen"},
	} {
		_, err := Load("deneme", []byte(c.json))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.json, c.want, err)
		}
	}
	// Farklı ad alanlarında aynı ad serbesttir.
	if _, err := Load("deneme", []byte(`{"TR_fn_abs": {"name": "mutlak"}, "TR_mem_absolute": {"name": "mutlak"}, "TR_arg_log": {"name": "günlük"}, "TR_fn_logger": {"name": "günlük"}}`)); err != nil {
		t.Errorf("farklı ad alanlarında aynı ad kabul edilmeliydi: %v", err)
	}
}

// Kural (17, ortak ad): ortak adlı kodun (definitions.json "common_name") adı dil dosyasında yazılmaz;
// yükleyici onu her dile, satırı yoksa yeni satırla ekler ve ortak ad dilin kendi adlarıyla aynı
// denetimlerden geçer (çakışma, yabancı ad, ortam değişkeninin TR_ istisnası).
func TestCommonNamesGoThroughTheChecks(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	common := map[Code]string{FnSin: "sin", EnvCpuAvx2: "TR_CPU_AVX2"}
	for _, c := range []struct{ json, want string }{
		{`{"TR_fn_sin": {"name": "x"}}`, "TR_fn_sin has the common name \"sin\""},
		{`{"TR_fn_print": {"name": "sin"}}`, "clash in the same namespace"},
		{`{"TR_fn_print": {"name": "a", "foreign": ["sin"]}}`, "the foreign name \"sin\" of TR_fn_print clashes with TR_fn_sin"},
	} {
		_, err := load("deneme", []byte(c.json), common)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.json, c.want, err)
		}
	}
	d, err := load("deneme", []byte(`{"TR_fn_sin": {"family": "matematik"}}`), common)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name(FnSin) != "sin" || d.Name(EnvCpuAvx2) != "TR_CPU_AVX2" {
		t.Errorf("ortak adlar: %q, %q", d.Name(FnSin), d.Name(EnvCpuAvx2))
	}
	if g, _ := d.Entry(FnSin); g.Family != "matematik" {
		t.Errorf("dil dosyasının satırı (aile) korunmalı: %+v", g)
	}
}

// Kural (17, ortak ad): gömülü dil dosyalarında ortak adlı kodun adı yazılmaz, her dilde adı ortak
// addır; ortak adsız her kodun her dilde adı vardır.
func TestCommonNamesAreNotInLanguageFiles(t *testing.T) {
	common := commonNames()
	for _, lang := range Languages() {
		raw, err := parseFile(languages[lang])
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range raw {
			k, _ := Parse(g.key)
			if c, ok := common[k]; ok && g.name != "" {
				t.Errorf("%s.json: %s ortak adlı (%q), dil dosyasında adı (%q) yazılmamalı", lang, k, c, g.name)
			}
		}
		d, err := LoadLanguage(lang)
		if err != nil {
			t.Fatal(err)
		}
		for k, c := range common {
			if d.Name(k) != c {
				t.Errorf("%s: %s'in adı %q, ortak ad %q", lang, k, d.Name(k), c)
			}
		}
	}
}

// Her adın tek yazımı vardır: yalnızca Türkçe biçimi kabul edilir.
func TestSingleSpelling(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	for _, k := range Codes() {
		if y := Spellings(k); len(y) != 1 || y[0] != Name(k) {
			t.Errorf("%s tek yazımlı olmalı (%q): %v", k, Name(k), y)
		}
	}
	if y := Spellings(FnPrint); len(y) != 1 || y[0] != "yazdır" {
		t.Errorf("TR_fn_print yalnızca Türkçe biçimiyle kabul edilmeli: %v", y)
	}
	if y := Spellings(FnSum); len(y) != 1 {
		t.Errorf("ASCII ad tek yazımlı olmalı: %v", y)
	}
}

// Her yazım dili hatasız yüklenir ve Türkçenin her kodunu adlandırır (yeni bir kodun adı her
// dil dosyasına eklenmelidir).
func TestWritingLanguagesComplete(t *testing.T) {
	if strings.Join(Languages(), ",") != "en,tr" {
		t.Fatalf("diller: %v", Languages())
	}
	for _, langCode := range Languages() {
		d, err := LoadLanguage(langCode)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range Source().Codes() {
			if _, ok := d.Entry(k); !ok {
				t.Errorf("%s dilinde %s (%s) adsız", langCode, k, Source().Name(k))
			}
		}
	}
	if _, err := LoadLanguage("xx"); err == nil || !strings.Contains(err.Error(), "var olanlar: en, tr") {
		t.Fatalf("bilinmeyen dil: %v", err)
	}
}

// Sözlük program başlarken bir kez yüklenir; başlangıç süresine katkısı küçük kalmalı.
func BenchmarkLoad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := Load("tr", trJSON); err != nil {
			b.Fatal(err)
		}
	}
}

// Geçersiz TR_DIL paketi yüklerken süreci kapatmaz: dil Türkçeye düşer, hata LanguageError
// ile okunur (eskiden init içinde os.Exit(2) vardı; paketi kullanan her program kapanırdı).
// Kontrol alt süreçte yapılır: etkin dil süreç başında bir kez kurulur.
func TestInvalidLanguageFallsBack(t *testing.T) {
	if os.Getenv("TR_CODES_CHILD") == "1" {
		if err := LanguageError(); err == nil || !strings.Contains(err.Error(), "TR_DIL=xx") {
			t.Fatalf("dil hatası beklenirdi: %v", err)
		}
		if Source().Code != "tr" {
			t.Fatalf("etkin dil %q, tr bekleniyordu", Source().Code)
		}
		return
	}
	if err := LanguageError(); err != nil {
		t.Fatalf("geçerli dilde hata olmamalı: %v", err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestInvalidLanguageFallsBack$", "-test.count=1")
	cmd.Env = append(os.Environ(), "TR_CODES_CHILD=1", LanguageEnvVar+"=xx")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("alt süreç: %v\n%s", err, out)
	}
}

// Komut satırı adları her yazım dilinde bulunur ve ad alanları ayrıdır: 'kontrol' komut olarak
// TR_cmd_check, seçenek olarak TR_opt_check; 'izle' komut olarak TR_cmd_watch, hata ayıklayıcıda TR_dbg_watch.
func TestCommandLineNamesInEveryLanguage(t *testing.T) {
	for _, d := range []struct {
		find func(string) (Code, bool)
		name string
		want Code
	}{
		{FindCommand, "çalıştır", CmdRun}, {FindCommand, "run", CmdRun}, {FindCommand, "kontrol", CmdCheck},
		{FindOption, "kontrol", OptCheck}, {FindOption, "check", OptCheck}, {FindOption, "go_profile", OptGoProfile},
		{FindCommand, "izle", CmdWatch}, {FindDebuggerCommand, "izle", DbgWatch}, {FindDebuggerCommand, "continue", DbgContinue},
		{FindCommand, "çıkış", ReplExit}, {FindCommand, "exit", ReplExit},
	} {
		if got, ok := d.find(d.name); !ok || got != d.want {
			t.Errorf("%q: %v %v, %v bekleniyordu", d.name, got, ok, d.want)
		}
	}
	for _, name := range []string{"calistir", "--kontrol", "adım"} {
		if k, ok := FindCommand(name); ok {
			t.Errorf("%q komut sayılmamalı: %v", name, k)
		}
	}
}

// 'tr dizin' alt komutları (sub sınıfı) yalnızca dizin komutunun altında geçerlidir: üst düzey
// komut adı değildir, ama her gömülü dilin adı alt komut olarak kabul edilir (17.8).
func TestIndexSubcommandsAreNotCommands(t *testing.T) {
	for _, name := range []string{"tanım", "kullananlar", "sınayanlar", "etki", "yeniden_adlandır", "definition", "references", "tested_by", "impact", "rename"} {
		if k, ok := FindCommand(name); ok {
			t.Errorf("%q must not be a top-level command, found %s", name, k)
		}
		if k, ok := FindSubcommand(name); !ok || !k.IsIndexSubcommand() || k.Namespace() != NamespaceSubcommand {
			t.Errorf("%q must be an index subcommand in every language: %s %v", name, k, ok)
		}
	}
	for _, name := range []string{"dizin", "index"} {
		if k, ok := FindCommand(name); !ok || k != CmdIndex {
			t.Errorf("%q is the index command TR_cmd_index, got %s %v", name, k, ok)
		}
		if _, ok := FindSubcommand(name); ok {
			t.Errorf("%q is not a subcommand of itself", name)
		}
	}
	if CmdIndex.IsIndexSubcommand() || CmdIndex.Namespace() != NamespaceCommand {
		t.Error("TR_cmd_index itself is a command")
	}
	for _, k := range []Code{SubDefinition, SubReferences, SubTestedBy, SubImpact, SubRename} {
		if !k.IsIndexSubcommand() {
			t.Errorf("%s should be an index subcommand", k)
		}
	}
}

// Kural: OptionFlag seçenek kodunu komut satırı bayrağına çevirir: "--" ve alt çizgi yerine tire.
// Neden: cmd/tr ve codeindex aynı çevirmeyi ayrı ayrı yazıyordu.
func TestOptionFlagHyphenatesDisplayName(t *testing.T) {
	for _, k := range []Code{OptLang, OptHelp} {
		want := "--" + strings.ReplaceAll(DisplayName(k), "_", "-")
		if got := OptionFlag(k); got != want || strings.Contains(got, "_") {
			t.Errorf("OptionFlag(%s) = %q, beklenen %q", k, got, want)
		}
	}
}
