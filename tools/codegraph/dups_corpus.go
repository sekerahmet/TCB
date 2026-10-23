package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// Bu dosya kod tekrarı aramasının girdisidir (dups ve similar komutları): Go dosyalarının
// sözdizimi ağaçları tek bir düğüm dizisine çevrilir. Her ağaç düğümü, önek sırasıyla, (tür,
// işleç, çocuk sayısı) üçlüsünden kurulan bir sayıdır; adlar ve değerler dizide yoktur (ayrı bir
// "atom" dizisinde durur, yalnızca fark özeti için). Çocuk sayısı dizide olduğu için önek sırası
// ağacı tek türlü çözer: iki dizi eşitse iki ağaç aynı biçimdedir.

// dupSkipDirs taranmayan klasörlerdir (sürüm denetimi, araç çıktıları, sınama verisi, kopya ağaçlar).
var dupSkipDirs = func() map[string]bool {
	m := map[string]bool{"vendor": true}
	for d := range skipDirs {
		m[d] = true
	}
	return m
}()

var generatedHeader = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// dupFunc bir işlev ya da metodun düğüm aralığıdır.
type dupFunc struct {
	name       string // "Ad" ya da "Tür.Ad"
	file       int
	from, to   int // düğüm dizisinde [from, to)
	line, last int // kaynak satırları
}

// dupCorpus bütün dosyaların düğüm dizisidir. Dosyalar arasında benzersiz ayraç değerleri durur,
// bu yüzden hiçbir eşleşme dosya sınırını geçmez.
type dupCorpus struct {
	seq    []uint32
	atom   []string // düğümün adı ya da değeri (Ident, BasicLit); öbürlerinde boş
	line   []int32
	last   []int32 // düğümün bittiği satır
	file   []int32
	files  []string
	isTest []bool // dosya başına
	funcs  []dupFunc
	kinds  map[string]uint32
	nodes  int // ayraçlar hariç düğüm sayısı
}

const sentinelBase = 1 << 31

// tableElements üretim kodunda bu kadar ya da daha çok öğesi olan bileşik sabit (ad tabloları, işleç
// kümeleri) veri sayılır, tek yaprak olur; test dosyalarında her bileşik sabit yapraktır.
const tableElements = 8

// loadCorpus root altındaki Go dosyalarını ayrıştırıp düğüm dizisine çevirir. Üretilmiş dosyalar
// ("Code generated ... DO NOT EDIT.") alınmaz; ayrıştırılamayan dosya atlanır ve adı skipped'e yazılır.
func loadCorpus(root string) (co *dupCorpus, skipped []string, err error) {
	co = &dupCorpus{kinds: map[string]uint32{}}
	rels := walkFiles(root, dupSkipDirs, ".go")
	fset := token.NewFileSet()
	for _, rel := range rels {
		src, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if rerr != nil {
			continue
		}
		if head := string(src[:min(len(src), 2000)]); generatedHeader.MatchString(head) {
			continue
		}
		f, perr := parser.ParseFile(fset, rel, src, 0)
		if perr != nil {
			skipped = append(skipped, rel)
			continue
		}
		co.addFile(fset, rel, f)
	}
	return co, skipped, nil
}

// flatNode ağaçtan çıkarılan düğümdür (çocuk sayısı geçişin sonunda bilinir).
type flatNode struct {
	kind     string
	atom     string
	line     int32
	last     int32
	children int
}

// openFrame ağaç gezisinde açık bir düğümdür; fn işlev bildirimidir.
type openFrame struct {
	idx int
	fn  bool
}

// localFunc bir dosyadaki işlevin yerel düğüm aralığıdır.
type localFunc struct {
	name     string
	from, to int
	line     int32
	last     int32
}

// flatten dosyanın ağacını önek sırasıyla düğümlere ve işlev aralıklarına çevirir. Tablolar (test
// dosyalarındaki bileşik sabitler, üretimde tableElements ve üstü öğeli olanlar) tek yaprak sayılır:
// tablo satırlarının birbirine benzemesi tekrar değildir. İçe aktarma bildirimleri alınmaz.
func flatten(fset *token.FileSet, f *ast.File, isTest bool) ([]flatNode, []localFunc) {
	var nodes []flatNode
	var funcs []localFunc
	var open []openFrame
	runs := siblingRuns(fset, f)
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			top := open[len(open)-1]
			open = open[:len(open)-1]
			if top.fn {
				funcs = append(funcs, localFunc{name: nodes[top.idx].atom, from: top.idx, to: len(nodes),
					line: nodes[top.idx].line, last: nodes[top.idx].last})
			}
			return true
		}
		if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			return false
		}
		if r, ok := runs[n]; ok { // kardeş dizisi: ilk üye tek yaprak, öbürleri atlanır (dups_siblings.go)
			if r.head {
				nodes = append(nodes, flatNode{kind: "SiblingRun", line: int32(fset.Position(n.Pos()).Line), last: r.endLine})
				if len(open) > 0 {
					nodes[open[len(open)-1].idx].children++
				}
			}
			return false
		}
		idx := len(nodes)
		nd := flatNode{kind: nodeKind(n), atom: nodeAtom(n),
			line: int32(fset.Position(n.Pos()).Line), last: int32(fset.Position(n.End()).Line)}
		if n == ast.Node(f) || n == ast.Node(f.Name) {
			nd.line, nd.last = 0, 0 // dosya ve paket adı: eşleşmenin satır aralığını uzatmasın
		}
		nodes = append(nodes, nd)
		if len(open) > 0 {
			nodes[open[len(open)-1].idx].children++
		}
		if cl, ok := n.(*ast.CompositeLit); ok && (isTest || len(cl.Elts) >= tableElements) {
			return false // tablo: yaprak
		}
		_, isFn := n.(*ast.FuncDecl)
		open = append(open, openFrame{idx: idx, fn: isFn})
		return true
	})
	return nodes, funcs
}

// addFile bir dosyanın düğümlerini ve işlevlerini diziye ekler, sonuna dosyaya özgü bir ayraç koyar.
func (co *dupCorpus) addFile(fset *token.FileSet, rel string, f *ast.File) {
	isTest := strings.HasSuffix(rel, "_test.go")
	fi := int32(len(co.files))
	co.files = append(co.files, rel)
	co.isTest = append(co.isTest, isTest)
	nodes, funcs := flatten(fset, f, isTest)
	base := len(co.seq)
	for _, nd := range nodes {
		key := nd.kind + "/" + strconv.Itoa(nd.children)
		id, ok := co.kinds[key]
		if !ok {
			id = uint32(len(co.kinds) + 1)
			co.kinds[key] = id
		}
		co.seq = append(co.seq, id)
		co.atom = append(co.atom, nd.atom)
		co.line = append(co.line, nd.line)
		co.last = append(co.last, nd.last)
		co.file = append(co.file, fi)
	}
	co.nodes += len(nodes)
	for _, lf := range funcs {
		co.funcs = append(co.funcs, dupFunc{name: lf.name, file: int(fi), from: base + lf.from, to: base + lf.to,
			line: int(lf.line), last: int(lf.last)})
	}
	co.seq = append(co.seq, sentinelBase+uint32(fi))
	co.atom = append(co.atom, "")
	co.line = append(co.line, 0)
	co.last = append(co.last, 0)
	co.file = append(co.file, fi)
}

// nodeKind düğümün türünü, işleci varsa işleciyle birlikte verir.
func nodeKind(n ast.Node) string {
	kind := fmt.Sprintf("%T", n)
	switch n := n.(type) {
	case *ast.BinaryExpr:
		kind += n.Op.String()
	case *ast.UnaryExpr:
		kind += n.Op.String()
	case *ast.AssignStmt:
		kind += n.Tok.String()
	case *ast.IncDecStmt:
		kind += n.Tok.String()
	case *ast.BranchStmt:
		kind += n.Tok.String()
	case *ast.GenDecl:
		kind += n.Tok.String()
	case *ast.RangeStmt:
		kind += n.Tok.String()
	case *ast.BasicLit:
		kind += n.Kind.String()
	}
	return kind
}

// nodeAtom düğümün adı ya da değeridir (fark özetinde gösterilir; eşleşmeye girmez). İşlev bildiriminde
// "Tür.Ad" ya da "Ad".
func nodeAtom(n ast.Node) string {
	switch n := n.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.BasicLit:
		return n.Value
	case *ast.FuncDecl:
		if n.Recv != nil && len(n.Recv.List) > 0 {
			return goload.RecvExprName(n.Recv.List[0].Type) + "." + n.Name.Name
		}
		return n.Name.Name
	}
	return ""
}
