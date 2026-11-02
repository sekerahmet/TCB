package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Görevin tipi (10.1, 10.5): TR_async f(x) TR_Task[T] verir, T çağrının tipidir; blok
// (TR_async { ... }) TR_Task[TR_NoneType] verir. Çağrının tipi bilinmiyorsa (tipsiz fonksiyon,
// kitaplığın fonksiyonu) görev parametresiz TR_Task'tır: denetleyicinin iç tipidir, g.TR_wait()'in
// tipi bilinmez ("") ve TR_Any yazılmaz. TR_wait_all(görevler) TR_List[TR_Task[T]] için TR_List[T]
// verir; öğe tipi bilinmeyen listede TR_List.

// taskType görevi başlatan deyimin içini denetler ve görev değerinin tipini verir.
func (c *Checker) taskType(s *ast.AsyncStatement) string {
	if s.Body != nil {
		c.checkStatement(s.Body)
		return containerType(typeTask, []string{typeNull})
	}
	if t := c.inferExpression(s.Call); t != "" && t != typeAny && c.knownType(t) {
		return containerType(typeTask, []string{t})
	}
	return typeTask
}

// waitAllType TR_wait_all(görevler) çağrısının tipidir; çağrı o değilse "".
func (c *Checker) waitAllType(e *ast.CallExpression, k codes.Code) string {
	if k != codes.FnWaitAll || len(e.Arguments) != 1 || len(e.NamedArgs) != 0 {
		return ""
	}
	list := shapeOf(c.itemTypes[e.Arguments[0]])
	if list.base != typeList || list.optional {
		return ""
	}
	if len(list.args) == 1 {
		if task := shapeOf(list.args[0]); task.base == typeTask && !task.optional && len(task.args) == 1 {
			return listType(task.args[0])
		}
	}
	return typeList
}
