package main

import (
	"go/ast"
	"go/types"
)

// Bir veri dosyasındaki bütün girdileri gezen test (`for _, k := range codes.Codes()`) her girdiyi korur; tek bir
// girdiyi arayan test (`codes.Name(k)`) etmez. Aralarındaki ayrım dosyadan teste giden zincirde değil, testin
// `range` deyiminde görünür: gezilen ifade, dosyayı yükleyen paketin içinde dosyaya dayanan bir koleksiyon (dilim,
// harita, dizi, yineleyici işlev) döndüren işlev ya da değişkendir. Bu dosya test dosyalarındaki `range` deyimlerini toplar,
// test_paths.go bunları gömülen dosyalara bağlar (`walks` kenarı: test → dosya).

// rangeRef bir range deyiminin gezdiği koleksiyonu üreten nesnelerdir.
type rangeRef struct {
	owner string   // deyimi içeren işlevin kimliği
	ids   []string // gezilen ifadedeki işlev/değişken düğümleri
	file  string
	line  int
}

// rangeUse range deyiminin gezdiği ifadedeki, koleksiyon döndüren modül nesnelerini kaydeder.
func (x *goExtractor) rangeUse(fc *fileCtx, owner string, rs *ast.RangeStmt) {
	if !fc.isTest { // kütüphanenin kendi döngüleri değil, testin döngüsü bütün listeyi gezmektir
		return
	}
	var ids []string
	ast.Inspect(rs.X, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := fc.u.Info.Uses[id]
		if ref := x.objectID(obj); ref != "" && yieldsCollection(obj) {
			ids = append(ids, ref)
		}
		return true
	})
	if len(ids) > 0 {
		x.idx.ranges = append(x.idx.ranges, rangeRef{owner: owner, ids: ids, file: fc.rel, line: x.line(rs.Pos())})
	}
}

// yieldsCollection nesne bir koleksiyon mu (değişken: tipi; işlev: sonuçlarından biri).
func yieldsCollection(obj types.Object) bool {
	switch o := obj.(type) {
	case *types.Var:
		return isCollection(o.Type())
	case *types.Func:
		res := o.Type().(*types.Signature).Results()
		for i := 0; i < res.Len(); i++ {
			if isCollection(res.At(i).Type()) {
				return true
			}
		}
	}
	return false
}

// isCollection tip dilim, dizi, harita ya da yineleyici işlev mi.
func isCollection(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Slice, *types.Array, *types.Map:
		return true
	case *types.Signature:
		return u.Params().Len() == 1 && u.Results().Len() == 0 // iter.Seq benzeri: func(yield func(...) bool)
	}
	return false
}

// walkersOf owner'dan (dosyayı yükleyen/okuyan işlev ya da gömen değişken) paketin içinde yukarı doğru çıkan her
// işlev ve değişkenin ürettiği koleksiyonu geziyor mu: öyleyse o range deyimleridir. Test dosyalarında tanımlı
// nesneler kapanışa girmez (test yardımcısı kütüphane kodu değildir).
func walkersOf(owner string, ranges []rangeRef, callers map[string][]string, pkgOf func(string) string, b *builder) []rangeRef {
	pkg := pkgOf(owner)
	closure := map[string]bool{owner: true}
	queue := []string{owner}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, c := range callers[id] {
			if n := b.nodes[c]; closure[c] || n == nil || pkgOf(c) != pkg || isTestFile(n.File) {
				continue
			}
			closure[c] = true
			queue = append(queue, c)
		}
	}
	var out []rangeRef
	for _, r := range ranges {
		for _, id := range r.ids {
			if closure[id] {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
