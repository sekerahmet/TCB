package codeindex

import (
	"reflect"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Bu dosya her bildirimin kullandığı dil yapılarını kaydeder (deneme 9 H1, deneme 10; kod-grafigi-40 tasarımı,
// TCB onaylı): AST türü (çağrı, işleç, indeks, metin yerleştirmesi, döngü ...), işlecin metni ve sol işlenenin
// çıkarılan türü. Dizin TR'dedir ve Go'yu bilmez; yapının hangi Go işlevlerince çalıştırıldığını kod grafiği
// yorumlayıcının gönderim tablosundan bulur ve bağlar.

// Construct bir bildirimin kullandığı bir dil yapısıdır; aynı (bildirim, tür, işleç, işlenen) bir kez, ilk yeriyle.
type Construct struct {
	In      string `json:"in"`
	Kind    string `json:"kind"`              // AST türünün adı: "InfixExpression", "CallExpression", ...
	Op      string `json:"op,omitempty"`      // işlecin metni ("@", "+", "==")
	Operand string `json:"operand,omitempty"` // sol işlenenin türü: yapı kimliği ya da "type:TR_type_Tensor"; bilinmezse boş
	Pos     Pos    `json:"pos"`
}

// recordedKinds çalışma anında iş yapan yapılardır (ad, değişmez değer ve blok kaydedilmez: her bildirimde vardır,
// gönderimleri bir değer okumaktır).
var recordedKinds = map[string]bool{
	"CallExpression": true, "IndexExpression": true, "MemberExpression": true, "InterpolatedString": true,
	"ForInStatement": true, "WhileStatement": true, "ListComprehension": true, "TryCatchStatement": true,
	"DeferStatement": true, "MatchExpression": true, "RangeExpression": true, "AsyncStatement": true,
	"ThrowStatement": true, "MultiAssignStatement": true, "ListLiteral": true, "DictLiteral": true,
}

// typeCode bir tür kodunun işlenen türü olarak yazılışıdır.
func typeCode(k codes.Code) string { return "type:" + k.String() }

// literalType değişmez değerin ve yerleşik çağrısının türüdür (tanımdaki `dönüş` tek türse).
func (x *indexer) literalType(sc *scope, e ast.Expression) string {
	switch e := e.(type) {
	case *ast.IntegerLiteral:
		return typeCode(codes.TypeInt)
	case *ast.FloatLiteral:
		return typeCode(codes.TypeFloat)
	case *ast.StringLiteral, *ast.InterpolatedString:
		return typeCode(codes.TypeString)
	case *ast.BooleanLiteral:
		return typeCode(codes.TypeBool)
	case *ast.ListLiteral:
		return typeCode(codes.TypeList)
	case *ast.DictLiteral:
		return typeCode(codes.TypeDict)
	case *ast.CallExpression:
		fn, ok := e.Function.(*ast.Identifier)
		if !ok || sc.locals[fn.Value] {
			return ""
		}
		if t, ok := x.lang.Find(codes.NamespaceType, fn.Value); ok {
			return typeCode(t) // tip çağrısı tipin değerini kurar (3.12)
		}
		k, ok := x.lang.Find(codes.NamespaceGlobal, fn.Value)
		if !ok {
			return ""
		}
		if def, ok := codes.DefinitionOf(k); ok && len(def.Signature.Return.Types) == 1 {
			return typeCode(def.Signature.Return.Types[0])
		}
	}
	return ""
}

// recordConstruct düğümün AST türünü (ve işleçse işlecini, sol işlenenin türünü) sahibine kaydeder.
func (x *indexer) recordConstruct(f *file, o owner, sc *scope, n any) {
	var c Construct
	switch n := n.(type) {
	case *ast.InfixExpression:
		c = Construct{Kind: "InfixExpression", Op: string(n.Operator), Operand: x.operandType(f, sc, n.Left), Pos: tokenPos(f, n.Token.Line, n.Token.Column, string(n.Operator))}
	case *ast.PrefixExpression:
		c = Construct{Kind: "PrefixExpression", Op: string(n.Operator), Operand: x.operandType(f, sc, n.Right), Pos: tokenPos(f, n.Token.Line, n.Token.Column, string(n.Operator))}
	case ast.Node:
		t := reflect.TypeOf(n)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if !recordedKinds[t.Name()] {
			return
		}
		c = Construct{Kind: t.Name(), Pos: Pos{File: f.rel, Line: n.Line()}}
	default:
		return
	}
	c.In = o.id
	key := c.In + "|" + c.Kind + "|" + c.Op + "|" + c.Operand
	if x.constructSeen[key] {
		return
	}
	x.constructSeen[key] = true
	x.constructs = append(x.constructs, c)
}

// operandType işlenenin türüdür: çıkarılan yapı türü, değişmez değer ya da yerleşik dönüşü.
func (x *indexer) operandType(f *file, sc *scope, e ast.Expression) string {
	if t, _ := x.exprType(f, sc, e); t != "" {
		return t
	}
	return x.literalType(sc, e)
}
