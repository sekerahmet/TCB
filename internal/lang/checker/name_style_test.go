package checker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// warningsOf kaynağı denetler ve uyarılarını döndürür (hata testi düşürür).
func warningsOf(t *testing.T, source string) []error {
	t.Helper()
	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	if errs := c.CheckProgram(prog); len(errs) > 0 {
		t.Fatalf("yazılış hata değildir: %v", errs)
	}
	return c.Warnings()
}

// Kural (dil tanımı 2.3b, PEP 8 düzeni; Rust'ın non_snake_case, non_camel_case_types ve
// non_upper_case_globals uyarıları gibi hata değil uyarı): sabitler BÜYÜK_YILAN, yapılar
// BüyükDeve, değişken, fonksiyon, parametre, alan ve metot adları küçük_yılan yazılır. Uyarı
// önerilen yazımı verir; büyük harfli değişkene "sabit" önerilir.
func TestNameStyleWarnings(t *testing.T) {
	for _, c := range []struct {
		source     string
		code       messages.Code
		line       int
		suggestion string
	}{
		{"değişken öğrenciSayısı = 1", messages.H1012, 1, `"öğrenci_sayısı"`},
		{"fonksiyon HazırModel() { sonuç 1 }", messages.H1012, 1, `"hazır_model"`},
		{"fonksiyon f(GirdiBoyu) { sonuç GirdiBoyu }", messages.H1012, 1, `"girdi_boyu"`},
		{"döngü Eleman içinde [1] { yazdır(Eleman) }", messages.H1012, 1, `"eleman"`},
		{"yapı Nokta { SolKenar = 0 }", messages.H1012, 1, `"sol_kenar"`},
		{"sabit en_büyük = 10", messages.H1013, 1, `"EN_BÜYÜK"`},
		{"sabit maxCount = 10", messages.H1013, 1, `"MAX_COUNT"`},
		{"yapı önbellek_durumu { }", messages.H1014, 1, `"ÖnbellekDurumu"`},
		{"yapı Önbellek_Durumu { }", messages.H1014, 1, `"ÖnbellekDurumu"`},
		{"değişken ADIM = 300", messages.H1015, 1, `"adım"`},
		{"değişken _GizliAd = 1", messages.H1012, 1, `"_gizli_ad"`},
	} {
		w := warningsOf(t, c.source)
		if len(w) != 1 {
			t.Errorf("%s: tek uyarı bekleniyordu, gelen %v", c.source, w)
			continue
		}
		expectCode(t, w[0], c.code, c.line)
		if !strings.Contains(w[0].Error(), c.suggestion) {
			t.Errorf("%s: uyarı %s önermeli: %v", c.source, c.suggestion, w[0])
		}
	}
}

// Kurala uyan adlar ve matematik adları (en çok üç harf ve rakam: W, X, Wq, dW, W1, QKV;
// X_veri'nin X'i) uyarı vermez; baştaki _ (modüle özel) ve $ (kaçış) yazılışa sayılmaz.
func TestNameStyleAcceptsConventionalAndMathNames(t *testing.T) {
	source := `sabit EN_BÜYÜK = 10
sabit D2 = 2
değişken öğrenci_sayısı2 = 1
değişken W = 1
değişken Wq, dW, W1, QKV = 1, 2, 3, 4
değişken X_veri = 1
değişken _iç = 1
değişken $eğer = 1
fonksiyon hazır_model(B, T) { sonuç B * T }
yapı GPT { katman = 1 }
yapı SıçramaBekçisi { pencere = 20 }
döngü _ içinde [1] { yazdır(1) }`
	if w := warningsOf(t, source); len(w) > 0 {
		t.Errorf("uyarı beklenmiyordu: %v", w)
	}
}

// Kitaplık ve GPU araçları da yazılış kuralına uyar (örnekler ve testler TestExamplesCheckWithoutErrors
// ile bütünüyle uyarısızdır; kitaplıkta gölgeleme bilinçli olabilir: adlı parametreler girdi, taban
// gibi yerleşik adları taşır).
func TestLibraryFollowsNameStyle(t *testing.T) {
	libs, _ := filepath.Glob(reporoot.Path("lib/*.tr"))
	tools, _ := filepath.Glob(reporoot.Path("tools/gpu/*.tr"))
	for _, file := range append(libs, tools...) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(strings.TrimPrefix(string(data), "\ufeff")))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Errorf("%s: %v", file, p.Errors())
			continue
		}
		c := New()
		c.CheckProgram(prog)
		for _, w := range c.Warnings() {
			if messages.Has(w, messages.H1012) || messages.Has(w, messages.H1013) || messages.Has(w, messages.H1014) || messages.Has(w, messages.H1015) {
				t.Errorf("%s yazılış kuralına (2.3b) uymalı: %v", file, w)
			}
		}
	}
}
