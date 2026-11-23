package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
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

// untrackedOrUnmeasured düzen testinin ölçmediği ama boyut satırı alan dosyanın iletisidir: git'in izlemediği yeni dosyada
// sebep codegraph değil, dosyanın eklenmemesidir (gaps, "Yeni dosyada yerleşim testinin iletisi yanıltıcı": D1
// 2026-11-08'de "the layout test does not measure it" yeni dosyada iki aracın ayrıldığını sandırdı).
func untrackedOrUnmeasured(root, rel string) string {
	if isUntracked(root, rel) {
		return fmt.Sprintf("%s: the file is not tracked by git; the layout test walks tracked files only (run `git add %s`)", rel, rel)
	}
	return fmt.Sprintf("%s: tests-for shows a size line, the layout test does not measure it", rel)
}

// Kural: boyut satırının kapsamı ve değeri düzen testininkidir. Düzen testinin ölçtüğü her Go dosyası (yolu codegraph'tan
// bağımsız yürünür: layoutWalk) boyut satırı alır, dosya kuralıyla; test dışıysa fonksiyon kuralıyla da. Dosya boyu, test
// dışı dosyalarda her func bildiriminin boyu ve adı düzen testinin hesabıyla (layoutrules.FileLines, FuncLines, FuncName)
// eşittir; codegraph'ın ölçmediği bir dosya da ölçtüğü fazladan bir dosya da düşer. Bir tarama tanıma eksiği boyut
// satırını sessizce düşürmez, ölçü kayarsa "80/80" sessizce yanlışlaşmaz (DENETİM, birleşme denetimi: 1067/1067).
func TestSizesMatchTheLayoutTest(t *testing.T) {
	t.Parallel()
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
		whole, got, stale, tests := ix.fileSizes(file)
		if !slices.Contains(tests, fileSizeTest) || whole == nil || stale {
			t.Errorf("%s: the layout test measures it, tests-for shows no file size (scan rules: %v, stale: %v)", rel, tests, stale)
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
		var want []string
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				want = append(want, fmt.Sprintf("%s %d", layoutrules.FuncName(fd), layoutrules.FuncLines(fset, fd)))
			}
		}
		funcs += sameFuncSizes(t, rel, want, got)
	}
	noSizesOffTheWalk(t, ix, g, ".go", walked)
	enoughCompared(t, ".go", len(walked), 1000, funcs, 3000)
}

// sameFuncSizes codegraph'ın fonksiyon boylarını (got) düzen testinin ölçtüğü "ad boy" listesiyle (want) sırasız
// karşılaştırır, farkta düşer; karşılaştırılan fonksiyon sayısını döndürür (TestSizesMatchTheLayoutTest ve
// TestTRSizesMatchTheLayoutTest'in ortak adımı).
func sameFuncSizes(t *testing.T, rel string, want []string, got []sized) int {
	t.Helper()
	have := make([]string, 0, len(got))
	for _, s := range got {
		have = append(have, fmt.Sprintf("%s %d", s.name, s.lines))
	}
	slices.Sort(want)
	slices.Sort(have)
	if !slices.Equal(want, have) {
		t.Errorf("%s: codegraph's functions %v, the layout test's %v", rel, have, want)
	}
	return len(want)
}

// noSizesOffTheWalk düzen testinin ölçmediği (walked'da olmayan) ext uzantılı bir graf dosyası boyut satırı alırsa düşer.
func noSizesOffTheWalk(t *testing.T, ix *index, g *Graph, ext string, walked map[string][]byte) {
	t.Helper()
	for i := range g.Nodes {
		file := &g.Nodes[i]
		if file.Kind != kindFile || !strings.HasSuffix(file.File, ext) || walked[file.File] != nil {
			continue
		}
		if _, _, _, tests := ix.fileSizes(file); len(tests) > 0 {
			t.Error(untrackedOrUnmeasured(ix.root, file.File))
		}
	}
}

// enoughCompared karşılaştırma boş geçmesin diye dosya ve fonksiyon sayısının alt sınırıdır: yürüyüş ya da graf
// bozulup az dosya verirse test sessizce geçmez.
func enoughCompared(t *testing.T, ext string, files, minFiles, funcs, minFuncs int) {
	t.Helper()
	if files < minFiles || funcs < minFuncs {
		t.Fatalf("only %d %s files and %d functions compared", files, ext, funcs)
	}
}

// Kural: boyut satırında sınırı aşan önce, sonra sınıra oranca en yakın; istisnanın sınırı tavan + paydır ve satır
// bunu söyler; %90 ve üstü "split before adding" der.
func TestTheSizeLinePutsTheNearestFirst(t *testing.T) {
	t.Parallel()
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

// trLayoutWalk düzen testinin ölçtüğü .tr dosyalarıdır, codegraph'tan bağımsız yürünür (internal/layout_tr_test.go'nun
// trSourceFiles'ı gibi): git'in izlediği, layoutrules.TRMeasured'ın ölçtüğü .tr dosyaları. Dosya yolu -> içerik.
func trLayoutWalk(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := reporoot.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".tr" {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if !layoutrules.TRMeasured(rel) {
			return nil
		}
		data, err := os.ReadFile(p)
		out[rel] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Kural: .tr dosyasında da boyut satırının kapsamı ve değeri düzen testininkidir (TestLayoutTRFileSize,
// TestLayoutTRFuncSize): düzen testinin ölçtüğü her .tr dosyası (yolu codegraph'tan bağımsız yürünür: trLayoutWalk) iki
// kuralla boyut satırı alır; dosya boyu ve sınırı (istisnada tavan + pay), her fonksiyonun adı, satırı ve boyu düzenin
// ölçüsüyle (layoutrules.TRLines, TRFuncs, TRFileMax) eşittir; codegraph'ın ölçmediği ya da fazladan ölçtüğü .tr dosyası düşer.
// Graf testdata gibi atlanan klasörlerin .tr dosyalarını okumaz (layoutrules.SkippedDir; düzen testi onları ölçer): o
// dosyaların boyut satırı yoktur, boyları düzen testinde kalır.
// Satır cevapta koşu satırlarının altında, "size:" ile başlar, kurallar sonda (2026-11-23: lib/pretrained.tr T1'in
// dalında 750 satıra çıktı, cevapta boyut satırı yoktu).
func TestTRSizesMatchTheLayoutTest(t *testing.T) {
	t.Parallel()
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	walked := trLayoutWalk(t, root)
	funcs := 0
	for rel, data := range walked {
		file := ix.byID[trFileID(rel)]
		if file == nil && slices.ContainsFunc(strings.Split(path.Dir(rel), "/"), layoutrules.SkippedDir) {
			continue
		}
		if file == nil {
			t.Errorf("%s: the layout test measures it, the graph has no node for it", rel)
			continue
		}
		whole, got, _, tests := ix.fileSizes(file)
		if !slices.Equal(tests, []string{trFileSizeTest, trFuncSizeTest}) || whole == nil {
			t.Errorf("%s: the layout test measures it, tests-for shows %v", rel, tests)
			continue
		}
		lines := layoutrules.TRLines(data)
		if max, exc := layoutrules.TRFileMax(rel); whole.lines != len(lines) || whole.max != max || whole.exception != exc {
			t.Errorf("%s: codegraph's file size %d/%d (exception %v), the layout test's %d/%d (exception %v)", rel, whole.lines, whole.max, whole.exception, len(lines), max, exc)
		}
		var want []string
		for _, f := range layoutrules.TRFuncs(lines) {
			want = append(want, fmt.Sprintf("%s:%d %d", f.Name, f.Line, f.Lines))
		}
		funcs += sameFuncSizes(t, rel, want, got)
	}
	noSizesOffTheWalk(t, ix, g, ".tr", walked)
	enoughCompared(t, ".tr", len(walked), 100, funcs, 500)
	for file, want := range map[string]string{
		"lib/pretrained.tr": "\nsize: file %d/%d (an exception: its limit is the ceiling plus the fix margin",
		"lib/audio.tr":      "\nsize: file %d/%d; functions",
	} {
		var out strings.Builder
		ix.printTestsForShort(&out, ix.byID[trFileID(file)])
		max, _ := layoutrules.TRFileMax(file)
		head := fmt.Sprintf(want, len(layoutrules.TRLines(walked[file])), max)
		run := strings.Index(out.String(), "\nrun: ")
		if size := strings.Index(out.String(), head); run < 0 || size < run || !strings.Contains(out.String(), "(TestLayoutTRFileSize, TestLayoutTRFuncSize)\n") {
			t.Errorf("%s: the short answer has the size line %q under its run: lines:\n%s", file, head, out.String())
		}
	}
}

// Kural: kanca tests-for cevabının ilk satırlarını gösterir (tools/edithook, maxLines); koşu satırları ve boyut satırı
// dosya satırının hemen altında, bildirim sayısından ve test gruplarından öncedir: liste ne kadar uzun olsa da görünür.
// Baş en çok 8 satırdır: dosya, üç koşu satırı, altın dosya, iki "not run" satırı (kalan Go testleri ve codegraph'ın
// kendi testleri, S3; codegraph dilim 6, D6-3), boyut.
func TestTheRunAndSizeLinesComeFirst(t *testing.T) {
	t.Parallel()
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	for _, file := range runPlanFiles(g) {
		var out strings.Builder
		ix.printTestsFor(&out, file, 12)
		lines := strings.Split(out.String(), "\n")
		decl := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "  declarations in this file: ") })
		run := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "run: go test -short ./") })
		size := slices.IndexFunc(lines, func(l string) bool {
			return strings.HasPrefix(l, sizeLinePrefix+"file ") && strings.Contains(l, "(TestLayoutFileSize")
		})
		if run != 1 || size < 0 || size > decl || decl > 8 {
			t.Errorf("%s: run line at %d, size line at %d, declarations at %d:\n%s", file.File, run, size, decl, strings.Join(lines[:min(len(lines), 8)], "\n"))
		}
	}
}

// brokenBuildFixture derlemesi kırık (`var _ = tanimsizAd`) ve `Checker.tupleLen` fonksiyonu 80 satır sınırını aşacak
// biçimde uzatılmış geçici bir kök klasör kurar.
func brokenBuildFixture(t *testing.T, repoRoot string) string {
	t.Helper()
	root := t.TempDir()
	rel := filepath.FromSlash("internal/lang/checker/unpack.go")
	data, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	extra := strings.Repeat("\n\t_ = 0", 69) + "\n\tvar _ = tanimsizAd"
	src := strings.Replace(string(data), "func (c *Checker) tupleLen(e ast.Expression) int {", "func (c *Checker) tupleLen(e ast.Expression) int {"+extra, 1) + "\n\n"
	if src == string(data) {
		t.Fatal("Checker.tupleLen signature not found in unpack.go")
	}
	dst := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// Kural: derleme kırıkken ve yeniden kurulum düşüp eski (bayat) grafik kullanılırken de tests-for fonksiyon boylarını
// graftan değil diskteki dosyadan go/parser ile ölçer; sınırı aşan ya da sınıra yaklaşan fonksiyon bayat graftaki eski
// boyuyla gizlenmez. Dosya sözdizimi hatası yüzünden ayrıştırılamıyorsa boyut satırı fonksiyon boyunun bayat olduğunu
// açıkça söyler.
func TestSizeLineReadsFunctionsFromDiskWhenBuildIsBrokenAndReportsStaleOnSyntaxError(t *testing.T) {
	t.Parallel()
	repoRoot, _, g := repoGraph(t)
	root := brokenBuildFixture(t, repoRoot)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/sekerahmet/TCB\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := *g
	stale.Extractor = extractorRevision - 1
	if err := writeGraph(filepath.Join(root, ".codegraph", "graph.json"), &stale); err != nil {
		t.Fatal(err)
	}
	var out, errb strings.Builder
	// Bayraksız: boyut satırı kısa cevapta da var (codegraph dilim 6, D6-1).
	if code := run([]string{"tests-for", "--root", root, "internal/lang/checker/unpack.go"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for exit %d: %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "rebuild failed") || !strings.Contains(errb.String(), "using the existing, stale graph") {
		t.Errorf("expected stale graph warning on stderr, got: %s", errb.String())
	}
	wantOver := "\nsize: file 135/600; functions at >=90% of their limit: Checker.tupleLen 88/80 (over the limit: the test fails) (TestLayoutFileSize, TestLayoutFuncSize)\n"
	if !strings.Contains(out.String(), wantOver) {
		t.Errorf("broken build did not report fresh function size %q:\n%s", wantOver, out.String())
	}
	rel := filepath.Join(root, filepath.FromSlash("internal/lang/checker/unpack.go"))
	if err := os.WriteFile(rel, []byte("package checker\n\nfunc broken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := runQueryOn(g, "tests-for", []string{"--root", root, "internal/lang/checker/unpack.go"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for on syntax error exit %d: %s", code, errb.String())
	}
	wantStale := "\nsize: file 3/600; functions: stale (syntax error: cannot parse file) (TestLayoutFileSize, TestLayoutFuncSize)\n"
	if !strings.Contains(out.String(), wantStale) {
		t.Errorf("syntax error did not report stale function sizes %q:\n%s", wantStale, out.String())
	}
}

// Kural: düzen testinin ölçmediği dosyanın iletisi git'in izlemediği yeni dosyada `git add` der, izlenen dosyada iki
// aracın ayrıldığını söyler (gaps, "Yeni dosyada yerleşim testinin iletisi yanıltıcı").
func TestAnUntrackedFileIsToldToBeAddedNotUnmeasured(t *testing.T) {
	t.Parallel()
	root, _ := untrackedGitFixture(t)
	if got := untrackedOrUnmeasured(root, "shape/untracked.go"); !strings.Contains(got, "not tracked by git") || !strings.Contains(got, "git add shape/untracked.go") {
		t.Errorf("untracked file: %s", got)
	}
	if got := untrackedOrUnmeasured(root, "shape/shape.go"); !strings.Contains(got, "the layout test does not measure it") {
		t.Errorf("tracked file: %s", got)
	}
}
