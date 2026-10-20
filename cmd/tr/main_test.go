package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bu paketteki testler 'main' paketinin iç işlevlerini (gömülü paket biçimi, aktar
// yönlendirmesi, işlemci notu) doğrudan çağıran beyaz kutu testleridir. Yalnızca derlenmiş
// 'tr' ikilisini çalıştıran uçtan uca testler cmd/tr/e2e'dadır.

// Gömülü paket biçiminin iki testi, paketi gerçek bir yorumlayıcı ikilisinin sonuna ekleyip
// çalıştırır. İkili yalnızca gerektiğinde, bir kez derlenir (exeDir TestMain'de silinir).
var (
	exeOnce  sync.Once
	exeDir   string
	trBinary string
	exeError error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if exeDir != "" {
		os.RemoveAll(exeDir)
	}
	os.Exit(code)
}

// trBuild yorumlayıcıyı geçici bir klasöre bir kez derler ve yolunu verir.
func trBuild(t *testing.T) string {
	t.Helper()
	exeOnce.Do(func() {
		exeDir, exeError = os.MkdirTemp("", "tr-cmd-test-*")
		if exeError != nil {
			return
		}
		name := "tr"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		trBinary = filepath.Join(exeDir, name)
		var output bytes.Buffer
		compile := exec.Command("go", "build", "-o", trBinary, ".")
		compile.Stdout, compile.Stderr = &output, &output
		if err := compile.Run(); err != nil {
			exeError = fmt.Errorf("%v\n%s", err, output.String())
		}
	})
	if exeError != nil {
		t.Fatalf("tr derlenemedi: %v", exeError)
	}
	return trBinary
}

// result bir komutun çalıştırılmasının sonucudur.
type result struct {
	output, failure string
	code            int
}

// runBinary ikiliyi verilen klasörde ve standart girişle çalıştırır (e2e'deki aynı adlı
// yardımcının kısası).
func runBinary(t *testing.T, exe, dir, input string, env []string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var output, failure bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &failure
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("%v çalıştırılamadı: %v", args, err)
		}
		code = exitError.ExitCode()
	}
	return result{output: output.String(), failure: failure.String(), code: code}
}

// wait sonucun çıkış kodunu ve çıktılarda geçmesi gereken parçaları denetler.
func wait(t *testing.T, s result, code int, inOutput, inError []string) {
	t.Helper()
	if s.code != code {
		t.Errorf("çıkış kodu %d, beklenen %d\nçıktı:\n%s\nhata:\n%s", s.code, code, s.output, s.failure)
	}
	for _, p := range inOutput {
		if !strings.Contains(s.output, p) {
			t.Errorf("standart çıktıda %q yok:\n%s", p, s.output)
		}
	}
	for _, p := range inError {
		if !strings.Contains(s.failure, p) {
			t.Errorf("standart hatada %q yok:\n%s", p, s.failure)
		}
	}
}

// template gömülü paketi olmayan yorumlayıcı ikilisinin baytlarıdır.
func template(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile(trBuild(t))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func TestReadsOldPackageFormat(t *testing.T) {
	code := []byte("yazdır(\"eski biçim çalışıyor\")\n")
	content := append(template(t), code...)
	footer := make([]byte, 16)
	binary.LittleEndian.PutUint64(footer[0:8], uint64(len(code)))
	copy(footer[8:], oldPackageSignature)
	content = append(content, footer...)

	filePath := filepath.Join(t.TempDir(), "eski.exe")
	if err := os.WriteFile(filePath, content, 0o755); err != nil {
		t.Fatal(err)
	}
	wait(t, runBinary(t, filePath, t.TempDir(), "", nil), 0, []string{"eski biçim çalışıyor"}, nil)

	// Derlemede şablonun sonundaki eski paket de doğru boyda atılır.
	pk, templateSize, found, err := extractPackage(content)
	if err != nil || !found || templateSize != len(template(t)) || pk.Main.Code != string(code) {
		t.Fatalf("eski paket ayıklanamadı: bulundu=%v boy=%d hata=%v", found, templateSize, err)
	}
}

func TestUnsupportedPackageVersion(t *testing.T) {
	body := []byte(`{"ana":{"kod":"yazdır(1)"}}`)
	content := append(template(t), body...)
	footer := make([]byte, footerSize)
	binary.LittleEndian.PutUint64(footer[0:8], uint64(len(body)))
	binary.LittleEndian.PutUint32(footer[8:12], 99)
	copy(footer[12:], packageSignature)
	content = append(content, footer...)
	filePath := filepath.Join(t.TempDir(), "gelecek.exe")
	if err := os.WriteFile(filePath, content, 0o755); err != nil {
		t.Fatal(err)
	}
	wait(t, runBinary(t, filePath, t.TempDir(), "", nil), 1, nil, []string{"❌ Gömülü uygulama okunamadı:", "sürümü 99 desteklenmiyor"})
}

// Birim testleri (süreç içinde).

func TestPackageRoundTrip(t *testing.T) {
	pk := &pkg{
		Main:    packageFile{Name: "ana.tr", Code: "aktar \"m\"\n", Imports: []int{0}},
		Modules: []packageFile{{Name: "m.tr", Code: "değişken a = 1\n"}},
	}
	coded, err := encodePackage(pk)
	if err != nil {
		t.Fatal(err)
	}
	exe := append([]byte("YORUMLAYICI"), coded...)
	back, templateSize, found, err := extractPackage(exe)
	if err != nil || !found || templateSize != len("YORUMLAYICI") {
		t.Fatalf("bulundu=%v boy=%d hata=%v", found, templateSize, err)
	}
	if back.Main.Code != pk.Main.Code || len(back.Modules) != 1 || back.Modules[0].Code != pk.Modules[0].Code || back.Main.Imports[0] != 0 {
		t.Fatalf("paket bozuldu: %+v", back)
	}
	if _, _, found, _ := extractPackage([]byte("sıradan bir ikili dosya")); found {
		t.Error("paketsiz ikilide paket bulunmamalı")
	}
	corrupt := append([]byte{}, coded...)
	binary.LittleEndian.PutUint64(corrupt[len(corrupt)-20:], 1<<40)
	if _, _, found, err := extractPackage(corrupt); !found || !messages.Has(err, messages.H8750) {
		t.Error("bozuk uzunluk alanı hata vermeli")
	}
}

func TestImportSitesAndRedirect(t *testing.T) {
	code := "// aktar \"yorumdaki\" sayılmaz\naktar \"a\"\nyazdır(\"aktar b\")\neğer doğru {\n    aktar \"alt/ç.tr\" olarak c\n}\n"
	sites := importSites(code)
	if len(sites) != 2 || sites[0].filePath != "a" || sites[0].ln != 2 || sites[1].filePath != "alt/ç.tr" || sites[1].ln != 5 {
		t.Fatalf("aktar yerleri yanlış: %+v", sites)
	}
	d := packageFile{Name: "x.tr", Code: code, Imports: []int{1, 0}}
	updated, err := redirectImports(d, []string{"/g/0/ç.tr", "/g/1/a.tr"})
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Replace(strings.Replace(code, `aktar "a"`, `aktar "/g/1/a.tr"`, 1), `aktar "alt/ç.tr"`, `aktar "/g/0/ç.tr"`, 1)
	if updated != expected {
		t.Fatalf("yönlendirilmiş kod:\n%s\nbeklenen:\n%s", updated, expected)
	}
	if strings.Count(updated, "\n") != strings.Count(code, "\n") {
		t.Error("satır sayısı değişmemeli")
	}
	if _, err := redirectImports(packageFile{Code: code, Imports: []int{0}}, []string{"x"}); !messages.Has(err, messages.H8770) {
		t.Error("aktarım sayısı uyuşmazlığı hata vermeli")
	}
}

func TestProcessorNote(t *testing.T) {
	if n := processorNoteText("amd64", "v3"); !strings.Contains(n, "AVX2") || strings.Contains(n, "\n") {
		t.Errorf("v3 notu tek satır ve AVX2 içermeli: %q", n)
	}
	if n := processorNoteText("amd64", "v4"); !strings.Contains(n, "AVX-512") {
		t.Errorf("v4 notu AVX-512 içermeli: %q", n)
	}
	for _, d := range [][2]string{{"amd64", "v1"}, {"amd64", "v2"}, {"amd64", ""}, {"arm64", "v3"}} {
		if n := processorNoteText(d[0], d[1]); n != "" {
			t.Errorf("%v için not beklenmiyordu: %q", d, n)
		}
	}
}
