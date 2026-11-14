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
		// 5.4: liste, sözlük ve yapı hedefinin zinciri aynı sınıflamadır (dilimin kopya olduğunu tip söyler)
		{"l[0..2]", []ast.WriteStepKind{ast.StepIndex, A}},
		{"d.TR_get(\"a\")", []ast.WriteStepKind{ast.StepMethod}},
		{"l.TR_first", []ast.WriteStepKind{ast.StepOther}},
		{"[1, 2]", []ast.WriteStepKind{ast.StepOther}},
		{"(l + m)", []ast.WriteStepKind{ast.StepOp}},
		{"Nokta(1, 2)", []ast.WriteStepKind{ast.StepCall}},
		{"p.liste()", []ast.WriteStepKind{ast.StepMethod}},
		{"[x TR_for x TR_in l]", []ast.WriteStepKind{ast.StepOther}},
		{"dn[\"p\"].x", []ast.WriteStepKind{ast.StepField, ast.StepIndex, A}},
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

// Kural (5.4, KA1, KA5): kap hedefinin zincirinde yazımdaki aralık dilimdir (SliceIndex); kabın
// öğesini veren üye (.TR_get(k) tek argümanla, .TR_first, .TR_last) kabı ve indeksi verir; negatif
// sabit indeks kaynağın indeksi olamaz, metin ve aralık tam sayı indeksi olamaz. Önerilerin yazımı
// deyimin parçasını değiştirir (StatementWith), dilimin üstündeki indeks kaynağa a + i ile gider.
func TestContainerChainCopies(t *testing.T) {
	steps := func(e string) []ast.WriteStep { return ast.WriteChain(writeBase(t, e)) }
	if r, ok := ast.SliceIndex(steps("l[1..3]")[0]); !ok || r == nil {
		t.Errorf("l[1..3] dilimdir")
	}
	for _, e := range []string{"l[i]", "l[r]", "f()"} {
		if _, ok := ast.SliceIndex(steps(e)[0]); ok {
			t.Errorf("%s yazımda dilim değildir", e)
		}
	}
	for _, c := range []struct {
		e    string
		want codes.Code
		key  bool
	}{
		{"d.TR_get(\"a\")", codes.MemGet, true}, {"l.TR_first", codes.MemFirst, false}, {"l.TR_last", codes.MemLast, false},
		{"d.TR_get(\"a\", 0)", codes.None, false}, {"l?.TR_first", codes.None, false}, {"l.TR_pop()", codes.None, false},
	} {
		recv, key, k := ast.ElementAccessor(steps(c.e)[0])
		if k != c.want || (k != codes.None) != (recv != nil) || (key != nil) != c.key {
			t.Errorf("%s: %v %v %v, beklenen %v", c.e, recv, key, k, c.want)
		}
	}
	index := func(e string) ast.Expression { return writeBase(t, "l["+e+"]").(*ast.IndexExpression).Index }
	for e, want := range map[string][2]bool{"-1": {true, false}, "0": {false, true}, "i": {false, true}, "-i": {false, true},
		"\"a\"": {false, false}, "0..2": {false, false}, "1.5": {false, false}} {
		if neg, maybe := ast.NegativeLiteral(index(e)), ast.MaybeInteger(index(e)); neg != want[0] || maybe != want[1] {
			t.Errorf("%s: negatif %v, tam sayı olabilir %v; beklenen %v", e, neg, maybe, want)
		}
	}
	for _, c := range []struct{ code, want, stmt string }{
		{"l[0..2][i] = 9", "0 + i", "l[0 + i] = 9"},
		{"l[1..3][0] += 5", "1", "l[1] += 5"},
		{"l[a..3][1] = 7", "a + 1", "l[a + 1] = 7"},
		{"l[a - 1..3][i + 1] = 7", "(a - 1) + (i + 1)", "l[(a - 1) + (i + 1)] = 7"},
		{"l[9223372036854775807..9223372036854775807][1] = 7", "9223372036854775807 + 1", "l[9223372036854775807 + 1] = 7"},
	} {
		lines := []string{c.code}
		p := parser.New(lexer.NewBody(c.code, true))
		s := p.ParseProgram().Statements[0].(*ast.AssignStatement)
		above := s.Target.(*ast.IndexExpression)
		r, _ := ast.SliceIndex(ast.WriteChain(above.Left)[0])
		got, ok := ast.SliceSourceIndex(lines, r, above.Index)
		if !ok || got != c.want {
			t.Errorf("%s: kaynağın indeksi %q, beklenen %q", c.code, got, c.want)
		}
		if stmt, ok := ast.StatementWith(lines, s.Target, s.Value, above, "l["+got+"]"); !ok || stmt != c.stmt {
			t.Errorf("%s: deyim %q, beklenen %q", c.code, stmt, c.stmt)
		}
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
