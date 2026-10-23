// Package testkit, birden çok paketin testlerinde aynı yazılan küçük yardımcıları tek yerde
// toplar: dosya yazma, bulanıklaştırma tohumları, yapı etiketi denetimi ve derlenmiş ikiliyi
// çalıştırıp sonucunu denetleme. TR programı çalıştıran yardımcılar internal/testkit/trrun'dadır.
// Yalnızca testler kullanır (_test.go dosyaları); katmanı tek başına sınanabilsin diye hiçbir iç
// paketi içe aktarmaz (depo kökü için reporoot hariç).
//
// Layer: test helper (internal/testkit); yalnızca reporoot'u içe aktarır, her katmanın testi
// kullanabilir.
package testkit

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// WritePath dosyayı (ara klasörleri açarak) yazar ve yolunu verir.
func WritePath(t testing.TB, filePath, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return filePath
}

// WriteFile klasör altına (bölü çizgisiyle yazılmış ad, ara klasörler açılarak) bir dosya yazar
// ve yolunu verir.
func WriteFile(t testing.TB, dir, name, content string) string {
	t.Helper()
	return WritePath(t, filepath.Join(dir, filepath.FromSlash(name)), content)
}

// MustContain çıktının verilen parçaların hepsini içerdiğini sınar; eksik her parça için çıktıyı da yazar.
func MustContain(t testing.TB, out string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(out, p) {
			t.Errorf("output lacks %q:\n%s", p, out)
		}
	}
}

// TRSeeds depodaki bütün .tr dosyalarını (tests/, examples/, lib/) bulanıklaştırma tohumu olarak
// ekler.
func TRSeeds(f *testing.F) {
	for _, dir := range []string{"tests", "examples", "lib"} {
		_ = filepath.WalkDir(filepath.Join(reporoot.Root(), dir), func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".tr") {
				if data, err := os.ReadFile(path); err == nil {
					f.Add(string(data))
				}
			}
			return nil
		})
	}
}

// GlobFiles depo köküne göre verilen kalıplara uyan dosyaların tam yollarını sırayla verir
// (kalıp sırası, içinde alfabetik); geçersiz kalıp testi düşürür.
func GlobFiles(tb testing.TB, patterns ...string) []string {
	tb.Helper()
	var all []string
	for _, pattern := range patterns {
		files, err := filepath.Glob(reporoot.Path(pattern))
		if err != nil {
			tb.Fatal(err)
		}
		all = append(all, files...)
	}
	return all
}

// CopyDir srcDir klasörünü (alt klasörleriyle) dstDir altına kopyalar; dstDir yoksa açılır.
// Dosyalar 0644, klasörler 0755 izniyle yazılır. Hata testi düşürür.
func CopyDir(t testing.TB, srcDir, dstDir string) {
	t.Helper()
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcDir, path)
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// HasJSONName alanın boş olmayan bir json adı ya da "-" etiketi taşıyıp taşımadığını söyler.
func HasJSONName(f *ast.Field) bool {
	if f.Tag == nil {
		return false
	}
	raw, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return false
	}
	tag, ok := reflect.StructTag(raw).Lookup("json")
	if !ok {
		return false
	}
	name, _, _ := strings.Cut(tag, ",")
	return name != "" || tag == "-"
}

// Result bir komutun çalıştırılmasının sonucudur.
type Result struct {
	Stdout, Stderr string
	Code           int
}

// RunBinary ikiliyi verilen klasörde (boşsa bulunulan klasörde) ve standart girişle çalıştırır;
// env verilirse ortama eklenir. Çalıştırılamazsa testi düşürür; çıkış kodu sonuçtadır.
func RunBinary(t testing.TB, exe, dir, input string, env []string, args ...string) Result {
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
			t.Fatalf("%v could not be run: %v", args, err)
		}
		code = exitError.ExitCode()
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

// Wait sonucun çıkış kodunu ve çıktılarda geçmesi gereken parçaları denetler.
func Wait(t testing.TB, s Result, code int, inOutput, inStderr []string) {
	t.Helper()
	if s.Code != code {
		t.Errorf("exit code %d, want %d\nstdout:\n%s\nstderr:\n%s", s.Code, code, s.Stdout, s.Stderr)
	}
	for _, p := range inOutput {
		if !strings.Contains(s.Stdout, p) {
			t.Errorf("stdout lacks %q:\n%s", p, s.Stdout)
		}
	}
	for _, p := range inStderr {
		if !strings.Contains(s.Stderr, p) {
			t.Errorf("stderr lacks %q:\n%s", p, s.Stderr)
		}
	}
}

// TeeWriter birden çok çıkışa (standart çıktı ve hata) paylaştırılan, eşzamanlı yazmaya dayanıklı bir
// tampondur.
type TeeWriter struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (e *TeeWriter) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.Write(p)
}

// String şimdiye kadar yazılanları verir.
func (e *TeeWriter) String() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.String()
}

// ScalesLinearly bir işin girdi boyuyla doğrusal büyüdüğünü sınar. prepare(n) n boylu girdiyi
// hazırlar (süreye girmez) ve ölçülecek işi verir; iş tekrar çalıştırılabilir olmalıdır. n ve
// 4n'lik işler art arda, yani aynı makine yükünde ölçülür; her süre üç denemenin en kısasıdır
// (Python timeit belgesi: yüksek değerler çoğunlukla başka süreçlerin karışmasıdır). Oran 8'i
// aşarsa test kalır: doğrusal işte ~4 (sabit maliyet baskınsa daha az), karesel bir gerilemede
// ~16'dır. Duvar saati tavanının aksine makinenin yükünden bağımsızdır: bütün takım paralel
// koşarken tek bir sürenin 90 katına uzadığı ölçüldü (2026-10-23).
func ScalesLinearly(t testing.TB, what string, n int, prepare func(n int) func()) {
	t.Helper()
	small, large, r := linearRatio(n, prepare)
	t.Logf("%s: %v at %d, %v at %d; ratio %.1f", what, small, n, large, linearFactor*n, r)
	if r > linearLimit {
		t.Errorf("%s does not grow linearly: %v at %d, %v at %d; ratio %.1f (linear ~%d, limit %g)",
			what, small, n, large, linearFactor*n, r, linearFactor, linearLimit)
	}
}

const (
	linearFactor = 4
	linearLimit  = 8.0
)

// linearRatio n ve linearFactor·n boylu işlerin en kısa sürelerini ve oranlarını verir.
func linearRatio(n int, prepare func(n int) func()) (small, large time.Duration, ratio float64) {
	best := func(f func()) time.Duration {
		m := time.Duration(1<<63 - 1)
		for i := 0; i < 3; i++ {
			start := time.Now()
			f()
			m = min(m, time.Since(start))
		}
		return max(m, time.Microsecond)
	}
	fs, fl := prepare(n), prepare(linearFactor*n)
	small, large = best(fs), best(fl)
	return small, large, float64(large) / float64(small)
}
