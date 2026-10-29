package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/layoutrules"
)

// Kural: boyut satırının değeri düzen testinin ölçüsüdür. TestLayoutFileSize'ın taradığı her el yazımı Go dosyası için
// codegraph'ın dosya boyu, test dışı dosyalarda her func bildiriminin boyu ve adı, düzen testinin hesabıyla
// (layoutrules.FileLines, FuncLines, FuncName) eşittir; ölçü kayarsa "80/80" sessizce yanlışlaşmaz.
func TestSizesMatchTheLayoutTest(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	files, funcs := 0, 0
	for i := range g.Nodes {
		file := &g.Nodes[i]
		if file.Kind != kindFile || !strings.HasSuffix(file.File, ".go") {
			continue
		}
		whole, got, tests := ix.fileSizes(file)
		if !slices.Contains(tests, fileSizeTest) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.File)))
		if err != nil {
			t.Fatal(err)
		}
		if whole == nil || whole.lines != layoutrules.FileLines(data) {
			t.Errorf("%s: codegraph's file size %v, the layout test counts %d", file.File, whole, layoutrules.FileLines(data))
		}
		files++
		if !slices.Contains(tests, funcSizeTest) {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file.File, data, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		var want, have []string
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				want = append(want, fmt.Sprintf("%s %d", layoutrules.FuncName(fd), layoutrules.FuncLines(fset, fd)))
			}
		}
		for _, s := range got {
			have = append(have, fmt.Sprintf("%s %d", s.name, s.lines))
		}
		slices.Sort(want)
		slices.Sort(have)
		if !slices.Equal(want, have) {
			t.Errorf("%s: codegraph's functions %v, the layout test's %v", file.File, have, want)
		}
		funcs += len(want)
	}
	if files < 500 || funcs < 3000 {
		t.Fatalf("only %d files and %d functions compared", files, funcs)
	}
}

// Kural: boyut satırında sınırı aşan önce, sonra sınıra oranca en yakın; istisnanın sınırı tavan + paydır ve satır
// bunu söyler; %90 ve üstü "split before adding" der.
func TestTheSizeLinePutsTheNearestFirst(t *testing.T) {
	max, exc := layoutrules.FuncMax("internal/lang/interp/compiler_loop.go:compiler.inLoop")
	if c := layoutrules.LongFuncs["internal/lang/interp/compiler_loop.go:compiler.inLoop"]; !exc || max != c+layoutrules.FixMargin(c) {
		t.Fatalf("an exception's limit is its ceiling plus the fix margin: %d", max)
	}
	if max, exc := layoutrules.FuncMax("a.go:f"); exc || max != layoutrules.FuncLimit {
		t.Fatalf("a function off the list has FuncLimit: %d", max)
	}
	list := []sized{{"low", 10, 80, false}, {"over", 90, 80, false}, {"near", 75, 80, false}, {"exc", 150, 189, true}}
	if s := list[2].String(); s != "near 75/80 (split before adding)" {
		t.Errorf("near: %q", s)
	}
	if s := list[1].String(); !strings.Contains(s, "over the limit") {
		t.Errorf("over: %q", s)
	}
	if s := list[3].String(); !strings.Contains(s, "exception") || strings.Contains(s, "split") {
		t.Errorf("exception at 79%%: %q", s)
	}
}

// Kural: kanca tests-for cevabının ilk satırlarını gösterir (tools/edithook, maxLines); koşu satırları ve boyut satırı
// dosya satırının hemen altında, bildirim sayısından ve test gruplarından öncedir: liste ne kadar uzun olsa da görünür.
func TestTheRunAndSizeLinesComeFirst(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	for _, file := range runPlanFiles(g) {
		var out strings.Builder
		ix.printTestsFor(&out, file, 12)
		lines := strings.Split(out.String(), "\n")
		decl := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "  declarations in this file: ") })
		run := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "run: go test -short ./") })
		size := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "  size (TestLayoutFileSize") })
		if run != 1 || size < 0 || size > decl || decl > 6 {
			t.Errorf("%s: run line at %d, size line at %d, declarations at %d:\n%s", file.File, run, size, decl, strings.Join(lines[:min(len(lines), 8)], "\n"))
		}
	}
}
