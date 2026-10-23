package interp

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural: exprNames ve exprCaptures ortak çocuk gezintisini (subExpressions) kullanır ama
// sonuçları eski ayrı gezginlerle bire bir aynıdır: aynı adlar aynı sırayla, aynı yakalama
// kararı. Yakalama kararı yanlış 'yok' derse sahiplik modeli (15.43) tensörü erken bırakır.
// Neden: iki gezgin birleştirilirken bilerek farklı yerlere inen kolları (if/eşle blokları,
// liste üretecinin çıktı ve koşulu) kaybetmek sessizce yanlış sonuç verirdi.
// Referans gezginler aşağıda eski uygulamanın birebir kopyasıdır.
func TestExprWalkersMatchReferenceOnRepositoryPrograms(t *testing.T) {
	root := reporoot.Root()
	programs, expressions := 0, 0
	for _, dir := range []string{"tests", "lib", "examples"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".tr" {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			p := parser.New(lexer.New(string(src)))
			prog := p.ParseProgram()
			if len(p.Errors()) > 0 {
				return nil // bilerek hatalı programlar (hata testleri) gezilmez
			}
			programs++
			forEachExpression(reflect.ValueOf(prog), map[uintptr]bool{}, func(e ast.Expression) {
				expressions++
				var got, want []string
				exprNames(e, func(n string) { got = append(got, n) })
				legacyExprNames(e, func(n string) { want = append(want, n) })
				if !slices.Equal(got, want) {
					t.Fatalf("%s: exprNames %v, referans %v (%T)", path, got, want, e)
				}
				if a, b := (blockFlagCache{}).exprCaptures(e), (blockFlagCache{}).legacyExprCaptures(e); a != b {
					t.Fatalf("%s: exprCaptures %v, referans %v (%T)", path, a, b, e)
				}
			})
			return nil
		})
	}
	if programs < 50 || expressions < 1000 {
		t.Fatalf("yeterli program gezilmedi: %d program, %d ifade", programs, expressions)
	}
}

// forEachExpression ağaçtaki her ifade düğümünü (iç içe her yerde) verir.
func forEachExpression(v reflect.Value, seen map[uintptr]bool, visit func(ast.Expression)) {
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			forEachExpression(v.Elem(), seen, visit)
		}
	case reflect.Ptr:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		if e, ok := v.Interface().(ast.Expression); ok {
			visit(e)
		}
		forEachExpression(v.Elem(), seen, visit)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				forEachExpression(v.Field(i), seen, visit)
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			forEachExpression(v.Index(i), seen, visit)
		}
	}
}

// --- Referans gezginler: birleştirmeden önceki uygulamanın birebir kopyası ---

func legacyExprNames(e ast.Expression, name func(string)) {
	switch x := e.(type) {
	case nil:
	case *ast.FunctionLiteral:
		if x.Name != "" {
			name(x.Name)
		}
	case *ast.InterpolatedString:
		for _, p := range x.Parts {
			legacyExprNames(p, name)
		}
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			legacyExprNames(el, name)
		}
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			legacyExprNames(p.Key, name)
			legacyExprNames(p.Value, name)
		}
	case *ast.RangeExpression:
		legacyExprNames(x.Start, name)
		legacyExprNames(x.End, name)
	case *ast.PrefixExpression:
		legacyExprNames(x.Right, name)
	case *ast.InfixExpression:
		legacyExprNames(x.Left, name)
		legacyExprNames(x.Right, name)
	case *ast.IfExpression:
		legacyExprNames(x.Condition, name)
		for _, ei := range x.ElseIfs {
			legacyExprNames(ei.Condition, name)
		}
	case *ast.MatchExpression:
		legacyExprNames(x.Subject, name)
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				legacyExprNames(p, name)
			}
			legacyExprNames(mc.ValueExpr, name)
		}
	case *ast.CallExpression:
		legacyExprNames(x.Function, name)
		for _, a := range x.Arguments {
			legacyExprNames(a, name)
		}
		for _, na := range x.NamedArgs {
			legacyExprNames(na.Value, name)
		}
	case *ast.IndexExpression:
		legacyExprNames(x.Left, name)
		legacyExprNames(x.Index, name)
	case *ast.MemberExpression:
		legacyExprNames(x.Object, name)
	case *ast.ListComprehension:
		// Çıktı ve koşul kendi ortamında değerlendirilir; yalnızca kaynak bu ortamdadır
		legacyExprNames(x.Iterable, name)
	}
}

func (fc blockFlagCache) legacyExprCaptures(e ast.Expression) bool {
	switch x := e.(type) {
	case nil:
		return false
	case *ast.FunctionLiteral:
		return true
	case *ast.InterpolatedString:
		for _, p := range x.Parts {
			if fc.legacyExprCaptures(p) {
				return true
			}
		}
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			if fc.legacyExprCaptures(el) {
				return true
			}
		}
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			if fc.legacyExprCaptures(p.Key) || fc.legacyExprCaptures(p.Value) {
				return true
			}
		}
	case *ast.RangeExpression:
		return fc.legacyExprCaptures(x.Start) || fc.legacyExprCaptures(x.End)
	case *ast.PrefixExpression:
		return fc.legacyExprCaptures(x.Right)
	case *ast.InfixExpression:
		return fc.legacyExprCaptures(x.Left) || fc.legacyExprCaptures(x.Right)
	case *ast.IfExpression:
		if fc.legacyExprCaptures(x.Condition) || fc.doesBlockCapture(x.Consequence) || fc.doesBlockCapture(x.Alternative) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if fc.legacyExprCaptures(ei.Condition) || fc.doesBlockCapture(ei.Consequence) {
				return true
			}
		}
	case *ast.MatchExpression:
		if fc.legacyExprCaptures(x.Subject) {
			return true
		}
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				if fc.legacyExprCaptures(p) {
					return true
				}
			}
			if fc.legacyExprCaptures(mc.ValueExpr) || fc.doesBlockCapture(mc.Body) {
				return true
			}
		}
	case *ast.CallExpression:
		if fc.legacyExprCaptures(x.Function) {
			return true
		}
		for _, a := range x.Arguments {
			if fc.legacyExprCaptures(a) {
				return true
			}
		}
		for _, na := range x.NamedArgs {
			if fc.legacyExprCaptures(na.Value) {
				return true
			}
		}
	case *ast.IndexExpression:
		return fc.legacyExprCaptures(x.Left) || fc.legacyExprCaptures(x.Index)
	case *ast.MemberExpression:
		return fc.legacyExprCaptures(x.Object)
	case *ast.ListComprehension:
		return fc.legacyExprCaptures(x.Iterable) || fc.legacyExprCaptures(x.Output) || fc.legacyExprCaptures(x.Condition)
	}
	return false
}
