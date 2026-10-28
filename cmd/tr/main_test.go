package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
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

// template gömülü paketi olmayan yorumlayıcı ikilisinin baytlarıdır.
func template(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile(trBuild(t))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func TestUnsupportedPackageVersion(t *testing.T) {
	body := []byte(`{"main":{"code":"TR_print(1)"}}`)
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
	testkit.Wait(t, testkit.RunBinary(t, filePath, t.TempDir(), "", nil), 1, nil, []string{"❌ Gömülü uygulama okunamadı:", "sürümü 99 desteklenmiyor"})
}

// Birim testleri (süreç içinde).

func TestPackageRoundTrip(t *testing.T) {
	pk := &pkg{
		Main:    packageFile{Name: "ana.tr", Code: "TR_import \"m\"\n", Imports: []int{0}},
		Modules: []packageFile{{Name: "m.tr", Code: "TR_var a = 1\n"}},
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
	binary.LittleEndian.PutUint64(corrupt[len(corrupt)-footerSize:], 1<<40)
	if _, _, found, err := extractPackage(corrupt); !found || !messages.Has(err, messages.H8750) {
		t.Error("bozuk uzunluk alanı hata vermeli")
	}
}

func TestImportSitesAndRedirect(t *testing.T) {
	code := source.Header + "\n# TR_import \"yorumdaki\" sayılmaz\nTR_import \"a\"\nTR_import \"alt/ç.tr\" TR_as c\nTR_print(\"TR_import b\")\n"
	sites := importSites(code)
	if len(sites) != 2 || sites[0].filePath != "a" || sites[0].ln != 3 || sites[1].filePath != "alt/ç.tr" || sites[1].ln != 4 {
		t.Fatalf("aktar yerleri yanlış: %+v", sites)
	}
	d := packageFile{Name: "x.tr", Code: code, Imports: []int{1, 0}}
	updated, err := redirectImports(d, []string{"/g/0/ç.tr", "/g/1/a.tr"})
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Replace(strings.Replace(code, `TR_import "a"`, `TR_import "/g/1/a.tr"`, 1), `TR_import "alt/ç.tr"`, `TR_import "/g/0/ç.tr"`, 1)
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
