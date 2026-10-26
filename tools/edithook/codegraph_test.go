package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// TestMain sahte araçları ve kancanın kiplerini de taşır: test ikilisi "go" adıyla kopyalanınca
// sahte derleyici, "codegraph" adıyla kopyalanınca sahte codegraph olur (cevabını yazar, 3 ile
// çıkar: ön denetimin yolu çıkış kodunu geçirmeli); `exe-path` kipi -exec programıdır.
func TestMain(m *testing.M) {
	switch strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") {
	case "go":
		os.Exit(fakeGo(os.Args[1:]))
	case "codegraph":
		fmt.Println("answer:", strings.Join(os.Args[1:], " "))
		os.Exit(3)
	}
	if code, ok := mode(os.Args[1:], os.Stdout, os.Stderr); ok {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fakeGo `go run -exec '<program>' <kip> ./tools/codegraph`'ı taklit eder: çalışma klasöründe
// "broken" dosyası varsa derleme hatası verir; yoksa yanındaki sahte codegraph'ı (gerekirse test
// ikilisinden kopyalar) derlenmiş ikili sayıp -exec programına verir.
func fakeGo(args []string) int {
	if _, err := os.Stat("broken"); err == nil {
		fmt.Fprintln(os.Stderr, "# github.com/sekerahmet/TCB/internal/lang/checker\nchecker.go:3:2: undefined: undefinedSymbolK2")
		return 1
	}
	if len(args) != 4 || args[0] != "run" || args[1] != "-exec" || args[3] != "./tools/codegraph" {
		fmt.Fprintln(os.Stderr, "fake go: unexpected arguments", args)
		return 1
	}
	prog, kip, ok := strings.Cut(strings.TrimPrefix(args[2], "'"), "' ")
	self, err := os.Executable()
	if !ok || err != nil {
		fmt.Fprintln(os.Stderr, "fake go: bad -exec", args[2], err)
		return 1
	}
	// Go gibi: önbellekte yoksa ikili geçici klasörde derlenir, -exec programına o yol verilir, klasör silinir ve ikili
	// önbelleğe girer; sonraki çağrı önbellekteki yolu verir.
	cached := filepath.Join(filepath.Dir(self), "codegraph"+filepath.Ext(self))
	bin := cached
	if _, err := os.Stat(cached); err != nil {
		tmp, err := os.MkdirTemp(filepath.Dir(self), "go-build")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer os.RemoveAll(tmp)
		defer copyFile(cached)
		if bin = filepath.Join(tmp, filepath.Base(cached)); copyFile(bin) != nil {
			fmt.Fprintln(os.Stderr, "fake go: copy failed")
			return 1
		}
	}
	cmd := exec.Command(prog, kip, bin)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// copyFile test ikilisini to'ya kopyalar.
func copyFile(to string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	in, err := os.Open(self)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Kural: kanca ve ön denetim codegraph'ı önce derler; araç bu ağaçla derlenmiyorsa (düzenlenen bir
// dil paketi ara hâlde) son derlenen ikilinin cevabını verir ve başına bayat olduğunu, derleme
// hatasıyla yazar. Son derlenen ikili yoksa ya da silinmişse (derleme önbelleği temizlendi) derleme
// hatası görünür.
func TestABrokenTreeFallsBackToTheLastBuiltCodegraph(t *testing.T) {
	// Ad tam "go" olmalı (TestMain onu tanır): Linux'ta test ikilisinin uzantısı ".test"tir, Windows'ta ".exe".
	fake := filepath.Join(t.TempDir(), "go")
	if runtime.GOOS == "windows" {
		fake += ".exe"
	}
	if err := copyFile(fake); err != nil {
		t.Fatal(err)
	}
	old := goTool
	goTool = fake
	t.Cleanup(func() { goTool = old })
	root := t.TempDir()
	broken := filepath.Join(root, "broken")
	breakTree := func() {
		if err := os.WriteFile(broken, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	noAnswer := func(when string) {
		if got := testsFor(target{root, "a.go"}); strings.Contains(got, "answer:") || !strings.Contains(got, "undefinedSymbolK2") {
			t.Fatalf("%s derleme hatası görünmeli:\n%s", when, got)
		}
	}

	breakTree()
	noAnswer("derlenmiş ikili yokken")
	if err := os.Remove(broken); err != nil {
		t.Fatal(err)
	}
	got := testsFor(target{root, "a.go"})
	if !strings.HasPrefix(got, "answer: tests-for a.go") || strings.Contains(got, "warning") {
		t.Fatalf("derlenen ağaçta cevap uyarısız gelmeli:\n%s", got)
	}

	breakTree()
	got = testsFor(target{root, "a.go"})
	if !strings.HasPrefix(got, "warning: codegraph does not build with this tree") ||
		!strings.Contains(got, "undefinedSymbolK2") || !strings.Contains(got, "answer: tests-for a.go") {
		t.Fatalf("derlenmeyen ağaçta son derlenen ikilinin cevabı bayat uyarısıyla gelmeli:\n%s", got)
	}
	var out, errOut strings.Builder
	t.Chdir(root)
	if code := runCodegraph([]string{"dups", "--changed", "main"}, &out, &errOut); code != 3 ||
		out.String() != "answer: dups --changed main\n" || !strings.Contains(errOut.String(), "may be stale") {
		t.Fatalf("ön denetimin yolu: çıkış %d, çıktı %q, hata %q", code, out.String(), errOut.String())
	}
	entries, err := reporoot.ReadDir(filepath.Join(root, ".codegraph"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "last-codegraph" {
		t.Fatalf(".codegraph'ta yalnız son derlenen ikilinin yolu kalmalı: %v", entries)
	}

	if err := os.Remove(filepath.Join(filepath.Dir(fake), "codegraph"+filepath.Ext(fake))); err != nil {
		t.Fatal(err)
	}
	noAnswer("son derlenen ikili silinince")
}
