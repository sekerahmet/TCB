package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (Evre 3 bitiş ölçütü; dil tanımı 15.46, 20.14, 20.15): hedef modelin eğitimi bir NaN'de
// kendiliğinden son kontrol noktasına döner, toplu işleri atlar ve sürer; kontrol noktası yoksa
// açık bir hatayla durur. Küçük boyutlarla CPU'da birkaç saniye sürer (--nan_adımı sınama seçeneği).
func TestTargetModelRollsBackOnNaN(t *testing.T) {
	script := reporoot.Path("tools/gpu/rgn512.tr")
	data := reporoot.Path("tests/data/text")
	args := []string{script, "--veri", data, "--cihaz", "cpu", "--hassasiyet", "fp32", "--adım", "12",
		"--toplu", "4", "--mikro", "2", "--bağlam", "32", "--günlük", "4", "--dağarcık", "1000",
		"--boyut", "64", "--bakış_sayısı", "4", "--kv_bakış_sayısı", "2", "--ara", "96", "--nan_adımı", "8"}

	dir := t.TempDir()
	s := run(t, trBinary, dir, "", nil, append(args, "--koşu", "koşu", "--kayıt_dakika", "0", "--atla", "1",
		"--kayıplar", "k.json")...)
	if s.code != 0 {
		t.Fatalf("kontrol noktalı koşu çıkış %d:\n%s\n%s", s.code, s.stdout, s.stderr)
	}
	for _, want := range []string{"sonlu_değil adım 8", "7. adıma dönüldü, 1 toplu iş atlandı", "adım 12 / 12"} {
		if !strings.Contains(s.stdout, want) {
			t.Errorf("çıktıda %q yok:\n%s", want, s.stdout)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "k.json"))
	if err != nil {
		t.Fatal(err)
	}
	var k struct {
		Steps []struct{ Loss float64 } `json:"steps"`
	}
	if err := json.Unmarshal(raw, &k); err != nil {
		t.Fatal(err)
	}
	if len(k.Steps) != 12 {
		t.Errorf("kayıp kaydı geri dönülen adımdan sürmeliydi: %d adım, 12 bekleniyordu", len(k.Steps))
	}

	s = run(t, trBinary, t.TempDir(), "", nil, args...)
	if s.code == 0 || !strings.Contains(s.stdout+s.stderr, "dönülecek kontrol noktası yok") {
		t.Fatalf("kontrol noktasız koşu açık hatayla durmalıydı (çıkış %d):\n%s\n%s", s.code, s.stdout, s.stderr)
	}
}
