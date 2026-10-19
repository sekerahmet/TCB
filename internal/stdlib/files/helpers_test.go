package files

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/stdlib/formats"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Testlerdeki VM'ler bu paketin ve yol alan öteki motorların yerleşiklerini içersin.
func init() {
	interp.AddEngine(formats.Register)
	interp.AddEngine(ml.Register)
	interp.AddEngine(Register)
}

// result bir TR programının çalışmasıdır.
type result struct {
	output, warning string
	value           interp.Value
	failure         error
}

// runTR kodu program klasörü programDir olan bir program gibi çalıştırır (çalışma
// klasörü testin ayarladığıdır).
func runTR(t *testing.T, programDir, code string) result {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), code)
	}
	var out, failure bytes.Buffer
	m := interp.New()
	m.Out, m.Err = &out, &failure
	if programDir != "" {
		m.SourceFile = filepath.Join(programDir, "program.tr")
	}
	d, err := m.Run(prog)
	m.WaitAsync()
	return result{output: out.String(), warning: failure.String(), value: d, failure: err}
}

// mustRunTR programı çalıştırır ve hatasız bitmesini bekler; son değerin görünüşünü verir.
func mustRunTR(t *testing.T, code string) string {
	t.Helper()
	s := runTR(t, "", code)
	if s.failure != nil {
		t.Fatalf("beklenmeyen hata: %v\n%s", s.failure, code)
	}
	return s.value.Inspect()
}

// errContains programın, verilen parçaları içeren bir hatayla durmasını bekler.
func errContains(t *testing.T, code string, parts ...string) {
	t.Helper()
	s := runTR(t, "", code)
	if s.failure == nil {
		t.Fatalf("hata bekleniyordu: %s", code)
	}
	for _, p := range parts {
		if !strings.Contains(s.failure.Error(), p) {
			t.Errorf("%s\n  hata %q içermeli: %v", code, p, s.failure)
		}
	}
}

// tmpWork çalışma klasörünü test bitene kadar yeni bir geçici klasör yapar.
func tmpWork(t *testing.T) string {
	t.Helper()
	k := t.TempDir()
	t.Chdir(k)
	t.Cleanup(CloseAll) // açık kalan satır okuyucuları (Windows'ta klasör silinemezdi)
	return k
}

func write(t *testing.T, fpath, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(fpath string) bool {
	_, err := os.Lstat(fpath)
	return err == nil
}

// dirLink hedef klasörü gösteren bir klasör bağlantısı kurar: önce sembolik
// bağlantı, olmazsa (Windows'ta yetki gerekir) dizin bağlantısı (junction). İkisi de
// kurulamazsa test atlanır.
func dirLink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err == nil {
		return
	}
	if runtime.GOOS != "windows" {
		t.Skip("sembolik bağlantı kurulamadı")
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", name, target).CombinedOutput(); err != nil {
		t.Skipf("dizin bağlantısı kurulamadı: %v %s", err, out)
	}
}

// forwardPath yolun TR metninde yazılışıdır (/ ile).
func forwardPath(fpath string) string { return filepath.ToSlash(fpath) }
