package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Bu dosya kardeş birim dizilerini tek yaprağa çevirir. Bildirim ve kayıt listeleri (bir `const ( ... )`
// bloğunun öğeleri, art arda `kaydet(TR_fn_embedding, yeni(x))` çağrıları, bir çekirdeğin `x := k.pointer("x")`
// parametre bildirimleri, art arda tek ifadeli test fonksiyonları) tekrar değildir: sıra bilgidir, birimler
// küçüktür ve her biri başka bir şey söyler. Ölçüt: aynı üst listede (dosya bildirimleri, bildirim bloğu
// öğeleri, ifade listesi, case gövdesi) art arda gelen, en az siblingRunMin tane, hepsi aynı biçimli ve her biri
// siblingMaxNodes düğümden küçük birimler. Tablo kuralının (tableElements) genellemesidir: o, elemanları
// bileşik sabitin içinde olan tabloyu, bu, elemanları bildirim ya da deyim olan tabloyu yaprak yapar.

const (
	siblingRunMin   = 3  // en az bu kadar ardışık, aynı biçimli kardeş
	siblingMaxNodes = 40 // her kardeş bundan küçük (düğüm sayısı)
)

// sibRun bir kardeş dizisinin üyesidir: head ilk üyedir (yaprak olur), öbürleri atlanır; endLine dizinin bittiği satır.
type sibRun struct {
	head    bool
	endLine int32
}

// siblingRuns dosyadaki kardeş dizilerinin üyelerini işaretler.
func siblingRuns(fset *token.FileSet, f *ast.File) map[ast.Node]sibRun {
	runs := map[ast.Node]sibRun{}
	mark := func(list []ast.Node) {
		start := 0
		for i := 1; i <= len(list); i++ {
			if i < len(list) && sameSmallShape(list[start], list[i]) {
				continue
			}
			if i-start >= siblingRunMin && smallUnit(list[start]) {
				end := int32(fset.Position(list[i-1].End()).Line)
				for k := start; k < i; k++ {
					runs[list[k]] = sibRun{head: k == start, endLine: end}
				}
			}
			start = i
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.File:
			mark(declNodes(n.Decls))
		case *ast.GenDecl:
			var list []ast.Node
			for _, s := range n.Specs {
				list = append(list, s)
			}
			mark(list)
		case *ast.BlockStmt:
			mark(stmtNodes(n.List))
		case *ast.CaseClause:
			mark(stmtNodes(n.Body))
		case *ast.CommClause:
			mark(stmtNodes(n.Body))
		}
		return true
	})
	return runs
}

func declNodes(ds []ast.Decl) []ast.Node {
	out := make([]ast.Node, len(ds))
	for i, d := range ds {
		out[i] = d
	}
	return out
}

func stmtNodes(ss []ast.Stmt) []ast.Node {
	out := make([]ast.Node, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// smallUnit birim kardeş olabilecek kadar küçük mü; içe aktarma bildirimi kardeş sayılmaz.
func smallUnit(n ast.Node) bool {
	if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
		return false
	}
	size := 0
	ast.Inspect(n, func(ast.Node) bool { size++; return size < siblingMaxNodes })
	return size < siblingMaxNodes
}

// sameSmallShape iki birimin düğüm türü dizisi aynı mı ve ikisi de küçük mü.
func sameSmallShape(a, b ast.Node) bool {
	return smallUnit(a) && smallUnit(b) && shapeKey(a) == shapeKey(b)
}

// shapeKey birimin önek sırasındaki düğüm türleridir (adlar ve değerler hariç).
func shapeKey(n ast.Node) string {
	var b strings.Builder
	ast.Inspect(n, func(x ast.Node) bool {
		if x != nil {
			b.WriteString(nodeKind(x))
			b.WriteByte(';')
		}
		return true
	})
	return b.String()
}
