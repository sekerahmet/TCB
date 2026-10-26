package formatter

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

func formatT(t *testing.T, src string) string {
	t.Helper()
	output, err := Format(src)
	if err != nil {
		t.Fatalf("Format hata verdi: %v\nkaynak:\n%s", err, src)
	}
	return output
}

// Gidiş-dönüş: beklenen standart biçim üretilmeli ve ikinci kez biçimlemek bir şey değiştirmemeli.
func TestStandardFormat(t *testing.T) {
	cases := []struct{ ad, input, expected string }{
		{"operatör boşlukları", "değişken   x=1+2*3\n", "değişken x = 1 + 2 * 3\n"},
		{"virgül ve parantez", "yazdır( 1 ,2,  3 )\n", "yazdır(1, 2, 3)\n"},
		{"tekli eksi", "x = -1\ny = a-b\nz = f(-x) * -2\nsonuç -1\n", "x = -1\ny = a - b\nz = f(-x) * -2\nsonuç -1\n"},
		{"değil ve ünlem", "a = !b\nc = değil (d)\n", "a = !b\nc = değil (d)\n"},
		{"girinti 4 boşluk", "eğer x {\n  yazdır(1)\n\t\teğer y {\nyazdır(2)\n}\n}\n",
			"eğer x {\n    yazdır(1)\n    eğer y {\n        yazdır(2)\n    }\n}\n"},
		{"süslü aynı satırda", "fonksiyon f(a)\n{\n    sonuç a\n}\n", "fonksiyon f(a) {\n    sonuç a\n}\n"},
		{"değilse önceki satıra", "eğer x {\n    a()\n}\ndeğilse\n{\n    b()\n}\n", "eğer x {\n    a()\n} değilse {\n    b()\n}\n"},
		{"yakala önceki satıra", "dene {\n    a()\n}\nyakala h {\n    b(h)\n}\n", "dene {\n    a()\n} yakala h {\n    b(h)\n}\n"},
		{"boş satırlar", "\n\n\na = 1\n\n\n\nb = 2\n\n\n", "a = 1\n\nb = 2\n"},
		{"blok başı/sonu boş satır", "eğer x {\n\n    a()\n\n}\n", "eğer x {\n    a()\n}\n"},
		{"satır sonu boşlukları", "a = 1   \nb = 2\t\n", "a = 1\nb = 2\n"},
		{"son satır sonu eklenir", "a = 1", "a = 1\n"},
		{"ileti kodu", "fırlat ileti( H6903 ,x )\n", "fırlat ileti(H6903, x)\n"},
		{"sözlük", "s = {\"a\" :1,b: {\"c\":2} }\n", "s = {\"a\": 1, b: {\"c\": 2}}\n"},
		{"sondaki blok", "y = l.filtrele{o>1}\n", "y = l.filtrele { o > 1 }\n"},
		{"aralık", "döngü i içinde 1 ..= 10 { yazdır(i) }\n", "döngü i içinde 1..=10 { yazdır(i) }\n"},
		{"dönüş tipi oku", "fonksiyon f(a:Tam)->Tam { sonuç a }\n", "fonksiyon f(a: Tam) -> Tam { sonuç a }\n"},
		{"lambda", "f = (a,b)=>a+b\ng = x=>x*2\n", "f = (a, b) => a + b\ng = x => x * 2\n"},
		{"devam satırı", "t = 1 +\n2\nl\n|> yazdır\n", "t = 1 +\n    2\nl\n    |> yazdır\n"},
		{"çok satırlı liste", "l = [\n1,\n  2\n    ]\n", "l = [\n    1,\n    2\n]\n"},
		{"kapanışla başlayan satır", "f(fonksiyon() {\nyazdır(1)\n}, 2)\n", "f(fonksiyon() {\n    yazdır(1)\n}, 2)\n"},
		{"CRLF", "a = 1\r\neğer a {\r\n  b()\r\n}\r\n", "a = 1\neğer a {\n    b()\n}\n"},
		// Çok satırlı metnin içindeki satır sonları da LF yazılır (karışık satır sonu kalmaz)
		{"CRLF çok satırlı metin", "m = \"a\r\nb\"\r\n", "m = \"a\nb\"\n"},
		{"test bloğu", "test \"ad\"{doğrula(1==1)}\n", "test \"ad\" { doğrula(1 == 1) }\n"},
		{"boş kaynak", "", ""},
	}
	for _, d := range cases {
		t.Run(d.ad, func(t *testing.T) {
			output := formatT(t, d.input)
			if output != d.expected {
				t.Fatalf("beklenen:\n%q\nalınan:\n%q", d.expected, output)
			}
			if second := formatT(t, output); second != output {
				t.Fatalf("idempotent değil:\n%q\n->\n%q", output, second)
			}
		})
	}
}

func TestCommentsKept(t *testing.T) {
	input := `// dosya başı yorumu
// ikinci yorum
değişken x = 1   // sondaki yorum
/* çok
   satırlı yorum */
eğer x {
        // blok içi yorum
    yazdır(x) /* satır içi */ ; yazdır(2)
    // kapanıştan önce
}
// dosya sonu
`
	output := formatT(t, input)
	for _, y := range []string{"// dosya başı yorumu", "// ikinci yorum", "// sondaki yorum",
		"/* çok\n   satırlı yorum */", "// blok içi yorum", "/* satır içi */", "// kapanıştan önce", "// dosya sonu"} {
		if !strings.Contains(output, y) {
			t.Errorf("yorum kayboldu: %q\nçıktı:\n%s", y, output)
		}
	}
	if !strings.Contains(output, "\n    // blok içi yorum\n") || !strings.Contains(output, "\n    // kapanıştan önce\n}") {
		t.Errorf("blok içi yorumlar girintilenmedi:\n%s", output)
	}
	if !strings.Contains(output, "değişken x = 1 // sondaki yorum\n") {
		t.Errorf("sondaki yorum satırında kalmadı:\n%s", output)
	}
	if second := formatT(t, output); second != output {
		t.Errorf("yorumlu kodda idempotent değil:\n%s\n->\n%s", output, second)
	}
}

// Yorum, '{' ile önceki satır arasındaysa '{' taşınmaz (yorum koddan sonra kalmasın diye).
func TestCommentedBraceNotMoved(t *testing.T) {
	input := "eğer x // neden\n{\n    a()\n}\n"
	if output := formatT(t, input); output != input {
		t.Fatalf("beklenmeyen çıktı:\n%q", output)
	}
}

func TestStringsKeptByteForByte(t *testing.T) {
	texts := []string{
		`"  iki  boşluk  "`, `'tek tırnak'`, `"kaçış \" \n \t \\ \{ \}"`, `"ara {x+1} değer"`,
		`"{p['ad']}"`, "\"çok\nsatırlı   \n  metin\"",
		// Ham metin (2.6a): ters bölü, süslü parantez ve boşluklar olduğu gibi
		"`C:\\veri\\  {x+1}  \\d{3}`", "`çok\n    satırlı  \n`", "``",
	}
	for _, m := range texts {
		input := "değişken x = 1\ndeğişken p = {}\ndeğişken s = " + m + "\n"
		output := formatT(t, input)
		if !strings.Contains(output, m) {
			t.Errorf("metin değişti: %s\nçıktı:\n%s", m, output)
		}
	}
}

// Satıra duyarlı yapılar: yeni satırdaki '(', '[' ve sözlük '{' önceki ifadeye bağlanmamalı.
func TestLineStructureKept(t *testing.T) {
	inputs := []string{
		"değişken f = 5\n(1 + 2) |> yazdır\n",
		"değişken l = [1]\n[2, 3] |> yazdır\n",
		"değişken s = l.filtrele\n{\"a\": 1} |> yazdır\n",
	}
	for _, g := range inputs {
		if output := formatT(t, g); output != g {
			t.Errorf("satır yapısı değişti:\n%q\n->\n%q", g, output)
		}
	}
}

// Anlamı değiştiren bir dönüşüm reddedilmeli.
func TestMeaningChangeRejected(t *testing.T) {
	old := transform
	defer func() { transform = old }()
	var meaning *MeaningChangedError

	// Bozuk dönüşüm: satırları birleştirir; yeni satırdaki '(' önceki ifadeye bağlanır (5(1 + 2)).
	transform = func(src string, _ bool, braces map[pos]int) string {
		return strings.ReplaceAll(src, "\n", " ") + "\n"
	}
	if _, err := Format("değişken f = 5\n(1 + 2) |> yazdır\n"); !errors.As(err, &meaning) {
		t.Fatalf("anlam değişikliği reddedilmedi, hata: %v", err)
	}

	// Sözcük değiştiren dönüşüm de reddedilmeli
	transform = func(src string, _ bool, braces map[pos]int) string {
		return strings.ReplaceAll(src, "1", "2")
	}
	if _, err := Format("a = 1\n"); !errors.As(err, &meaning) {
		t.Fatalf("sözcük değişikliği reddedilmedi, hata: %v", err)
	}
}

func TestSameMeaning(t *testing.T) {
	if err := SameMeaning("a=1+2\n", "a = 1 + 2\n"); err != nil {
		t.Fatalf("aynı anlamlı kaynaklar farklı sayıldı: %v", err)
	}
	if err := SameMeaning("değişken f = 5\n(1)\n", "değişken f = 5 (1)\n"); err == nil {
		t.Fatal("farklı anlamlı kaynaklar aynı sayıldı")
	}
}

func TestSyntaxErrorNotFormatted(t *testing.T) {
	_, err := Format("değişken = \n")
	var sh *SyntaxError
	if !errors.As(err, &sh) {
		t.Fatalf("sözdizimi hatası bildirilmedi: %v", err)
	}
}

// Depodaki tüm .tr dosyaları biçimlenebilmeli ve sonuç idempotent olmalı.
func TestRepoFiles(t *testing.T) {
	var files []string
	// scratch/ kullanıcının kişisel deneme klasörüdür (depoda değil); denetlenmez.
	for _, pattern := range []string{"examples/tr/*.tr", "examples/tr/helpers/*.tr", "tests/*.tr"} {
		d, _ := reporoot.Glob(reporoot.Path(pattern))
		files = append(files, d...)
	}
	if len(files) == 0 {
		t.Fatal("örnek dosya bulunamadı")
	}
	for _, fpath := range files {
		data, err := os.ReadFile(fpath)
		if err != nil {
			t.Fatal(err)
		}
		output, err := Format(string(data))
		if err != nil {
			t.Errorf("%s: %v", fpath, err)
			continue
		}
		if second, err := Format(output); err != nil || second != output {
			t.Errorf("%s: idempotent değil (hata: %v)", fpath, err)
		}
	}
}

func TestBOMKept(t *testing.T) {
	output := formatT(t, "\xef\xbb\xbfa=1\n")
	if output != "\xef\xbb\xbfa = 1\n" {
		t.Fatalf("BOM korunmadı: %q", output)
	}
}

// Kodlu dosya kodlu biçimde biçimlenir (17): başlık ve BOM korunur, kodlar olduğu gibi kalır,
// satır sonları (başlığınki de) LF olur.
func TestCodedFileFormats(t *testing.T) {
	src := "\xef\xbb\xbf//! tr kodlu 3 dil=en\r\nTR_kw_func   f(a){TR_kw_return a+1}\r\nTR_fn_print( f(1) )\r\n"
	got, err := Format(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := "\xef\xbb\xbf//! tr kodlu 3 dil=en\nTR_kw_func f(a) { TR_kw_return a + 1 }\nTR_fn_print(f(1))\n"; got != want {
		t.Errorf("kodlu biçimleme:\n%q\nbeklenen:\n%q", got, want)
	}
	// Biçimin değişmesi (kodlu ↔ Türkçe) anlam denetiminde reddedilir.
	var meaning *MeaningChangedError
	if err := SameMeaning(got, "fonksiyon f(a) { sonuç a + 1 }\nyazdır(f(1))\n"); !errors.As(err, &meaning) {
		t.Errorf("kodlu ve Türkçe biçim aynı sayıldı: %v", err)
	}
	// Kodlu sözcük kodla, kullanıcı adı metinle karşılaştırılır: TR_fn_print ile adı 'yazdır' olan
	// kullanıcı değişkeni aynı değildir.
	if err := SameMeaning("//! tr kodlu 3 dil=tr\nTR_fn_print\n", "//! tr kodlu 3 dil=tr\nyazdır\n"); !errors.As(err, &meaning) {
		t.Errorf("kod ile aynı adlı kullanıcı adı aynı sayıldı: %v", err)
	}
	// Dosyanın dili değerlerin metnini belirler (17.1): dili değişen dosya aynı anlamda değildir.
	if err := SameMeaning("//! tr kodlu 3 dil=tr\nTR_fn_print(TR_kw_true)\n", "//! tr kodlu 3 dil=en\nTR_fn_print(TR_kw_true)\n"); !errors.As(err, &meaning) {
		t.Errorf("dili değişen kodlu dosya aynı sayıldı: %v", err)
	}
}

// Devam satırında açılan gövde deyimin girintisine göredir.
func TestContinuationBody(t *testing.T) {
	cases := []struct{ ad, input, expected string }{
		{"devam satırında gövde", "eğer a ve\nb {\nyazdır(1)\n}\nyazdır(2)\n",
			"eğer a ve\n    b {\n    yazdır(1)\n}\nyazdır(2)\n"},
		{"parantezli koşul devamı", "eğer (a ve\nb) {\nyazdır(1)\n}\n",
			"eğer (a ve\n        b) {\n    yazdır(1)\n}\n"},
		{"iç blokta devam", "döngü i içinde 0..3 {\neğer i > 0 veya\ni < 2 {\nyazdır(i)\n}\n}\n",
			"döngü i içinde 0..3 {\n    eğer i > 0 veya\n        i < 2 {\n        yazdır(i)\n    }\n}\n"},
		// Bir argümanın içindeki gövde (parantez açıkken) eskisi gibi satırın girintisine göredir.
		{"devam satırında fonksiyon argümanı", "x = 1 +\nf(fonksiyon() {\nyazdır(1)\n})\n",
			"x = 1 +\n    f(fonksiyon() {\n        yazdır(1)\n    })\n"},
		// Ayrıştırıcı yeni satırdaki '-x'i önceki ifadeyle (eğer de bir ifadedir) ikili eksi okur;
		// biçim bu okumayı gösterir.
		{"bloktan sonra eksi", "eğer a {\nb()\n}\n-x\n", "eğer a {\n    b()\n}\n- x\n"},
	}
	for _, d := range cases {
		t.Run(d.ad, func(t *testing.T) {
			output := formatT(t, d.input)
			if output != d.expected {
				t.Fatalf("beklenen:\n%s\nalınan:\n%s", d.expected, output)
			}
			if second := formatT(t, output); second != output {
				t.Fatalf("idempotent değil:\n%q\n->\n%q", output, second)
			}
		})
	}
}
