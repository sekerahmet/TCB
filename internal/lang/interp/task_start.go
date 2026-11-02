package interp

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// taskStart 'eşzamanlı'yı derler: dönen kod görevi başlatır ve görev değerini verir (10.1). Gövde
// ya da çağrı kendi kök ortamında, ayrı bir görünümle (NewTaskVM) ayrı bir goroutine'de çalışır.
// Deyim biçimi değeri atar (concurrent), ifade biçimi verir.
func (c *compiler) taskStart(k *compilerScope, s *ast.AsyncStatement) func(v *VM, env *Environment) *TaskVal {
	sub := subScope(k)
	var bodyBlock *blockCode
	var call exprCode
	if s.Body != nil {
		sub.predefine(s.Body.Statements)
		bodyBlock = c.statements(sub, s.Body.Statements)
	} else if s.Call != nil {
		ast.ExprDefinedNames(s.Call, sub.define)
		call = c.expression(sub, s.Call)
	}
	info := sub.info
	line := s.Line()
	return func(v *VM, env *Environment) *TaskVal {
		v.wg.Add(1)
		asyncEnv := newEnv(env, info)
		asyncEnv.IsFuncRoot = true // görev içindeki 'ertele' blokları görev bitince çalışır
		task := v.NewTaskVM()
		t := newTask(line)
		task.task = t
		go func(e *Environment) {
			defer v.wg.Done()
			task.acquireGIL()
			defer task.releaseGIL()
			var result Value
			var err error
			defer func() {
				if r := recover(); r != nil {
					if !IsInterrupt(r) {
						err = messages.Internal(fmt.Sprint(r))
						task.reportAsyncError(line, err)
					} else {
						err = ErrInterrupted
					}
				}
				t.finish(result, err)
			}()
			defer func() {
				// Görevin ertelenen işleri görev bitince çalışır; hataları görev hatasına eklenir.
				// Durdurma ya da TR_exit yüzünden bitmek bir görev hatası değildir.
				err = addDeferError(err, task.runDefers(e))
				if reported := taskError(err); reported != nil {
					task.reportAsyncError(line, reported)
				}
			}()
			if bodyBlock != nil {
				_, _, err = bodyBlock.run(task, e) // bloğun değeri yoktur (10.1: TR_NoneType)
			} else if call != nil {
				result, err = call(task, e)
			}
		}(asyncEnv)
		return t
	}
}
