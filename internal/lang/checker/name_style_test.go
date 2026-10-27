package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// warningsOf kaynağı denetler ve uyarılarını döndürür (hata testi düşürür).
func warningsOf(t *testing.T, source string) []error {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
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
		{"TR_kw_var öğrenciSayısı = 1", messages.H1012, 1, `"öğrenci_sayısı"`},
		{"TR_kw_func HazırModel() { TR_kw_return 1 }", messages.H1012, 1, `"hazır_model"`},
		{"TR_kw_func f(GirdiBoyu) { TR_kw_return GirdiBoyu }", messages.H1012, 1, `"girdi_boyu"`},
		{"TR_kw_for Eleman TR_kw_in [1] { TR_fn_print(Eleman) }", messages.H1012, 1, `"eleman"`},
		{"TR_kw_struct Nokta { SolKenar = 0 }", messages.H1012, 1, `"sol_kenar"`},
		{"TR_kw_const en_büyük = 10", messages.H1013, 1, `"EN_BÜYÜK"`},
		{"TR_kw_const maxCount = 10", messages.H1013, 1, `"MAX_COUNT"`},
		{"TR_kw_struct önbellek_durumu { }", messages.H1014, 1, `"ÖnbellekDurumu"`},
		{"TR_kw_struct Önbellek_Durumu { }", messages.H1014, 1, `"ÖnbellekDurumu"`},
		{"TR_kw_var ADIM = 300", messages.H1015, 1, `"adım"`},
		{"TR_kw_var _GizliAd = 1", messages.H1012, 1, `"_gizli_ad"`},
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
	source := `TR_kw_const EN_BÜYÜK = 10
TR_kw_const D2 = 2
TR_kw_var öğrenci_sayısı2 = 1
TR_kw_var W = 1
TR_kw_var Wq, dW, W1, QKV = 1, 2, 3, 4
TR_kw_var X_veri = 1
TR_kw_var _iç = 1
TR_kw_var $eğer = 1
TR_kw_func hazır_model(B, T) { TR_kw_return B * T }
TR_kw_struct GPT { katman = 1 }
TR_kw_struct SıçramaBekçisi { pencere = 20 }
TR_kw_for _ TR_kw_in [1] { TR_fn_print(1) }`
	if w := warningsOf(t, source); len(w) > 0 {
		t.Errorf("uyarı beklenmiyordu: %v", w)
	}
}
