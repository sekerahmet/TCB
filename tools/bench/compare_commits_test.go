// Layer: tools (tools); not shipped to users.
package bench

import (
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

// Without its three commits the comparison refuses before it builds anything.
func TestCompareCommitsArguments(t *testing.T) {
	out, err := exec.Command(python(t), "-I", "compare_commits.py", "--base", "x").CombinedOutput()
	if err == nil {
		t.Errorf("accepted without --head and --bench:\n%s", out)
	}
}
