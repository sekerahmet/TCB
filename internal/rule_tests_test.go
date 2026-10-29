package internal_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// ruleSkipDirs README taramasında girilmeyen klasörlerdir.
var ruleSkipDirs = set(".git .claude .codegraph node_modules testdata")

var (
	ruleBacktick   = regexp.MustCompile("`([^`]+)`")
	ruleTestName   = regexp.MustCompile(`^(Test|Fuzz|Benchmark|Example)(\w*)$`)
	ruleTestPrefix = regexp.MustCompile(`^(Test|Fuzz|Benchmark|Example)(\S*)$`)
	ruleListItem   = regexp.MustCompile(`^\s*([-*+]|\d+\.)\s`)
	ruleSpecFile   = regexp.MustCompile(`^docs/spec/\d\d_[a-z0-9_]+\.md$`)
)

// ruleGuard bir README'nin bir kuralı koruyan olarak anıp tuttuğu bir test adı ya da test dosyasıdır.
type ruleGuard struct {
	readme string
	line   int
	token  string
}

// TestRuleTestsExist README'lerdeki her "Denetim:" maddesinin, "Kural | Test" tablosu
// satırının ve dil tanımındaki (docs/spec/NN_*.md) her "- *Test:*" satırının andığı test
// işlevleri ve test dosyaları depoda vardır. Yeniden adlandırılan bir test kuralı sessizce
// korumasız bırakmaz; jokerli ya da kalıplı test adı kabul edilmez.
func TestRuleTestsExist(t *testing.T) {
	root := reporoot.Root()
	funcs, files := ruleTestDecls(t, root)
	for _, g := range ruleGuards(t, root) {
		if p := ruleGuardProblem(root, g, funcs, files); p != "" {
			t.Error(p)
		}
	}
}

// ruleGuardProblem bir koruma simgesindeki sorunu verir (yoksa "").
func ruleGuardProblem(root string, g ruleGuard, funcs, files map[string]bool) string {
	switch {
	case strings.HasSuffix(g.token, ".go"):
		if !ruleFileExists(root, g, files) {
			return fmt.Sprintf("%s:%d: test dosyası yok: %s", g.readme, g.line, g.token)
		}
	case !ruleTestName.MatchString(g.token):
		return fmt.Sprintf("%s:%d: jokerli ya da kalıplı test adı: %s (test adlarını tek tek yazın)", g.readme, g.line, g.token)
	case !funcs[g.token]:
		return fmt.Sprintf("%s:%d: test işlevi yok: %s", g.readme, g.line, g.token)
	}
	return ""
}

// ruleTestDecls depodaki _test.go dosyalarının adlarını ve içlerindeki üst düzey işlev adlarını verir.
func ruleTestDecls(t *testing.T, root string) (funcs, files map[string]bool) {
	t.Helper()
	funcs, files = map[string]bool{}, map[string]bool{}
	fset := token.NewFileSet()
	err := reporoot.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && ruleSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		files[d.Name()] = true
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil // sözdizimi hatasını başka testler bildirir
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				funcs[fn.Name.Name] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return funcs, files
}

// ruleFileExists bir README'nin andığı test dosyasını arar: kökten verilen yol, README'nin
// klasörü ya da (yolsuz adda) depodaki herhangi bir yer.
func ruleFileExists(root string, g ruleGuard, files map[string]bool) bool {
	for _, p := range []string{g.token, filepath.Join(filepath.Dir(g.readme), g.token)} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err == nil {
			return true
		}
	}
	return !strings.Contains(g.token, "/") && files[g.token]
}

// ruleGuards bütün README'lerden ve dil tanımının bölümlerinden koruma simgelerini toplar.
func ruleGuards(t *testing.T, root string) []ruleGuard {
	t.Helper()
	var out []ruleGuard
	err := reporoot.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && ruleSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.Name() != "README.md" && !ruleSpecFile.MatchString(rel) {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, docGuards(rel, string(data))...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].readme != out[j].readme {
			return out[i].readme < out[j].readme
		}
		return out[i].line < out[j].line
	})
	return out
}

// docGuards bir belgedeki "Denetim:" maddelerinin, "Kural | Test" tablolarının ve dil
// tanımının "- *Test:*" satırlarının simgelerini çıkarır; kod çitlerinin içine bakılmaz.
func docGuards(rel, text string) []ruleGuard {
	var out []ruleGuard
	add := func(line int, s string) {
		for _, m := range ruleBacktick.FindAllStringSubmatch(s, -1) {
			tok := m[1]
			if strings.HasSuffix(tok, "_test.go") || ruleTestPrefix.MatchString(tok) && !ruleLowerAfterPrefix(tok) {
				out = append(out, ruleGuard{rel, line, tok})
			}
		}
	}
	inFence, inGuard := false, false
	var header []string
	tableRow := 0
	for i, raw := range strings.Split(text, "\n") {
		n, line := i+1, strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inFence, inGuard, header = !inFence, false, nil
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(line, "|") {
			inGuard = false
			cells := ruleCells(line)
			switch {
			case header == nil:
				header, tableRow = cells, 0
			case tableRow == 0:
				tableRow = 1 // ayırıcı satır
			default:
				if c := ruleTestColumn(header); c >= 0 && c < len(cells) {
					add(n, cells[c])
				}
			}
			continue
		}
		header = nil
		if line == "" || strings.HasPrefix(line, "#") || ruleListItem.MatchString(raw) {
			inGuard = false
		}
		if i := strings.Index(line, "Denetim:"); i >= 0 {
			inGuard = true
			line = line[i:]
		}
		// Dil tanımının Test satırı satır başında yazılır; girintili satırı kapı biçim hatası sayar.
		if strings.HasPrefix(raw, "- *Test:*") {
			add(n, line)
			continue
		}
		if inGuard {
			add(n, line)
		}
	}
	return out
}

// ruleLowerAfterPrefix "Testimiz" gibi go test'in test saymadığı adları ayırır.
func ruleLowerAfterPrefix(name string) bool {
	m := ruleTestPrefix.FindStringSubmatch(name)
	return m != nil && m[2] != "" && m[2][0] >= 'a' && m[2][0] <= 'z'
}

// Koruma satırlarında ters tırnak içindeki jokerli ya da kalıplı test adları yakalanır; düz
// metindeki jokerler (koruma satırında ters tırnak dışında ya da koruma satırı dışındaki metinde)
// hata vermez.
func TestDocGuards(t *testing.T) {
	doc := strings.Join([]string{
		"Düz metinde `BenchmarkSpeed*` ve TestGPU* geçebilir.",
		"Denetim: `TestVar`, düz metinde TestGPU* ve Benchmark*.",
		"- *Test:* Go: `TestGPU*`, `TestA|TestB`, `TestA/alt`, `Fuzz*`.",
	}, "\n")
	guards := docGuards("docs/spec/01_program.md", doc)
	wantTokens := []string{"TestVar", "TestGPU*", "TestA|TestB", "TestA/alt", "Fuzz*"}
	if len(guards) != len(wantTokens) {
		t.Fatalf("bulunan simgeler: %v, beklenen: %v", guards, wantTokens)
	}
	funcs := map[string]bool{"TestVar": true}
	if p := ruleGuardProblem("", guards[0], funcs, nil); p != "" {
		t.Errorf("geçerli test adında hata: %s", p)
	}
	for i, w := range wantTokens[1:] {
		g := guards[i+1]
		p := ruleGuardProblem("", g, funcs, nil)
		if !strings.Contains(p, "docs/spec/01_program.md:3") || !strings.Contains(p, w) || !strings.Contains(p, "tek tek") {
			t.Errorf("%s için beklenen satır, sözcük ve öneri iletisi alınmadı: %q", w, p)
		}
	}
}

func ruleCells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// ruleTestColumn "Kural" ve "Test" sütunlu tablonun test sütununun sırasını verir (yoksa -1).
func ruleTestColumn(header []string) int {
	hasRule, test := false, -1
	for i, h := range header {
		h = strings.ToLower(h)
		hasRule = hasRule || strings.HasPrefix(h, "kural")
		if test < 0 && strings.HasPrefix(h, "test") {
			test = i
		}
	}
	if !hasRule {
		return -1
	}
	return test
}
