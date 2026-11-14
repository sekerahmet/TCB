package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yerinde yazma kopyadan geçmez (15.6d): atamanın, bileşik atamanın ve TR_write_slice'ın hedefinin
// zincirinde kopya veren bir adım varsa yazma kopyada kalırdı; denetim hatasıdır (H6740, H6741). Tip
// bilinmeyen adımı yorumlayıcı çalışırken denetler (interp/copy_writes.go). Zincirin sınıfı
// ast.WriteChain'dedir.

// checkCopyWriteTarget atamanın ya da çoklu atamanın bir hedefini denetler: en dıştaki indeks
// yazmanın kendisidir, kural onun solundaki zincire uygulanır. first ve last deyimin ilk ve son
// parçasıdır (iletinin önerisindeki deyim). Tipler hedefin çıkarımında kaydedilmiştir (itemTypes).
func (c *Checker) checkCopyWriteTarget(target ast.Expression, first, last ast.Node) {
	ix, ok := target.(*ast.IndexExpression)
	if !ok || !c.tensorTyped(ix.Left) {
		return
	}
	c.checkCopyChain(ast.WriteChain(ix.Left), first, last)
}

// checkCopyWriteMulti çoklu atamanın t hedefini denetler; önerinin deyimi bütün çoklu atamadır.
func (c *Checker) checkCopyWriteMulti(s *ast.MultiAssignStatement, t ast.Expression) {
	if len(s.Values) > 0 {
		c.checkCopyWriteTarget(t, s.Targets[0], s.Values[len(s.Values)-1])
	}
}

// checkCopyWriteCall TR_write_slice'ın hedefini denetler: kural hedefin tamamına uygulanır.
// Hedefin tensör olduğunu imza söyler.
func (c *Checker) checkCopyWriteCall(s *ast.ExpressionStatement) {
	call, ok := s.Expression.(*ast.CallExpression)
	if !ok || len(call.Arguments) == 0 {
		return
	}
	if k, _ := c.calledBuiltin(call); k != codes.FnWriteSlice {
		return
	}
	c.checkCopyChain(ast.WriteChain(call.Arguments[0]), call, call)
}

// checkCopyChain zincirde yazmaya en yakın kopya adımını bildirir. Tensörde tensörle indeks kopyadır
// (maske de: denetim cinsi bilmez); tipi bilinmeyen indeks çalışma anına kalır.
func (c *Checker) checkCopyChain(chain []ast.WriteStep, first, last ast.Node) {
	j, clean := ast.NearestCopy(chain, func(i int) bool {
		s := chain[i]
		return !ast.LiteralIndex(s.Index) && c.tensorTyped(s.Left) && c.tensorTyped(s.Index)
	})
	if j < 0 {
		return
	}
	taken := func(name string) bool { _, ok := c.scope.Resolve(name); return ok }
	t, ok := ast.NewCopyWriteText(c.fileLines, first, last, chain[j], taken)
	if !ok {
		t = ast.CopyWriteText{Step: c.nodeText(chain[j].Expr), Write: "...", Copy: "c", K: "k", I: "i"}
	}
	c.addErrorAt(chain[j].Expr, CopyWriteError(chain[j], clean, t))
}

// CopyWriteError kopyaya yazmanın iletisidir; denetleyici ve yorumlayıcı aynı kodu verir (13.11'in
// istisnası). Temiz tabanlı tensörle indekste öneri geri yazmayı da içerir (H6740).
func CopyWriteError(s ast.WriteStep, clean bool, t ast.CopyWriteText) error {
	if s.Kind == ast.StepIndex && clean {
		return messages.New(messages.H6740, t.Step, t.Base, t.Index, t.Write, t.Copy, t.K, t.I)
	}
	kind := "other"
	switch s.Kind {
	case ast.StepOp:
		kind = "op"
	case ast.StepCall:
		kind = "call"
	case ast.StepMethod:
		kind = "method"
	}
	return messages.New(messages.H6741, t.Step, kind, t.Write, t.Copy)
}

// tensorTyped e'nin çıkarılan tipi tensör mü (isteğe bağlı tensör de: .TR_grad).
func (c *Checker) tensorTyped(e ast.Expression) bool {
	t, ok := c.itemTypes[e]
	return ok && shapeOf(t).base == typeTensor
}
