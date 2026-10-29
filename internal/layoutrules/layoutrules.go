// Package layoutrules, kod düzeni standardının (docs/development.md) boyut sınırlarının tek kaynağıdır: el yazımı bir
// dosyanın ve test dışı bir fonksiyonun en çok satırı, bugünkü istisnalar ve istisnanın düzeltme payı. Düzen testleri
// (internal/layout_test.go) sınırı bunlarla denetler; kod grafiği (tools/codegraph) düzenleme anında aynı sınırı
// gösterir. Sınır bir yerde değişir, iki okuyucusu aynı sayıyı görür.
//
// Layer: layout rules (internal/layoutrules); yalnızca Go standart kitaplığını içe aktarır, düzen testleri ve
// tools/ kullanır.
package layoutrules

import (
	"bytes"
	"go/ast"
	"go/token"
	"strings"
)

const (
	FileLimit = 600 // el yazımı bir dosyanın en fazla satırı
	FuncLimit = 80  // test dışı bir fonksiyonun en fazla satırı (func satırından kapanışa)
)

// FixMargin bir istisnanın tavanını aşmadan alabileceği küçük büyümedir (tavanın %10'u,
// en az 5 satır): hata düzeltmesi gibi küçük değişiklikler bölmeyi zorunlu kılmasın; kalıcı
// büyüme yine bölmeyi gerektirir.
func FixMargin(ceiling int) int {
	if p := ceiling / 10; p > 5 {
		return p
	}
	return 5
}

// FileLines düzenin saydığı dosya boyudur: satır sonu sayısı.
func FileLines(data []byte) int {
	return bytes.Count(data, []byte("\n"))
}

// FuncLines düzenin saydığı fonksiyon boyudur: func satırından kapanış parantezine.
func FuncLines(fset *token.FileSet, fd *ast.FuncDecl) int {
	return fset.Position(fd.End()).Line - fset.Position(fd.Pos()).Line + 1
}

// FuncName fonksiyonun istisna anahtarındaki adıdır ("dosya:Alici.Ad"'ın "Alici.Ad"ı; alıcısız işlevde yalnız ad).
func FuncName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	kind := fd.Recv.List[0].Type
	if y, ok := kind.(*ast.StarExpr); ok {
		kind = y.X
	}
	if g, ok := kind.(*ast.IndexExpr); ok {
		kind = g.X
	}
	if id, ok := kind.(*ast.Ident); ok {
		return id.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// FileMax dosyanın bugün aşamayacağı boydur: istisna listesinde değilse FileLimit, listedeyse tavan + düzeltme payı.
func FileMax(path string) (max int, exception bool) {
	return limitOf(BigFiles, path, FileLimit)
}

// FuncMax fonksiyonun ("dosya:Alici.Ad") bugün aşamayacağı boydur: FuncLimit ya da istisnanın tavanı + payı.
func FuncMax(key string) (max int, exception bool) {
	return limitOf(LongFuncs, key, FuncLimit)
}

func limitOf(ceilings map[string]int, key string, limit int) (int, bool) {
	if c, ok := ceilings[key]; ok {
		return c + FixMargin(c), true
	}
	return limit, false
}

// skippedDirs düzenin ölçmediği klasörlerdir: Go paketi olmayan ya da depoya ait olmayan yerler.
var skippedDirs = map[string]bool{".git": true, ".claude": true, "bin": true, "scratch": true, "testdata": true, "node_modules": true}

// SkippedDir düzen bu adlı klasörün altını ölçmüyor mu: skippedDirs, ya da '.' veya '_' ile başlayan ad (Go'nun kuralı).
func SkippedDir(name string) bool {
	return skippedDirs[name] || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}
