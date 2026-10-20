// Layer: tools (tools); not shipped to users.
package bench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// python returns the Python interpreter, or skips the test when there is none.
func python(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			// A Windows Store stub passes LookPath but fails to run.
			if exec.Command(p, "-I", "-c", "").Run() == nil {
				return p
			}
		}
	}
	t.Skip("python not found")
	return ""
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command(python(t), append([]string{"-I", "run.py"}, args...)...).CombinedOutput()
	return string(out), err
}

// run.py must show its options and reject unknown languages and tests without building anything.
func TestRunPyArguments(t *testing.T) {
	out, err := run(t, "--help")
	if err != nil {
		t.Fatalf("--help: %v\n%s", err, out)
	}
	for _, want := range []string{"--repeat", "--only", "--json", "--list"} {
		if !strings.Contains(out, want) {
			t.Errorf("--help lacks %s:\n%s", want, out)
		}
	}
	for _, bad := range [][]string{{"--only", "java"}, {"nosuchtest"}, {"--repeat", "x"}} {
		if out, err := run(t, bad...); err == nil {
			t.Errorf("%v accepted:\n%s", bad, out)
		}
	}
}

// Every test run.py lists has a program in each of the three languages.
func TestRunPyTestsExist(t *testing.T) {
	out, err := run(t, "--list")
	if err != nil {
		t.Fatalf("--list: %v\n%s", err, out)
	}
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	goSrc, pySrc := read(filepath.Join("go", "compare.go")), read(filepath.Join("py", "compare.py"))
	names := strings.Fields(out)
	if len(names) == 0 {
		t.Fatal("run.py --list printed nothing")
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join("tr", name+".tr")); err != nil {
			t.Errorf("%s: no TR program: %v", name, err)
		}
		if !strings.Contains(goSrc, `case "`+name+`"`) {
			t.Errorf("%s: no case in go/compare.go", name)
		}
		if !strings.Contains(pySrc, `"`+name+`"`) {
			t.Errorf("%s: not in py/compare.py", name)
		}
	}
}
