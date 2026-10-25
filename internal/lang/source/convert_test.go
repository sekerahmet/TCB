package source

import (
	"errors"
	"github.com/sekerahmet/TCB/internal/testkit"
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

// encodeDecode Türkçe kaynağın kodlu biçimini verir ve kodlu biçimin çözülünce özgün kaynağa
// döndüğünü sınar.
func encodeDecode(t *testing.T, tr string) string {
	t.Helper()
	coded, err := Encode(tr)
	if err != nil {
		t.Fatalf("%q kodlanamadı: %v", tr, err)
	}
	back, err := Decode(coded, codes.Active())
	if err != nil || back != tr {
		t.Fatalf("TR → kod → TR aynı değil (%v)\nözgün: %q\nkodlu: %q\ngeri:  %q", err, tr, coded, back)
	}
	return coded
}

// Kenar durumlar (17.3, 17.4): beklenen kodlu gövde başlıktan sonraki metindir.
func TestEncodeEdgeCases(t *testing.T) {
	cases := []struct{ name, tr, coded string }{
		{"anahtar kelime ve yerleşik", "değişken x = 1\nyazdır(x)\n", "TR_kw_var x = 1\nTR_fn_print(x)\n"},
		{"gölgelenen yerleşik kullanıcının adıdır",
			"fonksiyon f() {\n    değişken toplam = 1\n    sonuç toplam\n}\nyazdır(toplam([1, 2]))\n",
			"TR_kw_func f() {\n    TR_kw_var toplam = 1\n    TR_kw_return toplam\n}\nTR_fn_print(TR_fn_sum([1, 2]))\n"},
		{"yorum ve ham metin çevrilmez",
			"// eğer yazdır\n/* değişken */ değişken y = `yazdır {eğer}`\n",
			"// eğer yazdır\n/* değişken */ TR_kw_var y = `yazdır {eğer}`\n"},
		{"metin içi yerleştirme ve iç içe yerleştirme",
			"yazdır(\"a\\n{toplam([1])} \\{x\\} {\"iç {uzunluk_x(1)} {yazdır}\"}\")\nfonksiyon uzunluk_x(a) { sonuç a }\n",
			"TR_fn_print(\"a\\n{TR_fn_sum([1])} \\{x\\} {\"iç {uzunluk_x(1)} {TR_fn_print}\"}\")\nTR_kw_func uzunluk_x(a) { TR_kw_return a }\n"},
		{"üyeler tanımda ve kullanımda aynı kodla (Y5)",
			"yapı Sepet {\n    uzunluk = 0\n    fonksiyon ekle(x) { kendi.uzunluk += 1 }\n}\ndeğişken s = Sepet(uzunluk: 2)\ns.ekle(1)\n[1].ekle(2)\n",
			"TR_kw_struct Sepet {\n    TR_mem_length = 0\n    TR_kw_func TR_mem_append(x) { TR_kw_self.TR_mem_length += 1 }\n}\nTR_kw_var s = Sepet(TR_mem_length: 2)\ns.TR_mem_append(1)\n[1].TR_mem_append(2)\n"},
		{"kurucu bağlamsal sözcüktür",
			"yapı N {\n    a\n    fonksiyon kurucu(a) { kendi.a = a }\n}\n",
			"TR_kw_struct N {\n    a\n    TR_kw_func TR_ctx_init(a) { TR_kw_self.a = a }\n}\n"},
		{"adlı argüman yalnızca dilin öğesinde",
			"fonksiyon f(eksen = 0) { sonuç eksen }\nyazdır(toplam([1], eksen: 0), f(eksen: 1))\n",
			"TR_kw_func f(eksen = 0) { TR_kw_return eksen }\nTR_fn_print(TR_fn_sum([1], TR_arg_dim: 0), f(eksen: 1))\n"},
		{"örtük adlar yalnızca kendi bloklarında",
			"değişken o = 1\nyazdır([1].haritala { o * 2 }, [1].indirge(0) { birikim + o }, (o) => o)\ndene { fırlat \"x\" } yakala { yazdır(hata) }\ndene { fırlat \"y\" } yakala hata { yazdır(hata) }\n",
			"TR_kw_var o = 1\nTR_fn_print([1].TR_mem_map { TR_ctx_it * 2 }, [1].TR_mem_reduce(0) { TR_ctx_acc + TR_ctx_it }, (o) => o)\nTR_kw_try { TR_kw_throw \"x\" } TR_kw_catch { TR_fn_print(TR_ctx_error) }\nTR_kw_try { TR_kw_throw \"y\" } TR_kw_catch hata { TR_fn_print(hata) }\n"},
		{"test ve olarak",
			"aktar \"m\" olarak m\ntest \"t\" { doğrula(doğru) }\ndeğişken test = 1\n",
			"TR_kw_import \"m\" TR_ctx_as m\nTR_ctx_test \"t\" { TR_fn_assert(TR_kw_true) }\nTR_kw_var test = 1\n"},
		{"tip açıklaması; kabul edilmeyen harf yazımı (3.14) olduğu gibi kalır",
			"değişken a: Tam = 1\ndeğişken b: tam = 2\nyazdır(tip(a) == Tam)\n",
			"TR_kw_var a: TR_type_Int = 1\nTR_kw_var b: tam = 2\nTR_fn_print(TR_fn_type(a) == TR_type_Int)\n"},
		{"Türkçe harfler ve kod biçimine benzeyen adlar",
			"değişken İSİM_ı = \"Iİ\"\ndeğişken g100 = 1\nyazdır(İSİM_ı, g100)\n",
			"TR_kw_var İSİM_ı = \"Iİ\"\nTR_kw_var g100 = 1\nTR_fn_print(İSİM_ı, g100)\n"},
		{"kaçış '$' atılır", "değişken $eğer = 3\nyazdır($eğer)\n", "TR_kw_var eğer = 3\nTR_fn_print(eğer)\n"},
	}
	for _, d := range cases {
		t.Run(d.name, func(t *testing.T) {
			coded := encodeDecode(t, d.tr)
			if d.name == "kaçış '$' atılır" {
				// Kaçış dili sözcüğüyle çakışan ad için gereklidir: çözümde geri yazılır.
				return
			}
			if want := Header + "tr\n" + d.coded; coded != want {
				t.Errorf("kodlu biçim:\n%s\nbeklenen:\n%s", coded, want)
			}
		})
	}
}

// BOM ve CRLF satır sonları korunur; başlık BOM'dan sonra, gövdenin satır sonu biçimiyle yazılır.
func TestEncodeBOMAndCRLF(t *testing.T) {
	tr := "\xef\xbb\xbfdeğişken x = 1\r\nyazdır(\"a\r\nb\")\r\n"
	coded := encodeDecode(t, tr)
	if want := "\xef\xbb\xbf" + Header + "tr\r\nTR_kw_var x = 1\r\nTR_fn_print(\"a\r\nb\")\r\n"; coded != want {
		t.Errorf("kodlu: %q, beklenen %q", coded, want)
	}
	// Zaten kodlu dosyaya dokunulmaz.
	if again, err := Encode(coded); err != nil || again != coded {
		t.Errorf("kodlu dosya yeniden kodlandı: %q, %v", again, err)
	}
	// Türkçe dosya çözülmez.
	if same, err := Decode(tr, codes.Active()); err != nil || same != tr {
		t.Errorf("Türkçe dosya değişti: %q, %v", same, err)
	}
}

// fakeLanguage Türkçe dil dosyasındaki bazı adları değiştirerek ikinci bir yazım dili kurar:
// kaçışın (2.11) başka bir dilde nasıl çalıştığını sınamak için.
func fakeLanguage(t *testing.T) *codes.Language {
	t.Helper()
	data := string(codes.TRData())
	for old, fresh := range map[string]string{
		`"TR_kw_var": { "ad": "değişken"`: `"TR_kw_var": { "ad": "tanım"`,
		`"TR_fn_print": { "ad": "yazdır"`: `"TR_fn_print": { "ad": "sayaç"`,
		`"TR_kw_if": { "ad": "eğer"`:      `"TR_kw_if": { "ad": "kutu"`,
		`"TR_mem_append": { "ad": "ekle"`: `"TR_mem_append": { "ad": "koy"`,
		`"TR_ctx_it": { "ad": "o"`:        `"TR_ctx_it": { "ad": "bu"`,
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

// Hedef dilde bir kullanıcı adı dilin sözcüğüyle çakışırsa '$ad' yazılır (2.11).
func TestEscapeInOtherLanguage(t *testing.T) {
	d := fakeLanguage(t)
	tr := "değişken kutu = 1\ndeğişken sayaç = 2\nyazdır(kutu, sayaç)\nyapı K {\n    fonksiyon koy() { sonuç 1 }\n}\ndeğişken bu = 3\nyazdır([1].haritala { o + bu })\n"
	coded := encodeDecode(t, tr)
	got, err := Decode(coded, d)
	if err != nil {
		t.Fatal(err)
	}
	want := "tanım $kutu = 1\ntanım $sayaç = 2\nsayaç($kutu, $sayaç)\nyapı K {\n    fonksiyon $koy() { sonuç 1 }\n}\ntanım bu = 3\nsayaç([1].haritala { bu + $bu })\n"
	if got != want {
		t.Errorf("sahte dilde gösterim:\n%s\nbeklenen:\n%s", got, want)
	}
	// Türkçede çakışma yok: kaçış yazılmaz.
	if tr2, _ := Decode(coded, codes.Active()); strings.Contains(tr2, "$") {
		t.Errorf("Türkçe gösterimde gereksiz kaçış: %s", tr2)
	}
}

// Kayıpsızlık sınaması (17.4): depodaki bütün .tr dosyaları ve belgelerdeki ```tr kod
// blokları için TR → kod → TR bayt bayt aynıdır, kod → TR → kod da bayt bayt aynıdır. Kodlu
// dosyanın Türkçesiyle aynı çıktıyı verdiği (tr test ve örnekler) internal/devtools/testrunner'da
// sınanır.

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

// roundTrip bir kaynağın iki yönlü kimliğini sınar. Kaynak ayrıştırılamıyorsa (bilerek
// hatalı test dosyaları, belgelerdeki hata örnekleri) false döner.
func roundTrip(t *testing.T, name, text string) bool {
	t.Helper()
	b, err := Split(text)
	if err != nil {
		return false // tanınmayan başlık (bilerek hatalı fikstür)
	}
	if b.Coded {
		tr, err := Decode(text, codes.Active())
		var sh *SyntaxError
		if errors.As(err, &sh) {
			return false
		}
		if err != nil {
			t.Errorf("%s: kodlu dosya çözülemedi: %v", name, err)
			return false
		}
		// Türkçe gösterimden kodlanan metin dil=tr yazar; dosyanın dili korunarak (düzenleyicinin
		// kaydı gibi) özgüne dönmelidir.
		back, err := Encode(tr)
		if p, perr := Split(back); err == nil && perr == nil {
			p.Language = b.Language
			back = p.Join()
		}
		if err != nil || back != text {
			t.Errorf("%s: kod → TR → kod aynı değil (%v)\n--- özgün\n%s\n--- geri\n%s", name, err, text, back)
		}
		return true
	}
	coded, err := Encode(text)
	var sh *SyntaxError
	if errors.As(err, &sh) {
		return false
	}
	if err != nil {
		t.Errorf("%s: kodlanamadı: %v", name, err)
		return false
	}
	if !strings.Contains(coded, Header) {
		t.Errorf("%s: kodlu metinde başlık yok", name)
	}
	back, err := Decode(coded, codes.Active())
	if err != nil || back != text {
		t.Errorf("%s: TR → kod → TR aynı değil (%v)\n%s", name, err, testkit.FirstLineDiff(text, back, "özgün", "geri"))
		return false
	}
	if again, err := Encode(back); err != nil || again != coded {
		t.Errorf("%s: kod → TR → kod aynı değil (%v)\n%s", name, err, testkit.FirstLineDiff(coded, again, "özgün", "geri"))
	}
	return true
}

func TestRepoTRFilesEncodeLossless(t *testing.T) {
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
		if roundTrip(t, d, string(data)) {
			tested++
		} else {
			skipped = append(skipped, d)
		}
	}
	t.Logf("%d .tr dosyası sınandı; ayrıştırılamadığı için atlanan %d: %v", tested, len(skipped), skipped)
	for _, d := range skipped {
		// Bilerek hatalı dosyalar: dilin olumsuz testleri, araçların testdata fikstürleri ve hata örnekleri.
		if !strings.HasPrefix(d, "tests/negative/") && !strings.Contains(d, "/testdata/") &&
			!strings.HasPrefix(d, "examples/tr/hatalar/") && !strings.HasPrefix(d, "examples/en/errors/") {
			t.Errorf("%s ayrıştırılamadı; yalnızca bilerek hatalı olumsuz test dosyaları atlanabilir", d)
		}
	}
}

// trBlock belgelerdeki ```tr ... ``` kod bloklarıdır.
var trBlock = regexp.MustCompile("(?s)```tr\r?\n(.*?)```")

func TestDocCodeBlocksEncodeLossless(t *testing.T) {
	tested, skipped := 0, 0
	for _, d := range repoFiles(t, ".md") {
		data, err := os.ReadFile(reporoot.Path(d))
		if err != nil {
			t.Fatal(err)
		}
		for i, m := range trBlock.FindAllStringSubmatch(string(data), -1) {
			if roundTrip(t, d+" #"+strconv.Itoa(i+1), m[1]) {
				tested++
			} else {
				skipped++
			}
		}
	}
	t.Logf("belgelerde %d tr kod bloğu sınandı, %d blok ayrıştırılamadığı için atlandı", tested, skipped)
	if tested < 50 {
		t.Errorf("belgelerde yalnızca %d kod bloğu sınandı; tarama bozulmuş olabilir", tested)
	}
}

// A coded file may hold a code where its name cannot stand for it (a member code written as a
// global name): its view would not read back as the same program, so Decode must refuse it
// instead of showing it.
func TestDecodeRefusesLossyView(t *testing.T) {
	coded := Header + "tr\nTR_kw_var l = TR_mem_length\n"
	if out, err := Decode(coded, codes.Active()); !errors.Is(err, ErrLossy) {
		t.Fatalf("Decode gave %q, %v; ErrLossy expected", out, err)
	}
}

// Satırlar her gösterimde aynıdır (17.5): kodlu dosyanın 1. satırı başlığın altındaki satırdır,
// Türkçe gösterimin 1. satırıyla aynıdır; çeviri satır eklemez, silmez, birleştirmez.
func TestEncodeKeepsLineStructure(t *testing.T) {
	tr := "// başlık yorumu\n\nfonksiyon f(a) {\n    sonuç a + 1\n}\n\n/* iki\nsatırlık */\nyazdır(f(1))\ndeğişken s = \"çok\nsatırlı {f(2)}\"\n"
	coded := encodeDecode(t, tr)
	body := strings.TrimPrefix(coded, Header+"tr\n")
	if body == coded {
		t.Fatalf("kodlu çıktı başlıkla başlamalı: %q", coded)
	}
	trLines, codedLines := strings.Split(tr, "\n"), strings.Split(body, "\n")
	if len(trLines) != len(codedLines) {
		t.Fatalf("satır sayısı değişti: %d → %d\n%q", len(trLines), len(codedLines), body)
	}
	for i := range trLines {
		if (strings.TrimSpace(trLines[i]) == "") != (strings.TrimSpace(codedLines[i]) == "") {
			t.Errorf("%d. satırın boşluk durumu değişti: %q → %q", i+1, trLines[i], codedLines[i])
		}
	}
}

// Başka bir yazım dilinde yazılıp kodlanmış bir dosyada, etkin dilin bir yerleşiğiyle
// aynı adlı bir kullanıcı değişkeni ve o yerleşiğin çağrısı aynı kapsamda bulunduğunda
// (ör. İngilizcede 'var yazdır = 2; print(yazdır)' → 'A030 yazdır = 2; G100(yazdır)'),
// etkin dilde Decode(coded, codes.Active()) '$yazdır' kaçışını geri kodlayamadığı için
// ErrLossy ile başarısız olmamalıdır.
func TestDecodeKeepsEscapedUserNameApartFromBuiltin(t *testing.T) {
	coded := Header + "tr\nTR_kw_var yazdır = 2\nTR_fn_print(yazdır)\n"
	got, err := Decode(coded, codes.Active())
	if err != nil {
		t.Fatalf("etkin dilde çakışan kullanıcı değişkenli kodlu kaynak çözülemedi: %v", err)
	}
	want := "değişken $yazdır = 2\nyazdır($yazdır)\n"
	if got != want {
		t.Fatalf("çözülen metin:\n%s\nbeklenen:\n%s", got, want)
	}
}

// Kural: adı yazılmamış 'yakala' bloğunda kullanıcının 'değişken hata' tanımı örtük 'hata'yı
// gölgeler; okuma örtük A103 koduyla değil kullanıcı adıyla kodlanır. Neden: yoksa başka
// yazım dilinde program yanlış değişkeni okur.
func TestEncodeKeepsUserErrorVarApartFromImplicitCatchName(t *testing.T) {
	tr := "dene {\n    fırlat \"x\"\n} yakala {\n    değişken hata = 10\n    yazdır(hata)\n}\n"
	coded, err := Encode(tr)
	if err != nil {
		t.Fatalf("kodlanamadı: %v", err)
	}
	if strings.Contains(coded, "TR_ctx_error") {
		t.Fatalf("gölgelenmiş 'hata' değişkeni örtük TR_ctx_error koduyla kodlandı:\n%s", coded)
	}
}
