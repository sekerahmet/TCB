package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// dupTree geçici bir ağaç kurar: anahtar yol, değer ya testdata/dups'taki bir örnek dosyanın adı
// (".go.txt" olmadan) ya da "src:" ile başlayan satır içi kaynaktır.
func dupTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, what := range files {
		var body []byte
		if src, ok := strings.CutPrefix(what, "src:"); ok {
			body = []byte(src)
		} else {
			var err error
			if body, err = os.ReadFile(filepath.Join("testdata", "dups", what+".go.txt")); err != nil {
				t.Fatal(err)
			}
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func dupsOf(t *testing.T, root string, min int, rules bool) ([]*dupCluster, map[string]int) {
	t.Helper()
	co, _, err := loadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	clusters, _ := co.findDups(min)
	if !rules {
		return clusters, nil
	}
	return applyRules(clusters, co)
}

// hasCluster kümelerde verilen dosya:işlev konumlarının hepsini taşıyan var mı.
func hasCluster(clusters []*dupCluster, want ...string) *dupCluster {
next:
	for _, c := range clusters {
		for _, w := range want {
			found := false
			for _, l := range c.Locs {
				if l.File+":"+l.Func == w || l.File == w {
					found = true
				}
			}
			if !found {
				continue next
			}
		}
		return c
	}
	return nil
}

// Gemini tekrar denetiminde (Parti 3 ve 4, onaylı bulgular) bulunan kopyalar, bulundukları kodun
// örnekleriyle: araç hepsini yakalar. Örnekler testdata/dups'tadır (main 04c83dc0'dan alıntı).
func TestDupsFindsKnownDuplicates(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		min   int
		want  []string
		exact bool // adlar ve değerler de aynı mı
	}{
		{"teeWriter in two test packages (Parti 3, Bulgu 8)", map[string]string{
			"internal/stdlib/checkpoint/checkpoint_test.go": "tee_checkpoint_test",
			"internal/stdlib/logging/log_test.go":           "tee_logging_test",
		}, 40, []string{"internal/stdlib/checkpoint/checkpoint_test.go", "internal/stdlib/logging/log_test.go"}, true},
		{"resign closures (Parti 3, Bulgu 1)", map[string]string{
			"internal/stdlib/checkpoint/a_test.go": "resign_a",
			"internal/stdlib/checkpoint/b_test.go": "resign_b",
		}, 80, []string{"internal/stdlib/checkpoint/a_test.go", "internal/stdlib/checkpoint/b_test.go"}, true},
		{"registerIsFinite and registerRounding (Parti 4, Bulgu 13)", map[string]string{
			"internal/stdlib/ml/builtin_math.go": "math_wrappers",
		}, 100, []string{"internal/stdlib/ml/builtin_math.go:registerIsFinite", "internal/stdlib/ml/builtin_math.go:registerRounding"}, false},
	}
	for _, c := range cases {
		clusters, _ := dupsOf(t, dupTree(t, c.files), c.min, true)
		got := hasCluster(clusters, c.want...)
		if got == nil {
			t.Errorf("%s: not found among %d clusters", c.name, len(clusters))
			continue
		}
		if got.Diff.Identical != c.exact {
			t.Errorf("%s: identical = %v, want %v (%+v)", c.name, got.Diff.Identical, c.exact, got.Diff)
		}
		for _, l := range got.Locs {
			if l.From <= 0 || l.To < l.From {
				t.Errorf("%s: bad span %+v", c.name, l)
			}
		}
	}
}

func TestDupsThresholdAndDeterminism(t *testing.T) {
	root := dupTree(t, map[string]string{"a_test.go": "tee_checkpoint_test", "b_test.go": "tee_logging_test"})
	if clusters, _ := dupsOf(t, root, 5000, false); len(clusters) != 0 {
		t.Errorf("nothing is that big: %d", len(clusters))
	}
	var outputs []string
	for i := 0; i < 2; i++ {
		var out, errb bytes.Buffer
		if code := run([]string{"dups", "--root", root, "--min", "30", "--json"}, &out, &errb); code != exitOK {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		outputs = append(outputs, out.String())
	}
	if outputs[0] != outputs[1] {
		t.Error("two runs print different output")
	}
	var res dupResult
	if err := json.Unmarshal([]byte(outputs[0]), &res); err != nil || res.Schema != dupSchema || len(res.Clusters) != 1 || len(res.Clusters[0].Locs) != 2 {
		t.Errorf("json: %v %+v", err, res)
	}
}

// loopSource aynı biçimde, yalnızca sayı türü farklı bir işlev yazar.
func loopSource(name, typ string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package p\n\nfunc %s(x []%s) %s {\n\tvar s %s\n", name, typ, typ, typ)
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&b, "\tfor i := range x {\n\t\ts += x[i] * x[i]\n\t\tif s > %d {\n\t\t\ts -= x[i]\n\t\t}\n\t}\n", i)
	}
	b.WriteString("\treturn s\n}\n")
	return "src:" + b.String()
}

// Bilerek ayrı tutulan yerler uyarı vermez; kural kapatılınca (--no-exclude) görünürler.
func TestDupsExclusionsSilenceTheWarning(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		rule  string
	}{
		{"gpu simulator vs tensor", map[string]string{
			"internal/compute/gpu/sim_ops.go":   loopSource("simSum", "float32"),
			"internal/compute/tensor/reduce.go": loopSource("sum", "float32"),
		}, "gpu-simulator-vs-tensor"},
		{"float64 reference in a test", map[string]string{
			"internal/compute/tensor/ref_test.go": loopSource("refSum", "float64"),
			"internal/compute/tensor/reduce.go":   loopSource("sum", "float32"),
		}, "test-float64-reference"},
	}
	for _, c := range cases {
		root := dupTree(t, c.files)
		if raw, _ := dupsOf(t, root, 60, false); len(raw) == 0 {
			t.Fatalf("%s: the pair must be a duplicate without the rules", c.name)
		}
		kept, silenced := dupsOf(t, root, 60, true)
		if len(kept) != 0 || silenced[c.rule] != 1 {
			t.Errorf("%s: kept %d, silenced %v", c.name, len(kept), silenced)
		}
		var out, errb bytes.Buffer
		run([]string{"dups", "--root", root, "--min", "60", "--no-exclude"}, &out, &errb)
		if !strings.Contains(out.String(), "1 duplicate candidate") {
			t.Errorf("%s: --no-exclude shows it:\n%s", c.name, out.String())
		}
	}
	// Aynı çift iki üretim dosyasındaysa kural uygulanmaz.
	prod := dupTree(t, map[string]string{"a/x.go": loopSource("f", "float32"), "b/y.go": loopSource("g", "float32")})
	if kept, _ := dupsOf(t, prod, 60, true); len(kept) != 1 {
		t.Errorf("two production copies are reported: %d", len(kept))
	}
}

// Üretilmiş dosyalar okunmaz; sınama tablolarının benzer satırları tekrar sayılmaz.
func TestDupsIgnoresGeneratedFilesAndTestTables(t *testing.T) {
	generated := "src:// Code generated by go run ./tools/gen/x; DO NOT EDIT.\n\n" + strings.TrimPrefix(loopSource("f", "int"), "src:")
	root := dupTree(t, map[string]string{"a/gen.go": generated, "b/gen2.go": generated})
	if clusters, _ := dupsOf(t, root, 60, false); len(clusters) != 0 {
		t.Errorf("generated copies are not duplicates: %d", len(clusters))
	}
	table := func(name string) string {
		var b strings.Builder
		fmt.Fprintf(&b, "package p\n\nvar %s = []struct{ a, b int }{\n", name)
		for i := 0; i < 60; i++ {
			fmt.Fprintf(&b, "\t{%d, %d},\n", i, i*2)
		}
		b.WriteString("}\n")
		return "src:" + b.String()
	}
	tests := dupTree(t, map[string]string{"a/x_test.go": table("rowsA"), "b/y_test.go": table("rowsB")})
	if clusters, _ := dupsOf(t, tests, 60, false); len(clusters) != 0 {
		t.Errorf("test tables are one leaf: %d", len(clusters))
	}
	prod := dupTree(t, map[string]string{"a/x.go": table("rowsA"), "b/y.go": table("rowsB")})
	if clusters, _ := dupsOf(t, prod, 60, false); len(clusters) != 0 {
		t.Errorf("a production table of 8 or more elements is data too: %d", len(clusters))
	}
}

func TestMatchGlob(t *testing.T) {
	for _, c := range []struct {
		pattern, path string
		want          bool
	}{
		{"internal/compute/gpu/sim*.go", "internal/compute/gpu/sim_ops.go", true},
		{"internal/compute/gpu/sim*.go", "internal/compute/gpu/kernel.go", false},
		{"internal/compute/tensor/**", "internal/compute/tensor/a/b.go", true},
		{"internal/compute/tensor/**", "internal/compute/gpu/b.go", false},
		{"**/*_test.go", "a/b/c_test.go", true},
		{"**/*_test.go", "a/b/c.go", false},
	} {
		if got := matchGlob(c.pattern, c.path); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v", c.pattern, c.path, got)
		}
	}
}

// Yapısı tam aynı olmayan (yakın) kopyaları dups değil similar bulur: Parti 3 Bulgu 14 (ShortNumber/FormatBytes),
// Parti 4 Bulgu 13 (registerIsFinite/registerRounding) ve Bulgu 2 (exprNames/exprCaptures).
func TestSimilarFindsNearMissCopies(t *testing.T) {
	for _, c := range []struct {
		fixture, target, closest string
		score                    float64
	}{
		{"bind_numbers", "ShortNumber", "FormatBytes", 0.6},
		{"math_wrappers", "registerIsFinite", "registerRounding", 0.6},
		{"scope_walks", "exprNames", "blockFlagCache.exprCaptures", 0.3},
	} {
		co, _, err := loadCorpus(dupTree(t, map[string]string{"a.go": c.fixture}))
		if err != nil {
			t.Fatal(err)
		}
		found := co.findFunc(c.target)
		if len(found) != 1 {
			t.Fatalf("%s: %d", c.target, len(found))
		}
		res := co.similarTo(found[0], 3)
		if len(res.Matches) == 0 || res.Matches[0].Func != c.closest || res.Matches[0].Score < c.score {
			t.Errorf("%s: %+v", c.target, res.Matches)
		}
	}
}

func TestSimilarRanksTheTemplateCopyFirst(t *testing.T) {
	root := dupTree(t, map[string]string{"bind.go": "bind_numbers", "other.go": loopSource("loop", "int")})
	co, _, err := loadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	found := co.findFunc("ShortNumber")
	if len(found) != 1 {
		t.Fatalf("ShortNumber: %d", len(found))
	}
	res := co.similarTo(found[0], 5)
	if len(res.Matches) == 0 || res.Matches[0].Func != "FormatBytes" || res.Matches[0].Score < 0.5 {
		t.Errorf("FormatBytes is the closest: %+v", res.Matches)
	}
	for _, m := range res.Matches {
		if m.Func == "loop" {
			t.Errorf("an unrelated function must not rank: %+v", m)
		}
	}
	var out, errb bytes.Buffer
	if code := run([]string{"similar", "--root", root, "nope"}, &out, &errb); code != exitNotFound {
		t.Errorf("unknown function: %d", code)
	}
}

// copyBranchRepo main'de iki benzemeyen dosya ve ondan ayrılan claude/new dalında bunlardan birine
// benzeyen yeni bir dosya (new.go) taşıyan geçici bir git deposu kurar.
func copyBranchRepo(t *testing.T) string {
	t.Helper()
	root := dupTree(t, map[string]string{"old1.go": loopSource("a", "int"), "old2.go": loopSource("b", "int")})
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "base")
	gitIn(t, root, "checkout", "-q", "-b", "claude/new")
	testkit.WritePath(t, filepath.Join(root, "new.go"), strings.TrimPrefix(loopSource("c", "int"), "src:"))
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "copy")
	return root
}

// gitIn bir git komutunu dizinde çalıştırır.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git %v: %v %s", args, err, out)
	}
}

// --changed yalnızca dalın eklediği koda dokunan adayları bırakır: dalda var olan koda benzeyen yeni
// işlev yakalanır, daldan önce var olan çift (dalın dokunmadığı) görünmez.
func TestDupsChangedOnlyReportsWhatTheBranchTouched(t *testing.T) {
	root := copyBranchRepo(t)
	var out, errb bytes.Buffer
	if code := run([]string{"dups", "--root", root, "--min", "60", "--json", "--changed", "claude/new"}, &out, &errb); code != exitOK {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var res dupResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || len(res.Clusters[0].Locs) != 3 {
		t.Fatalf("one cluster of the three copies touches the branch: %+v", res)
	}
	changed := 0
	for _, l := range res.Clusters[0].Locs {
		if l.Changed {
			changed++
			if l.File != "new.go" {
				t.Errorf("only new.go changed: %+v", l)
			}
		}
	}
	if changed != 1 {
		t.Errorf("one location is marked changed: %d", changed)
	}
	// Kontrol edilmemiş dal: satır numaraları çalışma ağacının olduğundan reddedilir.
	gitIn(t, root, "checkout", "-q", "main")
	out.Reset()
	errb.Reset()
	if code := run([]string{"dups", "--root", root, "--min", "60", "--changed", "main..claude/new"}, &out, &errb); code != exitFail || !strings.Contains(errb.String(), "check the branch out") {
		t.Errorf("a branch that is not checked out: %d %s", code, errb.String())
	}
}

// main sonradan ilerlediyse (dalın tabanından sonra bir kopya main'de kaldırıldı) hiçbir --changed biçimi
// main'deki bu işi dalın değişikliği saymaz: yalnızca birleşme tabanından beri dalda eklenen satırlar.
func TestDupsChangedUsesTheMergeBaseNotTheTreeDiff(t *testing.T) {
	root := copyBranchRepo(t)
	// main ilerler: old2.go'daki kopya oradan kaldırılır; dal eski tabanda kalır.
	gitIn(t, root, "checkout", "-q", "main")
	gitIn(t, root, "rm", "-q", "old2.go")
	gitIn(t, root, "commit", "-q", "-m", "remove one copy on main")
	gitIn(t, root, "checkout", "-q", "claude/new")
	for _, arg := range []string{"main..HEAD", "main...HEAD", "main", "claude/new", "main..", "..HEAD"} {
		var out, errb bytes.Buffer
		if code := run([]string{"dups", "--root", root, "--min", "60", "--json", "--changed", arg}, &out, &errb); code != exitOK {
			t.Fatalf("%s: exit %d: %s", arg, code, errb.String())
		}
		var res dupResult
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		var changed []string
		for _, c := range res.Clusters {
			for _, l := range c.Locs {
				if l.Changed {
					changed = append(changed, l.File)
				}
			}
		}
		if len(changed) != 1 || changed[0] != "new.go" {
			t.Errorf("--changed %s marks %v, want only new.go (what the branch added since the merge base)", arg, changed)
		}
	}
	// Dal main'e birleşmiş gibi, sağ uç çıkış yapılmış değilken: main'in ucu için aralık reddedilir.
	var out, errb bytes.Buffer
	if code := run([]string{"dups", "--root", root, "--min", "60", "--changed", "claude/new..main"}, &out, &errb); code != exitFail {
		t.Errorf("a range whose right end is not the checked-out commit: %d", code)
	}
}
