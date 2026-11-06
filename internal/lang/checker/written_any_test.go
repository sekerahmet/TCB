package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.7a): her değeri tutan genel bir tip yoktur; TR_Any tip yazımının hiçbir yerinde, değer olarak
// da yazılmaz. Her satır bir kez bildirilir. Satırlarda ANY yazılır: TestTRCodeIsTyped depodaki kodda genel
// tip aramasın, bu satırlar bilerek hatalı.
func TestWrittenAnyIsAnError(t *testing.T) {
	for _, code := range []string{
		`TR_var x: ANY = 5`,
		`TR_const x: ANY = 5`,
		`TR_var x: ANY? = TR_none`,
		`TR_var l: TR_List[ANY] = [1]`,
		`TR_var d: TR_Dict[TR_String, ANY] = {"a": 1}`,
		`TR_var d: TR_Dict[ANY, TR_Int] = {"a": 1}`,
		`TR_var t: (TR_Int, ANY) = (1, 2)`,
		`TR_var f: (ANY) -> TR_Int = (x) => 1`,
		`TR_var f: (TR_Int) -> ANY = (x) => x`,
		`TR_var l: TR_List[TR_List[(TR_Int) -> ANY?]] = []`,
		`TR_func f(a: ANY) { TR_print(a) }`,
		`TR_func f() -> ANY { TR_return 1 }`,
		`TR_func f() -> (TR_Int, ANY) { TR_return (1, 2) }`,
		`TR_var g = (a: ANY) => 1`,
		`TR_struct N { x: ANY }`,
		`TR_struct N { x: TR_Int
TR_func m(a: ANY) { TR_print(a) } }`,
		`TR_enum D { A(x: ANY), B }`,
		`TR_interface I { TR_func m(a: ANY) }`,
		`TR_print(TR_Type(5) == ANY)`,
		`TR_var t = ANY`,
		`TR_print("\(ANY)")`,
	} {
		errs := checkCoded(t, strings.ReplaceAll(code, "ANY", "TR_"+"Any"))
		found := 0
		for _, e := range errs {
			if messages.Has(e, messages.H1147) {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%s: bir H1147 bekleniyordu: %v", code, errs)
		}
	}
}

// H1147'nin iki önerisi kopyalanınca denetimden geçer; benzer adlar etkilenmez.
func TestWrittenAnySuggestionsRun(t *testing.T) {
	for _, code := range []string{
		`TR_var x: TR_Int = 5`,
		`TR_var g: TR_JSON = {"k": 1}`,
		`TR_var Any = 1
TR_print(Any)`,
		`TR_var a = {"TR_Any": 1}
TR_print(a)`,
	} {
		if errs := checkCoded(t, code); len(errs) != 0 {
			t.Errorf("%s: hata beklenmiyordu: %v", code, errs)
		}
	}
}
