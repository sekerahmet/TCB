package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// repoScanFixture scanFiles'ın depo taramalarını (checks paketi) ve tests ile pkgs listelerini taşıyan bir
// tools/full_check.sh'le git deposu kurar; grafın yolunu verir. Betiğin satırları gerçek betiğin düzen kuralları ve
// depo taramaları adımlarıyla aynı biçimdedir (tools/full_check.sh).
func repoScanFixture(t *testing.T, tests, pkgs []string) (root, graph string) {
	t.Helper()
	root = auditModule(t, scanFiles)
	writeRepoScanScript(t, root, tests, pkgs)
	gitMust(t, root, "init", "-q", "-b", "main")
	commitAll(t, root, "fixture")
	return root, buildInto(t, root)
}

func writeRepoScanScript(t *testing.T, root string, tests, pkgs []string) {
	t.Helper()
	testkit.WriteFile(t, root, repoScanScript, "#!/usr/bin/env bash\nrepo_scans=(\n\t"+strings.Join(tests, "\n\t")+"\n)\n"+
		"repo_scan_pkgs=(\n\t"+strings.Join(pkgs, "\n\t")+"\n)\n"+
		"\tgo test -short -count=1 ./internal/ 2>&1 | grep -v '^ok'\n"+
		"\t[ \"${PIPESTATUS[0]}\" -eq 0 ] || fail \"düzen kuralları\"\n"+
		"\tgo test -short -count=1 -run \"^($(printf '%s|' \"${repo_scans[@]}\" | sed 's/|$//'))\\$\" \\\n"+
		"\t\t\"${repo_scan_pkgs[@]}\" 2>&1 | grep -v '^ok'\n"+
		"\t[ \"${PIPESTATUS[0]}\" -eq 0 ] || fail \"depo taramaları\"\n")
}

// scanFixtureTests scanFiles'ta depoyu tarayan testlerdir (checks paketi; TestPackagesOnly klasör, TestCopy geçici
// klasör gezer: tarama değil).
var scanFixtureTests = []string{"TestByHelper", "TestGoOnly", "TestRoots", "TestTracked"}

// Kural (codegraph tarama listesi tasarımı, K2–K5): changed --go-tests seçimden sonra tam denetimin depo taramaları
// listelerini graftaki taramalarla karşılaştırır. Eksik test ya da paket eklenecek satırı ve yerini (S1, S2), fazla ad
// silinecek satırı (S3) söyler, sonda S4 ile 5'te çıkar; satır eklenince 0. Betik yoksa sessiz, okunamazsa ya da bir
// parçası yoksa nedeniyle 1 (S5); bayat grafta karşılaştırma S6 ile atlanır. Neden: listeye yazılmayan yeni bir tarama
// kipte geçiyor, G4'te düşüyordu (2026-11-14 – 2026-11-18'de dört tur).
func TestChangedGoTestsStopsWhenTheRepoScanListLacksATest(t *testing.T) {
	t.Parallel()
	changedGoTests := func(root, graph string, extra ...string) (int, string, string) {
		return commandIn("changed", root, graph, append([]string{"--go-tests", "--base", "HEAD", "HEAD"}, extra...)...)
	}
	stop := "codegraph: the commit check stops here: G4's TestFullCheckRepoScansMatchesScanNodes fails on the same lists"

	t.Run("lacking test", func(t *testing.T) {
		t.Parallel()
		root, graph := repoScanFixture(t, []string{"TestByHelper", "TestRoots", "TestTracked"}, []string{"./checks"})
		code, out, errOut := changedGoTests(root, graph)
		if code != exitScanList {
			t.Fatalf("exit %d, want %d:\n%s%s", code, exitScanList, out, errOut)
		}
		testkit.MustContain(t, errOut, "codegraph: tools/full_check.sh: the repo_scans list lacks TestGoOnly, a test that scans "+
			"the repository (checks/checks_test.go:", "); add this line to repo_scans after the line TestByHelper:\n\tTestGoOnly\n", stop)
		if strings.Contains(errOut, "repo_scan_pkgs") {
			t.Errorf("only repo_scans lacks a name:\n%s", errOut)
		}
		writeRepoScanScript(t, root, scanFixtureTests, []string{"./checks"})
		if code, _, errOut := changedGoTests(root, graph); code != exitOK || strings.Contains(errOut, "repo_scans") {
			t.Errorf("with the line added: exit %d:\n%s", code, errOut)
		}
	})
	t.Run("lacking first test and package, extra names", func(t *testing.T) {
		t.Parallel()
		root, graph := repoScanFixture(t, []string{"TestGoOnly", "TestGone", "TestRoots", "TestTracked"}, []string{"./old"})
		code, _, errOut := changedGoTests(root, graph)
		if code != exitScanList {
			t.Fatalf("exit %d, want %d:\n%s", code, exitScanList, errOut)
		}
		testkit.MustContain(t, errOut, "TestByHelper, a test that scans the repository (checks/checks_test.go:",
			"); add this line as the first line of repo_scans:\n\tTestByHelper\n",
			"codegraph: tools/full_check.sh: the repo_scan_pkgs list lacks ./checks, the package of TestGoOnly; "+
				"add this line as the first line of repo_scan_pkgs:\n\t./checks\n",
			"codegraph: tools/full_check.sh: the repo_scans list names TestGone, which scans no repository files now; remove its line\n",
			"codegraph: tools/full_check.sh: the repo_scan_pkgs list names ./old, which scans no repository files now; remove its line\n",
			stop)
	})
	t.Run("no script", func(t *testing.T) {
		t.Parallel()
		root, graph := repoScanFixture(t, nil, nil)
		gitMust(t, root, "rm", "-q", repoScanScript)
		commitAll(t, root, "no script")
		if code, _, errOut := changedGoTests(root, graph); code != exitOK || strings.Contains(errOut, "repo_scans") {
			t.Errorf("without tools/full_check.sh: exit %d:\n%s", code, errOut)
		}
	})
	t.Run("stale graph", func(t *testing.T) {
		t.Parallel()
		root, graph := repoScanFixture(t, nil, []string{"./checks"})
		testkit.WriteFile(t, root, "lib/a.go", "package lib\n\nfunc A() {}\n")
		code, _, errOut := changedGoTests(root, graph, "--no-rebuild")
		if code != exitOK || strings.Contains(errOut, "lacks") {
			t.Errorf("stale graph: exit %d, want 0 without the comparison:\n%s", code, errOut)
		}
		testkit.MustContain(t, errOut, staleRepoScans+"\n")
	})
	t.Run("unreadable script", func(t *testing.T) {
		t.Parallel()
		root, graph := repoScanFixture(t, scanFixtureTests, []string{"./checks"})
		script := filepath.Join(root, repoScanScript)
		good, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		cut := func(part string) string { return strings.Replace(string(good), part, "", 1) }
		for _, c := range []struct{ text, reason string }{
			{cut("repo_scans=("), "array repo_scans not found"},
			{cut("repo_scan_pkgs=("), "array repo_scan_pkgs not found"},
			{cut(`|| fail "düzen kuralları"`), `the layout-rules go test line (… || fail "düzen kuralları") not found`},
			{cut(`"${repo_scan_pkgs[@]}" 2>&1`), `the depo taramaları step does not run ${repo_scans[@]} across "${repo_scan_pkgs[@]}"`},
		} {
			testkit.WriteFile(t, root, repoScanScript, c.text)
			code, _, errOut := changedGoTests(root, graph)
			if code != exitFail || !strings.Contains(errOut, "codegraph: tools/full_check.sh: cannot check the repo_scans list: "+c.reason+"\n") {
				t.Errorf("%s: exit %d, want 1 and the reason:\n%s", c.reason, code, errOut)
			}
		}
		// Var ama okunamayan betik (dosyanın yerinde klasör): damga da okunamadığından sorgu yolu bayat grafa (S6) düşer;
		// okuma hatasının kendi nedeni karşılaştırmanın işleviyle sınanır.
		if err := os.Remove(script); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(script, 0o755); err != nil {
			t.Fatal(err)
		}
		var w strings.Builder
		if code := checkRepoScans(root, &Graph{}, false, &w); code != exitFail ||
			!strings.HasPrefix(w.String(), "codegraph: tools/full_check.sh: cannot check the repo_scans list: cannot read: ") {
			t.Errorf("unreadable script: exit %d, want 1 and the reason:\n%s", code, w.String())
		}
	})
}
