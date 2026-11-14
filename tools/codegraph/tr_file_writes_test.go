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

// e2eWrittenScripts cmd/trlang/e2e'nin test dosyalarında .tr dosyasına yazılan betikleri codegraph'tan bağımsız, düz
// sözdizimi ağacından bulur: testkit.WriteFile(t, dir, "ad.tr", metin), testkit.WritePath(t, yol, metin) ve
// os.WriteFile(yol, []byte(metin), ...); yol ya da ad yazılı bir dize ya da son parçası yazılı olan filepath.Join, metin
// yazılı bir dize ya da paketin bir dize sabitidir, çağrı bir Test işlevinin içindedir. Değişkenden, tablodan ya da
// yardımcıdan gelenler sayılmaz.
func e2eWrittenScripts(t *testing.T, root string) []writtenTRScript {
	t.Helper()
	dir := filepath.Join(root, "cmd", "trlang", "e2e")
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

// Kural: e2e'de .tr dosyasına yazılıp trlang ikilisine verilen her betiğin kullandığı ad kodları (kitaplıktan aktarılan
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
			if edge(g, memberID(m.Path+"/cmd/trlang/e2e", "", s.test), "name:"+u.Code, edgeTests) == nil {
				t.Errorf("%s (%s) writes a TR script that uses %s, but no tests edge comes from the test", s.at, s.test, u.Code)
			}
		}
	}
	if len(scripts) < 5 || checked < 10 {
		t.Fatalf("the scan found %d scripts with %d name uses: it no longer sees how e2e writes TR scripts", len(scripts), checked)
	}
}

// trlangRunFiles: yer tutuculu bir TR programı strings.NewReplacer(...).Replace ile doldurulup .tr dosyasına yazılır
// (trlang ikilisi koşar) ya da strings.ReplaceAll ile doldurulup süreç içinde okuyucuya verilir.
var trlangRunFiles = map[string]string{
	"internal/lang/codes/tr.json": "{\n  \"TR_fn_default_device\": { \"name\": \"varsayılan_cihaz\", \"family\": \"x\" }\n}\n",
	"internal/lang/lexer/lexer.go": `package lexer

// New TR metnini okur.
func New(input string) int { return len(input) }
`,
	"app/run_test.go": `package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/audit/internal/lang/lexer"
)

const program = "TR_import \"{zincir}\"\nTR_default_device(\"cpu\")\n"

func TestRunByTrlang(t *testing.T) {
	text := strings.NewReplacer("{zincir}", "lib/zincir").Replace(program)
	os.WriteFile(filepath.Join(t.TempDir(), "cihaz.tr"), []byte(text), 0o600)
}

func TestRunInProcess(t *testing.T) { lexer.New(strings.ReplaceAll(program, "{zincir}", "lib/zincir")) }
`,
}

// Kural (codegraph dilim 6, D6-4 (a), Karar 5): strings.Replace, ReplaceAll ve (*Replacer).Replace TR metnini ilk
// argümandan geçirir. .tr dosyasına yazılıp trlang ikilisiyle koşulan metinden gelen ad kodu kenarı "trlang" işaretini
// taşır ve TR testi gibi her paketin yerleşiğine ulaşır: testin paketi yerleşiğin paketini içe aktarmasa da
// (assistant_test.TestChainDeviceTransitions → ml.defaultDeviceBuiltin). Süreç içinde koşulan metnin kenarı işaretsizdir;
// onun bağı inTestBinary süzgecinden geçer (TestNameCodeTestsMustLinkTheFunctionsPackage).
func TestATRProgramRunByTheTrlangBinaryReachesItsBuiltins(t *testing.T) {
	g, err := buildGraph(auditModule(t, trlangRunFiles))
	if err != nil {
		t.Fatal(err)
	}
	const app = "go:func:example.com/audit/app."
	byTrlang, inProcess := edge(g, app+"TestRunByTrlang", "name:TR_fn_default_device", edgeTests), edge(g, app+"TestRunInProcess", "name:TR_fn_default_device", edgeTests)
	if byTrlang == nil || byTrlang.Via != viaTrlang {
		t.Errorf("a program filled by (*strings.Replacer).Replace and written to a .tr file links its codes with via %q: %+v", viaTrlang, byTrlang)
	}
	if inProcess == nil || inProcess.Via != "" {
		t.Errorf("a program filled by strings.ReplaceAll and run in the process links its codes without a mark: %+v", inProcess)
	}

	const m = "example.com/m"
	fn := "go:func:" + m + "/ml.defaultDeviceBuiltin"
	h := &Graph{Module: m, Nodes: []Node{
		{ID: pkgID(m + "/ml"), Kind: kindPackage},
		{ID: pkgID(m + "/assistant"), Kind: kindPackage},
		{ID: fn, Kind: kindFunc, Name: "ml.defaultDeviceBuiltin", File: "ml/device.go"},
		{ID: "name:TR_fn_default_device", Kind: kindName, Name: "TR_fn_default_device"},
		{ID: "go:func:" + m + "/assistant.TestChainDeviceTransitions", Kind: kindFunc, Name: "assistant.TestChainDeviceTransitions", File: "assistant/device_test.go"},
		{ID: "go:func:" + m + "/assistant.TestInProcess", Kind: kindFunc, Name: "assistant.TestInProcess", File: "assistant/device_test.go"},
	}, Edges: []Edge{
		{From: fn, To: "name:TR_fn_default_device", Kind: edgeUsesName},
		{From: "go:func:" + m + "/assistant.TestChainDeviceTransitions", To: "name:TR_fn_default_device", Kind: edgeTests, Via: viaTrlang},
		{From: "go:func:" + m + "/assistant.TestInProcess", To: "name:TR_fn_default_device", Kind: edgeTests},
	}}
	var got []string
	for _, nt := range newIndex(h).testsThroughNames(newIndex(h).byID[fn]) {
		got = append(got, nt.test.Name)
	}
	if strings.Join(got, ", ") != "assistant.TestChainDeviceTransitions" {
		t.Errorf("tests through the name codes of ml.defaultDeviceBuiltin: %q, want only the test that runs the program with trlang (assistant imports no ml)", got)
	}
}
