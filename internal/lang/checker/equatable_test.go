package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Kural (13.10, 15.5, 4.5, 7.6–7.8): eşitlenebilirliğin tek tanımı. Tensör, fonksiyon ve arayüz
// eşitlenemez; onları iç içe tutan liste, sözlük, demet, yapı alanı ve durumlu tipin durum alanı da. T?,
// T'nin kuralındadır. Kendi tipinde alanı olan yapı sonlu biter; tipi bilinmeyen ad eşitlenebilir sayılır.
func TestEquatableIsOneDefinition(t *testing.T) {
	const defs = `TR_struct Kutu {
    t: TR_Tensor
}
TR_struct Düz {
    n: TR_Int
}
TR_struct Düğüm {
    n: TR_Int
    s: Düğüm?
}
TR_struct Halka {
    s: Halka?
    k: TR_List[Kutu]
}
TR_struct Derin {
    d: TR_Dict[TR_String, (TR_Int, Kutu?)]
}
TR_struct Fonk {
    f: (TR_Int) -> TR_Int
}
TR_struct Kat {
    k: Katman
}
TR_enum C {
    Var(t: TR_Tensor)
    Yok
}
TR_enum D {
    Bir(n: TR_Int)
    İki
}
TR_enum E {
    Bir(d: Derin)
}
TR_interface Katman {
    TR_func ileri(x: TR_Tensor) -> TR_Tensor
}
`
	p := parser.New(lexer.NewBody(defs, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	c := New()
	c.CheckProgram(prog)
	// Tablodaki tipler denetleyicinin iç adlarıyla (codes.Name) kurulur: TR_List → typeList.
	internal := map[string]string{"TR_Int": typeInt, "TR_Float": typeFloat, "TR_String": typeString,
		"TR_List": typeList, "TR_Dict": typeDict, "TR_Tensor": typeTensor, "TR_Function": typeFunction,
		"TR_Any": typeAny, "TR_BuiltinFunction": codes.Name(codes.TypeBuiltinFunction)}
	cases := map[string]string{
		"TR_Int": "", "TR_String?": "", "(TR_Int, TR_String)": "", "TR_List[TR_Dict[TR_String, TR_Int]]": "",
		"Düz": "", "Düğüm": "", "Düğüm?": "", "D": "", "TR_Any": "", "Bilinmeyen": "", "T": "",
		"TR_Tensor": holdsTensor, "TR_Tensor?": holdsTensor, "TR_Tensor[TR_Float]": holdsTensor,
		"TR_List[TR_Tensor]": holdsTensor, "TR_List[TR_List[TR_Tensor?]]": holdsTensor,
		"TR_Dict[TR_String, TR_Tensor]": holdsTensor, "(TR_List[TR_Tensor], TR_Int)": holdsTensor,
		"(TR_Int, (TR_String, TR_Tensor))": holdsTensor, "Kutu": holdsTensor, "Kutu?": holdsTensor,
		"Halka": holdsTensor, "Derin": holdsTensor, "C": holdsTensor, "E": holdsTensor,
		"TR_List[C]":  holdsTensor,
		"TR_Function": holdsFunction, "TR_List[TR_Function]": holdsFunction, "(TR_Function, TR_Int)": holdsFunction,
		"Fonk": holdsFunction, "TR_BuiltinFunction": holdsFunction,
		"Katman": holdsInterface, "TR_List[Katman]": holdsInterface, "Kat": holdsInterface, "Katman?": holdsInterface,
	}
	for typ, want := range cases {
		inner := mapTypeNames(typ, func(n string) string {
			if v, ok := internal[n]; ok {
				return v
			}
			return n
		})
		if got := c.unequatable(inner); got != want {
			t.Errorf("%s (%s): beklenen %q, gelen %q", typ, inner, want, got)
		}
	}
}
