package interp

import (
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.12): tip çağrısı metni 2.4'ün ondalık tam sayı yazımıyla ve 2.5'in ondalık yazımıyla
// okur; önde isteğe bağlı işaret, '_' yalnız rakamlar arasında, boşluk ve yerel ayar yok.
func TestTypeCallReadsProgramNumberSpelling(t *testing.T) {
	ints := map[string]int64{"42": 42, "-1_000": -1000, "+7": 7, "007": 7, "-9223372036854775808": math.MinInt64}
	for s, want := range ints {
		got, err := parseIntText(s)
		if err != nil || got.(*IntVal).Val != want {
			t.Errorf("TR_Int(%q) = %v, %v; want %d", s, got, err, want)
		}
	}
	for _, s := range []string{"", "+", " 42", "42 ", "3.5", "0x1F", "1__0", "1_", "_1", "1e3", "٤٢"} {
		if _, err := parseIntText(s); !messages.Has(err, messages.H2640) {
			t.Errorf("TR_Int(%q): %v; want H2640", s, err)
		}
	}
	if _, err := parseIntText("9223372036854775808"); !messages.Has(err, messages.H2641) {
		t.Errorf("TR_Int overflow: %v; want H2641", err)
	}
	floats := map[string]float64{"3": 3, "-1.5": -1.5, "2e-3": 0.002, "3.2E+4": 32000, "1_000.5": 1000.5, "inf": math.Inf(1), "-inf": math.Inf(-1)}
	for s, want := range floats {
		if got, ok := parseFloatText(s); !ok || got != want {
			t.Errorf("TR_Float(%q) = %v, %v; want %v", s, got, ok, want)
		}
	}
	if got, ok := parseFloatText("nan"); !ok || !math.IsNaN(got) {
		t.Errorf("TR_Float(\"nan\") = %v, %v", got, ok)
	}
	for _, s := range []string{"1,5", ".5", "1.", "1e", "1e999", "Inf", "+inf", "NaN", "infinity", "0x1p3", " 1.0", "1.0 "} {
		if _, ok := parseFloatText(s); ok {
			t.Errorf("TR_Float(%q) was read; want an error", s)
		}
	}
}
