package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// `time-ceilings`: duvar saati tavanı olan testler. `_test.go` dosyalarında geçen sürenin
// (`time.Since(x)`, `time.Until(x)`, `a.Sub(b)` ya da bunlardan atanan yerel değişken) sabit bir
// sayıyla ya da süreyle karşılaştırıldığı yerler. Kural (internal/README.md "Testler kararlıdır"):
// süre testi büyümeye bakar (`testkit.ScalesLinearly`); bir beklemenin erken bittiğini gösteren tavan
// ayırt ettiği sürenin yarısı kadar geniş seçilir. Çıktı karar değil listedir: her satırın tavanı
// yük altındaki bir makinede (yavaş CI, paralel koşu) yanlış alarm verebilir. İki değer de değişkense
// (`fast > slow`) ya da sınır bir değişkendeyse (`limit := 3 * time.Second`) görülmez.

// timeCeiling bir adaydır.
type timeCeiling struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Test     string `json:"test"`
	Compare  string `json:"compare"`  // karşılaştırmanın kaynağı
	Constant string `json:"constant"` // sabit tavan (süre ise okunur biçimiyle)
}

// elapsedFuncs geçen süreyi veren time işlevleri.
var elapsedFuncs = map[string]bool{"Since": true, "Until": true}

// timeAudit bir işlevdeki "geçen süre" değişkenlerini izler.
type timeAudit struct {
	m       *goload.Module
	u       *goload.Unit
	tainted map[types.Object]bool
	test    string
	file    string
	out     []timeCeiling
}

// findTimeCeilings denetimi çalıştırır.
func findTimeCeilings(m *goload.Module) []timeCeiling {
	var out []timeCeiling
	forEachFile(m, func(u *goload.Unit, f *ast.File, rel string) {
		if !isTestFile(rel) {
			return
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			a := &timeAudit{m: m, u: u, tainted: map[types.Object]bool{}, test: fd.Name.Name, file: rel}
			ast.Inspect(fd.Body, a.visit)
			out = append(out, a.out...)
		}
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// visit bir düğümü işler: atamalar değişkeni "geçen süre" yapar, karşılaştırmalar kaydedilir.
func (a *timeAudit) visit(n ast.Node) bool {
	switch s := n.(type) {
	case *ast.AssignStmt:
		if len(s.Lhs) == len(s.Rhs) {
			for i := range s.Lhs {
				if a.elapsed(s.Rhs[i]) {
					a.taint(s.Lhs[i])
				}
			}
		}
	case *ast.ValueSpec:
		for i, v := range s.Values {
			if i < len(s.Names) && a.elapsed(v) {
				a.taint(s.Names[i])
			}
		}
	case *ast.BinaryExpr:
		switch s.Op {
		case token.LSS, token.LEQ, token.GTR, token.GEQ:
			a.compare(s)
		}
	}
	return true
}

// taint atamanın sol tarafındaki adı "geçen süre" olarak işaretler.
func (a *timeAudit) taint(e ast.Expr) {
	if id, ok := e.(*ast.Ident); ok {
		if o := a.u.Info.Defs[id]; o != nil {
			a.tainted[o] = true
		} else if o := a.u.Info.Uses[id]; o != nil {
			a.tainted[o] = true
		}
	}
}

// elapsed ifade geçen süre mi içeriyor: time.Since/Until çağrısı, Time.Sub çağrısı ya da işaretli bir değişken.
func (a *timeAudit) elapsed(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if a.isElapsedCall(x) {
				found = true
			}
		case *ast.Ident:
			if o := a.u.Info.Uses[x]; o != nil && a.tainted[o] {
				found = true
			}
		}
		return !found
	})
	return found
}

// isElapsedCall çağrı time.Since / time.Until ya da (time.Time).Sub mu.
func (a *timeAudit) isElapsedCall(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := a.u.Info.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "time" {
		return false
	}
	if elapsedFuncs[fn.Name()] && fn.Type().(*types.Signature).Recv() == nil {
		return true
	}
	if fn.Name() == "Sub" {
		recv := fn.Type().(*types.Signature).Recv()
		return recv != nil && strings.HasSuffix(recv.Type().String(), "time.Time")
	}
	return false
}

// constant ifade sabit mi; öyleyse değerinin okunur biçimini verir.
func (a *timeAudit) constant(e ast.Expr) (string, bool) {
	tv, ok := a.u.Info.Types[e]
	if !ok || tv.Value == nil {
		return "", false
	}
	if named, ok := tv.Type.(*types.Named); ok && named.Obj().Name() == "Duration" && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "time" {
		if v, exact := constInt64(tv); exact {
			return time.Duration(v).String(), true
		}
	}
	return tv.Value.String(), true
}

// compare bir karşılaştırmanın bir yanı geçen süre, öbür yanı sabitse kaydeder.
func (a *timeAudit) compare(b *ast.BinaryExpr) {
	for _, pair := range [][2]ast.Expr{{b.X, b.Y}, {b.Y, b.X}} {
		if !a.elapsed(pair[0]) {
			continue
		}
		if c, ok := a.constant(pair[1]); ok {
			file, line := position(a.m, b.Pos())
			a.out = append(a.out, timeCeiling{File: file, Line: line, Test: a.test, Compare: a.source(b), Constant: c})
			return
		}
	}
}

// source ifadenin kaynak metnidir.
func (a *timeAudit) source(e ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, a.m.Fset, e)
	return strings.Join(strings.Fields(buf.String()), " ")
}

// runTimeCeilings `time-ceilings` komutudur.
func runTimeCeilings(args []string, stdout, stderr io.Writer) int {
	o, rest := parseAuditFlags("time-ceilings", args, stderr)
	if o == nil {
		return exitUsage
	}
	if len(rest) != 0 {
		fmt.Fprintf(stderr, "codegraph time-ceilings: unexpected argument %q\n%s", rest[0], usage)
		return exitUsage
	}
	m, code := loadAudit(o, stderr, true)
	if m == nil {
		return code
	}
	var list []timeCeiling
	for _, c := range findTimeCeilings(m) {
		if underIn(c.File, o.in) {
			list = append(list, c)
		}
	}
	if o.asJSON {
		return writeJSON(stdout, stderr, map[string]any{"schema": "codegraph-time-ceilings/1", "ceilings": list})
	}
	fmt.Fprintf(stdout, "%d wall-clock ceiling(s) in tests (a duration compared with a constant)\n", len(list))
	forEachLimited(stdout, list, o.limit, "  ", moreHint, func(c timeCeiling) string {
		return fmt.Sprintf("%s:%d  %s  ceiling %s  `%s`", c.File, c.Line, c.Test, c.Constant, c.Compare)
	})
	return exitOK
}

// constInt64 sabit bir tam sayı değerini verir (süreler nanosaniyedir).
func constInt64(tv types.TypeAndValue) (int64, bool) {
	v, exact := constant.Int64Val(constant.ToInt(tv.Value))
	return v, exact
}
