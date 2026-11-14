package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yerinde yazma kopyadan geçmez (15.6d), çalışma anında: atamanın, bileşik atamanın ve
// TR_write_slice'ın hedefinin zincirinde kopya veren bir adım varsa hata (H6740, H6741; denetimle aynı
// kod). Denetleyici tipi bilinen zinciri derlemeden önce keser (checker/copy_writes.go); burada tipi
// bilinmeyen ve denetimsiz koşulan program yakalanır. Zincirin biçimi derlemede bilinir
// (ast.WriteChain): çalışırken yalnız indeks adımlarında indeksin tensör olup olmadığı
// (copyStepIndex) ve kopya adımlı zincirin sonunda değerin tensör olup olmadığı sınanır.

// Kapta da (5.4): yazılan değer liste, sözlük ya da yapı nesnesiyse zincirdeki Copies() adımı ve
// liste ya da metin dilimi (yazımdaki aralık ya da çalışırken gelen aralık) hatadır, denetimle aynı
// kodla (H2740, H2741, H2743; checker.ContainerWrite). Alan hedefi ve yerinde değiştirici metodun
// (.TR_append) alıcısı yalnız bu kurala girer.

// writeStmt yerinde yazan deyimin ilk ve son parçasıdır (iletinin önerisindeki deyim): atamada hedef
// ve değer, çoklu atamada ilk hedef ve son değer, çağrıda çağrının kendisi.
type writeStmt struct{ first, last ast.Node }

// copyWrite bir hedefin derlenmiş zinciridir.
type copyWrite struct {
	chain  []ast.WriteStep
	stmt   writeStmt
	k      *compilerScope
	call   bool                 // TR_write_slice: hedefin tensör olduğunu imza söyler
	field  bool                 // alan hedefi ya da yerinde değiştiricinin alıcısı: yalnız kap kuralı
	accept func(Value) bool     // kap kuralının değerleri: alanda yapı, alıcıda liste ve sözlük; nil hepsi
	above  *ast.IndexExpression // en dıştaki indeks (yazmanın kendisi): dilimin üstündeki indeks (H2743)
}

// beginCopyWrite base'in zincirindeki indeks adımlarını copyStepIndex için işaretler; zincirde kopya
// verebilecek adım yoksa nil. Zincir derlendikten sonra end çağrılır.
func (c *compiler) beginCopyWrite(k *compilerScope, base ast.Expression, stmt writeStmt, call bool) *copyWrite {
	w := &copyWrite{chain: ast.WriteChain(base), stmt: stmt, k: k, call: call}
	risky := false
	for i, s := range w.chain {
		_, slice := ast.SliceIndex(s)
		risky = risky || s.Copies() || slice
		if ix, ok := s.Expr.(*ast.IndexExpression); ok && !ast.LiteralIndex(s.Index) && i < 64 {
			if c.copySteps == nil {
				c.copySteps = map[*ast.IndexExpression]uint64{}
			}
			c.copySteps[ix] = 1 << uint(i)
			risky = true
		}
	}
	if !risky {
		return nil
	}
	return w
}

// fieldWrite w'yi yalnız kap kuralına bağlar (alan hedefi, yerinde değiştiricinin alıcısı); accept
// kuralın değerleridir. w nil olabilir.
func (w *copyWrite) fieldWrite(accept func(Value) bool) *copyWrite {
	if w != nil {
		w.field, w.accept = true, accept
	}
	return w
}

// withAbove en dıştaki indeksi kaydeder (indeks hedefi); w nil olabilir.
func (w *copyWrite) withAbove(ix *ast.IndexExpression) *copyWrite {
	if w != nil {
		w.above = ix
	}
	return w
}

// end zincirin kodunu sarar: değer hesaplanınca kopya adımı varsa hata verir.
func (w *copyWrite) end(c *compiler, code exprCode) exprCode {
	if w == nil {
		return code
	}
	for _, s := range w.chain {
		if ix, ok := s.Expr.(*ast.IndexExpression); ok {
			delete(c.copySteps, ix)
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		saved, savedRanges := v.copyIndexSteps, v.copyRangeSteps
		v.copyIndexSteps, v.copyRangeSteps = 0, 0
		val, err := code(v, env)
		mask, ranges := v.copyIndexSteps, v.copyRangeSteps
		v.copyIndexSteps, v.copyRangeSteps = saved, savedRanges
		if err != nil {
			return val, err
		}
		if _, tensor := val.(IndexAssignable); (tensor || w.call) && !w.field {
			j, clean := ast.NearestCopy(w.chain, func(i int) bool { return i < 64 && mask&(1<<uint(i)) != 0 })
			if j < 0 {
				return val, nil
			}
			return nil, w.fault(v, j, clean)
		}
		if w.call || !containerValue(val) || w.accept != nil && !w.accept(val) {
			return val, nil // kural tensörün ve kabın yazmasıdır
		}
		j, clean := ast.NearestCopy(w.chain, func(i int) bool {
			_, slice := ast.SliceIndex(w.chain[i])
			return slice || i < 64 && ranges&(1<<uint(i)) != 0
		})
		if j < 0 {
			return val, nil
		}
		return nil, w.containerFault(v, j, clean)
	}
}

// structValue değer yapı nesnesi mi (alan hedefi); listOrDict liste ya da sözlük mü (yerinde değiştiricinin
// alıcısı; başka alıcıda, TR_Logger'da, metot yazma değildir).
func structValue(val Value) bool { _, ok := val.(*StructInstanceVal); return ok }

func listOrDict(val Value) bool {
	switch val.(type) {
	case *ListVal, *DictVal:
		return true
	}
	return false
}

// containerValue yazılan değer liste, sözlük ya da yapı nesnesi mi (5.4).
func containerValue(val Value) bool {
	switch val.(type) {
	case *ListVal, *DictVal, *StructInstanceVal:
		return true
	}
	return false
}

// copyStepIndex işaretli indeks adımının indeksini sarar: indeks tensörse adım tensörle indekstir
// (kap liste ya da sözlükse indeksleme kendi hatasını verir); aralıksa liste ya da metin tabanında
// dilimdir (5.4; tensörde görünüm, end tensörün kuralında ona bakmaz).
func (c *compiler) copyStepIndex(e *ast.IndexExpression, index exprCode) exprCode {
	bit, ok := c.copySteps[e]
	if !ok {
		return index
	}
	return func(v *VM, env *Environment) (Value, error) {
		val, err := index(v, env)
		switch val.(type) {
		case IndexAssignable:
			v.copyIndexSteps |= bit
		case *RangeVal:
			v.copyRangeSteps |= bit
		}
		return val, err
	}
}

// inPlaceReceiver metot çağrısının alıcısının kodudur. Yerinde değiştirici metodun ("in_place",
// .TR_append) alıcısı yazmanın hedefidir: değeri liste ya da sözlükse zinciri kap kuralına girer (5.4).
func (c *compiler) inPlaceReceiver(k *compilerScope, e *ast.CallExpression, mem *ast.MemberExpression) exprCode {
	var w *copyWrite
	if !mem.Safe && codes.IsInPlace(mem.NameToken.Code) && len(e.Arguments) > 0 {
		w = c.beginCopyWrite(k, mem.Object, writeStmt{e, e}, false).fieldWrite(listOrDict)
	}
	return w.end(c, c.expression(k, mem.Object))
}

// writeSliceTarget TR_write_slice çağrısının hedefidir; çağrı o değilse nil.
func writeSliceTarget(e *ast.CallExpression) ast.Expression {
	if id, ok := e.Function.(*ast.Identifier); ok && id.Token.Code == codes.FnWriteSlice && len(e.Arguments) > 0 {
		return e.Arguments[0]
	}
	return nil
}

// fault kopyaya yazmanın hatasıdır; öneri çalışan dosyanın metninden kurulur.
func (w *copyWrite) fault(v *VM, j int, clean bool) error {
	step := w.chain[j]
	t, ok := ast.NewCopyWriteText(sourceLines(v), w.stmt.first, w.stmt.last, step, w.taken)
	if !ok {
		t = ast.CopyWriteText{Step: "...", Write: "...", Copy: "c", K: "k", I: "i"}
	}
	r := ast.RangeOf(step.Expr)
	return messages.At(r.Line, r.Col, checker.CopyWriteError(step, clean, t))
}

// containerFault kapta kopyaya yazmanın hatasıdır (5.4). Tipler bilinmediği için dilimin üstündeki
// indeks, aralık ya da metin olmayan her indekstir; kabın öğesini veren üye adıyla tanınır.
func (w *copyWrite) containerFault(v *VM, j int, clean bool) error {
	step, above := w.chain[j], w.above
	if j > 0 {
		above, _ = w.chain[j-1].Expr.(*ast.IndexExpression)
	}
	_, _, k := ast.ElementAccessor(step)
	cw := checker.ContainerWrite{Step: step, Above: above, Clean: clean,
		IntAbove: above != nil && ast.MaybeInteger(above.Index), Accessor: k != codes.None}
	r := ast.RangeOf(step.Expr)
	return messages.At(r.Line, r.Col, cw.Error(sourceLines(v), w.stmt.first, w.stmt.last, w.taken, "..."))
}

// taken önerinin adı kapsamda tanımlı mı.
func (w *copyWrite) taken(name string) bool { return w.k.resolve(name).kind != resolveDynamic }

// sourceLines çalışan dosyanın satırlarıdır (önerinin yazımı).
func sourceLines(v *VM) []string {
	code := v.SourceCode
	if v.source != nil {
		code = v.source.Code
	}
	return strings.Split(strings.ReplaceAll(code, "\r\n", "\n"), "\n")
}
