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
// olmalı.
func TestTRSortedOnePerLine(t *testing.T) {
	checkSortedOnePerLine(t, trJSON, "ad")
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
	if _, ok := Suggest(NamespaceGlobal, "yazdir", func(k Code) bool { return k.Letter() == 'A' }); ok {
		t.Error("süzgeç yerleşik kodu dışlamalıydı")
	}
}

func TestIsCodeForm(t *testing.T) {
	for _, c := range []struct {
		k Code
		s string
	}{{A + 1, "A001"}, {G + 100, "G100"}, {Y + 400, "Y400"}, {W + 936, "W936"}} {
		if c.k.String() != c.s {
			t.Errorf("%d: %s, %s bekleniyordu", c.k, c.k, c.s)
		}
		if k, err := Parse(c.s); err != nil || k != c.k {
			t.Errorf("Coz(%q) = %v, %v", c.s, k, err)
		}
	}
	for _, s := range []string{"", "A01", "A0001", "Z001", "A000", "a001", "A0x1"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Coz(%q) hata vermeliydi", s)
		}
	}
	if (A+13).Namespace() != NamespaceGlobal || (A+200).Namespace() != NamespaceType || (Y+1).Namespace() != NamespaceMember ||
		(T+900).Namespace() != NamespaceParam || (G+100).Namespace() != NamespaceGlobal || (H+1).Namespace() != NamespaceNone {
		t.Error("ad alanları yanlış")
	}
	// Ayrılmış ad biçimi (2.3a): bütün büyük harfler, aile harfi olmasa da
	for s, want := range map[string]bool{"G100": true, "K001": true, "Z999": true, "A000": true,
		"g100": false, "G10": false, "G1000": false, "G10x": false, "Ç100": false, "": false} {
		if IsCodeForm(s) != want {
			t.Errorf("KodBicimi(%q) = %v, %v bekleniyordu", s, !want, want)
		}
	}
}

// Yabancı adlar (başka dillerden alışkanlık: lr, axis) kabul edilmez; FindForeign ve Suggest
// asıl adı verir. accept süzgeci öbür kodları dışlar.
func TestFindForeign(t *testing.T) {
	for _, c := range []struct {
		name string
		code Code
	}{{"lr", T + 910}, {"axis", T + 902}} {
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
	if _, _, ok := FindForeign(NamespaceParam, "lr", func(k Code) bool { return k == T+902 }); ok {
		t.Error("süzgeç öbür kodları dışlamalıydı")
	}
}

// Yukle kod listesi hatalarını hangi kodların çakıştığını söyleyerek yakalar.
func TestClashesCaught(t *testing.T) {
	for _, c := range []struct{ json, want string }{
		{`{"G100": {"ad": "a"}, "G100": {"ad": "b"}}`, "G100 is defined twice"},
		{`{"G100": {"ad": "yazdır"}, "G101": {"ad": "yazdır"}}`, "G100 \"yazdır\" and G101"},
		{`{"G100": {"ad": "yazdır"}, "G101": {"ad": "yazdir"}}`, "G100 \"yazdır\" and G101 \"yazdir\""},
		{`{"A001": {"ad": "eğer"}, "G100": {"ad": "eger"}}`, "A001 \"eğer\" and G100"},
		{`{"G100": {"ad": "yazdır"}, "G101": {"ad": "girdi", "yabancı": ["yazdir"]}}`, "the foreign name \"yazdir\" of G101 clashes with G100"},
		{`{"G100": {"ad": "a", "yabancı": ["çık"]}, "G101": {"ad": "b", "yabancı": ["cik"]}}`, "\"cik\" is registered for both G100 and G101"},
		{`{"G100": {"ad": "a", "yabancı": ["a"]}}`, "must not be the name itself"},
		{`{"G100": {"ad": "a", "eski": ["b"]}}`, "unknown field \"eski\""},
		{`{"G100": {"ad": "a b"}}`, "contain spaces"},
		{`{"G100": {"ad": "yazdır", "es": ["yaz"]}}`, "unknown field \"es\""},
		{`{"X100": {"ad": "a"}}`, "family letter"},
		{`{"H100": {"ad": "a"}}`, "is not used yet"},
		{`{"G100": {"ad": "a", "bilinmeyen": 1}}`, "bilinmeyen"},
	} {
		_, err := Load("deneme", []byte(c.json))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.json, c.want, err)
		}
	}
	// Farklı ad alanlarında aynı ad serbesttir.
	if _, err := Load("deneme", []byte(`{"G303": {"ad": "sırala"}, "Y202": {"ad": "sırala"}, "W932": {"ad": "sırala"}}`)); err != nil {
		t.Errorf("farklı ad alanlarında aynı ad kabul edilmeliydi: %v", err)
	}
}

// Her adın tek yazımı vardır: yalnızca Türkçe biçimi kabul edilir.
func TestSingleSpelling(t *testing.T) {
	for _, k := range Codes() {
		if y := Spellings(k); len(y) != 1 || y[0] != Name(k) {
			t.Errorf("%s tek yazımlı olmalı (%q): %v", k, Name(k), y)
		}
	}
	if y := Spellings(G + 100); len(y) != 1 || y[0] != "yazdır" {
		t.Errorf("G100 yalnızca Türkçe biçimiyle kabul edilmeli: %v", y)
	}
	if y := Spellings(G + 400); len(y) != 1 {
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
		for _, k := range Active().Codes() {
			if _, ok := d.Entry(k); !ok {
				t.Errorf("%s dilinde %s (%s) adsız", langCode, k, Active().Name(k))
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
		if Active().Code != "tr" {
			t.Fatalf("etkin dil %q, tr bekleniyordu", Active().Code)
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
// K004, seçenek olarak K101; 'izle' komut olarak K012, hata ayıklayıcıda K307.
func TestCommandLineNamesInEveryLanguage(t *testing.T) {
	for _, d := range []struct {
		find func(string) (Code, bool)
		name string
		want Code
	}{
		{FindCommand, "çalıştır", K + 3}, {FindCommand, "run", K + 3}, {FindCommand, "kontrol", K + 4},
		{FindOption, "kontrol", K + 101}, {FindOption, "check", K + 101}, {FindOption, "go_profile", K + 104},
		{FindCommand, "izle", K + 12}, {FindDebuggerCommand, "izle", K + 307}, {FindDebuggerCommand, "continue", K + 303},
		{FindCommand, "çıkış", K + 200}, {FindCommand, "exit", K + 200},
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

// 'tr dizin' alt komutları (K016-K020) yalnızca dizin komutunun altında geçerlidir: üst düzey
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
		if k, ok := FindCommand(name); !ok || k != K015 {
			t.Errorf("%q is the index command K015, got %s %v", name, k, ok)
		}
		if _, ok := FindSubcommand(name); ok {
			t.Errorf("%q is not a subcommand of itself", name)
		}
	}
	if K015.IsIndexSubcommand() || K015.Namespace() != NamespaceCommand {
		t.Error("K015 itself is a command")
	}
	for _, k := range []Code{K016, K017, K018, K019, K020} {
		if !k.IsIndexSubcommand() {
			t.Errorf("%s should be an index subcommand", k)
		}
	}
}

// Kural: OptionFlag seçenek kodunu komut satırı bayrağına çevirir: "--" ve alt çizgi yerine tire.
// Neden: cmd/tr ve codeindex aynı çevirmeyi ayrı ayrı yazıyordu.
func TestOptionFlagHyphenatesDisplayName(t *testing.T) {
	for _, k := range []Code{K100, K106} {
		want := "--" + strings.ReplaceAll(DisplayName(k), "_", "-")
		if got := OptionFlag(k); got != want || strings.Contains(got, "_") {
			t.Errorf("OptionFlag(%s) = %q, beklenen %q", k, got, want)
		}
	}
}
