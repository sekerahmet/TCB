// Layer: tools (tools); not shipped to users.
package bench

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// samples writes one benchmark's results in the go test format.
func samples(t *testing.T, dir, name string, ns ...int) string {
	t.Helper()
	var b strings.Builder
	for _, v := range ns {
		fmt.Fprintf(&b, "BenchmarkStep-8   \t 400\t %d ns/op\t 12 B/op\n", v)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The gate's one rule: a difference counts when it lies outside the copy's difference and the
// quartiles of the two arms do not meet. A slowdown that counts exits with 1, nothing else does.
func TestCompareCommitsVerdict(t *testing.T) {
	base := []int{100, 101, 102, 103, 104, 105, 106}
	cases := []struct {
		name       string
		head, copy []int
		want       string
		fails      bool
	}{
		{"slower", []int{120, 121, 122, 123, 124, 125, 126}, []int{101, 102, 103, 104, 105, 106, 107}, "gerçek", true},
		{"faster", []int{80, 81, 82, 83, 84, 85, 86}, []int{101, 102, 103, 104, 105, 106, 107}, "gerçek", false},
		{"quartiles meet", []int{104, 105, 106, 107, 108, 109, 110}, []int{100, 101, 102, 103, 104, 105, 106}, "gerçek değil", false},
		{"inside the copy", []int{120, 121, 122, 123, 124, 125, 126}, []int{125, 126, 127, 128, 129, 130, 131}, "gerçek değil", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(python(t), "-I", "compare_commits.py", "--verdict",
				samples(t, dir, "base.txt", base...), samples(t, dir, "head.txt", c.head...), samples(t, dir, "copy.txt", c.copy...))
			out, err := cmd.CombinedOutput()
			if (err != nil) != c.fails {
				t.Errorf("exit: %v; fails = %v\n%s", err, c.fails, out)
			}
			if !strings.HasSuffix(strings.TrimSpace(string(out)), ", "+c.want) {
				t.Errorf("verdict %q lacks %q", out, c.want)
			}
		})
	}
}

// A gate that finds no benchmark says so: a misspelled name must not pass as "no regression".
func TestCompareCommitsNoBenchmark(t *testing.T) {
	dir := t.TempDir()
	empty := samples(t, dir, "empty.txt")
	full := samples(t, dir, "full.txt", 100, 101, 102, 103)
	for _, files := range [][]string{{empty, empty, empty}, {full, empty, full}, {full, full, empty}} {
		cmd := exec.Command(python(t), append([]string{"-I", "compare_commits.py", "--verdict"}, files...)...)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(out), "kıyas bulunamadı") {
			t.Errorf("%v: %v\n%s", files, err, out)
		}
	}
}

// gateRepo builds a two-commit module in a temporary repository: each commit's benchmark sleeps for
// the given milliseconds. Work is the symbol whose alignment the gate reports.
func gateRepo(t *testing.T, baseMs, headMs int) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(ms int) {
		t.Helper()
		src := fmt.Sprintf(`package p

import (
	"testing"
	"time"
)

//go:noinline
func Work() { time.Sleep(%d * time.Millisecond) }

func BenchmarkStep(b *testing.B) {
	for range b.N {
		Work()
	}
}
`, ms)
		if err := os.WriteFile(filepath.Join(dir, "p", "p_test.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gate\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	write(baseMs)
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("tag", "base")
	write(headMs)
	git("add", ".")
	git("commit", "-q", "-m", "head")
	git("tag", "head")
	return dir
}

// runGate runs the whole gate on gateRepo's two commits.
func runGate(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	script, err := filepath.Abs("compare_commits.py")
	if err != nil {
		t.Fatal(err)
	}
	all := append([]string{"-I", script, "--base", "base", "--head", "head", "--package", "p", "--symbol", "Work",
		"--rounds", "3", "--count", "2", "--benchtime", "3x", "--out", filepath.Join(dir, "out")}, args...)
	cmd := exec.Command(python(t), all...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// The whole gate: both sides are built with the alignment flag and land aligned, the arms take turns
// in rotation, a real slowdown exits with 1, a misspelled benchmark with 2.
func TestCompareCommitsEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	dir := gateRepo(t, 1, 40)
	out, err := runGate(t, dir, "--bench", "BenchmarkStep")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("slowdown: %v\n%s", err, out)
	}
	for _, want := range []string{
		"base derlendi: -ldflags=-funcalign=64",
		"head derlendi: -ldflags=-funcalign=64",
		"base@64 Work:", "head@64 Work:",
		", gerçek\n",
	} {
		if !strings.Contains(strings.ReplaceAll(out, "\r\n", "\n"), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "64'e göre 0"); got != 2 {
		t.Errorf("%d aligned arms; 2 expected:\n%s", got, out)
	}
	var first []string
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Fields(line); len(f) == 4 && f[0] == "tur" {
			if len(first) == int(f[1][0]-'0') {
				first = append(first, f[2])
			}
		}
	}
	if len(first) != 3 || first[0] == first[1] || first[1] == first[2] {
		t.Errorf("rounds start with %v; the turn must rotate", first)
	}
	out, err = runGate(t, dir, "--bench", "BenchmarkNoSuch")
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Errorf("misspelled benchmark: %v\n%s", err, out)
	}
}

// Without its three commits the comparison refuses before it builds anything.
func TestCompareCommitsArguments(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "compare_commits.py", "--base", "x").CombinedOutput()
	if err == nil {
		t.Errorf("accepted without --head and --bench:\n%s", out)
	}
}
