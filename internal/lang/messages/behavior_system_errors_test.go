package messages

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Test dışı Go kodunda aynı işlevdeki işletim sistemi ve GÇ hataları (`os`, `io`, `syscall`,
// `purego`, `exec`, `filepath.Abs`, GÇ metotları, `gpu.Status()`, `turkish.FileErrorReason`)
// `messages.SystemError`, `messages.SystemReason` ya da `turkish.FileError` geçmeden `messages.New`
// veya `messages.Text` yuvalarına verilemez (8.5, DENETİM 732 N1). Başka işlevden dönen ham hatalar
// paketlerin kendi çalışma testlerindedir.
//
// rawSystemErrorExempt kapsam dışı FileErrorReason kullanımlarıdır (yalnız küçülür).
var rawSystemErrorExempt = map[string]int{
	// TR_input okuma hatası: sahibi Motor, kapsam dışı (DENETİM 718 N2).
	"internal/lang/interp/builtins.go H2507": 1,
	// DENETİM 892 Ş1: hata ayıklayıcının cevabına gider, TR programının değerine girmez; sahibi araçlar, kapsam dışı.
	"internal/devtools/debugger/dap_launch.go H8121": 1,
}

var (
	guardErrorSlots = map[string]bool{"error": true, "reason": true, "status": true, "detail": true}
	guardNonErrPkg  = map[string]bool{
		"os.Getenv": true, "os.Getpid": true, "os.Args": true,
		"os.IsNotExist": true, "os.IsPermission": true, "os.SameFile": true,
	}
	guardFormatSel = map[string]bool{
		"jsonwrite.Write": true, "jsonwrite.WriteOptions": true,
		"binary.Read": true, "binary.Write": true, "rand.Read": true,
	}
	guardSysPkgs    = map[string]bool{"os": true, "io": true, "syscall": true, "purego": true, "exec": true}
	guardSysMethods = map[string]bool{
		"Read": true, "ReadAt": true, "ReadByte": true, "Write": true, "Seek": true,
		"Stat": true, "Info": true, "Sync": true, "Close": true, "Truncate": true,
		"Discard": true, "Find": true, "Call": true, "find": true, "start": true,
		"readBits": true, "readFloats": true,
	}
	guardSysLocals = map[string]bool{
		"openLibrary": true, "ltOpen": true, "atomicWrite": true, "startJob": true,
		"readRaw": true, "readGGUFTensor": true,
	}
)

type rawSysViolation struct {
	key    string
	detail string
}

func TestMessagesCarryNoRawSystemErrors(t *testing.T) {
	load()
	root := reporoot.Root()
	found := map[string]int{}
	details := map[string][]string{}
	err := reporoot.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_generated.go") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		fset := token.NewFileSet()
		f, parseErr := goparser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, v := range rawSystemErrorViolations(fset, f, rel) {
			found[v.key]++
			details[v.key] = append(details[v.key], v.detail)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key, n := range found {
		switch allowed := rawSystemErrorExempt[key]; {
		case n > allowed:
			t.Errorf("%s: sistem hatası SystemError/FileError geçmeden iletiye veriliyor (%d kez, izinli %d): %v", key, n, allowed, details[key])
		case n < allowed:
			t.Errorf("%s: izin haritası eskimiş (%d kaldı, listede %d); kaydı küçültün", key, n, allowed)
		}
	}
	for key := range rawSystemErrorExempt {
		if found[key] == 0 {
			t.Errorf("%s: artık ihlal yok; kaydı rawSystemErrorExempt haritasından silin", key)
		}
	}
}

func TestSystemErrorGuardDetector(t *testing.T) {
	load()
	const badSrc = `package sample
import (
	"io"
	"os"
	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)
func bad(r io.Reader, p []byte) error {
	if _, err := os.Open("a"); err != nil {
		return messages.New(messages.H6600, "a", err)
	}
	if _, err := io.ReadFull(r, p); err != nil {
		_ = messages.Text(messages.H8222, "a", err)
	}
	_ = messages.New(messages.H6813, "gpu", "cpu", "fn", gpu.Status())
	return messages.New(messages.H2100, "a", turkish.FileErrorReason("a", io.ErrUnexpectedEOF))
}
`
	fset := token.NewFileSet()
	fBad, err := goparser.ParseFile(fset, "synth_bad.go", badSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := rawSystemErrorViolations(fset, fBad, "synth_bad.go"); len(got) != 4 {
		t.Fatalf("4 ihlal bekleniyordu, gelen (%d): %v", len(got), got)
	}

	const goodSrc = `package sample
import (
	"os"
	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)
func good() error {
	if _, err := os.Open("a"); err != nil {
		return messages.New(messages.H6600, "a", messages.SystemError("a", err))
	}
	return messages.New(messages.H6813, "gpu", "cpu", "fn", gpu.StatusMessage())
}
`
	fGood, err := goparser.ParseFile(fset, "synth_good.go", goodSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := rawSystemErrorViolations(fset, fGood, "synth_good.go"); len(got) != 0 {
		t.Fatalf("sarılmış çağrılarda 0 ihlal bekleniyordu, gelen: %v", got)
	}
}

func rawSystemErrorViolations(fset *token.FileSet, f *ast.File, rel string) []rawSysViolation {
	inMessages := f.Name.Name == "messages"
	var out []rawSysViolation
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		out = append(out, walkFuncForRawSystemErrors(fset, fd.Body, rel, inMessages)...)
	}
	return out
}

func walkFuncForRawSystemErrors(fset *token.FileSet, body *ast.BlockStmt, rel string, inMessages bool) []rawSysViolation {
	formatReaders := map[string]bool{}
	var out []rawSysViolation
	var walk func(ast.Node, map[string]bool)
	walk = func(n ast.Node, tainted map[string]bool) {
		if n == nil {
			return
		}
		switch s := n.(type) {
		case *ast.IfStmt:
			walkIfForRawSystemErrors(s, tainted, walk)
			return
		case *ast.AssignStmt:
			noteAssignTaint(s, tainted, formatReaders)
		case *ast.CallExpr:
			out = append(out, checkMessageCallArgs(fset, s, rel, inMessages, tainted, formatReaders)...)
		}
		ast.Inspect(n, func(ch ast.Node) bool {
			if ch == n || ch == nil {
				return true
			}
			walk(ch, tainted)
			return false
		})
	}
	walk(body, map[string]bool{})
	return out
}

func walkIfForRawSystemErrors(s *ast.IfStmt, tainted map[string]bool, walk func(ast.Node, map[string]bool)) {
	sub := maps.Clone(tainted)
	if s.Init != nil {
		walk(s.Init, sub)
	}
	walk(s.Cond, sub)
	walk(s.Body, sub)
	if s.Else != nil {
		walk(s.Else, sub)
	}
	if as, ok := s.Init.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
		return
	}
	for k, v := range sub {
		if v {
			tainted[k] = true
		}
	}
}

func noteAssignTaint(s *ast.AssignStmt, tainted, formatReaders map[string]bool) {
	if len(s.Rhs) == 1 && len(s.Lhs) == 1 {
		if dst, ok := s.Lhs[0].(*ast.Ident); ok && isFormatReaderCall(s.Rhs[0]) {
			formatReaders[dst.Name] = true
		}
	}
	if len(s.Rhs) != 1 || len(s.Lhs) == 0 {
		return
	}
	id, ok := s.Lhs[len(s.Lhs)-1].(*ast.Ident)
	if !ok || id.Name == "_" {
		return
	}
	switch {
	case isSystemErrorSource(s.Rhs[0], formatReaders):
		tainted[id.Name] = true
	case isIdentExpr(s.Rhs[0]):
		tainted[id.Name] = tainted[s.Rhs[0].(*ast.Ident).Name]
	case isCallExpr(s.Rhs[0]):
		delete(tainted, id.Name)
	}
}

func checkMessageCallArgs(fset *token.FileSet, s *ast.CallExpr, rel string, inMessages bool, tainted, formatReaders map[string]bool) []rawSysViolation {
	if len(s.Args) < 2 {
		return nil
	}
	kind := messageBuilderName(s.Fun, inMessages)
	if kind == "" {
		return nil
	}
	cname := messageCodeName(s.Args[0])
	code, err := Parse(cname)
	if err != nil {
		return nil
	}
	params := definitions.Params[code]
	var out []rawSysViolation
	for i, arg := range s.Args[1:] {
		if i >= len(params) || !guardErrorSlots[params[i]] || isSafeMessageArg(arg) {
			continue
		}
		bad := isSystemErrorSource(arg, formatReaders) || isRawErrorField(arg)
		if id, ok := arg.(*ast.Ident); ok && tainted[id.Name] {
			bad = true
		}
		if bad {
			out = append(out, rawSysViolation{
				key:    rel + " " + cname,
				detail: fmt.Sprintf("%s:%d %s.%s (%s)", rel, fset.Position(arg.Pos()).Line, cname, params[i], kind),
			})
		}
	}
	return out
}

func messageBuilderName(fun ast.Expr, inMessages bool) string {
	switch x := fun.(type) {
	case *ast.SelectorExpr:
		if pkg, ok := x.X.(*ast.Ident); ok && pkg.Name == "messages" && (x.Sel.Name == "New" || x.Sel.Name == "Text") {
			return x.Sel.Name
		}
	case *ast.Ident:
		if inMessages && (x.Name == "New" || x.Name == "Text") {
			return x.Name
		}
	}
	return ""
}

func messageCodeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.Ident:
		return x.Name
	}
	return ""
}

func isSafeMessageArg(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return x.Name == "ErrCacheLocked" || x.Name == "ErrCacheInsufficient"
	case *ast.CallExpr:
		switch fun := x.Fun.(type) {
		case *ast.Ident:
			switch fun.Name {
			case "SystemError", "SystemReason", "fileErrorReason", "New", "Internal", "Internalf", "reservedReason", "panicReason":
				return true
			}
		case *ast.SelectorExpr:
			if pkg, ok := fun.X.(*ast.Ident); ok {
				if pkg.Name == "messages" && (fun.Sel.Name == "SystemError" || fun.Sel.Name == "SystemReason" || fun.Sel.Name == "New" || fun.Sel.Name == "Internal" || fun.Sel.Name == "Internalf" || fun.Sel.Name == "ErrorIn") {
					return true
				}
				if pkg.Name == "codes" && fun.Sel.Name == "NameRef" {
					return true
				}
			}
			return fun.Sel.Name == "errName" && len(x.Args) == 1
		}
	}
	return false
}

func isSystemErrorSource(e ast.Expr, formatReaders map[string]bool) bool {
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && (id.Name == "io" || id.Name == "fs" || id.Name == "os") && strings.HasPrefix(sel.Sel.Name, "Err") {
			return true
		}
	}
	ce, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch fun := ce.Fun.(type) {
	case *ast.Ident:
		return guardSysLocals[fun.Name]
	case *ast.SelectorExpr:
		if id, ok := fun.X.(*ast.Ident); ok {
			full := id.Name + "." + fun.Sel.Name
			if guardNonErrPkg[full] || guardFormatSel[full] || formatReaders[id.Name] {
				return false
			}
			if guardSysPkgs[id.Name] || full == "filepath.Abs" || full == "turkish.FileErrorReason" || full == "gpu.Status" {
				return true
			}
		}
		return guardSysMethods[fun.Sel.Name]
	}
	return false
}

func isFormatReaderCall(e ast.Expr) bool {
	ce, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && (pkg.Name == "csv" || pkg.Name == "json" || pkg.Name == "gzip")
}

func isRawErrorField(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && (sel.Sel.Name == "failure" || sel.Sel.Name == "waitErr")
}

func isIdentExpr(e ast.Expr) bool { _, ok := e.(*ast.Ident); return ok }
func isCallExpr(e ast.Expr) bool  { _, ok := e.(*ast.CallExpr); return ok }
