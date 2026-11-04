package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

const squareFn = "TR_func kare(x: TR_Int) -> TR_Int {\n    TR_return x * x\n}\n"

// Kural (10.1): TR_async f(x) TR_Task[T] verir, T çağrının tipidir; blok TR_Task[TR_NoneType]
// verir. g.TR_wait() T, g.TR_done() TR_Bool verir. Dönüşü bilinmeyen çağrının görevi parametresiz
// TR_Task'tır (denetleyicinin iç tipi): beklenen sonucun tipi bilinmez, tipli yere verilebilir.
func TestATaskHasItsCallsType(t *testing.T) {
	expectTypes(t, map[string]string{
		squareFn + "TR_var g = TR_async kare(2)":                                  containerType(typeTask, []string{typeInt}),
		"TR_var g = TR_async { TR_print(1) }":                                     containerType(typeTask, []string{typeNull}),
		"TR_var g = TR_async " + unknownValue:                                     typeTask,
		squareFn + "TR_var g = TR_async kare(2)\nTR_var n = g.TR_wait()":          typeInt,
		squareFn + "TR_var g = TR_async kare(2)\nTR_var b = g.TR_done()":          typeBool,
		"TR_var g = TR_async " + unknownValue + "\nTR_var n = g.TR_wait()":        typeAny,
		squareFn + "TR_var l = TR_wait_all([TR_async kare(1), TR_async kare(2)])": listType(typeInt),
	})
	for _, source := range []string{
		squareFn + "TR_var g: TR_Task[TR_Int] = TR_async kare(2)\nTR_var n: TR_Int = g.TR_wait()",
		"TR_var g = TR_async " + unknownValue + "\nTR_var s: TR_String = g.TR_wait()\nTR_var n: TR_Int = g.TR_wait()",
		"TR_var g: TR_Task[TR_Int] = TR_async " + unknownValue,
		"TR_var g: TR_Task[TR_NoneType] = TR_async { TR_print(1) }",
		squareFn + "TR_var l: TR_List[TR_Int] = TR_wait_all([TR_async kare(1)])",
	} {
		noErrors(t, source)
	}
	singleError(t, squareFn+"TR_var g: TR_Task[TR_String] = TR_async kare(2)", messages.H1001, 4, "TR_Task[TR_String]", "TR_Task[TR_Int]")
	singleError(t, squareFn+"TR_var g = TR_async kare(2)\nTR_var s: TR_String = g.TR_wait()", messages.H1001, 5)
	singleError(t, squareFn+"TR_var l: TR_List[TR_String] = TR_wait_all([TR_async kare(1)])", messages.H1001, 4)
	singleError(t, squareFn+"TR_var g = TR_async kare(2)\nTR_var b = g.TR_done(1)", messages.H2011, 5)
	singleError(t, squareFn+"TR_var g = TR_async kare(2)\nTR_var b = g.TR_append(1)", messages.H2474, 5)
	singleError(t, "TR_var g = TR_async { TR_print(1) }\ng.TR_append(1)", messages.H2474, 2)
	singleError(t, "TR_func f() {\n    TR_var g = TR_async { TR_print(1) }\n    g.TR_append(1)\n}", messages.H2474, 3)
}

// Kural (10.1): görevin içi denetlenir; değer olarak yazılan TR_async'in çağrısındaki ya da
// bloğundaki tanımsız ad da bildirilir.
func TestATaskValuesInsideIsChecked(t *testing.T) {
	singleErrorAt(t, "TR_var g = TR_async TR_print(yok)", 1, 30)
	singleErrorAt(t, "TR_var g = TR_async { TR_print(yok) }", 1, 32)
}

// Kural (10.1): değeri olmayan görevin tipi TR_Task[TR_NoneType]'tır: g.TR_wait() değer yerinde
// H1143'tür, deyim olarak geçer. Değer veren görevin ve tipi bilinmeyen görevin TR_wait'i değer verir.
func TestWaitingForATaskWithoutAValue(t *testing.T) {
	expectTypes(t, map[string]string{
		noValueFn + "TR_var g = TR_async f()":     containerType(typeTask, []string{typeNull}),
		noValueFn + "TR_var g = TR_async { f() }": containerType(typeTask, []string{typeNull}),
	})
	for _, source := range []string{
		"TR_var g = TR_async { f() }; TR_var y = g.TR_wait()",
		"TR_var g = TR_async f(); TR_print(g.TR_wait())",
		"TR_func h(g: TR_Task[TR_NoneType]) { TR_var y = g.TR_wait() }",
	} {
		_, errs := checkErrs(t, noValueFn+source)
		if !hasCode(errs, messages.H1143) {
			t.Errorf("H1143 beklenirdi, gelen %v\nkaynak:\n%s", errs, source)
		}
	}
	for _, source := range []string{
		noValueFn + "TR_var g = TR_async { f() }; g.TR_wait()",
		squareFn + "TR_var g = TR_async kare(2); TR_var y = g.TR_wait()",
		"TR_var g = TR_async " + unknownValue + "; TR_var y = g.TR_wait()",
	} {
		noErrors(t, source)
	}
	singleError(t, noValueFn+"TR_var g: TR_Task[TR_Int] = TR_async f()", messages.H1001, 2)
}
