package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (Evre 3 bitiş ölçütü; dil tanımı 15.36, 20.14, 20.15): hedef modelin eğitimi bir NaN'de
// kendiliğinden son kontrol noktasına döner, toplu işleri atlar ve sürer; kontrol noktası yoksa
// açık bir hatayla durur. Küçük boyutlarla CPU'da birkaç saniye sürer (--nan_adımı sınama seçeneği).
func TestTargetModelRollsBackOnNaN(t *testing.T) {
	script := reporoot.Path("tools/gpu/rgn512.tr")
	data := reporoot.Path("tests/data/text")
	args := []string{script, "--veri", data, "--device", "cpu", "--hassasiyet", "fp32", "--step", "12",
		"--toplu", "4", "--mikro", "2", "--bağlam", "32", "--logger", "4", "--dağarcık", "1000",
		"--embed_dim", "64", "--num_heads", "4", "--num_kv_heads", "2", "--ara", "96", "--nan_adımı", "8"}

	dir := t.TempDir()
	s := testkit.RunBinary(t, trBinary, dir, "", nil, append(args, "--koşu", "koşu", "--kayıt_dakika", "0", "--skip", "1",
		"--losses", "k.json")...)
	if s.Code != 0 {
		t.Fatalf("kontrol noktalı koşu çıkış %d:\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
	for _, want := range []string{"sonlu_değil adım 8", "7. adıma dönüldü, 1 toplu iş atlandı", "adım 12 / 12"} {
		if !strings.Contains(s.Stdout, want) {
			t.Errorf("çıktıda %q yok:\n%s", want, s.Stdout)
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

	s = testkit.RunBinary(t, trBinary, t.TempDir(), "", nil, args...)
	if s.Code == 0 || !strings.Contains(s.Stdout+s.Stderr, "dönülecek kontrol noktası yok") {
		t.Fatalf("kontrol noktasız koşu açık hatayla durmalıydı (çıkış %d):\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
}

// Kural (20.2, 20.6, 20.14): hedef modelin kayıp kaydı kontrol noktasından sürdürülünce ve geri
// dönülünce geçerli adımları tam ve bir kez içerir: 4 adım, sonra aynı koşu klasöründen 8'e
// sürdürme, 6. adımda NaN ve 4'e dönüş → 8 adım. Düz değerler iç içe yerinde yenilendiği için
// `izlenen = {kayıt: kayıt, ...}` ile tutulan `kayıt` listesine bağ kopmaz.
// Neden: eskiden yükleme iç listeyi yenisiyle değiştiriyor, kayıt değişkeni eski listede
// kalıyordu (5 adım yazılıyordu; Gemini kararlılık denetimi, bulgu 4).
func TestTargetModelLossLogSurvivesResumeAndRollback(t *testing.T) {
	script := reporoot.Path("tools/gpu/rgn512.tr")
	data := reporoot.Path("tests/data/text")
	base := []string{script, "--veri", data, "--device", "cpu", "--hassasiyet", "fp32",
		"--toplu", "4", "--mikro", "2", "--bağlam", "32", "--logger", "4", "--dağarcık", "1000",
		"--embed_dim", "64", "--num_heads", "4", "--num_kv_heads", "2", "--ara", "96"}
	dir := t.TempDir()
	if s := testkit.RunBinary(t, trBinary, dir, "", nil, append(base, "--step", "4", "--koşu", "koşu",
		"--kayıt_dakika", "0", "--losses", "k.json")...); s.Code != 0 {
		t.Fatalf("ilk koşu çıkış %d:\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
	if s := testkit.RunBinary(t, trBinary, dir, "", nil, append(base, "--step", "8", "--koşu", "koşu",
		"--kayıt_dakika", "15", "--nan_adımı", "6", "--skip", "1", "--losses", "k.json")...); s.Code != 0 {
		t.Fatalf("devam koşusu çıkış %d:\n%s\n%s", s.Code, s.Stdout, s.Stderr)
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
	if len(k.Steps) != 8 {
		t.Fatalf("kayıp kaydı %d adım, 8 bekleniyordu", len(k.Steps))
	}
}

// Kural (Evre 3 bitiş ölçütü): sorun kalıcıysa (aynı kontrol noktasına art arda üçüncü dönüş)
// hedef modelin eğitimi sonsuz döngüye girmez, açık bir hatayla durur. Çok düşük bir bekçi eşiği
// olağan dalgalanmayı da sıçrama sayar ve kalıcı bir sorunu taklit eder.
func TestTargetModelStopsWhenRollbackRepeats(t *testing.T) {
	script := reporoot.Path("tools/gpu/rgn512.tr")
	data := reporoot.Path("tests/data/text")
	s := testkit.RunBinary(t, trBinary, t.TempDir(), "", nil, script, "--veri", data, "--device", "cpu", "--hassasiyet", "fp32",
		"--step", "12", "--toplu", "4", "--mikro", "2", "--bağlam", "32", "--logger", "4", "--dağarcık", "1000",
		"--embed_dim", "64", "--num_heads", "4", "--num_kv_heads", "2", "--ara", "96", "--koşu", "koşu",
		"--kayıt_dakika", "0", "--skip", "1", "--bekçi_pencere", "2", "--bekçi_eşik", "0.0001")
	if s.Code == 0 || !strings.Contains(s.Stdout+s.Stderr, "üç kez dönüldü") {
		t.Fatalf("aynı noktaya üçüncü dönüşte durmalıydı (çıkış %d):\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
}
