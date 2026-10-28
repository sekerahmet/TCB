package main

import (
	"encoding/json"
	"fmt"
	"github.com/sekerahmet/TCB/tools/internal/genmain"
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

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// messageCodeText bir ileti kodunun metnidir ("H6903").
var messageCodeText = regexp.MustCompile(`^H\d{4}$`)

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

// wrongCountOnPurpose eksik değerli çağrının çalışma hatasını sınayan TR testidir
// (tests/hatalar_ertele_test.tr: TR_Message("H6903") değersiz kurulunca hata verir).
func wrongCountOnPurpose(c call) bool {
	return c.code == "H6903" && c.args == 0 && strings.Contains(filepath.ToSlash(c.pos), "tests/hatalar_ertele_test.tr:")
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
	trCalls := scanTR(t, root)
	for _, c := range trCalls {
		used[c.code] = true
	}
	calls = append(calls, trCalls...)
	for _, c := range calls {
		d, ok := defs[c.code]
		if !ok {
			t.Errorf("%s: %s tanımlı değil", c.pos, c.code)
			continue
		}
		if c.args != len(d.Param) && !wrongCountOnPurpose(c) {
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
	wrappers := codeWrappers(t, root)
	fset := token.NewFileSet()
	err := reporoot.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
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
			// Kodu ve değerlerini messages.New'e ileten sarmalayıcılar (syntaxError, NewReason ...)
			// da değer sayısıyla sınanır.
			if ci, ok := wrapperAt(wrappers, f.Name.Name, pkg, fn); ok && ci < len(c.Args) && c.Ellipsis == token.NoPos {
				if code := codeArg(c.Args[ci]); code != "" {
					calls = append(calls, call{code: code, args: len(c.Args) - ci - 1, pos: pos})
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

// scanTR depodaki .tr dosyalarındaki ileti(H…, değerler) çağrılarıdır: kitaplık hata metnini
// katalogdan alır, bu çağrılar da bir kodu kullanmak ve tanımdaki kadar değer vermek zorundadır.
// Çağrı sözcük birimlerinden okunur (kodlu dosyada ad kodla, kodsuzda Türkçe adla bulunur);
// değer sayısı en dıştaki virgüllerle sayılır.
func scanTR(t *testing.T, root string) (calls []call) {
	t.Helper()
	name := codes.Source().Name(codes.TypeMessage)
	err := reporoot.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if n := e.Name(); path != root && (strings.HasPrefix(n, ".") || n == "scratch" || n == "testdata" || n == "negative" || n == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".tr") {
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parts, err := source.Split(string(text))
		if err != nil {
			return nil // başlığı bozuk dosya bu testin konusu değil
		}
		calls = append(calls, messageCalls(parts.Lexer(), name, path)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return calls
}

// messageCalls sözcük birimi akışındaki name(H…, ...) çağrılarını verir.
func messageCalls(lx *lexer.Lexer, name, path string) (calls []call) {
	var toks []lexer.Token
	for tok := lx.NextToken(); tok.Type != lexer.TokEOF; tok = lx.NextToken() {
		toks = append(toks, tok)
	}
	for i := 0; i+2 < len(toks); i++ {
		// İleti kodu bir metin olarak yazılır: TR_Message("H6903", x).
		if (toks[i].Code != codes.TypeMessage && toks[i].Literal != name) || toks[i+1].Type != lexer.TokLParen ||
			toks[i+2].Type != lexer.TokString || !messageCodeText.MatchString(toks[i+2].Literal) {
			continue
		}
		args, depth := 0, 0
		for _, tok := range toks[i+1:] {
			switch {
			case tok.Type.IsOpener():
				depth++
			case tok.Type.IsCloser():
				depth--
			case tok.Type == lexer.TokComma && depth == 1:
				args++
			}
			if depth == 0 {
				break
			}
		}
		calls = append(calls, call{code: toks[i+2].Literal, args: args, pos: fmt.Sprintf("%s:%d", path, toks[i].Line)})
	}
	return calls
}

func isSelector(e ast.Expr) bool { _, ok := e.(*ast.SelectorExpr); return ok }

// codeWrappers kodu ve değerlerini messages.New/Text'e ileten fonksiyonlardır (anahtar → kod
// parametresinin sırası): bir messages.Code parametresi ve son değişken sayılı parametresi
// gövdede messages.New(kod, değerler...) olarak kullanılır. Anahtar paket.Ad, metotta .Ad.
func codeWrappers(t *testing.T, root string) map[string]int {
	t.Helper()
	out := map[string]int{}
	fset := token.NewFileSet()
	err := reporoot.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		if rel, _ := filepath.Rel(root, filepath.Dir(path)); filepath.ToSlash(rel) == "internal/lang/messages" {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			var names []string
			codeParam, variadic := -1, ""
			for _, p := range fd.Type.Params.List {
				for _, n := range p.Names {
					if s, ok := p.Type.(*ast.SelectorExpr); ok && s.Sel.Name == "Code" {
						if x, ok := s.X.(*ast.Ident); ok && x.Name == "messages" {
							codeParam = len(names)
						}
					}
					if _, ok := p.Type.(*ast.Ellipsis); ok {
						variadic = n.Name
					}
					names = append(names, n.Name)
				}
			}
			if codeParam < 0 || variadic == "" {
				continue
			}
			forwards := false
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok || c.Ellipsis == token.NoPos || len(c.Args) != 2 {
					return true
				}
				if pkg, fn := callee(c); pkg == "messages" && (fn == "New" || fn == "Text") {
					a, ok1 := c.Args[0].(*ast.Ident)
					b, ok2 := c.Args[1].(*ast.Ident)
					forwards = forwards || ok1 && ok2 && a.Name == names[codeParam] && b.Name == variadic
				}
				return true
			})
			if !forwards {
				continue
			}
			key := f.Name.Name + "." + fd.Name.Name
			if fd.Recv != nil {
				key = "." + fd.Name.Name
			}
			out[key] = codeParam
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// wrapperAt çağrılan fonksiyon bir kod sarmalayıcısı mı; öyleyse kod parametresinin sırası.
func wrapperAt(wrappers map[string]int, filePkg, pkg, fn string) (int, bool) {
	if pkg == "" {
		ci, ok := wrappers[filePkg+"."+fn]
		return ci, ok
	}
	if ci, ok := wrappers[pkg+"."+fn]; ok {
		return ci, true
	}
	ci, ok := wrappers["."+fn]
	return ci, ok
}

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

// ileti(H…, değerler) çağrısının kodu ve değer sayısı sözcük birimlerinden okunur: iç içe çağrıdaki
// ve liste içindeki virgüller sayılmaz, kodsuz çağrı yoksayılır.
func TestMessageCallsScanner(t *testing.T) {
	parts, err := source.Split("TR_Message(\"H6903\", a, f(b, c))\nTR_Message(\"H6904\")\nTR_Message(x, 1)\ng(\"H6905\", 1)\nTR_Message(\"H6900\", [1, 2], {\"a\": 1})\n")
	if err != nil {
		t.Fatal(err)
	}
	got := messageCalls(parts.Lexer(), codes.Source().Name(codes.TypeMessage), "x.tr")
	want := []call{{"H6903", 2, "x.tr:1"}, {"H6904", 0, "x.tr:2"}, {"H6900", 2, "x.tr:5"}}
	if len(got) != len(want) {
		t.Fatalf("%v, %v bekleniyordu", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d. çağrı: %v, %v bekleniyordu", i, got[i], want[i])
		}
	}
}
