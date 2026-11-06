package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (2.8): tanımsız adın kendisi ve aynı satırda ondan kalan deyimler etkisiz değer hatası
// vermez; asıl sorunu tanımsız ad hatası söyler. Aynı ad sonraki satırda bildirilmese de susar.
func TestUndefinedNameLeavesNoUnusedValue(t *testing.T) {
	singleError(t, "TR_var a = TR_true\nTR_var b = a and a", messages.H1010, 2, "TR_and")
	singleError(t, "TR_var a = TR_true\nTR_var c = not a", messages.H1010, 2, "TR_not")
	singleError(t, "TR_var a = TR_true\nTR_var c = a\nc = not a", messages.H1010, 3, "TR_not")
	singleError(t, "TR_func f(a: TR_Bool) -> TR_Bool {\n    TR_return not a\n}", messages.H1010, 2, "TR_not")
	singleError(t, "TR_var a = 1\nTR_var b = a or a + 1", messages.H1010, 2, "TR_or")
	singleError(t, "TR_var a = TR_true\nTR_var b = a and a\nTR_var c = a and a", messages.H1010, 2, "TR_and")
}

// Kural (2.8): susma yalnız tanımsız adın bıraktığı deyimlerdedir; öbür etkisiz değerler hatadır.
func TestUnusedValueStaysOutsideTheUndefinedName(t *testing.T) {
	singleError(t, "TR_var a = 1\na", messages.H1023, 2)
	for _, d := range []struct {
		source string
		want   []messages.Code
	}{
		{"TR_var a = 1\ntanımsız\na", []messages.Code{messages.H1010, messages.H1023}},        // başka satır
		{"tanımsız + 1", []messages.Code{messages.H1023, messages.H1010}},                     // yalın ad değil
		{"TR_var a = 1\nTR_var b = a + c a", []messages.Code{messages.H1023, messages.H1010}}, // değer yalın ad değil
	} {
		errs, coded := checkErrs(t, d.source)
		if len(coded) != len(d.want) {
			t.Errorf("%q: %d hata bekleniyordu: %v", d.source, len(d.want), errs)
			continue
		}
		for i, c := range d.want {
			if !messages.Has(coded[i], c) {
				t.Errorf("%q: %d. hata %s olmalı: %v", d.source, i, c, errs)
			}
		}
	}
}

// Kural (kopyalanınca çalışır): başka dilin karşılaştırmasının sözcükleri tek başlarına TR_not ya da
// TR_in önermez; ikisi de karşılaştırmanın yazımını söyler. Sözcükler ikinci deyimde yalın olmayabilir.
func TestForeignComparisonSuggestsItsWholeSpelling(t *testing.T) {
	for source, want := range map[string]string{
		"TR_var l = [1]\nTR_var x = 1\nTR_var c = x not in l":        "TR_not l.TR_contains(x)",
		"TR_var x = 1\nTR_var c = x not in [1]":                      "TR_not l.TR_contains(x)",
		"TR_var x = 1\nTR_var c = x not in (1, 2)":                   "TR_not l.TR_contains(x)",
		"TR_var x = 1\nTR_var c = x is not 1":                        "x != 1",
		"TR_var x = 1\nTR_var c = x is not -1":                       "x != 1",
		"TR_var x = 1\nTR_var c = x is not (1)":                      "x != 1",
		"TR_var x = 1\nTR_var c = x in [1]":                          "l.TR_contains(x)",
		"TR_var x = 1\nTR_var y = TR_true\nTR_var c = y or x in [1]": "l.TR_contains(x)",
	} {
		errs, _ := checkErrs(t, source)
		found := false
		for _, e := range errs {
			found = found || strings.Contains(e, want)
			for _, wrong := range []string{"'TR_not'", "'TR_in'"} {
				if strings.Contains(e, wrong) {
					t.Errorf("%q: %s önerilmemeli: %s", source, wrong, e)
				}
			}
		}
		if !found {
			t.Errorf("%q: %q önerilmeli: %v", source, want, errs)
		}
	}
}

// Kural: sözcükler ayrı satırdaysa karşılaştırma değildir, ad kendi ipucunu alır.
func TestForeignComparisonIsOnOneLine(t *testing.T) {
	errs, _ := checkErrs(t, "not\nin")
	if len(errs) != 2 || !strings.Contains(errs[0], "'TR_not'") {
		t.Errorf("ayrı satırdaki 'not' kendi ipucunu almalı: %v", errs)
	}
}

// Kural (2.3a): öneri dosyanın yazımıdır; bir dilde yazılan satırda kimlikli öneri verilmez.
func TestForeignComparisonIsAFileHint(t *testing.T) {
	errs, _ := checkErrsIn(t, "tr", "TR_var x = 1\nTR_var c = x not in [1]")
	if got := strings.Join(errs, "\n"); strings.Contains(got, "TR_contains") {
		t.Errorf("yazılan satırda kimlikli öneri olmamalı: %s", got)
	}
}

// Kural: kullanıcının tanımladığı sözcük karşılaştırmada da onun adıdır; yalnız tanımsız olan bildirilir.
func TestDefinedWordOfAComparisonIsNotReported(t *testing.T) {
	errs, coded := checkErrs(t, "TR_var not = 1\nTR_var x = 1\nTR_var c = x not in [1]")
	undefined := 0
	for i, e := range errs {
		if messages.Has(coded[i], messages.H1010) {
			undefined++
			if !strings.Contains(e, "'in'") || !strings.Contains(e, "TR_not l.TR_contains(x)") {
				t.Errorf("yalnız 'in' tanımsızdır ve karşılaştırmanın yazımını alır: %s", e)
			}
		}
	}
	if undefined != 1 {
		t.Errorf("tek tanımsız ad bekleniyordu: %v", errs)
	}
}
