package interp

import (
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
}

func newTask(line int) *TaskVal { return &TaskVal{line: line, done: make(chan struct{})} }

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
	return nil, messages.New(messages.H2428, ValTask, name, "")
}

// wait waits until the task stops, with the interpreter lock released, and gives its result or its
// error. A stop applied to the program abandons the wait (1.5).
func (t *TaskVal) wait(y *VM) (Value, error) {
	if !t.stopped() {
		if y.task == t {
			// Görev kendi durmasını bekleyemez: sonsuza dek kilitlenirdi.
			return nil, messages.New(messages.H2918, CallPrefix(codes.MemWait), t.line)
		}
		if y.exclusive != nil {
			return nil, messages.New(messages.H2522, CallPrefix(codes.MemWait), y.exclusive)
		}
		if err := y.awaitReady(nil, t.done, false); err != nil {
			return nil, err
		}
	}
	return t.result, t.err
}

// Compile-time check: the interpreter finds the members by type assertion.
var _ PropertyGetter = (*TaskVal)(nil)
