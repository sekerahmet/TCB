package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// writtenTRScript e2e testinin .tr dosyasına yazdığı bir betiktir: yazan test, çağrının yeri ve metni.
type writtenTRScript struct {
	test string
	at   string
	text string
}

// e2eWrittenScripts cmd/tr/e2e'nin test dosyalarında .tr dosyasına yazılan betikleri codegraph'tan bağımsız, düz
// sözdizimi ağacından bulur: testkit.WriteFile(t, dir, "ad.tr", metin), testkit.WritePath(t, yol, metin) ve
// os.WriteFile(yol, []byte(metin), ...); yol ya da ad yazılı bir dize ya da son parçası yazılı olan filepath.Join, metin
// yazılı bir dize ya da paketin bir dize sabitidir, çağrı bir Test işlevinin içindedir. Değişkenden, tablodan ya da
// yardımcıdan gelenler sayılmaz.
func e2eWrittenScripts(t *testing.T, root string) []writtenTRScript {
	t.Helper()
	dir := filepath.Join(root, "cmd", "tr", "e2e")
	fset := token.NewFileSet()
	entries, err := reporoot.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	consts := map[string]string{}
	var files []*ast.File
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.CONST {
				for _, sp := range g.Specs {
					vs := sp.(*ast.ValueSpec)
					for i, n := range vs.Names {
						if i < len(vs.Values) {
							if s, ok := literal(vs.Values[i]); ok {
								consts[n.Name] = s
							}
						}
					}
				}
			}
		}
	}
	text := func(e ast.Expr) (string, bool) {
		if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
			if _, isConv := c.Fun.(*ast.ArrayType); isConv {
				e = c.Args[0]
			}
		}
		if id, ok := e.(*ast.Ident); ok {
			s, ok := consts[id.Name]
			return s, ok
		}
		return literal(e)
	}
	path := func(e ast.Expr) bool {
		if c, ok := e.(*ast.CallExpr); ok && len(c.Args) > 0 {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Join" {
				e = c.Args[len(c.Args)-1]
			}
		}
		s, ok := literal(e)
		return ok && strings.HasSuffix(s, ".tr")
	}
	helpers := writeHelpers(files)
	var out []writtenTRScript
	for _, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && strings.HasPrefix(fd.Name.Name, "Test") {
				out = append(out, writesIn(fd, fset, text, path, helpers)...)
			}
		}
	}
	return out
}

// fileWriter bir dosya yazanın yol ve içerik argümanlarının sırasıdır.
type fileWriter struct{ path, content int }

// knownWriters paketle nitelenmiş dosya yazanlardır (paket adı.işlev).
var knownWriters = map[string]fileWriter{"testkit.WriteFile": {2, 3}, "testkit.WritePath": {1, 2}, "os.WriteFile": {0, 1}}

// writeHelpers e2e'nin kendi dosya yazan yardımcılarıdır (writeTR): yolunu ve içeriğini kendi parametrelerinden bir
// yazana ileten işlev de yazandır, sabit noktaya kadar (codegraph'ın tr_file_writes.go kuralının bağımsız yazımı).
func writeHelpers(files []*ast.File) map[string]fileWriter {
	helpers := map[string]fileWriter{}
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || strings.HasPrefix(fd.Name.Name, "Test") || helpers[fd.Name.Name] != (fileWriter{}) {
					continue
				}
				params := map[string]int{}
				i := 0
				for _, fl := range fd.Type.Params.List {
					for _, n := range fl.Names {
						params[n.Name] = i
						i++
					}
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					c, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					w, ok := callWriter(c, helpers)
					if !ok {
						return true
					}
					pa, pok := c.Args[w.path].(*ast.Ident)
					ca, cok := c.Args[w.content].(*ast.Ident)
					if pok && cok {
						if pi, ok := params[pa.Name]; ok {
							if ci, ok := params[ca.Name]; ok {
								helpers[fd.Name.Name] = fileWriter{pi, ci}
								changed = true
							}
						}
					}
					return true
				})
			}
		}
	}
	return helpers
}

// callWriter çağrı bir dosya yazanınsa (bilinen ya da yardımcı) argüman sıralarıdır.
func callWriter(c *ast.CallExpr, helpers map[string]fileWriter) (fileWriter, bool) {
	var w fileWriter
	var ok bool
	switch fn := c.Fun.(type) {
	case *ast.SelectorExpr:
		if pkg, isIdent := fn.X.(*ast.Ident); isIdent {
			w, ok = knownWriters[pkg.Name+"."+fn.Sel.Name]
		}
	case *ast.Ident:
		w, ok = helpers[fn.Name]
	}
	if !ok || w.path >= len(c.Args) || w.content >= len(c.Args) {
		return fileWriter{}, false
	}
	return w, true
}

// writesIn bir Test işlevinin .tr dosyasına yazdığı betiklerdir (e2eWrittenScripts): bilinen yazanlarla ve e2e'nin
// kendi yardımcılarıyla (helpers).
func writesIn(fd *ast.FuncDecl, fset *token.FileSet, text func(ast.Expr) (string, bool), path func(ast.Expr) bool, helpers map[string]fileWriter) []writtenTRScript {
	var out []writtenTRScript
	ast.Inspect(fd, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		w, ok := callWriter(c, helpers)
		if !ok {
			return true
		}
		p, content := c.Args[w.path], c.Args[w.content]
		if s, ok := text(content); ok && path(p) {
			pos := fset.Position(content.Pos())
			out = append(out, writtenTRScript{fd.Name.Name, filepath.Base(pos.Filename) + ":" + strconv.Itoa(pos.Line), s})
		}
		return true
	})
	return out
}

// literal yazılı bir Go dizesinin değeridir.
func literal(e ast.Expr) (string, bool) {
	b, ok := e.(*ast.BasicLit)
	if !ok || b.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(b.Value)
	return s, err == nil
}

// Kural: e2e'de .tr dosyasına yazılıp tr ikilisine verilen her betiğin kullandığı ad kodları (kitaplıktan aktarılan
// adlar dahil) onu yazan testten bir tests kenarı alır: alt süreçte koşan TR metni de onu yazan testin sınadığı adlardır.
// Betikler codegraph'tan bağımsız bulunur (e2eWrittenScripts); taramanın boş kalmaması için en az birkaç betik vardır.
func TestEveryScriptAnE2ETestWritesLinksToItsNameCodes(t *testing.T) {
	root, m, g := repoGraph(t)
	library := os.DirFS(filepath.Join(root, "lib"))
	scripts := e2eWrittenScripts(t, root)
	checked := 0
	for _, s := range scripts {
		uses, _ := codeindex.Snippet(s.text, library)
		for _, u := range uses {
			checked++
			if edge(g, memberID(m.Path+"/cmd/tr/e2e", "", s.test), "name:"+u.Code, edgeTests) == nil {
				t.Errorf("%s (%s) writes a TR script that uses %s, but no tests edge comes from the test", s.at, s.test, u.Code)
			}
		}
	}
	if len(scripts) < 5 || checked < 10 {
		t.Fatalf("the scan found %d scripts with %d name uses: it no longer sees how e2e writes TR scripts", len(scripts), checked)
	}
}
