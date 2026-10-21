package e2e

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
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
	if exec.Command(python, "-c", "import torch, transformers, safetensors").Run() != nil {
		t.Skip("torch, transformers ya da safetensors yok")
	}
	dir := t.TempDir()
	writeFile(t, dir, "karsilastir.tr", readyModelCompare)
	for _, c := range []struct {
		arch string
		tied bool
	}{{"llama", true}, {"qwen3", false}} {
		out := filepath.Join(dir, c.arch)
		args := []string{"-I", reporoot.Path("tools/hf/small_model.py"), "--arch", c.arch, "--out", out}
		if c.tied {
			args = append(args, "--tied")
		}
		if b, err := exec.Command(python, args...).CombinedOutput(); err != nil {
			t.Fatalf("%s modeli kurulamadı: %v\n%s", c.arch, err, b)
		}
		s := run(t, trBinary, dir, "", nil, filepath.Join(dir, "karsilastir.tr"), "--klasör", out)
		if s.code != 0 {
			t.Fatalf("%s: TR çıkış %d:\n%s\n%s", c.arch, s.code, s.stdout, s.stderr)
		}
		diff, err := strconv.ParseFloat(strings.TrimSpace(s.stdout), 64)
		if err != nil || diff > 1e-5 {
			t.Errorf("%s: en büyük logit farkı %q (en çok 1e-5)", c.arch, s.stdout)
		}
	}
}
