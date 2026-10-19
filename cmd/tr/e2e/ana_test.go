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

// trIkilisi TestMain'in derlediği 'tr' yorumlayıcısının yoludur. Komut satırı testleri
// gerçek ikiliyi os/exec ile çalıştırır; böylece çıkış kodları ve standart çıktı/hata
// ayrımı da sınanır.
var trIkilisi string

// cikisSozdizimi 'tr kodla -' komutunun metin ayrıştırılamadığında verdiği çıkış kodudur
// (cmd/tr/kodlu.go'daki aynı adlı sabit; değişirse oradaki belgeyle birlikte değişir).
const cikisSozdizimi = 2

func TestMain(m *testing.M) {
	klasor, err := os.MkdirTemp("", "tr-uctan-uca-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "geçici klasör açılamadı:", err)
		os.Exit(1)
	}
	ad := "tr"
	if runtime.GOOS == "windows" {
		ad += ".exe"
	}
	trIkilisi = filepath.Join(klasor, ad)
	derle := exec.Command("go", "build", "-o", trIkilisi, "github.com/sekerahmet/TCB/cmd/tr")
	derle.Stdout, derle.Stderr = os.Stderr, os.Stderr
	if err := derle.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tr derlenemedi:", err)
		os.RemoveAll(klasor)
		os.Exit(1)
	}
	kod := m.Run()
	os.RemoveAll(klasor)
	os.Exit(kod)
}

// sonuc bir komutun çalıştırılmasının sonucudur.
type sonuc struct {
	cikti, hata string
	kod         int
}

// calistir ikiliyi verilen klasörde (boşsa bulunulan klasörde) ve standart girişle çalıştırır.
func calistir(t *testing.T, ikili, klasor, girdi string, ortam []string, args ...string) sonuc {
	t.Helper()
	ctx, iptal := context.WithTimeout(context.Background(), 2*time.Minute)
	defer iptal()
	cmd := exec.CommandContext(ctx, ikili, args...)
	cmd.Dir = klasor
	cmd.Stdin = strings.NewReader(girdi)
	if ortam != nil {
		cmd.Env = append(os.Environ(), ortam...)
	}
	var cikti, hata bytes.Buffer
	cmd.Stdout, cmd.Stderr = &cikti, &hata
	err := cmd.Run()
	kod := 0
	if err != nil {
		var cikisHatasi *exec.ExitError
		if !errors.As(err, &cikisHatasi) {
			t.Fatalf("%v çalıştırılamadı: %v", args, err)
		}
		kod = cikisHatasi.ExitCode()
	}
	return sonuc{cikti: cikti.String(), hata: hata.String(), kod: kod}
}

// tr yorumlayıcıyı çalıştırır.
func tr(t *testing.T, klasor string, args ...string) sonuc {
	t.Helper()
	return calistir(t, trIkilisi, klasor, "", nil, args...)
}

// dosyaYaz klasör altına (ara klasörleri açarak) bir dosya yazar.
func dosyaYaz(t *testing.T, klasor, ad, icerik string) string {
	t.Helper()
	yol := filepath.Join(klasor, filepath.FromSlash(ad))
	if err := os.MkdirAll(filepath.Dir(yol), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yol, []byte(icerik), 0o644); err != nil {
		t.Fatal(err)
	}
	return yol
}

// bekle sonucun çıkış kodunu ve çıktılarda geçmesi gereken parçaları denetler.
func bekle(t *testing.T, s sonuc, kod int, ciktida, hatada []string) {
	t.Helper()
	if s.kod != kod {
		t.Errorf("çıkış kodu %d, beklenen %d\nçıktı:\n%s\nhata:\n%s", s.kod, kod, s.cikti, s.hata)
	}
	for _, p := range ciktida {
		if !strings.Contains(s.cikti, p) {
			t.Errorf("standart çıktıda %q yok:\n%s", p, s.cikti)
		}
	}
	for _, p := range hatada {
		if !strings.Contains(s.hata, p) {
			t.Errorf("standart hatada %q yok:\n%s", p, s.hata)
		}
	}
}

// trSurumu yorumlayıcının sürümünü kaynaktan (cmd/tr/main.go'daki VERSION sabiti) okur:
// 'tr sürüm' çıktısı kaynaktaki sürümle karşılaştırılır.
func trSurumu(t *testing.T) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, reporoot.Yol("cmd/tr/main.go"), nil, 0)
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
				if ad.Name != "VERSION" || i >= len(v.Values) {
					continue
				}
				if lit, ok := v.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					surum, err := strconv.Unquote(lit.Value)
					if err == nil {
						return surum
					}
				}
			}
		}
	}
	t.Fatal("cmd/tr/main.go'da VERSION sabiti bulunamadı")
	return ""
}
