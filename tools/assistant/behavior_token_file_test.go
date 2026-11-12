package assistant_test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Belirteç dosyasının sırası (status, C, Ö2): belirteç bellekte üretilir, sunucu kapıyı tutar, dosya ancak sonra
// yazılır. Kapı doluyken açılan ikinci kopya H7625 ile düşer ve dosyaya dokunmaz: ilk kopyanın ekranı ve dosyayı okuyan
// istemciler belirteçlerini yitirmez. İki çağrı yeri sınanır: asistan.tr'nin ekranlı kipi ve screen_demo.tr. Kapıyı bu
// test tutar; programlar modelsizdir (ekranlı kip modelleri kapıdan sonra yükler).
func TestTokenFileIsWrittenOnlyAfterThePortIsHeld(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	for _, d := range []struct {
		name string
		args []string
	}{
		{"asistan", []string{chainProgram, "--kip", "canlı", "--ekran", port, "--dil_modeli", "yok.gguf"}},
		{"screen_demo", []string{fakeScreen, "--ekran", port}},
	} {
		t.Run(d.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "belirtec")
			const old = "önceki kopyanın belirteci"
			if err := os.WriteFile(file, []byte(old), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(trlangBinary(t), append(d.args, "--belirteç_dosyası", file)...)
			cmd.Dir = reporoot.Path(".")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("dolu kapıyla açıldı:\n%s", out)
			}
			// H7625'in metni ekranın dilindedir; kapının numarası ve iletinin önerisi (`TR_port: 0`) dilden bağımsızdır.
			if s := string(out); !strings.Contains(s, port) || !strings.Contains(s, "`TR_port: 0`") {
				t.Errorf("H7625 (kapı kullanılıyor) bekleniyordu:\n%s", out)
			}
			got, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != old {
				t.Errorf("belirteç dosyası değişti: %q", got)
			}
		})
	}
}
