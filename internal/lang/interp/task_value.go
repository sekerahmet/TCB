package interp

import (
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- GÖREV DEĞERİ (10.1) ---
//
// TR_async f(x) ve TR_async { ... } görevi başlatır ve görev değerini verir. g.TR_wait() görev
// durana kadar yorumlayıcı kilidini bırakarak bekler, sonucunu verir ya da hatasını fırlatır (10.6);
// g.TR_done() görevin durup durmadığını söyler, beklemez. Görev, ertelenen işleri de bitince durur.

// ValTask is the type name of TaskVal.
var ValTask = TypeName(codes.TypeTask)

// TaskVal is a task's value (10.1). result and err are written once, before done closes; after that
// they are only read.
type TaskVal struct {
	line   int           // the line the task was started on
	done   chan struct{} // closes when the task has stopped
	result Value         // the task's result: the function's return, none for a block
	err    error         // the task's error; nil when it ended without one
	// held says the task's value was kept (TR_async as an expression): its error waits for a
	// waiter and is written only if nobody waited for it (10.6).
	held bool
	// waited says a .TR_wait() or TR_wait_all(görevler) got the task's end: its error was thrown
	// there, so it is not written again (10.6).
	waited atomic.Bool
}

func newTask(line int, held bool) *TaskVal {
	return &TaskVal{line: line, done: make(chan struct{}), held: held}
}

func (t *TaskVal) Type() ValueType { return ValTask }
func (t *TaskVal) IsTruthy() bool  { return true }

// Inspect is the task's text (3.14): its type's name, the same for every reader.
func (t *TaskVal) Inspect() string { return codes.ValueNames().Name(codes.TypeTask) }

// stopped reports whether the task has stopped.
func (t *TaskVal) stopped() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

// finish records the task's end and wakes its waiters.
func (t *TaskVal) finish(result Value, err error) {
	if result == nil {
		result = Null
	}
	t.result, t.err = result, err
	close(t.done)
}

// GetProperty gives .TR_wait() and .TR_done().
func (t *TaskVal) GetProperty(kd codes.Code, name string) (Value, error) {
	switch kd {
	case codes.MemWait:
		return &BuiltinVal{Name: codes.Name(kd), NoNamedArgs: true, Fn: func(y *VM, args []Value, _ map[string]Value) (Value, error) {
			if len(args) != 0 {
				return nil, ArgCountError(kd, 0, len(args))
			}
			return t.wait(y)
		}}, nil
	case codes.MemDone:
		return &BuiltinVal{Name: codes.Name(kd), NoNamedArgs: true, Fn: func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
			if len(args) != 0 {
				return nil, ArgCountError(kd, 0, len(args))
			}
			return BoolToVal(t.stopped()), nil
		}}, nil
	}
	return nil, messages.New(messages.H2474, ValTask, name, "") // denetimdeki hatanın kendisi (13.11)
}

// wait waits until the task stops and gives its result or its error.
func (t *TaskVal) wait(y *VM) (Value, error) {
	if err := t.await(y, codes.MemWait); err != nil {
		return nil, err
	}
	return t.result, t.err
}

// await waits until the task stops, with the interpreter lock released; fn is the waiting builtin
// (for its errors). A stop applied to the program abandons the wait (1.5). Once it returns nil the
// task counts as waited: its error reaches the waiter (10.6).
func (t *TaskVal) await(y *VM, fn codes.Code) error {
	if !t.stopped() {
		if y.task == t {
			// Görev kendi durmasını bekleyemez: sonsuza dek kilitlenirdi.
			return messages.New(messages.H2918, CallPrefix(fn), t.line)
		}
		if y.exclusive != nil {
			return messages.New(messages.H2522, CallPrefix(fn), y.exclusive)
		}
		if err := y.awaitReady(nil, t.done, false); err != nil {
			return err
		}
	}
	t.waited.Store(true)
	return nil
}

// holdTaskError keeps the error of a held task until the program or the test ends (10.6).
func (v *VM) holdTaskError(t *TaskVal, err error) {
	v.taskErrorsMu.Lock()
	v.heldTasks = append(v.heldTasks, heldTaskError{t, err})
	v.taskErrorsMu.Unlock()
}

// reportUnwaitedTasks writes the errors of the held tasks nobody waited for, with the line each
// task started on, as reportAsyncError writes them (10.6): the program then ends with code 1, a
// test fails. A task waited for later (a TR_defer's g.TR_wait()) is not written.
func (v *VM) reportUnwaitedTasks() {
	v.taskErrorsMu.Lock()
	held := v.heldTasks
	v.heldTasks = nil
	v.taskErrorsMu.Unlock()
	for _, h := range held {
		if !h.task.waited.Load() {
			v.reportAsyncError(h.task.line, h.err)
		}
	}
}

// heldTaskError is a held task that ended with an error, and the error to write.
type heldTaskError struct {
	task *TaskVal
	err  error
}

// waitTasks is TR_wait_all(görevler) (10.5): it waits for every task of the list and gives their
// results in the list's order. When tasks ended with an error, all are still waited; the first one
// in the list's order is thrown and the others join its TR_suppressed in order (9.4), so none is
// lost. The finishing order does not change which error is thrown.
func waitTasks(y *VM, arg Value) (Value, error) {
	l, ok := arg.(*ListVal)
	if !ok {
		return nil, ArgTypeError(codes.FnWaitAll, 1, arg, codes.TypeList)
	}
	tasks := make([]*TaskVal, len(l.Elements))
	for i, e := range l.Elements {
		t, ok := e.(*TaskVal)
		if !ok {
			return nil, messages.New(messages.H2919, CallPrefix(codes.FnWaitAll), i, TypeAndSummary(e))
		}
		tasks[i] = t
	}
	results := make([]Value, len(tasks))
	var thrown error
	for i, t := range tasks {
		if err := t.await(y, codes.FnWaitAll); err != nil {
			return nil, err
		}
		results[i] = t.result
		thrown = addDeferError(thrown, t.err)
	}
	if thrown != nil {
		return nil, thrown
	}
	return &ListVal{Elements: results}, nil
}

// Compile-time check: the interpreter finds the members by type assertion.
var _ PropertyGetter = (*TaskVal)(nil)
