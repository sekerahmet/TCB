package source

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
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
		{"anahtar kelime ve yerleşik", "değişken x = 1\nyazdır(x)\n", "A030 x = 1\nG100(x)\n"},
		{"gölgelenen yerleşik kullanıcının adıdır",
			"fonksiyon f() {\n    değişken toplam = 1\n    sonuç toplam\n}\nyazdır(toplam([1, 2]))\n",
			"A020 f() {\n    A030 toplam = 1\n    A021 toplam\n}\nG100(G400([1, 2]))\n"},
		{"yorum ve ham metin çevrilmez",
			"// eğer yazdır\n/* değişken */ değişken y = `yazdır {eğer}`\n",
			"// eğer yazdır\n/* değişken */ A030 y = `yazdır {eğer}`\n"},
		{"metin içi yerleştirme ve iç içe yerleştirme",
			"yazdır(\"a\\n{toplam([1])} \\{x\\} {\"iç {uzunluk_x(1)} {yazdır}\"}\")\nfonksiyon uzunluk_x(a) { sonuç a }\n",
			"G100(\"a\\n{G400([1])} \\{x\\} {\"iç {uzunluk_x(1)} {G100}\"}\")\nA020 uzunluk_x(a) { A021 a }\n"},
		{"üyeler tanımda ve kullanımda aynı kodla (Y5)",
			"yapı Sepet {\n    uzunluk = 0\n    fonksiyon ekle(x) { kendi.uzunluk += 1 }\n}\ndeğişken s = Sepet(uzunluk: 2)\ns.ekle(1)\n[1].ekle(2)\n",
			"A050 Sepet {\n    Y001 = 0\n    A020 Y004(x) { A051.Y001 += 1 }\n}\nA030 s = Sepet(Y001: 2)\ns.Y004(1)\n[1].Y004(2)\n"},
		{"kurucu bağlamsal sözcüktür",
			"yapı N {\n    a\n    fonksiyon kurucu(a) { kendi.a = a }\n}\n",
			"A050 N {\n    a\n    A020 A102(a) { A051.a = a }\n}\n"},
		{"adlı argüman yalnızca dilin öğesinde",
			"fonksiyon f(eksen = 0) { sonuç eksen }\nyazdır(toplam([1], eksen: 0), f(eksen: 1))\n",
			"A020 f(eksen = 0) { A021 eksen }\nG100(G400([1], T902: 0), f(eksen: 1))\n"},
		{"örtük adlar yalnızca kendi bloklarında",
			"değişken o = 1\nyazdır([1].haritala { o * 2 }, [1].indirge(0) { birikim + o }, (o) => o)\ndene { fırlat \"x\" } yakala { yazdır(hata) }\ndene { fırlat \"y\" } yakala hata { yazdır(hata) }\n",
			"A030 o = 1\nG100([1].Y201 { A104 * 2 }, [1].Y207(0) { A105 + A104 }, (o) => o)\nA060 { A062 \"x\" } A061 { G100(A103) }\nA060 { A062 \"y\" } A061 hata { G100(hata) }\n"},
		{"test ve olarak",
			"aktar \"m\" olarak m\ntest \"t\" { doğrula(doğru) }\ndeğişken test = 1\n",
			"A070 \"m\" A100 m\nA101 \"t\" { G700(A040) }\nA030 test = 1\n"},
		{"tip açıklaması; kabul edilmeyen harf yazımı (3.14) olduğu gibi kalır",
			"değişken a: Tam = 1\ndeğişken b: tam = 2\nyazdır(tip(a) == Tam)\n",
			"A030 a: A200 = 1\nA030 b: tam = 2\nG100(G201(a) == A200)\n"},
		{"Türkçe harfler ve kod biçimine benzeyen adlar",
			"değişken İSİM_ı = \"Iİ\"\ndeğişken g100 = 1\nyazdır(İSİM_ı, g100)\n",
			"A030 İSİM_ı = \"Iİ\"\nA030 g100 = 1\nG100(İSİM_ı, g100)\n"},
		{"kaçış '$' atılır", "değişken $eğer = 3\nyazdır($eğer)\n", "A030 eğer = 3\nG100(eğer)\n"},
	}
	for _, d := range cases {
		t.Run(d.name, func(t *testing.T) {
			coded := encodeDecode(t, d.tr)
			if d.name == "kaçış '$' atılır" {
				// Kaçış dili sözcüğüyle çakışan ad için gereklidir: çözümde geri yazılır.
				return
			}
			if want := Header + "\n" + d.coded; coded != want {
				t.Errorf("kodlu biçim:\n%s\nbeklenen:\n%s", coded, want)
			}
		})
	}
}

// BOM ve CRLF satır sonları korunur; başlık BOM'dan sonra, gövdenin satır sonu biçimiyle yazılır.
func TestEncodeBOMAndCRLF(t *testing.T) {
	tr := "\xef\xbb\xbfdeğişken x = 1\r\nyazdır(\"a\r\nb\")\r\n"
	coded := encodeDecode(t, tr)
	if want := "\xef\xbb\xbf" + Header + "\r\nA030 x = 1\r\nG100(\"a\r\nb\")\r\n"; coded != want {
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
		`"A030": { "ad": "değişken"`: `"A030": { "ad": "tanım"`,
		`"G100": { "ad": "yazdır"`:   `"G100": { "ad": "sayaç"`,
		`"A001": { "ad": "eğer"`:     `"A001": { "ad": "kutu"`,
		`"Y004": { "ad": "ekle"`:     `"Y004": { "ad": "koy"`,
		`"A104": { "ad": "o"`:        `"A104": { "ad": "bu"`,
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
	err := filepath.WalkDir(root, func(path string, g fs.DirEntry, err error) error {
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
	if b.Coded && hasDeadCode(b) {
		// Ölü kodlar okunur ama çözülüp yeniden kodlanınca adla kalır (17.6).
		return false
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
		if back, err := Encode(tr); err != nil || back != text {
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
		t.Errorf("%s: TR → kod → TR aynı değil (%v)\n%s", name, err, firstDiff(text, back))
		return false
	}
	if again, err := Encode(back); err != nil || again != coded {
		t.Errorf("%s: kod → TR → kod aynı değil (%v)\n%s", name, err, firstDiff(coded, again))
	}
	return true
}

// hasDeadCode kodlu gövdede ölü (taşınmış ya da kaldırılmış) bir kod var mı?
func hasDeadCode(b Parts) bool {
	l := b.Lexer()
	for tok := l.NextToken(); tok.Type != lexer.TOKEN_EOF; tok = l.NextToken() {
		if g, ok := codes.Active().Entry(tok.Code); ok && g.Dead() {
			return true
		}
	}
	return false
}

// firstDiff iki metnin ayrıldığı ilk satırı gösterir.
func firstDiff(a, b string) string {
	as, bs := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y string
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		if x != y {
			return "satır " + itoa(i+1) + ":\n  özgün: " + x + "\n  geri:  " + y
		}
	}
	return "(fark yok)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
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
		if !strings.HasPrefix(d, "tests/negative/") {
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
			if roundTrip(t, d+" #"+itoa(i+1), m[1]) {
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

// A coded file may hold a code that is dead in the view language (moved or removed): its view
// would not read back as the same program, so Decode must refuse it instead of showing it.
func TestDecodeRefusesLossyView(t *testing.T) {
	coded := Header + "\nA030 l = [1]\nG200(l)\n"
	if out, err := Decode(coded, codes.Active()); !errors.Is(err, ErrLossy) {
		t.Fatalf("Decode gave %q, %v; ErrLossy expected", out, err)
	}
}
