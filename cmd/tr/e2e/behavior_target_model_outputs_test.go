package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: hedef modelin eğitimi sonuçlarını her adım sayısında yazar: hız dosyası (--çıktı), kayıp
// kaydı ve son ağırlıklar (--kaydet). Kaydedilen ağırlıklar --ağırlık ile yüklenip --adım 0 ile
// yalnızca değerlendirilince eğitimin son doğrulama kaybını verir.
// Neden: tek adım sayısında (uzun koşu, 12207) hız dosyası satırı yarım aralık sınırıyla (6103,5)
// düşüyordu ve ondan sonra yazılan ağırlıklar kayboluyordu; A/B ölçümleri çift sayıyla (120)
// koştuğu için görünmemişti.
func TestTargetModelWritesResultsAfterOddSteps(t *testing.T) {
	script := reporoot.Path("tools/gpu/rgn512.tr")
	data := reporoot.Path("tests/data/text")
	valid := reporoot.Path("tests/data/text/text_001.npy")
	small := []string{"--veri", data, "--doğrulama", valid, "--değerlendirme_parça", "128", "--cihaz", "cpu",
		"--hassasiyet", "fp32", "--toplu", "4", "--mikro", "2", "--bağlam", "32", "--günlük", "1", "--dağarcık", "1000",
		"--boyut", "64", "--bakış_sayısı", "4", "--kv_bakış_sayısı", "2", "--ara", "96"}
	dir := t.TempDir()
	s := testkit.RunBinary(t, trBinary, dir, "", nil, append([]string{script, "--adım", "3", "--çıktı", "hiz.json",
		"--kayıplar", "k.json", "--kaydet", "son.trt"}, small...)...)
	if s.Code != 0 {
		t.Fatalf("3 adımlık koşu çıkış %d:\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
	for _, f := range []string{"hiz.json", "k.json", "son.trt"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s yazılmadı: %v", f, err)
		}
	}
	var speed struct {
		Measured struct {
			MedianMs float64 `json:"median_ms"`
		} `json:"measured"`
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "hiz.json")); err == nil {
		if err := json.Unmarshal(raw, &speed); err != nil || speed.Measured.MedianMs <= 0 {
			t.Errorf("hız dosyası geçersiz (%v): %s", err, raw)
		}
	}
	evalLine := regexp.MustCompile(`DEĞERLENDİRME adım (\d+): kayıp ([\d.]+)`)
	trained := evalLine.FindAllStringSubmatch(s.Stdout, -1)
	if len(trained) == 0 || trained[len(trained)-1][1] != "3" {
		t.Fatalf("son adımda değerlendirme yok:\n%s", s.Stdout)
	}

	e := testkit.RunBinary(t, trBinary, dir, "", nil, append([]string{script, "--adım", "0", "--ağırlık", "son.trt",
		"--çıktı", "bos.json"}, small...)...)
	if e.Code != 0 {
		t.Fatalf("yalnızca değerlendirme çıkış %d:\n%s\n%s", e.Code, e.Stdout, e.Stderr)
	}
	got := evalLine.FindStringSubmatch(e.Stdout)
	if got == nil || got[1] != "0" || got[2] != trained[len(trained)-1][2] {
		t.Errorf("kaydedilen ağırlıkların değerlendirmesi %v, eğitimin son değerlendirmesi %v", got, trained[len(trained)-1])
	}
}
