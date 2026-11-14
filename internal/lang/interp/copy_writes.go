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

// writeStmt yerinde yazan deyimin ilk ve son parçasıdır (iletinin önerisindeki deyim): atamada hedef
// ve değer, çoklu atamada ilk hedef ve son değer, çağrıda çağrının kendisi.
type writeStmt struct{ first, last ast.Node }

// copyWrite bir hedefin derlenmiş zinciridir.
type copyWrite struct {
	chain []ast.WriteStep
	stmt  writeStmt
	k     *compilerScope
	call  bool // TR_write_slice: hedefin tensör olduğunu imza söyler
}

// beginCopyWrite base'in zincirindeki indeks adımlarını copyStepIndex için işaretler; zincirde kopya
// verebilecek adım yoksa nil. Zincir derlendikten sonra end çağrılır.
func (c *compiler) beginCopyWrite(k *compilerScope, base ast.Expression, stmt writeStmt, call bool) *copyWrite {
	w := &copyWrite{chain: ast.WriteChain(base), stmt: stmt, k: k, call: call}
	risky := false
	for i, s := range w.chain {
		risky = risky || s.Copies()
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
		saved := v.copyIndexSteps
		v.copyIndexSteps = 0
		val, err := code(v, env)
		mask := v.copyIndexSteps
		v.copyIndexSteps = saved
		if err != nil {
			return val, err
		}
		if _, tensor := val.(IndexAssignable); !tensor && !w.call {
			return val, nil // hedef tensör değil: kural tensörün yazmasıdır
		}
		j, clean := ast.NearestCopy(w.chain, func(i int) bool { return i < 64 && mask&(1<<uint(i)) != 0 })
		if j < 0 {
			return val, nil
		}
		return nil, w.fault(v, j, clean)
	}
}

// copyStepIndex işaretli indeks adımının indeksini sarar: indeks tensörse adım tensörle indekstir
// (kap liste ya da sözlükse indeksleme kendi hatasını verir).
func (c *compiler) copyStepIndex(e *ast.IndexExpression, index exprCode) exprCode {
	bit, ok := c.copySteps[e]
	if !ok {
		return index
	}
	return func(v *VM, env *Environment) (Value, error) {
		val, err := index(v, env)
		if _, tensor := val.(IndexAssignable); tensor {
			v.copyIndexSteps |= bit
		}
		return val, err
	}
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
	code := v.SourceCode
	if v.source != nil {
		code = v.source.Code
	}
	lines := strings.Split(strings.ReplaceAll(code, "\r\n", "\n"), "\n")
	taken := func(name string) bool { return w.k.resolve(name).kind != resolveDynamic }
	step := w.chain[j]
	t, ok := ast.NewCopyWriteText(lines, w.stmt.first, w.stmt.last, step, taken)
	if !ok {
		t = ast.CopyWriteText{Step: "...", Write: "...", Copy: "c", K: "k", I: "i"}
	}
	r := ast.RangeOf(step.Expr)
	return messages.At(r.Line, r.Col, checker.CopyWriteError(step, clean, t))
}
