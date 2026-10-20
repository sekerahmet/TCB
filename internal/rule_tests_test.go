package internal_test

import (
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
	ruleBacktick = regexp.MustCompile("`([^`]+)`")
	ruleTestName = regexp.MustCompile(`^(Test|Fuzz|Benchmark|Example)(\w*)$`)
	ruleListItem = regexp.MustCompile(`^\s*([-*+]|\d+\.)\s`)
)

// ruleGuard bir README'nin bir kuralı koruyan olarak anıp tuttuğu bir test adı ya da test dosyasıdır.
type ruleGuard struct {
	readme string
	line   int
	token  string
}

// TestRuleTestsExist README'lerdeki her "Denetim:" maddesinin ve "Kural | Test" tablosu
// satırının andığı test işlevleri ve test dosyaları depoda vardır. Yeniden adlandırılan bir test
// kuralı sessizce korumasız bırakmaz.
func TestRuleTestsExist(t *testing.T) {
	root := reporoot.Root()
	funcs, files := ruleTestDecls(t, root)
	for _, g := range ruleGuards(t, root) {
		switch {
		case strings.HasSuffix(g.token, ".go"):
			if !ruleFileExists(root, g, files) {
				t.Errorf("%s:%d: test dosyası yok: %s", g.readme, g.line, g.token)
			}
		case !funcs[g.token]:
			t.Errorf("%s:%d: test işlevi yok: %s", g.readme, g.line, g.token)
		}
	}
}

// ruleTestDecls depodaki _test.go dosyalarının adlarını ve içlerindeki üst düzey işlev adlarını verir.
func ruleTestDecls(t *testing.T, root string) (funcs, files map[string]bool) {
	t.Helper()
	funcs, files = map[string]bool{}, map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
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

// ruleGuards bütün README'lerden koruma simgelerini toplar.
func ruleGuards(t *testing.T, root string) []ruleGuard {
	t.Helper()
	var out []ruleGuard
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && ruleSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "README.md" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, readmeGuards(filepath.ToSlash(rel), string(data))...)
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

// readmeGuards bir README'deki "Denetim:" maddelerinin ve "Kural | Test" tablolarının
// simgelerini çıkarır; kod çitlerinin içine bakılmaz.
func readmeGuards(rel, text string) []ruleGuard {
	var out []ruleGuard
	add := func(line int, s string) {
		for _, m := range ruleBacktick.FindAllStringSubmatch(s, -1) {
			tok := m[1]
			if strings.HasSuffix(tok, "_test.go") || ruleTestName.MatchString(tok) && !ruleLowerAfterPrefix(tok) {
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
		if inGuard {
			add(n, line)
		}
	}
	return out
}

// ruleLowerAfterPrefix "Testimiz" gibi go test'in test saymadığı adları ayırır.
func ruleLowerAfterPrefix(name string) bool {
	m := ruleTestName.FindStringSubmatch(name)
	return m != nil && m[2] != "" && m[2][0] >= 'a' && m[2][0] <= 'z'
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
