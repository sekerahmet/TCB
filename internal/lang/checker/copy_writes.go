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
//
// Hedef liste, sözlük ya da yapıysa da (5.4): zincir adlardan, alanlardan ve kap öğelerinden geçer;
// Copies() adımları (işlem, çağrı, metot, öbürü) ve liste ya da metin dilimi kopya adımıdır (H2740,
// H2741, H2743). Kural atamanın indeks ve alan hedefine ve yerinde değiştirici metodun ("in_place",
// .TR_append) alıcısına uygulanır.

// checkCopyWriteTarget atamanın ya da çoklu atamanın bir hedefini denetler: en dıştaki indeks ya da
// alan yazmanın kendisidir, kural onun altındaki zincire uygulanır. first ve last deyimin ilk ve son
// parçasıdır (iletinin önerisindeki deyim). Tipler hedefin çıkarımında kaydedilmiştir (itemTypes).
func (c *Checker) checkCopyWriteTarget(target ast.Expression, first, last ast.Node) {
	switch t := target.(type) {
	case *ast.IndexExpression:
		if c.tensorTyped(t.Left) {
			c.checkCopyChain(ast.WriteChain(t.Left), first, last)
		} else if c.containerTyped(t.Left) {
			c.checkContainerChain(t.Left, t, first, last)
		}
	case *ast.MemberExpression:
		if !t.Safe && c.structTyped(t.Object) { // alan yalnız yapıda yazılır; listenin alanı kendi hatasıdır
			c.checkContainerChain(t.Object, nil, first, last)
		}
	}
}

// checkCopyWriteMulti çoklu atamanın t hedefini denetler; önerinin deyimi bütün çoklu atamadır.
func (c *Checker) checkCopyWriteMulti(s *ast.MultiAssignStatement, t ast.Expression) {
	if len(s.Values) > 0 {
		c.checkCopyWriteTarget(t, s.Targets[0], s.Values[len(s.Values)-1])
	}
}

// checkCopyWriteCall TR_write_slice'ın hedefini denetler: kural hedefin tamamına uygulanır.
// Hedefin tensör olduğunu imza söyler. Yerinde değiştirici metodun alıcısı liste ya da sözlükse
// alıcının zinciri de denetlenir (5.4); başka alıcıda (TR_Logger) metot yazma değildir.
func (c *Checker) checkCopyWriteCall(s *ast.ExpressionStatement) {
	call, ok := s.Expression.(*ast.CallExpression)
	if !ok {
		return
	}
	if f, ok := call.Function.(*ast.MemberExpression); ok && !f.Safe && codes.IsInPlace(f.NameToken.Code) {
		if r := shapeOf(c.receivers[call]).base; (r == typeList || r == typeDict) && len(call.Arguments) > 0 { // argümansız çağrı H2011
			c.checkContainerChain(f.Object, nil, call, call)
		}
		return
	}
	if len(call.Arguments) == 0 {
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

// ContainerWrite liste, sözlük ya da yapı hedefinin zincirindeki kopya adımıdır (5.4); denetleyici ve
// yorumlayıcı aynı kodu verir (13.11'in istisnası). Above dilimin üstündeki indeks, IntAbove onun tam
// sayı olduğu, Accessor adımın kabın öğesini veren üye olduğu, Clean adımın altında başka kopya
// adımı olmadığıdır (öneri ancak o zaman yazmayı kaynağa götürür).
type ContainerWrite struct {
	Step                      ast.WriteStep
	Above                     *ast.IndexExpression
	IntAbove, Accessor, Clean bool
}

// Error iletidir: kabın öğesini veren üyede H2741 (kabın indeksi), tam sayı indeksli liste diliminde
// H2743 (kaynağın indeksi ve ad), öbüründe H2740 (ad). Negatif sabit indeks dilimin sonundan sayar,
// kaynağın indeksi olmaz: H2740. stepText deyim tek satıra sığmazsa adımın yazımıdır.
func (w ContainerWrite) Error(lines []string, first, last ast.Node, taken func(string) bool, stepText string) error {
	s := w.Step
	t, ok := ast.NewCopyWriteText(lines, first, last, s, taken)
	if !ok {
		t = ast.CopyWriteText{Step: stepText, Write: "...", Copy: "c"}
	}
	if w.Accessor && w.Clean {
		return messages.New(messages.H2741, t.Step, accessorDirect(lines, first, last, s))
	}
	if r, lit := ast.SliceIndex(s); lit && w.Clean && w.Above != nil && w.IntAbove && !ast.NegativeLiteral(w.Above.Index) {
		base, ok1 := ast.SourceText(lines, s.Left)
		index, ok2 := ast.SliceSourceIndex(lines, r, w.Above.Index)
		i, _ := ast.SourceText(lines, w.Above.Index)
		source, ok3 := ast.StatementWith(lines, first, last, w.Above, base+"["+index+"]")
		if !ok1 || !ok2 || !ok3 {
			base, source, i = "...", "...", "..."
		}
		return messages.New(messages.H2743, t.Step, base, source, i, t.Write, t.Copy)
	}
	kind := "other"
	switch s.Kind {
	case ast.StepOp:
		kind = "op"
	case ast.StepCall:
		kind = "call"
	case ast.StepMethod:
		kind = "method"
	case ast.StepIndex:
		kind = "slice"
	}
	return messages.New(messages.H2740, t.Step, kind, t.Write, t.Copy)
}

// accessorDirect deyimin kabın öğesini veren üye yerine kabın indeksiyle yazılmış hâlidir (H2741):
// d.TR_get(k) → d[k], l.TR_first → l[0], l.TR_last → l[-1].
func accessorDirect(lines []string, first, last ast.Node, s ast.WriteStep) string {
	recv, key, k := ast.ElementAccessor(s)
	r, ok := ast.SourceText(lines, recv)
	switch recv.(type) {
	case *ast.InfixExpression, *ast.PrefixExpression:
		r = "(" + r + ")"
	}
	index := map[codes.Code]string{codes.MemFirst: "0", codes.MemLast: "-1"}[k]
	if key != nil {
		var ok2 bool
		index, ok2 = ast.SourceText(lines, key)
		ok = ok && ok2
	}
	direct, ok3 := ast.StatementWith(lines, first, last, s.Expr, r+"["+index+"]")
	if !ok || !ok3 {
		return "..."
	}
	return direct
}

// tensorTyped e'nin çıkarılan tipi tensör mü (isteğe bağlı tensör de: .TR_grad).
func (c *Checker) tensorTyped(e ast.Expression) bool {
	t, ok := c.itemTypes[e]
	return ok && shapeOf(t).base == typeTensor
}

// structTyped e'nin çıkarılan tipi bir yapı mı (isteğe bağlısı da): alan hedefinin nesnesi.
func (c *Checker) structTyped(e ast.Expression) bool {
	t, ok := c.itemTypes[e]
	return ok && c.knownStruct(shapeOf(t).base)
}

// containerTyped e'nin çıkarılan tipi liste, sözlük ya da yapı mı (isteğe bağlısı da: d.TR_get(k)).
// Tipi bilinmeyen hedefi yorumlayıcı denetler (1.2).
func (c *Checker) containerTyped(e ast.Expression) bool {
	t, ok := c.itemTypes[e]
	b := shapeOf(t).base
	return ok && (b == typeList || b == typeDict || c.knownStruct(b))
}

// checkContainerChain liste, sözlük ya da yapı hedefinin zincirinde yazmaya en yakın kopya adımını
// bildirir (5.4). above en dıştaki indekstir (yazmanın kendisi; alan ve metot hedefinde nil): zincirin
// ilk adımı dilimse onun üstündeki indekstir (H2743).
func (c *Checker) checkContainerChain(base ast.Expression, above *ast.IndexExpression, first, last ast.Node) {
	chain := ast.WriteChain(base)
	j, clean := ast.NearestCopy(chain, func(i int) bool { return c.sliceStep(chain[i]) })
	if j < 0 {
		return
	}
	if j > 0 {
		above, _ = chain[j-1].Expr.(*ast.IndexExpression)
	}
	w := ContainerWrite{Step: chain[j], Above: above, Clean: clean,
		IntAbove: above != nil && c.itemTypes[above.Index] == typeInt, Accessor: c.containerAccessor(chain[j])}
	taken := func(name string) bool { _, ok := c.scope.Resolve(name); return ok }
	step, ok := ast.SourceText(c.fileLines, chain[j].Expr)
	if !ok {
		step = "..." // dosyasız denetim: adımın yazımı bilinmez
	}
	c.addErrorAt(chain[j].Expr, w.Error(c.fileLines, first, last, taken, step))
}

// sliceStep adım liste ya da metin tabanında aralıkla indeks mi (3.6: dilim yeni bir değerdir).
func (c *Checker) sliceStep(s ast.WriteStep) bool {
	if s.Kind != ast.StepIndex {
		return false
	}
	if _, lit := ast.SliceIndex(s); !lit && c.itemTypes[s.Index] != typeRange {
		return false
	}
	left := c.itemTypes[s.Left]
	return shapeOf(left).base == typeList || left == typeString
}

// containerAccessor adım liste ya da sözlüğün öğesini veren üye mi (.TR_get(k), .TR_first, .TR_last).
func (c *Checker) containerAccessor(s ast.WriteStep) bool {
	recv, _, k := ast.ElementAccessor(s)
	if k == codes.None {
		return false
	}
	t := c.itemTypes[recv]
	if call, ok := s.Expr.(*ast.CallExpression); ok {
		t = c.receivers[call]
	}
	b := shapeOf(t).base
	return b == typeList || b == typeDict && k == codes.MemGet
}
