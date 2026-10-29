package process

import (
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Yardımın gösterdiği her varsayılan, okuyucuya geri verilince aynı değeri verir (DENETİM 3. adım,
// bulgu 3): yardım kişinin dilinde yazar, okuyucu her gömülü yazım dilinin değer adını kabul eder
// (17.8). Önceden sonsuz varsayılan hiçbir dilde, TR_LANG=en'in "true" yazımı da okunmuyordu.
func TestHelpDefaultsReadBack(t *testing.T) {
	f := func(x float64) interp.Value { return &interp.FloatVal{Val: x} }
	list := func(v ...interp.Value) interp.Value { return &interp.ListVal{Elements: v} }
	for _, c := range []struct {
		kind, item optKind
		value      interp.Value
	}{
		{kindInt, 0, interp.NewInt(3)},
		{kindFloat, 0, f(0.5)}, {kindFloat, 0, f(math.Inf(1))}, {kindFloat, 0, f(math.Inf(-1))}, {kindFloat, 0, f(math.NaN())},
		{kindText, 0, &interp.StringVal{Val: "x"}},
		{kindList, kindBool, list(interp.True, interp.False)},
		{kindList, kindFloat, list(f(math.Inf(1)), f(1.5))},
		{kindList, kindInt, list(interp.NewInt(1), interp.NewInt(2))},
	} {
		s := &option{name: "x", optKind: c.kind, itemKind: c.item, defaultVal: c.value}
		text := defaultText(c.value)
		got, err := parseValue(s, text, true)
		if err != nil || got.Inspect() != c.value.Inspect() {
			t.Errorf("%s varsayılanı %q geri okunamadı: %v, %v", c.value.Type(), text, got, err)
		}
	}
	// Mantık değeri her dilin adıyla (kişinin dili: yardım) okunur; sonsuz ve tanımsızın adı her dilde
	// "inf" ve "nan"dır (3.2). Okunan değerin metni dilden bağımsızdır (3.14).
	for text, want := range map[string]string{"true": "true", "doğru": "true", "false": "false", "inf": "inf", "-inf": "-inf", "nan": "nan"} {
		kind := kindFloat
		if want == "true" || want == "false" {
			kind = kindBool
		}
		if got, err := parsePlainValue(kind, text); err != nil || got.Inspect() != want {
			t.Errorf("%q: %v, %v; beklenen %s", text, got, err, want)
		}
	}
}
