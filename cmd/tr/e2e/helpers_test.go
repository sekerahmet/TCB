// Package e2e, 'tr' komutunun uçtan uca (kara kutu) testleridir: TestMain yorumlayıcıyı
// bir kez derler, testler onu os/exec ile alt süreç olarak çalıştırır; çıkış kodları, standart
// çıktı/hata ayrımı, kesme sinyalleri, kontrol noktasından devam, belgelerdeki kod blokları ve
// VS Code eklentisinin dosyaları burada sınanır. 'main' paketinin iç işlevlerini çağıran beyaz
// kutu testleri cmd/tr'de kalır. Koşmak için: go test ./cmd/tr/e2e -run <TestAdı>
//
// Layer: command (cmd); yalnızca testlerden oluşur.
package e2e

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
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
type result = testkit.Result

// tr yorumlayıcıyı çalıştırır.
func tr(t *testing.T, dir string, args ...string) result {
	t.Helper()
	return testkit.RunBinary(t, trBinary, dir, "", nil, args...)
}

// pair iki bağımsız işi (ör. ayrı klasörlerdeki iki kopyanın koşusu) iki paralel alt testte
// aynı anda yapar ve sonuçlarını verir: alt testleri bir grup alt testi bekler (Go'nun
// "parallel group" kalıbı). Biri düşerse t de orada durur.
func pair[T any](t *testing.T, f1, f2 func(t *testing.T) T) (r1, r2 T) {
	t.Helper()
	ok := t.Run("ikili", func(t *testing.T) {
		t.Run("1", func(t *testing.T) { t.Parallel(); r1 = f1(t) })
		t.Run("2", func(t *testing.T) { t.Parallel(); r2 = f2(t) })
	})
	if !ok {
		t.FailNow()
	}
	return r1, r2
}

// hasPythonModules Python modüllerinin kurulu olup olmadığını onları yüklemeden söyler
// (importlib.util.find_spec): torch'u yalnızca yoklamak için yüklemek birkaç saniye sürer.
func hasPythonModules(python string, modules ...string) bool {
	code := "import importlib.util as u, sys; sys.exit(0 if all(u.find_spec(m) for m in sys.argv[1:]) else 1)"
	return exec.Command(python, append([]string{"-c", code}, modules...)...).Run() == nil
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
