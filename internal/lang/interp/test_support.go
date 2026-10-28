package interp

import (
	"errors"
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// registerTestBuiltins test yazmak için doğrulama yerleşiklerini kaydeder.
// İkisi de test blokları dışında da çalışır (bir tür "assert").
func (v *VM) registerTestBuiltins() {
	// doğrula(koşul, [mesaj]): koşul yanlışsa hata fırlatır.
	v.RegisterNoNamed(codes.FnAssert, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, ArgCountError(codes.FnAssert, Counts(1, 2), len(args), messages.Text(messages.H2311))
		}
		b, ok := args[0].(*BoolVal)
		if !ok {
			// Koşul TR_Bool'dur (3.10): TR_assert(l) neyi sorduğunu söylemez.
			return nil, messages.New(messages.H2612, args[0].Type())
		}
		if b.Val {
			return True, nil
		}
		if m, ok := args[len(args)-1].(*MessageVal); ok && len(args) == 2 {
			return nil, m.Error() // katalog iletisi olduğu gibi, kendi koduyla
		}
		if len(args) == 2 {
			return nil, messages.New(messages.H2456, y.textOf(args[1])) // kullanıcının değeri dosyanın dilinde (3)
		}
		return nil, messages.New(messages.H2457, testShow(args[0]))
	})

	// eşit_mi(beklenen, gerçek, [mesaj]): iki değer '==' ile eşit değilse ikisini de gösteren hata fırlatır.
	v.RegisterNoNamed(codes.FnAssertEqual, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, ArgCountError(codes.FnAssertEqual, Counts(2, 3), len(args), messages.Text(messages.H2312))
		}
		if err := typeTextError("==", args[0], args[1]); err != nil {
			return nil, err
		}
		if ValuesEqual(args[0], args[1]) {
			return True, nil
		}
		if len(args) == 3 {
			return nil, messages.New(messages.H2517, y.textOf(args[2]), testShow(args[0]), testShow(args[1]))
		}
		return nil, messages.New(messages.H2458, testShow(args[0]), testShow(args[1]))
	})
}

// testShow bir değeri tipiyle birlikte gösterir; metinler tırnak içinde yazılır ki
// "5" ile 5 ayırt edilebilsin.
func testShow(d Value) string {
	if s, ok := d.(*StringVal); ok {
		return messages.Text(messages.H2459, s.Val, ValString)
	}
	return messages.Text(messages.H2424, Shown(d), d.Type())
}

// RunTest runs a single test block in a new scope opened over the file's top-level definitions
// (GlobalEnv). The 'eşzamanlı' tasks the test started are awaited, then (as at the program's top
// level) its 'ertele' blocks run. If it returns an error (assertions included) the test failed;
// an uncaught error in a task the test started fails the test too (10.1). The program must
// already be loaded with Run.
func (v *VM) RunTest(t *ast.TestStatement) (err error) {
	// Testin içinde TR_exit süreci bitirmez, testi düşürür (1.12).
	v.testing.Add(1)
	defer v.testing.Add(-1)
	var taskErrs []error
	v.taskErrorsMu.Lock()
	v.taskErrors = &taskErrs
	v.taskErrorsMu.Unlock()
	defer func() {
		v.taskErrorsMu.Lock()
		v.taskErrors = nil
		v.taskErrorsMu.Unlock()
		if err == nil && len(taskErrs) > 0 {
			err = errors.Join(taskErrs...)
		}
	}()
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
			err = messages.At(t.Line(), 0, messages.Internal(fmt.Sprint(r)))
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
		defer func() { err = addDeferError(err, v.runDefers(env)) }()
		if t.Body != nil {
			_, _, err = kd.run(v, env)
		}
		// Programın üst düzeyindeki gibi: 'ertele' blokları görevler bittikten sonra.
		if err == nil {
			v.Blocking(v.wg.Wait)
		}
	}()
	v.Blocking(v.wg.Wait)
	return err
}
