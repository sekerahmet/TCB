package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// checkProcessArguments TR_run_process ile TR_spawn_process'in argüman listesindeki yolu denetler (21.1): yol argümanı
// metin olarak yazılır. Kod çalışma anınınkidir (H7490). Yol çekirdek tip değildir, TR_Path(...) değeri
// tipsiz kalır (TestBuiltinReturnTypes): denetim yolu yazıldığı yerde görür (değişmezdeki TR_Path(...)
// çağrısı, tipi yazılı ad, öğe tipi yazılı liste); tipi bilinmeyen öğe çalışma anında yakalanır.
func (c *Checker) checkProcessArguments(e *ast.CallExpression, argTypes []string) {
	k, recv := c.calledBuiltin(e)
	if k != codes.FnRunProcess && k != codes.FnSpawnProcess || recv != nil || len(e.Arguments) < 2 {
		return
	}
	list := e.Arguments[1]
	if lit, ok := list.(*ast.ListLiteral); ok {
		for i, item := range lit.Elements {
			if isPathType(c.itemTypes[item]) || c.pathCall(item) {
				c.addPathItemError(k, item, i+1, item, "")
			}
		}
		return
	}
	if s := shapeOf(argTypes[1]); normalizeTypeName(s.base) == typeList && len(s.args) == 1 && isPathType(s.args[0]) {
		c.addPathItemError(k, list, 1, list, "[0]")
	}
}

// isPathType t'nin yol tipi olduğunu söyler.
func isPathType(t string) bool {
	return normalizeTypeName(requiredType(t)) == codes.Name(codes.TypePath)
}

// pathCall item'ın yol veren yerleşik çağrısı olduğunu söyler.
func (c *Checker) pathCall(item ast.Expression) bool {
	call, ok := item.(*ast.CallExpression)
	if !ok {
		return false
	}
	ret, single := SingleReturn(c.builtinCallCode(call, ""))
	return single && ret == codes.TypePath
}

// addPathItemError listedeki yolun hatasıdır; öneri öğenin kendi yazımını kullanır.
func (c *Checker) addPathItemError(k codes.Code, at ast.Expression, order int, item ast.Expression, suffix string) {
	text, ok := c.exprText(item)
	if !ok {
		text = codes.Spelling(codes.TypePath) + `("...")`
	}
	c.addErrorAt(at, messages.New(messages.H7490, codes.CallRef(k), order, text+suffix))
}
