package ast_test

import (
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// writeBase `<e>[0] = 1.0`'ın en dıştaki indeksinin solu, yani yazmanın zinciridir.
func writeBase(t *testing.T, e string) ast.Expression {
	t.Helper()
	p := parser.New(lexer.NewBody(e+"[0] = 1.0", true))
	prog := p.ParseProgram()
	if len(p.Errs()) != 0 {
		t.Fatalf("%s: %v", e, p.Errs())
	}
	return prog.Statements[0].(*ast.AssignStatement).Target.(*ast.IndexExpression).Left
}

func kinds(chain []ast.WriteStep) []ast.WriteStepKind {
	var out []ast.WriteStepKind
	for _, s := range chain {
		out = append(out, s.Kind)
	}
	return out
}

// Kural (15.6d): yerinde yazmanın hedefi adlardan, alanlardan, kap öğelerinden ve görünüm
// işlemlerinden kurulur; öbür her adım kopya verir. Zincir yazmaya en yakın adımdan köke iner ve
// ilk kopya adımında durur.
func TestWriteChainSteps(t *testing.T) {
	A := ast.StepName
	cases := []struct {
		e    string
		want []ast.WriteStepKind
	}{
		{"t", []ast.WriteStepKind{A}},
		{"t[i]", []ast.WriteStepKind{ast.StepIndex, A}},
		{"p.w", []ast.WriteStepKind{ast.StepField, A}},
		{"w.TR_grad", []ast.WriteStepKind{ast.StepField, A}},
		{"d.TR_bias", []ast.WriteStepKind{ast.StepField, A}}, // kitaplık yapısının alanı (tr/layers)
		{"p.0", []ast.WriteStepKind{ast.StepElement, A}},
		{"t.TR_slice(0, 0, 1)", []ast.WriteStepKind{ast.StepView, A}},
		{"t.TR_transpose()", []ast.WriteStepKind{ast.StepView, A}},
		{"t.TR_permute([1, 0])", []ast.WriteStepKind{ast.StepView, A}},
		{"(t * 2.0)", []ast.WriteStepKind{ast.StepOp}},
		{"(-t)", []ast.WriteStepKind{ast.StepOp}},
		{"TR_Tensor(t)", []ast.WriteStepKind{ast.StepCall}},
		{"al(t)", []ast.WriteStepKind{ast.StepCall}},
		{"t.TR_to_dtype(\"float64\")", []ast.WriteStepKind{ast.StepMethod}},
		{"t.TR_reshape([2, 2])", []ast.WriteStepKind{ast.StepMethod}},
		{"t.TR_to_device(\"cpu\")", []ast.WriteStepKind{ast.StepMethod}},
		{"t.TR_tolist", []ast.WriteStepKind{ast.StepOther}},
		{"[t][0]", []ast.WriteStepKind{ast.StepIndex, ast.StepOther}},
		{"t[idx].TR_slice(0, 0, 1)", []ast.WriteStepKind{ast.StepView, ast.StepIndex, A}},
		{"f(t)[idx]", []ast.WriteStepKind{ast.StepIndex, ast.StepCall}},
		{"self.k[l].TR_transpose()", []ast.WriteStepKind{ast.StepView, ast.StepIndex, ast.StepField, A}},
	}
	for _, c := range cases {
		if got := kinds(ast.WriteChain(writeBase(t, c.e))); !slices.Equal(got, c.want) {
			t.Errorf("%s: adımlar %v, beklenen %v", c.e, got, c.want)
		}
	}
	checkEveryTensorMember(t)
}

// checkEveryTensorMember (15.6d, DENETİM 693 N2): tensör döndüren her tensör üyesi sınıflanır; görünüm yalnız 15.6d'nin
// listesindeki motor metotlarıdır, .TR_grad alandır (15 onu alan sayar), öbürü kopya adımıdır. Yeni bir
// üye kendiliğinden izinli sayılmaz.
func checkEveryTensorMember(t *testing.T) {
	views := map[codes.Code]bool{codes.MemSlice: true, codes.MemTranspose: true, codes.MemPermute: true}
	seen := 0
	for _, k := range codes.Codes() {
		def, _ := codes.DefinitionOf(k)
		if def.Kind != codes.KindMethod && def.Kind != codes.KindProperty {
			continue
		}
		for _, m := range def.Members {
			if m.Type != codes.TypeTensor || !slices.Contains(m.Signature.Return.Types, codes.TypeTensor) {
				continue
			}
			seen++
			e := "t." + codes.Spelling(k)
			if def.Kind == codes.KindMethod {
				e += "()"
			}
			s := ast.WriteChain(writeBase(t, e))[0]
			switch {
			case views[k]:
				if s.Kind != ast.StepView {
					t.Errorf("%s görünümdür, adım %v", e, s.Kind)
				}
			case k == codes.MemGrad:
				if s.Kind != ast.StepField {
					t.Errorf("%s alandır, adım %v", e, s.Kind)
				}
			case !s.Copies():
				t.Errorf("%s 15.6d'nin görünüm listesinde yok, kopya adımı olmalı: %v", e, s.Kind)
			}
		}
	}
	if seen < 7 {
		t.Errorf("yalnız %d tensör üyesi bulundu", seen)
	}
}

// Kural (15.6d, K4): yazmaya en yakın kopya adımı bildirilir; altında başka kopya yoksa tabanı temizdir.
func TestNearestCopy(t *testing.T) {
	chain := ast.WriteChain(writeBase(t, "f(t)[idx][j]")) // [j] [idx] f(t)
	tensorIndex := func(i int) bool { return i == 1 }
	if j, clean := ast.NearestCopy(chain, tensorIndex); j != 1 || clean {
		t.Errorf("f(t)[idx]: %d %v, beklenen 1 false", j, clean)
	}
	chain = ast.WriteChain(writeBase(t, "t[idx][j]"))
	if j, clean := ast.NearestCopy(chain, tensorIndex); j != 1 || !clean {
		t.Errorf("t[idx]: %d %v, beklenen 1 true", j, clean)
	}
	if j, _ := ast.NearestCopy(chain, func(int) bool { return false }); j != -1 {
		t.Errorf("kopyasız zincir: %d", j)
	}
}
