package e2e

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// readyModelCompare tr/hazir ile yüklenen modelin logitlerini transformers'ın yazdığı logits.json
// ile karşılaştırır ve en büyük mutlak farkı yazar.
const readyModelCompare = `aktar "tr/hazir"
değişken arg = komut_satırı({klasör: ""})
değişken beklenen = json_çöz(dosya_oku(yol(arg.klasör, "logits.json")))
değişken model = hazır_model(arg.klasör)
değişken logit = model.ileri(tensör(beklenen["ids"]))[0].değer
değişken en_büyük = 0.0
döngü i, satır içinde beklenen["logits"] {
    döngü j, x içinde satır { en_büyük = maks(en_büyük, mutlak(x - logit[i][j])) }
}
yazdır(en_büyük)
`

// Kural (Evre 4, hazır modeller; dil tanımı 15.47): Hugging Face biçiminde kaydedilmiş bir model
// (config.json + model.safetensors) tr/hazir ile yüklenince aynı token kimlikleri için
// transformers'la float32 yuvarlama düzeyinde aynı logitleri verir. İki mimari: Llama (paylaşımlı
// gömme, RoPE tabanı 10⁵) ve Qwen3 (bakış boyutu gizli boyuttan bağımsız, QK-norm, RoPE tabanı
// 10⁶ rope_parameters içinde, ayrı çıkış katmanı). Model rastgele ağırlıklarla yerelde kurulur;
// ağ yoktur. transformers yoksa atlanır.
func TestReadyModelMatchesTransformers(t *testing.T) {
	python := pythonOrSkip(t)
	if !hasPythonModules(python, "torch", "transformers", "safetensors") {
		t.Skip("torch, transformers ya da safetensors yok")
	}
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "karsilastir.tr", readyModelCompare)
	// İki model aynı anda kurulur (her kurulum torch'u yeniden yükler).
	build := func(arch string, tied bool) func(t *testing.T) string {
		return func(t *testing.T) string {
			out := filepath.Join(dir, arch)
			args := []string{"-I", reporoot.Path("tools/hf/small_model.py"), "--arch", arch, "--out", out}
			if tied {
				args = append(args, "--tied")
			}
			if b, err := exec.Command(python, args...).CombinedOutput(); err != nil {
				t.Fatalf("%s modeli kurulamadı: %v\n%s", arch, err, b)
			}
			return out
		}
	}
	llama, qwen3 := pair(t, build("llama", true), build("qwen3", false))
	for _, c := range []struct{ arch, out string }{{"llama", llama}, {"qwen3", qwen3}} {
		s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "karsilastir.tr"), "--klasör", c.out)
		if s.Code != 0 {
			t.Fatalf("%s: TR çıkış %d:\n%s\n%s", c.arch, s.Code, s.Stdout, s.Stderr)
		}
		diff, err := strconv.ParseFloat(strings.TrimSpace(s.Stdout), 64)
		if err != nil || diff > 1e-5 {
			t.Errorf("%s: en büyük logit farkı %q (en çok 1e-5)", c.arch, s.Stdout)
		}
	}
}
