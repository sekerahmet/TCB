package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/layoutrules"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// layoutWalk düzen testinin ölçtüğü Go dosyalarıdır, codegraph'tan bağımsız yürünür (internal/layout_test.go'nun
// goFiles'ı gibi): sizeRoots altında, git'in izlediği, layoutrules.SkippedDir'e düşmeyen klasörlerdeki, üretilmiş
// başlığı olmayan .go dosyaları. Dosya yolu -> içerik.
func layoutWalk(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, k := range sizeRoots {
		err := reporoot.WalkDir(filepath.Join(root, k), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if layoutrules.SkippedDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), p, data, parser.ParseComments|parser.PackageClauseOnly)
			if err != nil {
				return err
			}
			if !ast.IsGenerated(f) {
				rel, _ := filepath.Rel(root, p)
				out[filepath.ToSlash(rel)] = data
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// sizeRoots internal/layout_test.go'nun layoutRoots'udur. Kopyadır, bilerek: tarama tanıyıcısı (scans.go) kuralın
// kapsamını o test dosyasının paket düzeyi listesinden okur; liste başka pakete taşınırsa kapsam bilinmez olur.
var sizeRoots = []string{"cmd", "internal", "lib", "tools"}

// Kural: boyut satırının kapsamı ve değeri düzen testininkidir. Düzen testinin ölçtüğü her Go dosyası (yolu codegraph'tan
// bağımsız yürünür: layoutWalk) boyut satırı alır, dosya kuralıyla; test dışıysa fonksiyon kuralıyla da. Dosya boyu, test
// dışı dosyalarda her func bildiriminin boyu ve adı düzen testinin hesabıyla (layoutrules.FileLines, FuncLines, FuncName)
// eşittir; codegraph'ın ölçmediği bir dosya da ölçtüğü fazladan bir dosya da düşer. Bir tarama tanıma eksiği boyut
// satırını sessizce düşürmez, ölçü kayarsa "80/80" sessizce yanlışlaşmaz (DENETİM, birleşme denetimi: 1067/1067).
func TestSizesMatchTheLayoutTest(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	walked := layoutWalk(t, root)
	funcs := 0
	for rel, data := range walked {
		file := ix.byID[fileID(rel)]
		if file == nil {
			t.Errorf("%s: the layout test measures it, the graph has no node for it", rel)
			continue
		}
		whole, got, tests := ix.fileSizes(file)
		if !slices.Contains(tests, fileSizeTest) || whole == nil {
			t.Errorf("%s: the layout test measures it, tests-for shows no file size (scan rules: %v)", rel, tests)
			continue
		}
		if whole.lines != layoutrules.FileLines(data) {
			t.Errorf("%s: codegraph's file size %d, the layout test counts %d", rel, whole.lines, layoutrules.FileLines(data))
		}
		if isTestFile(rel) {
			continue
		}
		if !slices.Contains(tests, funcSizeTest) {
			t.Errorf("%s: the layout test measures its functions, tests-for shows none", rel)
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, rel, data, parser.SkipObjectResolution)
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
			t.Errorf("%s: codegraph's functions %v, the layout test's %v", rel, have, want)
		}
		funcs += len(want)
	}
	for i := range g.Nodes {
		file := &g.Nodes[i]
		if file.Kind != kindFile || !strings.HasSuffix(file.File, ".go") || walked[file.File] != nil {
			continue
		}
		if _, _, tests := ix.fileSizes(file); len(tests) > 0 {
			t.Errorf("%s: tests-for shows a size line, the layout test does not measure it", file.File)
		}
	}
	if len(walked) < 1000 || funcs < 3000 {
		t.Fatalf("only %d files and %d functions compared", len(walked), funcs)
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
