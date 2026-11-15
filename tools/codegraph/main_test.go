package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural: `help`'in changed satırı ucu çıkış yapılmış HEAD olan farkın çalışma ağacına kadar alındığını söyler (gaps,
// "`changed`'in `help` satırı çalışma ağacını söylemiyor": yalnız README söylüyordu, yan oturumlar `changed HEAD`'i
// istek olarak yazdı; kural changeHunks'ta).
func TestHelpSaysChangedHeadReadsTheWorkingTree(t *testing.T) {
	t.Parallel()
	var out, errOut strings.Builder
	if code := run([]string{"help"}, &out, &errOut); code != exitOK {
		t.Fatalf("help: exit %d: %s", code, errOut.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "changed <") {
			if !strings.Contains(line, `"changed HEAD", the diff runs to the working tree`) {
				t.Errorf("the changed line does not say the working tree is included:\n%s", line)
			}
			return
		}
	}
	t.Fatalf("help has no changed line:\n%s", out.String())
}

// Kural (K6): `explain` ve `history` komut değildir. İkisi çıkış 2 verir ve aynı işi gören yolu yazar: `explain`
// `query`'nin öbür adıydı, ileti aynı argümanlarla `query` satırını verir; `history`'yi git cevaplar, ileti adın dosyasını
// bulan `query` adımını ve her ad türü için sabitlenmiş `git log` biçimini verir. İletideki her biçim git'li fikstür
// deposunda bir kez koşar ve o bildirimi (dosyayı, kodu) değiştiren commit'i verir, öbürünü vermez.
func TestRetiredCommandsNameTheirReplacement(t *testing.T) {
	t.Parallel()
	var out, errOut strings.Builder
	if code := run([]string{"explain", "shape.Square.Area"}, &out, &errOut); code != exitUsage || out.Len() != 0 {
		t.Fatalf("explain: exit %d, want %d; stdout %q", code, exitUsage, out.String())
	}
	if want := "codegraph: \"explain\" was another name for query and is gone: go run ./tools/codegraph query shape.Square.Area\n"; errOut.String() != want {
		t.Errorf("explain: stderr\n%q\nwant\n%q", errOut.String(), want)
	}
	if code, got, _ := cli(t, "query", "shape.Square.Area"); code != exitOK || !strings.HasPrefix(got, "shape.Square.Area [method]") {
		t.Errorf("the query the message names: exit %d\n%s", code, got)
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"history", "shape.Orphan"}, &out, &errOut); code != exitUsage || out.Len() != 0 {
		t.Fatalf("history: exit %d, want %d; stdout %q", code, exitUsage, out.String())
	}
	if errOut.String() != historyGone+"\n" {
		t.Errorf("history: stderr\n%q\nwant\n%q", errOut.String(), historyGone+"\n")
	}

	root := gitFixture(t)
	shape := filepath.Join(root, "shape", "shape.go")
	replaceIn(t, shape, "return s.Side * s.Side", "return s.Side * s.Side * 1")
	commitAll(t, root, "shape: touch Area")
	replaceIn(t, shape, "func Orphan() {}", "func Orphan() { _ = 1 }")
	commitAll(t, root, "shape: touch Orphan")
	replaceIn(t, shape, "type Square struct{ Side float64 }", "type Square struct{ Side float64 } // touched")
	commitAll(t, root, "shape: touch Square")
	replaceIn(t, filepath.Join(root, "shape", "leaf.go"), "func leaf() {}", "func leaf() { _ = 2 }")
	commitAll(t, root, "shape: touch leaf.go")
	tr := filepath.Join(root, "internal", "lang", "messages", "tr.json")
	defs := filepath.Join(root, "internal", "lang", "messages", "definitions.json")
	replaceIn(t, tr, "\"H0002\": \"second message\"", "\"H0002\": \"second message\",\n\"H0003\": \"third\"")
	replaceIn(t, defs, "\"H0002\": { \"param\": [] }", "\"H0002\": { \"param\": [] },\n\"H0003\": { \"param\": [] }")
	commitAll(t, root, "messages: add H0003")

	forms := regexp.MustCompile(`git log (.+?) for a (function|method|type|file|code)\b`).FindAllStringSubmatch(historyGone, -1)
	cases := map[string]struct{ name, file, want string }{
		"function": {"Orphan", "shape/shape.go", "shape: touch Orphan"},
		"method":   {"Area", "shape/shape.go", "shape: touch Area"},
		"type":     {"Square", "shape/shape.go", "shape: touch Square"},
		"file":     {"", "shape/leaf.go", "shape: touch leaf.go"},
		"code":     {"H0003", "", "messages: add H0003"},
	}
	if len(forms) != len(cases) {
		t.Fatalf("the message has %d git log forms, want %d:\n%s", len(forms), len(cases), historyGone)
	}
	for _, f := range forms {
		c := cases[f[2]]
		line := strings.NewReplacer("<name>", c.name, "<file>", c.file, "<code>", c.name).Replace(f[1])
		got, err := gitOut(root, append([]string{"log"}, splitArgs(strings.ReplaceAll(line, "'", "\""))...)...)
		if err != nil {
			t.Errorf("%s: git log %s: %v", f[2], line, err)
			continue
		}
		subjects := string(got)
		if !strings.Contains(subjects, c.want) {
			t.Errorf("%s: git log %s does not list %q:\n%s", f[2], line, c.want, subjects)
		}
		for _, other := range cases {
			if other.want != c.want && strings.Contains(subjects, other.want) {
				t.Errorf("%s: git log %s also lists %q:\n%s", f[2], line, other.want, subjects)
			}
		}
	}
	checkRetiredAudits(t)
}

// checkRetiredAudits codegraph 7 Dilim C'de (K3) emekli olan üç komutun iletisini ve iletinin önerdiği yolu sınar:
// package-state'in yerine query'nin writes_var satırı, time-ceilings'in yerine git grep deseni, captures'ın yerine
// işlevin dosya:satır'ını veren query. Her komut çıkış 2 verir ve stdout'a yazmaz.
func checkRetiredAudits(t *testing.T) {
	t.Helper()
	for _, cmd := range []string{"package-state", "time-ceilings", "captures"} {
		var out, errOut strings.Builder
		if code := run([]string{cmd, "x"}, &out, &errOut); code != exitUsage || out.Len() != 0 || errOut.String() != retiredGone[cmd]+"\n" {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", cmd, code, out.String(), errOut.String())
		}
	}
	root := auditModule(t, varAccessFiles)
	var out, errOut strings.Builder
	if code := run([]string{"query", "a.counter", "--root", root, "--graph", filepath.Join(t.TempDir(), "g.json")}, &out, &errOut); code != exitOK ||
		!strings.Contains(out.String(), "<- writes_var") || !strings.Contains(out.String(), "a.Set [func]") {
		t.Errorf("package-state's replacement (query <variable>): exit %d\n%s%s", code, out.String(), errOut.String())
	}
	re := regexp.MustCompile(`git grep -n -E '([^']+)'`).FindStringSubmatch(retiredGone["time-ceilings"])
	if re == nil {
		t.Fatalf("time-ceilings: no git grep pattern in %q", retiredGone["time-ceilings"])
	}
	ceiling := regexp.MustCompile(re[1])
	for line, want := range map[string]bool{"if d > 1500*time.Millisecond {": true, "if time.Since(start) > time.Second {": true,
		"if elapsed > 10 * time.Second {": true, "time.Sleep(10 * time.Millisecond)": false} {
		if ceiling.MatchString(line) != want {
			t.Errorf("time-ceilings' pattern on %q: %v, want %v", line, !want, want)
		}
	}
	if code, got, _ := cli(t, "query", "shape.Bump"); code != exitOK || !strings.Contains(got, "shape.Bump [func] shape/shape.go:82") {
		t.Errorf("captures' replacement (query <function>): exit %d\n%s", code, got)
	}
}

// Kural: paketin üst testleri ilk satırda t.Parallel() çağırır; paket G4'te tek süreçte 154,7 sn sürüyordu, en uzun
// iş önce dağıtılınca 53,9 sn (codegraph 7 Dilim C, K1). Süreç genelini değiştiren test sırayla kalır: gövdesinde ya da
// paket içinden çağırdığı bir yardımcıda Setenv, Unsetenv ya da Chdir varsa (Go böyle bir testte ya da onun bir
// üstünde t.Parallel()'i panikle durdurur). Seçim çağrıya bakar, metne değil: ters tırnaklı fikstürlerdeki
// t.Setenv (env_names_test.go) sayılmaz.
func TestCodegraphTestsRunInParallel(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	paths, err := reporoot.Glob("*_test.go")
	if err != nil || len(paths) < 50 {
		t.Fatalf("test files: %d, %v", len(paths), err)
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
				funcs[fd.Name.Name] = fd
			}
		}
	}
	sequential := processWideTests(funcs)
	var stay []string
	for name, fd := range funcs {
		p := testingTParam(fd)
		if !strings.HasPrefix(name, "Test") || p == "" {
			continue
		}
		first := len(fd.Body.List) > 0 && isParallelCall(fd.Body.List[0], p)
		switch {
		case sequential[name]:
			stay = append(stay, name)
		case !first:
			t.Errorf("%s: %s does not call %s.Parallel() first; the tests of this package run in parallel, only a test that sets the environment or the working directory (Setenv, Unsetenv, Chdir), itself or through a helper, stays sequential", fset.Position(fd.Pos()), name, p)
		}
	}
	slices.Sort(stay)
	if want := []string{"TestQueryFromSubdirectoryUsesRootGraph", "TestTRGraphIsIndependentOfTheActiveLanguage"}; !slices.Equal(stay, want) {
		t.Errorf("sequential tests %v, want %v: a new one needs its reason here", stay, want)
	}
}

// processWideTests gövdesinde ya da çağırdığı paket içi bir işlevde Setenv, Unsetenv ya da Chdir çağrısı olan
// işlevlerdir (sabit noktaya kadar).
func processWideTests(funcs map[string]*ast.FuncDecl) map[string]bool {
	marked := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for name, fd := range funcs {
			if marked[name] {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fn := c.Fun.(type) {
				case *ast.SelectorExpr:
					marked[name] = marked[name] || fn.Sel.Name == "Setenv" || fn.Sel.Name == "Unsetenv" || fn.Sel.Name == "Chdir"
				case *ast.Ident:
					marked[name] = marked[name] || marked[fn.Name]
				}
				return true
			})
			changed = changed || marked[name]
		}
	}
	return marked
}

// testingTParam tek parametresi *testing.T olan işlevde o parametrenin adıdır; değilse boş.
func testingTParam(fd *ast.FuncDecl) string {
	ps := fd.Type.Params.List
	if len(ps) != 1 || len(ps[0].Names) != 1 {
		return ""
	}
	if st, ok := ps[0].Type.(*ast.StarExpr); ok {
		if sel, ok := st.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "T" {
			return ps[0].Names[0].Name
		}
	}
	return ""
}

// isParallelCall deyim p.Parallel() mi.
func isParallelCall(s ast.Stmt, p string) bool {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return false
	}
	c, ok := es.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Parallel" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == p
}
