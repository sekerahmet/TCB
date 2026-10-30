package source

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// shownBody kodlu gövdenin (başlıksız) lang'deki görünümünün gövdesidir: görünüm başlığı ('# TR 4
// view=tr') atılır.
func shownBody(t *testing.T, body string, lang *codes.Language) string {
	t.Helper()
	shown, err := Show(Header+"\n"+body, lang)
	if err != nil {
		t.Fatalf("%q gösterilemedi: %v", body, err)
	}
	head := viewHeader(lang) + "\n"
	if !strings.HasPrefix(shown, head) {
		t.Fatalf("görünüm %q başlığıyla başlamalı: %q", head, shown)
	}
	return strings.TrimPrefix(shown, head)
}

// Kural (17.7): görünümde yalnız dilin sözcükleri çevrilir: kullanıcı adları, metinler, sayılar ve
// yorumlar yazıldığı gibi kalır; metnin içindeki yerleştirmede dilin sözcükleri çevrilir, metnin
// kendisi kalır; satır düzeni korunur. Neden: görünüm kodun bir dilde okunan biçimidir.
func TestShowTranslatesOnlyTheLanguageWords(t *testing.T) {
	cases := []struct{ name, coded, tr string }{
		{"ileti kodu bir ad değildir", "TR_print(TR_Message(\"H6903\", 1))\n", "yazdır(İleti(\"H6903\", 1))\n"},
		{"anahtar kelime ve yerleşik", "TR_var x = 1\nTR_print(x)\n", "değişken x = 1\nyazdır(x)\n"},
		{"kullanıcının adı kullanıcınındır",
			"TR_func f() -> TR_Int {\n    TR_var toplam = 1\n    TR_return toplam\n}\nTR_print(TR_sum([1, 2]))\n",
			"fonksiyon f() -> Tam {\n    değişken toplam = 1\n    sonuç toplam\n}\nyazdır(toplam([1, 2]))\n"},
		{"yorum ve ham metin çevrilmez",
			"# eğer yazdır\nTR_var y = `yazdır {eğer}` # değişken\n",
			"# eğer yazdır\ndeğişken y = `yazdır {eğer}` # değişken\n"},
		{"metin içi yerleştirme ve iç içe yerleştirme",
			"TR_print(\"a\\n\\(TR_sum([1])) {x} \\(\"iç \\(uzunluk_x(1)) \\(TR_print)\")\")\nTR_func uzunluk_x(a: TR_Int) -> TR_Int { TR_return a }\n",
			"yazdır(\"a\\n\\(toplam([1])) {x} \\(\"iç \\(uzunluk_x(1)) \\(yazdır)\")\")\nfonksiyon uzunluk_x(a: Tam) -> Tam { sonuç a }\n"},
		{"kurucu bağlamsal sözcüktür",
			"TR_struct N {\n    a: TR_Int\n    TR_func TR_init(a: TR_Int) { TR_self.a = a }\n}\n",
			"yapı N {\n    a: Tam\n    fonksiyon kurucu(a: Tam) { kendi.a = a }\n}\n"},
		{"adlı argüman",
			"TR_func f(eksen: TR_Int = 0) -> TR_Int { TR_return eksen }\nTR_print(TR_sum([1], TR_dim: 0), f(eksen: 1))\n",
			"fonksiyon f(eksen: Tam = 0) -> Tam { sonuç eksen }\nyazdır(toplam([1], eksen: 0), f(eksen: 1))\n"},
		{"örtük adlar",
			"TR_var o = 1\nTR_print([1].TR_map { o * 2 }, [1].TR_reduce(0, (birikim, x) => birikim + x), (o) => o)\nTR_try { TR_throw \"x\" } TR_catch { TR_print(TR_error) }\nTR_try { TR_throw \"y\" } TR_catch hata { TR_print(hata) }\n",
			"değişken o = 1\nyazdır([1].haritala { o * 2 }, [1].indirge(0, (birikim, x) => birikim + x), (o) => o)\ndene { fırlat \"x\" } yakala { yazdır(hata) }\ndene { fırlat \"y\" } yakala hata { yazdır(hata) }\n"},
		{"test ve olarak",
			"TR_import \"m\" TR_as m\nTR_test \"t\" { TR_assert(TR_true) }\nTR_var test = 1\n",
			"aktar \"m\" olarak m\ntest \"t\" { doğrula(doğru) }\ndeğişken test = 1\n"},
		{"tip açıklaması; kabul edilmeyen harf yazımı (3.14) olduğu gibi kalır",
			"TR_var a: TR_Int = 1\nTR_var b: tam = 2\nTR_print(TR_Type(a) == TR_Int)\n",
			"değişken a: Tam = 1\ndeğişken b: tam = 2\nyazdır(Tip(a) == Tam)\n"},
		{"Türkçe harfler ve eski kod biçimine benzeyen adlar",
			"TR_var İSİM_ı = \"Iİ\"\nTR_var g100 = 1\nTR_print(İSİM_ı, g100)\n",
			"değişken İSİM_ı = \"Iİ\"\ndeğişken g100 = 1\nyazdır(İSİM_ı, g100)\n"},
		{"satır düzeni korunur",
			"# başlık yorumu\n\nTR_func f(a: TR_Int) -> TR_Int {\n    TR_return a + 1\n}\n\n# iki\n# satırlık\nTR_print(f(1))\nTR_var s = \"çok\nsatırlı \\(f(2))\"\n",
			"# başlık yorumu\n\nfonksiyon f(a: Tam) -> Tam {\n    sonuç a + 1\n}\n\n# iki\n# satırlık\nyazdır(f(1))\ndeğişken s = \"çok\nsatırlı \\(f(2))\"\n"},
	}
	for _, d := range cases {
		t.Run(d.name, func(t *testing.T) {
			if got := shownBody(t, d.coded, codes.Source()); got != d.tr {
				t.Errorf("görünüm:\n%s\nbeklenen:\n%s", got, d.tr)
			}
		})
	}
}

// BOM ve CRLF satır sonları korunur; başlık görünüm başlığı olur. Başlıksız metin olduğu gibi kalır.
func TestShowKeepsBOMAndCRLF(t *testing.T) {
	coded := "\xef\xbb\xbf" + Header + "\r\nTR_var x = 1\r\nTR_print(\"a\r\nb\")\r\n"
	if got, err := Show(coded, codes.Source()); err != nil || got != "\xef\xbb\xbf"+viewHeader(codes.Source())+"\r\ndeğişken x = 1\r\nyazdır(\"a\r\nb\")\r\n" {
		t.Errorf("görünüm %q (%v)", got, err)
	}
	plain := "değişken x = 1\n"
	if same, err := Show(plain, codes.Source()); err != nil || same != plain {
		t.Errorf("başlıksız metin değişti: %q, %v", same, err)
	}
}

// fakeLanguage Türkçe dil dosyasındaki bazı adları değiştirerek ikinci bir yazım dili kurar:
// görünümün işaretinin ('$ad') başka bir dilde nasıl çalıştığını sınamak için.
func fakeLanguage(t *testing.T) *codes.Language {
	t.Helper()
	data := string(codes.TRData())
	for old, fresh := range map[string]string{
		`"TR_kw_var": { "name": "değişken"`: `"TR_kw_var": { "name": "tanım"`,
		`"TR_fn_print": { "name": "yazdır"`: `"TR_fn_print": { "name": "sayaç"`,
		`"TR_kw_if": { "name": "eğer"`:      `"TR_kw_if": { "name": "kutu"`,
		`"TR_mem_append": { "name": "ekle"`: `"TR_mem_append": { "name": "koy"`,
	} {
		if !strings.Contains(data, old) {
			t.Fatalf("dil dosyasında %q bulunamadı", old)
		}
		data = strings.Replace(data, old, fresh, 1)
	}
	d, err := codes.Load("xx", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Kural (17.7): görünüm, dilin bir sözcüğüyle aynı yazılan kullanıcı adını işaretleyebilir ('$ad');
// bu bir kural değildir, okuyanı uyarır. Çakışmayan dilde işaret yoktur.
func TestShowMarksUserNamesThatReadAsWords(t *testing.T) {
	body := "TR_var kutu = 1\nTR_var sayaç = 2\nTR_print(kutu, sayaç)\nTR_struct K {\n    TR_func koy() -> TR_Int { TR_return 1 }\n}\nTR_var bu = 3\nTR_print([1].TR_map((x) => x + bu))\n"
	want := "tanım $kutu = 1\ntanım $sayaç = 2\nsayaç($kutu, $sayaç)\nyapı K {\n    fonksiyon $koy() -> Tam { sonuç 1 }\n}\ntanım bu = 3\nsayaç([1].haritala((x) => x + bu))\n"
	if got := shownBody(t, body, fakeLanguage(t)); got != want {
		t.Errorf("sahte dilde gösterim:\n%s\nbeklenen:\n%s", got, want)
	}
	if tr := shownBody(t, body, codes.Source()); strings.Contains(tr, "$") {
		t.Errorf("Türkçe gösterimde gereksiz işaret: %s", tr)
	}
	// Yerleşikle aynı adlı kullanıcı değişkeni yerleşiğin çağrısının yanında işaretlenir.
	if got := shownBody(t, "TR_var yazdır = 2\nTR_print(yazdır)\n", codes.Source()); got != "değişken $yazdır = 2\nyazdır($yazdır)\n" {
		t.Errorf("çakışan ad: %q", got)
	}
}

// skippedDirs depoya ait olmayan ya da üretilmiş klasörlerdir.
var skippedDirs = map[string]bool{".git": true, ".claude": true, "bin": true, "scratch": true, "node_modules": true}

// repoFiles depodaki uzantısı verilen bütün dosyalardır (göreli yollarıyla).
func repoFiles(t *testing.T, ext string) []string {
	t.Helper()
	var files []string
	root := reporoot.Root()
	err := reporoot.WalkDir(root, func(path string, g fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if g.IsDir() && path != root && skippedDirs[g.Name()] {
			return filepath.SkipDir
		}
		if !g.IsDir() && strings.HasSuffix(g.Name(), ext) {
			rel, _ := filepath.Rel(root, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// shows kodlu bir metnin her gömülü dilde gösterildiğini ve görünümün satırlarının dosyanınkiyle
// aynı olduğunu sınar (17.7). Ayrıştırılamayan metin (bilerek hatalı fikstürler, hata örnekleri)
// false döner.
func shows(t *testing.T, name, text string) bool {
	t.Helper()
	for _, code := range codes.Languages() {
		lang, err := codes.LoadLanguage(code)
		if err != nil {
			t.Fatal(err)
		}
		shown, err := Show(text, lang)
		var sh *SyntaxError
		if errors.As(err, &sh) {
			return false
		}
		if err != nil {
			t.Errorf("%s: %s görünümü yazılamadı: %v", name, code, err)
			return false
		}
		if a, b := strings.Count(text, "\n"), strings.Count(shown, "\n"); a != b {
			t.Errorf("%s: %s görünümünün satır sayısı %d, dosyanınki %d", name, code, b, a)
		}
	}
	return true
}

// Depodaki her kodlu .tr dosyası her gömülü dilde gösterilir; satırları dosyanınkidir (17.7).
func TestRepoTRFilesShow(t *testing.T) {
	files := repoFiles(t, ".tr")
	if len(files) < 40 {
		t.Fatalf("depoda yalnızca %d .tr dosyası bulundu; tarama bozulmuş olabilir", len(files))
	}
	tested, skipped := 0, []string{}
	for _, d := range files {
		data, err := os.ReadFile(reporoot.Path(d))
		if err != nil {
			t.Fatal(err)
		}
		if b, err := Split(string(data)); err != nil || !b.Coded {
			skipped = append(skipped, d) // başlıksız dosya bir dosya değildir (2.1): bilerek bozuk fikstür
			continue
		}
		if shows(t, d, string(data)) {
			tested++
		} else {
			skipped = append(skipped, d)
		}
	}
	t.Logf("%d .tr dosyası sınandı; ayrıştırılamadığı için atlanan %d: %v", tested, len(skipped), skipped)
	for _, d := range skipped {
		// Bilerek hatalı dosyalar: dilin olumsuz testleri, araçların testdata fikstürleri ve hata örnekleri.
		if !strings.HasPrefix(d, "tests/negative/") && !strings.Contains(d, "/testdata/") &&
			!strings.HasPrefix(d, "examples/tr/hatalar/") {
			t.Errorf("%s ayrıştırılamadı; yalnızca bilerek hatalı olumsuz test dosyaları atlanabilir", d)
		}
	}
}

// trBlock belgelerdeki ```tr ... ``` kod bloklarıdır.
var trBlock = regexp.MustCompile("(?s)```tr\r?\n(.*?)```")

// Belgelerdeki her ```tr bloğu (başlıksız blok kodlu gövdedir) her gömülü dilde gösterilir.
func TestDocCodeBlocksShow(t *testing.T) {
	tested, skipped := 0, 0
	for _, d := range repoFiles(t, ".md") {
		data, err := os.ReadFile(reporoot.Path(d))
		if err != nil {
			t.Fatal(err)
		}
		for i, m := range trBlock.FindAllStringSubmatch(string(data), -1) {
			text := m[1]
			if b, err := Split(text); err == nil && !b.Coded {
				text = Header + "\n" + text // başlıksız blok kodlu bir gövdedir (kodların örnekleri)
			}
			if b, err := Split(text); err == nil && b.Coded && shows(t, d+" #"+strconv.Itoa(i+1), text) {
				tested++
			} else {
				skipped++
			}
		}
	}
	t.Logf("belgelerde %d tr kod bloğu sınandı, %d blok atlandı", tested, skipped)
	if tested < 50 {
		t.Errorf("belgelerde yalnızca %d kod bloğu sınandı; tarama bozulmuş olabilir", tested)
	}
}

// Kural (2.3): aynı görünen iki yazım aynı addır; görünüm kullanıcının adını sözcük çözümleyicinin
// okuduğu NFC biçimiyle yazar (ayrık o + U+0308 → ö); metin değişmezindeki ve yorumdaki ayrık yazım
// bayt bayt kalır.
func TestShowWritesNFC(t *testing.T) {
	const rest = " = \"ö\" # ö\n"
	if got, want := shownBody(t, "TR_var ögr"+rest+"TR_print(ögr)\n", codes.Source()), "değişken ögr"+rest+"yazdır(ögr)\n"; got != want {
		t.Errorf("gösterim %q, beklenen %q", got, want)
	}
}

// Kural (2.3a): kullanıcı fonksiyonunun TR_ ile yazılmış parametresi (adlı argüman kimliği) görünümde,
// kullanımları ve adlı argümanıyla birlikte, yazımıyla kalır; kitaplık fonksiyonunun parametresi
// dilin sözcüğüyle gösterilir. Neden: düz yazılmış kullanıcı parametrelerinin çoğu (x, n, adım) bir
// dilin parametre sözcüğüdür; görünüm kullanıcının kimlikle yazdığını kimlik olarak gösterir.
func TestUserFunctionIdentityParameterStaysSpelled(t *testing.T) {
	coded := Header + "\nTR_func ölçekle(x: TR_Int, TR_lr: TR_Int) -> TR_Int { TR_return x * TR_lr }\nTR_print(ölçekle(3, TR_lr: 2))\nTR_print(TR_AdamW([], TR_lr: 0.1))\n"
	for _, code := range codes.Languages() {
		lang, err := codes.LoadLanguage(code)
		if err != nil {
			t.Fatal(err)
		}
		shown, err := Show(coded, lang)
		if err != nil {
			t.Fatalf("%s: gösterilemedi: %v", code, err)
		}
		if strings.Count(shown, "TR_lr") != 4 {
			t.Errorf("%s: kullanıcı parametresi yazımıyla kalmalı:\n%s", code, shown)
		}
	}
}

// A coded file may hold an identity where it cannot stand (a member written as a global name,
// 2.3a): it cannot be read, so Show refuses it instead of showing it.
func TestShowRefusesAnUnreadableFile(t *testing.T) {
	coded := Header + "\nTR_var l = TR_length\n"
	if out, err := Show(coded, codes.Source()); err == nil {
		t.Fatalf("Show gave %q; an error expected", out)
	}
}
