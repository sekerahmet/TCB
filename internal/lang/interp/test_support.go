package interp

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// registerTestBuiltins test yazmak için doğrulama yerleşiklerini kaydeder.
// İkisi de test blokları dışında da çalışır (bir tür "assert").
func (v *VM) registerTestBuiltins() {
	// doğrula(koşul, [mesaj]): koşul yanlışsa hata fırlatır.
	v.RegisterNoNamed(codes.G700, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, ArgCountError(codes.G700, "1 ya da 2", len(args), "koşul, [mesaj]")
		}
		if args[0].IsTruthy() {
			return TRUE, nil
		}
		if len(args) == 2 {
			return nil, fmt.Errorf("doğrulama başarısız: %s", args[1].Inspect())
		}
		return nil, fmt.Errorf("doğrulama başarısız: koşul yanlış (%s)", testShow(args[0]))
	})

	// eşit_mi(beklenen, gerçek, [mesaj]): iki değer '==' ile eşit değilse ikisini de gösteren hata fırlatır.
	v.RegisterNoNamed(codes.G701, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, ArgCountError(codes.G701, "2 ya da 3", len(args), "beklenen, gerçek, [mesaj]")
		}
		if err := typeTextError("==", args[0], args[1]); err != nil {
			return nil, err
		}
		if ValuesEqual(args[0], args[1]) {
			return TRUE, nil
		}
		extra := ""
		if len(args) == 3 {
			extra = args[2].Inspect() + " — "
		}
		return nil, fmt.Errorf("eşitlik başarısız: %sbeklenen %s, gerçek %s", extra, testShow(args[0]), testShow(args[1]))
	})
}

// testShow bir değeri tipiyle birlikte gösterir; metinler tırnak içinde yazılır ki
// "5" ile 5 ayırt edilebilsin.
func testShow(d Value) string {
	if s, ok := d.(*StringVal); ok {
		return fmt.Sprintf("%q (%s)", s.Val, VAL_STRING)
	}
	return fmt.Sprintf("%s (%s)", d.Inspect(), d.Type())
}

// RunTest runs a single test block in a new scope opened over the file's top-level definitions
// (GlobalEnv). 'ertele' blocks in the body run when the test ends and the 'eşzamanlı' tasks the
// test started are awaited. If it returns an error (assertions included) the test failed. The
// program must already be loaded with Run.
func (v *VM) RunTest(t *ast.TestStatement) (err error) {
	if !v.holdsGIL {
		v.acquireGIL()
		defer v.releaseGIL()
	}
	stackSize := len(v.CallStack)
	defer func() {
		if r := recover(); r != nil {
			// Çöküşte yarıda kalan çağrıların çerçevelerini at (sonraki testler için)
			v.trimStack(stackSize)
			if IsInterrupt(r) {
				err = ErrInterrupted
				return
			}
			err = fmt.Errorf("Satır %d: İç hata (çökme önlendi): %v", t.Line(), r)
		}
	}()
	var stmts []ast.Statement
	if t.Body != nil {
		stmts = t.Body.Statements
	}
	kd, info := v.compileUnderGlobal(stmts)
	env := newEnv(v.GlobalEnv, info)
	env.IsFuncRoot = true
	func() {
		// Testin ertelenen işleri test bitince çalışır; hataları testin hatasına eklenir (9.4)
		defer func() { err = addDeferError(err, env.RunDefers()) }()
		if t.Body != nil {
			_, _, err = kd.run(v, env)
		}
	}()
	v.Blocking(v.wg.Wait)
	return err
}
