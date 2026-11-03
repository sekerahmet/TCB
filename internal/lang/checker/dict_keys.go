package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Sözlüğün anahtar tipi (3.7): anahtar tipi bilinen sözlüğü başka tipte anahtarla okumak ya da ona
// yazmak (d[1], d[2] = 3, .TR_get(5), .TR_contains(5)) denetim hatasıdır, H1140: çalışma anında okuma
// eksik anahtar hatası (H2647) ya da sessizce TR_none ve TR_false verir, yazma sözlüğe başka tipte
// anahtar ekler. Ondalık anahtar (değişmezde, indekste, TR_Dict[TR_Float, V] yazımında) çalışma
// anının koduyla H2646'dır (13.11'in istisnası). Tipi bilinmeyen ya da isteğe bağlı anahtar, öğe tipsiz
// sözlük ve fonksiyon karar vermez.

// checkDictKey key tipindeki anahtarın container'ın anahtarı olarak kullanılmasını denetler; hata
// at'in yerindedir.
func (c *Checker) checkDictKey(at ast.Node, container, key string) {
	s := shapeOf(container)
	if s.optional || normalizeTypeName(s.base) != typeDict || len(s.args) != 2 {
		return
	}
	if key == typeFloat {
		c.addErrorAt(at, messages.New(messages.H2646, c.typeArg(key)))
		return
	}
	if shapeOf(key).optional || !c.knownType(key) || !c.knownType(s.args[0]) || functionType(key) {
		return
	}
	if !c.assignable(s.args[0], key, nil) {
		c.addErrorAt(at, messages.New(messages.H1140, c.typeArg(key), c.typeArg(container), c.typeArg(s.args[0])))
	}
}

// checkDictKeyArg sözlüğün .TR_get(k) ve .TR_contains(k) argümanını anahtar olarak denetler.
func (c *Checker) checkDictKeyArg(e *ast.CallExpression, objType string, argTypes []string) {
	if len(argTypes) != 1 {
		return
	}
	switch c.builtinCallCode(e, objType) {
	case codes.MemGet, codes.MemContains:
		c.checkDictKey(e.Arguments[0], objType, argTypes[0])
	}
}

// checkLiteralKeys sözlük değişmezinin ondalık anahtarlarını denetler (H2646): {1.5: 1}.
func (c *Checker) checkLiteralKeys(keys []ast.Expression, types []string) {
	for i, t := range types {
		if t == typeFloat {
			c.addErrorAt(keys[i], messages.New(messages.H2646, c.typeArg(t)))
		}
	}
}
