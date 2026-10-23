package codeindex

import (
	"reflect"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// Bu dosya her bildirimin kullandığı dil yapılarını kaydeder (deneme 9 H1, deneme 10; kod-grafigi-40 tasarımı,
// TCB onaylı): çağrı, indeks, üye erişimi, metin yerleştirmesi, döngü ... Dizin TR'dedir ve Go'yu bilmez; yapının
// hangi Go işlevlerince çalıştırıldığını kod grafiği yorumlayıcının gönderim tablosundan bulur ve bağlar.

// Construct bir bildirimin kullandığı bir dil yapısıdır; aynı (bildirim, tür) bir kez, ilk yeriyle.
type Construct struct {
	In   string `json:"in"`
	Kind string `json:"kind"` // AST türünün adı: "CallExpression", "InterpolatedString", ...
	Pos  Pos    `json:"pos"`
}

// recordedKinds çalışma anında iş yapan yapılardır (ad, değişmez değer ve blok kaydedilmez: her bildirimde vardır,
// gönderimleri bir değer okumaktır).
var recordedKinds = map[string]bool{
	"CallExpression": true, "IndexExpression": true, "MemberExpression": true, "InterpolatedString": true,
	"ForInStatement": true, "WhileStatement": true, "ListComprehension": true, "TryCatchStatement": true,
	"DeferStatement": true, "MatchExpression": true, "RangeExpression": true, "AsyncStatement": true,
	"ThrowStatement": true, "MultiAssignStatement": true, "ListLiteral": true, "DictLiteral": true,
}

// recordConstruct düğümün AST türünü sahibine kaydeder.
func (x *indexer) recordConstruct(f *file, o owner, n any) {
	node, ok := n.(ast.Node)
	if !ok {
		return
	}
	t := reflect.TypeOf(n)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if !recordedKinds[t.Name()] {
		return
	}
	c := Construct{In: o.id, Kind: t.Name(), Pos: Pos{File: f.rel, Line: node.Line()}}
	key := c.In + "|" + c.Kind
	if x.constructSeen[key] {
		return
	}
	x.constructSeen[key] = true
	x.constructs = append(x.constructs, c)
}
