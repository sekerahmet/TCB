package parser

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kalıtım yazımı (7.1) tek ileti verir ve ileti ortak parçayı alanla kurmayı önerir; önerilen yazım
// ayrıştırılır (çalışması tests/yapilar_test.tr'de sınanır).
func TestStructInheritanceSuggestsAField(t *testing.T) {
	// alan tipi tek bir addır: noktalı yazımda çalışan bir öneri yoktur, ileti yazım önermez
	errs := parseErrs("TR_struct A(nn.Module) { y = 2 }")
	if len(errs) != 1 || !messages.Has(errs[0], messages.H0275) || strings.Contains(messages.ErrorIn(errs[0], "en"), "{") {
		t.Errorf("A(nn.Module): want one H0275 without a spelling, got %v", errs)
	}
	for _, c := range []struct{ code, want string }{
		{"TR_struct B { x = 1 }\nTR_struct A(B) { y = 2 }", "TR_struct A { b: B }"},
		{"TR_struct A(TR_Linear) { y = 2 }", "TR_struct A { linear: TR_Linear }"},
	} {
		errs := parseErrs(c.code)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H0275) {
			t.Fatalf("%q: want one H0275, got %v", c.code, errs)
		}
		if got := messages.ErrorIn(errs[0], "en"); !strings.Contains(got, c.want) {
			t.Errorf("%q: the message does not suggest %q: %s", c.code, c.want, got)
		}
		if errs := parseErrs(c.want + "\nTR_print(1)"); len(errs) != 0 {
			t.Errorf("the suggested %q does not parse: %v", c.want, errs)
		}
	}
}

// Gövdenin her yanlışı tek ileti verir ve yapının geri kalanı okunur: yinelenen tanımdan sonra
// kapanan '}' ikinci bir hata vermez (7.1).
func TestStructBodyErrorsAreReportedOnce(t *testing.T) {
	for _, c := range []struct {
		code string
		want messages.Code
	}{
		{"TR_struct A {\n    TR_func f() { TR_return 1 }\n    TR_func f() { TR_return 2 }\n}\nTR_print(1)", messages.H0145},
		{"TR_struct A {\n    x = 1\n    x = 2\n}\nTR_print(1)", messages.H0146},
		{"TR_struct A {\n    x = 1\n    TR_func x() { TR_return 1 }\n}\nTR_print(1)", messages.H0277},
		{"TR_struct A {\n    TR_func x() { TR_return 1 }\n    x = 1\n}\nTR_print(1)", messages.H0277},
		{"TR_struct A {\n    x = 1\n    y\n}\nTR_print(1)", messages.H0276},
	} {
		errs := parseErrs(c.code)
		if len(errs) != 1 || !messages.Has(errs[0], c.want) {
			t.Errorf("%q: want one %v, got %v", c.code, c.want, errs)
		}
	}
	if errs := parseErrs("TR_struct A {\n    x\n    y: TR_Int = 1\n    z = 2\n}"); len(errs) != 0 {
		t.Errorf("fields without a default before defaulted ones: %v", errs)
	}
}
