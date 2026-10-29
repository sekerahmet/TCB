package parser

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// named, opt, tuple ve fn beklenen tipleri kurar (konumsuz: Dump belirteçleri atlar). Ad, kodlu
// dosyadaki gibi sözcük çözümleyicinin verdiği kimliktir (TR_Int → TR_type_Int).
func named(name string, args ...ast.TypeRef) ast.TypeRef {
	return ast.TypeRef{Name: codedLexer(name).NextToken().Literal, Args: args}
}

func opt(t ast.TypeRef) ast.TypeRef {
	t.Optional = true
	return t
}

func tuple(elems ...ast.TypeRef) ast.TypeRef { return ast.TypeRef{Elems: elems} }

func fn(ret ast.TypeRef, params ...ast.TypeRef) ast.TypeRef {
	if params == nil {
		params = []ast.TypeRef{}
	}
	return ast.TypeRef{Params: params, Return: &ret}
}

// dumpType tipi ast.Dump biçiminde yazar (Dump bir düğüm ister: tip boş bir tanıma konur).
func dumpType(t *ast.TypeRef) string {
	return ast.Dump(&ast.VarStatement{Type: t})
}

// parseClean kodu ayrıştırır; hata varsa testi durdurur.
func parseClean(t *testing.T, code string) *ast.Program {
	t.Helper()
	p := New(codedLexer(code))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("%q: %v", code, errs)
	}
	return prog
}

// Kural (3.5, 3.6, 3.6a, 3.7, 6.9, 6.10, 15.1): dil tanımının yazdığı her tip yazımı ve iç içe
// biçimleri TypeRef'e ayrışır. Parantezden sonra '->' geliyorsa fonksiyon tipi, gelmiyorsa demet
// tipidir, tek öğeli parantez gruplar; '->' sağdan birleşir; fonksiyon tipinin ardındaki '?' dönüş
// tipinindir, parantezli fonksiyon tipinin ardındaki fonksiyonun kendisinin. TR_AnyTensor
// (15.1) ve TR_Task (10.1) henüz kod değildir, kendi izlerinde kaydedilir.
func TestTypeFormsParse(t *testing.T) {
	i, s, b, f := named("TR_Int"), named("TR_String"), named("TR_Bool"), named("TR_Float")
	cases := []struct {
		src  string
		want ast.TypeRef
	}{
		{"TR_Int", i},
		{"TR_Int?", opt(i)},
		{"TR_List[TR_Int]", named("TR_List", i)},
		{"TR_List[TR_Float?]", named("TR_List", opt(f))},
		{"TR_List[TR_Int]?", opt(named("TR_List", i))},
		{"TR_Dict[TR_String, TR_Int]", named("TR_Dict", s, i)},
		{"TR_Dict[TR_String, TR_List[TR_Int?]]", named("TR_Dict", s, named("TR_List", opt(i)))},
		{"TR_Dict[(TR_Int, TR_Int), TR_Int]", named("TR_Dict", tuple(i, i), i)},
		{"(TR_Int, TR_String)", tuple(i, s)},
		{"(TR_Int, TR_String)?", opt(tuple(i, s))},
		{"(TR_Int, (TR_Bool, TR_Float))", tuple(i, tuple(b, f))},
		{"(TR_Int)", i},
		{"((TR_Int) -> TR_Bool)?", opt(fn(b, i))},
		{"(TR_Int) -> TR_Bool", fn(b, i)},
		{"(TR_Int, TR_Int) -> TR_Int", fn(i, i, i)},
		{"() -> TR_Int", fn(i)},
		{"(TR_Int) -> TR_NoneType", fn(named("TR_NoneType"), i)},
		{"(TR_Int) -> (TR_Float, TR_Int)", fn(tuple(f, i), i)},
		{"(TR_Int) -> TR_Bool?", fn(opt(b), i)},
		{"() -> (TR_Int, TR_Int) -> TR_Int", fn(fn(i, i, i))},
		{"((TR_Int) -> TR_Bool) -> TR_Int", fn(i, fn(b, i))},
		{"TR_List[(TR_Int) -> TR_Bool]", named("TR_List", fn(b, i))},
		{"TR_Tensor[TR_Int]", named("TR_Tensor", i)},
		{"TR_Tensor?", opt(named("TR_Tensor"))},
		{"Kutu[T]", named("Kutu", named("T"))},
		{"T?", opt(named("T"))},
	}
	for _, c := range cases {
		prog := parseClean(t, "TR_var x: "+c.src+" = TR_none")
		got := prog.Statements[0].(*ast.VarStatement).Type
		if g, w := dumpType(got), dumpType(&c.want); g != w {
			t.Errorf("%s:\ngot  %s\nwant %s", c.src, g, w)
		}
	}
}

// Parametre, alan, durum alanı, dönüş, arayüz metodunun dönüşü ve ok fonksiyonunun parametresi aynı
// tip okuyucuyu kullanır. Birden çok değerin dönüşü bir demet tipidir (6.3).
func TestTypeFormsInEveryPosition(t *testing.T) {
	listInt := named("TR_List", named("TR_Int"))
	optStr := opt(named("TR_String"))
	prog := parseClean(t, `TR_func f(a: TR_List[TR_Int], b: TR_String? = TR_none) -> TR_List[TR_Int]? { TR_return TR_none }
TR_func g() -> (TR_Int, TR_String) { TR_return 1, "a" }
TR_struct S { a: TR_List[TR_Int], b: TR_String? = TR_none }
TR_enum E { A(x: TR_List[TR_Int]) }
TR_interface I { TR_func m(a: TR_String?) -> TR_List[TR_Int] }
TR_var h = (a: TR_List[TR_Int]) => a`)
	check := func(where string, got *ast.TypeRef, want ast.TypeRef) {
		t.Helper()
		if g, w := dumpType(got), dumpType(&want); g != w {
			t.Errorf("%s:\ngot  %s\nwant %s", where, g, w)
		}
	}
	f := prog.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.FunctionLiteral)
	check("parametre", f.Parameters[0].Type, listInt)
	check("varsayılanlı parametre", f.Parameters[1].Type, optStr)
	check("dönüş", f.ReturnType, opt(listInt))
	g := prog.Statements[1].(*ast.ExpressionStatement).Expression.(*ast.FunctionLiteral)
	check("birden çok değer", g.ReturnType, tuple(named("TR_Int"), named("TR_String")))
	st := prog.Statements[2].(*ast.StructStatement)
	check("alan", st.Fields[0].Type, listInt)
	check("varsayılanlı alan", st.Fields[1].Type, optStr)
	en := prog.Statements[3].(*ast.StructStatement)
	check("durum alanı", en.Variants[0].Fields[0].Type, listInt)
	m := prog.Statements[4].(*ast.StructStatement).Methods["m"]
	check("arayüz parametresi", m.Parameters[0].Type, optStr)
	check("arayüz dönüşü", m.ReturnType, listInt)
	h := prog.Statements[5].(*ast.VarStatement).Value.(*ast.FunctionLiteral)
	check("ok fonksiyonu parametresi", h.Parameters[0].Type, listInt)
}

// Yanlış yazımlar hatadır; '->'süz '()' de. Tek öğeli parantez gruplamadır (3.6a), hata değildir.
func TestMalformedTypesAreErrors(t *testing.T) {
	for _, src := range []string{
		"TR_var x: TR_List[ = 1",
		"TR_var x: TR_List[TR_Int = 1",
		"TR_var x: TR_List[] = 1",
		"TR_var x: TR_Dict[TR_String TR_Int] = 1",
		"TR_var x: TR_Int?? = 1",
		"TR_var x: TR_Int? ? = 1",
		"TR_var x: (TR_Int) -> = 1",
		"TR_var x: () = 1",
		"TR_var x: (TR_Int, ) = 1",
		"TR_func f() -> { TR_return 1 }",
		"TR_func f(a: TR_Int??) { }",
		"TR_func f(a: TR_List[) { }",
		"TR_struct S { a: TR_Int?? }",
		"TR_interface I { TR_func m() -> }",
	} {
		if errs := parseErrors(src); len(errs) == 0 {
			t.Errorf("%q hata vermeli", src)
		}
	}
}

// Tipin içindeki her ad tip görevini taşır (kodlu biçime çevirici onu tip adı olarak çevirir).
func TestNamesInsideTypesAreTypeRoles(t *testing.T) {
	src := "TR_var x: TR_Dict[TR_String, (TR_Int) -> TR_Bool?] = {}"
	p := New(codedLexer(src))
	p.RecordRoles()
	p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	for _, name := range []string{"TR_Dict", "TR_String", "TR_Int", "TR_Bool"} {
		col := strings.Index(src, name) + 1
		if got := p.Roles()[[2]int{1, col}]; got != RoleType {
			t.Errorf("%s (1:%d): görev %v, beklenen RoleType", name, col, got)
		}
	}
}
