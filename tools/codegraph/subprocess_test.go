package main

import (
	"strings"
	"testing"
)

// Kural: bir test dosyası `go build` ile bir ana paketi derleyip ikiliyi çalıştırıyorsa o paketin işlevleri için
// tests-for "no tests found" demez; ikiliyi koşan testleri klasör başına sayan "covered by subprocess tests" satırını
// yazar. TestMain'de derleyen paketin bütün testleri, bir yardımcıda derleyen paketin o yardımcıya varan testleri
// sayılır; derlenmeyen paketin işlevi testsiz kalır (bağımsız denetim Y5).
func TestTestsForCountsSubprocessTestsOfABuiltPackage(t *testing.T) {
	root := auditModule(t, map[string]string{
		"app/main.go": "package main\n\nfunc main() { greet() }\n\nfunc greet() {}\n\nfunc farewell() {}\n",
		"app/main_test.go": `package main

import (
	"os/exec"
	"testing"
)

func buildApp(t *testing.T) { _ = exec.Command("go", "build", "-o", t.TempDir(), ".").Run() }

func TestBinary(t *testing.T) { buildApp(t) }

func TestGreet(t *testing.T) { greet() }
`,
		"app/e2e/e2e_test.go": `package e2e

import (
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	_ = exec.Command("go", "build", "-o", os.TempDir(), "example.com/audit/app").Run()
	os.Exit(m.Run())
}

func TestRuns(t *testing.T) {}

func TestOther(t *testing.T) {}
`,
		"lib/lib.go": "package lib\n\nfunc Helper() {}\n",
	})
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	ask := func(name string) string {
		var out, errb strings.Builder
		if code := run([]string{"tests-for", name, "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
			t.Fatalf("tests-for %s: %d %s", name, code, errb.String())
		}
		return out.String()
	}
	got := ask("app.farewell")
	if strings.Contains(got, "no tests found") || !strings.Contains(got, "covered by subprocess tests (3;") ||
		!strings.Contains(got, "app (1)") || !strings.Contains(got, "app/e2e (2)") {
		t.Errorf("farewell: TestBinary through buildApp and the two e2e tests, TestMain not counted:\n%s", got)
	}
	if got := ask("app.greet"); !strings.Contains(got, "app.TestGreet") || !strings.Contains(got, "covered by subprocess tests") {
		t.Errorf("greet: its direct test and the subprocess line:\n%s", got)
	}
	if got := ask("lib.Helper"); !strings.Contains(got, "no tests found") {
		t.Errorf("lib is not built by any test:\n%s", got)
	}
}
