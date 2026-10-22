package main

import (
	"encoding/json"
	"github.com/sekerahmet/TCB/tools/internal/genmain"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// migrated ileti kodlarına geçirilmiş paketlerdir: test dışı kodlarında fmt.Errorf ve errors.New
// kullanılmaz (her ileti bir koddur). Geçiş ilerledikçe liste büyür; bkz.
// internal/lang/messages/README.md.
var migrated = []string{
	"cmd/tr",
	"internal/compute/dataload",
	"internal/compute/gpu",
	"internal/compute/tensor",
	"internal/compute/tokenize",
	"internal/devtools/debugger",
	"internal/devtools/formatter",
	"internal/devtools/profiler",
	"internal/devtools/testrunner",
	"internal/lang/checker",
	"internal/lang/codes",
	"internal/lang/hint",
	"internal/lang/interp",
	"internal/lang/lexer",
	"internal/lang/parser",
	"internal/lang/source",
	"internal/lang/turkish",
	"internal/stdlib/checkpoint",
	"internal/stdlib/common/bind",
	"internal/stdlib/dataset",
	"internal/stdlib/files",
	"internal/stdlib/formats",
	"internal/stdlib/logging",
	"internal/stdlib/ml",
	"internal/stdlib/process",
	"internal/stdlib/tokenizer",
}

// Üretilmiş sabitler katalogla aynıdır.
func TestGeneratedUpToDate(t *testing.T) {
	genmain.AssertUpToDate(t, reporoot.Root(), "msgconsts", outputPath, Generate)
}

// call bir ileti çağrısıdır (messages.New / messages.Text; paketin kendi içinde New / Text).
type call struct {
	code string
	args int
	pos  string
}

// Depodaki her ileti çağrısı tanımdaki kadar değer verir; tanımlı her kod kullanılır;
// geçirilmiş paketlerde fmt.Errorf ve errors.New yoktur; ileti sözcüğü Go'da çekimlenmez.
func TestMessageUsage(t *testing.T) {
	root := reporoot.Root()
	data, err := os.ReadFile(filepath.Join(root, "internal", "lang", "messages", "definitions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var defs map[string]struct {
		Param []string `json:"param"`
	}
	if err := json.Unmarshal(data, &defs); err != nil {
		t.Fatal(err)
	}
	calls, banned, used := scan(t, root)
	for _, c := range calls {
		d, ok := defs[c.code]
		if !ok {
			t.Errorf("%s: %s tanımlı değil", c.pos, c.code)
			continue
		}
		if c.args != len(d.Param) {
			t.Errorf("%s: %s %d değer bekler (%v), %d verildi", c.pos, c.code, len(d.Param), d.Param, c.args)
		}
	}
	var unused []string
	for k := range defs {
		if !used[k] {
			unused = append(unused, k)
		}
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		t.Errorf("kullanılmayan ileti kodları (silin ya da numarayı boş bırakın): %v", unused)
	}
	for _, b := range banned {
		if strings.HasPrefix(b.code, "turkish.") {
			t.Errorf("%s: %s ile ileti sözcüğü çekimleniyor; eki şablona yazın ({ad|dat}, {ad|gen} ...)", b.pos, b.code)
			continue
		}
		t.Errorf("%s: ileti kodlarına geçirilmiş pakette %s; messages.New(messages.Hxxxx, ...) yazın", b.pos, b.code)
	}
}

// scan depodaki test dışı Go dosyalarını gezer: ileti çağrıları, geçirilmiş paketlerdeki yasak
// çağrılar ve anılan (kullanılan) kodlar. Bir kod New/Text'in ilk argümanı olmadan da
// kullanılabilir: dosya işlemi (turkish.FileError'a verilen messages.H2100) ya da platforma
// özgü neden tablosu.
func scan(t *testing.T, root string) (calls, banned []call, used map[string]bool) {
	t.Helper()
	used = map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := e.Name()
		if e.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || name == "scratch" || name == "testdata" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		// Üretilmiş sabit dosyası kodları bildirir, kullanmaz.
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "codes_generated.go" {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		rel = filepath.ToSlash(rel)
		inMessages := rel == "internal/lang/messages"
		isMigrated := false
		for _, m := range migrated {
			isMigrated = isMigrated || rel == m
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if e, ok := n.(ast.Expr); ok {
				if code := codeArg(e); code != "" && (inMessages || isSelector(e)) {
					used[code] = true
				}
			}
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pos := fset.Position(c.Pos()).String()
			pkg, fn := callee(c)
			if isMigrated && (pkg == "fmt" && fn == "Errorf" || pkg == "errors" && fn == "New") {
				banned = append(banned, call{code: pkg + "." + fn, pos: pos})
			}
			// İleti sözcüğünü Go çekimlemez: hâl eki şablondadır ({ad|dat}), dilin çekimcisi ekler.
			if pkg == "turkish" && (fn == "Suffix" || fn == "NumberSuffix") && rel != "internal/lang/turkish" {
				banned = append(banned, call{code: pkg + "." + fn, pos: pos})
			}
			if (pkg == "messages" || inMessages && pkg == "") && (fn == "New" || fn == "Text") && len(c.Args) > 0 {
				if code := codeArg(c.Args[0]); code != "" {
					calls = append(calls, call{code: code, args: len(c.Args) - 1, pos: pos})
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return calls, banned, used
}

func isSelector(e ast.Expr) bool { _, ok := e.(*ast.SelectorExpr); return ok }

// callee çağrılan fonksiyonun paket seçicisini ve adını verir (pkg.Fn ya da Fn).
func callee(c *ast.CallExpr) (pkg, fn string) {
	switch f := c.Fun.(type) {
	case *ast.SelectorExpr:
		if x, ok := f.X.(*ast.Ident); ok {
			return x.Name, f.Sel.Name
		}
	case *ast.Ident:
		return "", f.Name
	}
	return "", ""
}

// codeArg ilk argüman bir ileti sabitiyse (messages.H7003 ya da H7003) adını verir.
func codeArg(e ast.Expr) string {
	var name string
	switch x := e.(type) {
	case *ast.SelectorExpr:
		if p, ok := x.X.(*ast.Ident); ok && p.Name == "messages" {
			name = x.Sel.Name
		}
	case *ast.Ident:
		name = x.Name
	}
	if len(name) == 5 && name[0] == 'H' && strings.Trim(name[1:], "0123456789") == "" {
		return name
	}
	return ""
}

// retired.json silinmiş kodları tutar; hiçbiri katalogda ya da tanımlarda yeniden yok, her kaydın
// commit'i ve gerekçesi dolu.
func TestRetiredCodesStayRetired(t *testing.T) {
	root := reporoot.Root()
	data, err := os.ReadFile(filepath.Join(root, retiredPath))
	if err != nil {
		t.Fatal(err)
	}
	var retired map[string]struct {
		Commit string `json:"commit"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(data, &retired); err != nil {
		t.Fatalf("%s: %v", retiredPath, err)
	}
	for code, r := range retired {
		if len(code) != 5 || code[0] != 'H' {
			t.Errorf("%s: %q ileti kodu biçiminde değil", retiredPath, code)
		}
		if r.Commit == "" || r.Reason == "" {
			t.Errorf("%s: %s için commit ve gerekçe yazılmalı", retiredPath, code)
		}
	}
	trData, err := os.ReadFile(filepath.Join(root, trPath))
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := orderedTexts(trData)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkRetired(root, keys); err != nil {
		t.Error(err)
	}
}

// Üretici, silinmiş bir kodu katalogda ya da tanımlarda görürse çalışmaz.
func TestGenerateRejectsRetiredCode(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "lang", "messages")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tr.json", `{"H0001": "bir", "H0002": "iki"}`)
	write("definitions.json", `{"H0001": {"param": []}, "H0002": {"param": []}}`)
	if _, err := Generate(root); err != nil {
		t.Fatalf("retired.json olmadan üretilmeli: %v", err)
	}
	write("retired.json", `{"H0003": {"commit": "abc1234", "reason": "x"}}`)
	if _, err := Generate(root); err != nil {
		t.Fatalf("başka bir kod silinmişken üretilmeli: %v", err)
	}
	write("retired.json", `{"H0002": {"commit": "abc1234", "reason": "x"}}`)
	if _, err := Generate(root); err == nil || !strings.Contains(err.Error(), "H0002") {
		t.Errorf("silinmiş H0002 yeniden tanımlıyken üretici hata vermeli: %v", err)
	}
	// Yalnızca tanımlarda yeniden bulunan da yakalanır.
	write("tr.json", `{"H0001": "bir"}`)
	if _, err := Generate(root); err == nil || !strings.Contains(err.Error(), "definitions.json") {
		t.Errorf("tanımlarda kalan silinmiş kod yakalanmalı: %v", err)
	}
}
