// Package e2e, 'tr' komutunun uçtan uca (kara kutu) testleridir: TestMain yorumlayıcıyı
// bir kez derler, testler onu os/exec ile alt süreç olarak çalıştırır; çıkış kodları, standart
// çıktı/hata ayrımı, kesme sinyalleri, kontrol noktasından devam, belgelerdeki kod blokları ve
// VS Code eklentisinin dosyaları burada sınanır. 'main' paketinin iç işlevlerini çağıran beyaz
// kutu testleri cmd/tr'de kalır. Koşmak için: go test ./cmd/tr/e2e -run <TestAdı>
//
// Layer: command (cmd); yalnızca testlerden oluşur.
package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// trBinary TestMain'in derlediği 'tr' yorumlayıcısının yoludur. Komut satırı testleri
// gerçek ikiliyi os/exec ile çalıştırır; böylece çıkış kodları ve standart çıktı/hata
// ayrımı da sınanır.
var trBinary string

// exitSyntax 'tr kodla -' komutunun metin ayrıştırılamadığında verdiği çıkış kodudur
// (cmd/tr/coded.go'daki aynı adlı sabit; değişirse oradaki belgeyle birlikte değişir).
const exitSyntax = 2

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tr-uctan-uca-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "geçici klasör açılamadı:", err)
		os.Exit(1)
	}
	ad := "tr"
	if runtime.GOOS == "windows" {
		ad += ".exe"
	}
	trBinary = filepath.Join(dir, ad)
	build := exec.Command("go", "build", "-o", trBinary, "github.com/sekerahmet/TCB/cmd/tr")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tr derlenemedi:", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// result bir komutun çalıştırılmasının sonucudur.
type result struct {
	stdout, stderr string
	code           int
}

// run ikiliyi verilen klasörde (boşsa bulunulan klasörde) ve standart girişle çalıştırır.
func run(t *testing.T, exe, dir, input string, env []string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("%v çalıştırılamadı: %v", args, err)
		}
		code = exitError.ExitCode()
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// tr yorumlayıcıyı çalıştırır.
func tr(t *testing.T, dir string, args ...string) result {
	t.Helper()
	return run(t, trBinary, dir, "", nil, args...)
}

// writeFile klasör altına (ara klasörleri açarak) bir dosya yazar.
func writeFile(t *testing.T, dir, ad, content string) string {
	t.Helper()
	filePath := filepath.Join(dir, filepath.FromSlash(ad))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return filePath
}

// wait sonucun çıkış kodunu ve çıktılarda geçmesi gereken parçaları denetler.
func wait(t *testing.T, s result, code int, inOutput, inStderr []string) {
	t.Helper()
	if s.code != code {
		t.Errorf("çıkış kodu %d, beklenen %d\nçıktı:\n%s\nhata:\n%s", s.code, code, s.stdout, s.stderr)
	}
	for _, p := range inOutput {
		if !strings.Contains(s.stdout, p) {
			t.Errorf("standart çıktıda %q yok:\n%s", p, s.stdout)
		}
	}
	for _, p := range inStderr {
		if !strings.Contains(s.stderr, p) {
			t.Errorf("standart hatada %q yok:\n%s", p, s.stderr)
		}
	}
}

// trVersion yorumlayıcının sürümünü kaynaktan (cmd/tr/main.go'daki Version sabiti) okur:
// 'tr sürüm' çıktısı kaynaktaki sürümle karşılaştırılır.
func trVersion(t *testing.T) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, reporoot.Path("cmd/tr/main.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			v := s.(*ast.ValueSpec)
			for i, ad := range v.Names {
				if ad.Name != "Version" || i >= len(v.Values) {
					continue
				}
				if lit, ok := v.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					version, err := strconv.Unquote(lit.Value)
					if err == nil {
						return version
					}
				}
			}
		}
	}
	t.Fatal("cmd/tr/main.go'da Version sabiti bulunamadı")
	return ""
}
