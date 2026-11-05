package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bayt okuyan ve yazan çağrılarda hassasiyetin cinsi (22.1, 22.3): değişmez `TR_dtype:` çağrının
// cinsini tutmuyorsa hata denetimdedir, kodu çalışma anındakidir (13.11). Değişkenle verilen ad
// çalışırken denetlenir.

// byteDTypeKinds hassasiyet adlarının cinsidir (15.1).
var byteDTypeKinds = map[string]codes.Code{
	"float32": codes.TypeFloat, "float64": codes.TypeFloat, "bf16": codes.TypeFloat, "fp16": codes.TypeFloat,
	"int64": codes.TypeInt, "int32": codes.TypeInt, "int16": codes.TypeInt, "int8": codes.TypeInt,
	"uint32": codes.TypeInt, "uint16": codes.TypeInt, "uint8": codes.TypeInt,
	"bool": codes.TypeBool,
}

// byteNumberCalls tek sayı okuyan ve yazan çağrılar: cinsi ve öbür cinsin çağrısı.
var byteNumberCalls = map[codes.Code][2]codes.Code{
	codes.MemReadInt:     {codes.TypeInt, codes.MemReadFloat},
	codes.MemReadFloat:   {codes.TypeFloat, codes.MemReadInt},
	codes.FnIntToBytes:   {codes.TypeInt, codes.FnFloatToBytes},
	codes.FnFloatToBytes: {codes.TypeFloat, codes.FnIntToBytes},
}

// checkByteDType çağrının değişmez hassasiyetini denetler.
func (c *Checker) checkByteDType(e *ast.CallExpression) {
	name, given := literalDType(e)
	if !given {
		return
	}
	k, known := byteDTypeKinds[name]
	if !known {
		return
	}
	if want, ok := fromBytesKind(e); ok {
		if k != want {
			c.addErrorAt(e, messages.New(messages.H6976, c.word(codes.FnFromBytes), name, c.word(k), c.word(want), "type_arg", codes.Spelling(codes.FnFromBytes), codes.Spelling(k)))
		}
		return
	}
	called, _ := c.calledBuiltin(e)
	call, ok := byteNumberCalls[called]
	if !ok || k == call[0] {
		return
	}
	if k == codes.TypeBool {
		c.addErrorAt(e, messages.New(messages.H7060, c.word(called)))
		return
	}
	c.addErrorAt(e, messages.New(messages.H7054, c.word(called), name, c.word(k), c.word(call[0]), codes.Spelling(call[1])))
}

// literalDType çağrının metin değişmezi olan TR_dtype: değeridir.
func literalDType(e *ast.CallExpression) (string, bool) {
	for _, a := range e.NamedArgs {
		if a.Token.Code == codes.ArgDtype {
			s, ok := a.Value.(*ast.StringLiteral)
			if ok {
				return s.Value, true
			}
		}
	}
	return "", false
}

// fromBytesKind `TR_from_bytes[E](...)` çağrısının E'sidir.
func fromBytesKind(e *ast.CallExpression) (codes.Code, bool) {
	ix, ok := e.Function.(*ast.IndexExpression)
	if !ok {
		return codes.None, false
	}
	f, ok := ix.Left.(*ast.Identifier)
	if !ok || f.Token.Code != codes.FnFromBytes {
		return codes.None, false
	}
	t, ok := ix.Index.(*ast.Identifier)
	if !ok {
		return codes.None, false
	}
	switch k := t.Token.Code; k {
	case codes.TypeFloat, codes.TypeInt, codes.TypeBool:
		return k, true
	}
	return codes.None, false // öğe cinsi olmayan E çalışma hatasıdır (H6971)
}
