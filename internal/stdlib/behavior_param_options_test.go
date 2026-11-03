package stdlib

import (
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// optionSamples her örneklenen tipten bir değerin yazımıdır (checker.ProbedTypes).
var optionSamples = map[codes.Code]string{
	codes.TypeInt:      "1",
	codes.TypeFloat:    "1.5",
	codes.TypeString:   `"a"`,
	codes.TypeBool:     "TR_true",
	codes.TypeList:     "[1]",
	codes.TypeDict:     `{"a": 1}`,
	codes.TypeTensor:   "TR_Tensor([1.0])",
	codes.TypeNoneType: "TR_none",
}

// Kural (13.9, 13.11; 4a4): denetleyici checker.FunctionArgCodes'taki yerleşik fonksiyonların konumlu
// argümanını tanımın seçeneklerine göre denetler (TR_Int|TR_Float|TR_Tensor üç imzadır) ve seçenek
// dışı tipi çalışma anının koduyla yakalar. Bu yüzden her denetlenen fonksiyonda, her denetlenen
// parametrede ve her örnek tipte motor seçeneği kabul eder (hatasız), seçenek dışı tipe denetimin
// verdiği kodu verir; denetimin karar vermediği tipte (liste, Tam: önerili kod) yine reddeder. Öbür
// argümanlar ilk seçeneğin örneğidir. Program denetleyicisiz koşulur.
func TestFunctionParamOptionsMatchEngine(t *testing.T) {
	chdirTemp(t)
	if len(checker.FunctionArgCodes) < 39 {
		t.Errorf("denetlenen fonksiyon %d (39 bekleniyordu)", len(checker.FunctionArgCodes))
	}
	for k := range checker.FunctionArgCodes {
		def, ok := codes.DefinitionOf(k)
		if !ok || def.Kind != codes.KindFunction {
			t.Errorf("%s bir yerleşik fonksiyon değil", k)
			continue
		}
		if bad := probeOptions(t, k, def.Signature); bad != "" {
			t.Errorf("%s: %s (tip@argüman:motorun kodu)", k, bad)
		}
	}
}

// probeOptions k'nın denetlenen parametrelerini örneklerle koşar; boş sonuç uyuştuğudur, değilse
// uyuşmayan (tip, argüman, motorun kodu) üçlüleri.
func probeOptions(t *testing.T, k codes.Code, sig codes.Signature) string {
	base := make([]string, len(sig.Params))
	for i, p := range sig.Params {
		if opts, ok := checker.ParamOptions(p); ok {
			base[i] = optionSamples[opts[0]]
		}
	}
	if code := probeCall(t, k, base, -1, ""); code != "none" {
		return "örnek argümanlarla " + code
	}
	var bad []string
	for i, p := range sig.Params {
		opts, ok := checker.ParamOptions(p)
		if !ok || base[i] == "" {
			continue
		}
		for _, typ := range checker.ProbedTypes {
			code := probeCall(t, k, base, i, optionSamples[typ])
			want, decided := checker.ArgTypeCode(k, typ)
			var good bool
			switch {
			case checker.OptionAccepts(opts, typ):
				good = code == "none"
			case decided:
				good = code == want.String()
			default:
				good = code != "none" // çalışma anının önerili kodu
			}
			if !good {
				bad = append(bad, strings.TrimPrefix(codes.Spelling(typ), "TR_")+"@"+string(rune('1'+i))+":"+code)
			}
		}
	}
	sort.Strings(bad)
	return strings.Join(bad, " ")
}

// probeCall k'yı base argümanlarıyla (i. argüman sample) çağırır ve çağrının hata kodunu verir
// (hatasızsa "none").
func probeCall(t *testing.T, k codes.Code, base []string, i int, sample string) string {
	var args []string
	for j, a := range base {
		if j == i {
			a = sample
		}
		if a == "" {
			break
		}
		args = append(args, a)
	}
	src := `TR_print("PROBE", TR_error_code { ` + codes.Spelling(k) + "(" + strings.Join(args, ", ") + `) })`
	out := runExampleOutput(t, parseCoded(t, src))
	at := strings.LastIndex(out, "PROBE ")
	if at < 0 {
		return "?"
	}
	return strings.TrimSpace(strings.SplitN(out[at+len("PROBE "):], "\n", 2)[0])
}
