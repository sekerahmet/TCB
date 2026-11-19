// Layer: tools (tools); not shipped to users.
package bench

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Temel ölçü setinin S4, S12, S13 ve S14 programları (tools/bench/baseline/README.md); yardımcılar
// behavior_baseline_test.go'dadır (trlang, program, programFails).

// Katalog programı (S4) Dost'un dosyalarını göreli yolla aktarır ve denetimden geçer. Dil modelinin yolu gerçek
// model ister (B'nin uyuşma adımı); burada ses girdisi küçük VITS modeliyle kurulur ve kataloğun ölçmediği bağlam durur.
func TestBaselineCatalogChecks(t *testing.T) {
	if out, err := exec.Command(trlang(t), "check", "baseline/catalog_memory.tr").CombinedOutput(); err != nil {
		t.Fatalf("trlang check: %v\n%s", err, out)
	}
	catalog := reporoot.Path("tools/assistant/katalog.json")
	got := program(t, "baseline/catalog_memory.tr", "--catalog", catalog, "--entry", "mms-tts-tur", "--path", reporoot.Path("tests/data/vits_tiny"), "--context", "0")
	if got["kind"] != "voice" || got["context"] != 0.0 || got["filled"] != 0.0 {
		t.Errorf("mms-tts-tur: %v, want a voice at context 0", got)
	}
	if _, ok := got["load_ms"].(float64); !ok {
		t.Errorf("load_ms missing: %v", got)
	}
	out := programFails(t, "baseline/catalog_memory.tr", "--catalog", catalog, "--entry", "qwen3-4b-q4km", "--path", "missing.gguf", "--context", "99")
	if !strings.Contains(out, "is not one of qwen3-4b-q4km's measured contexts [45, 7981]") {
		t.Errorf("unmeasured context: %s", out)
	}
	if out := programFails(t, "baseline/catalog_memory.tr", "--catalog", catalog, "--entry", "none", "--path", "x", "--context", "0"); !strings.Contains(out, "--entry none is not in") {
		t.Errorf("unknown entry: %s", out)
	}
}

// Matris programı (S12) işlemcide 64² birlerle: iki çekirdekte ve iki tipte köşe öğesi kenardır (64), süreler
// --repeat kadardır.
func TestBaselineMatmul(t *testing.T) {
	for _, kernel := range []string{"tr", "vendor"} {
		for _, dtype := range []string{"float32", "bf16"} {
			got := program(t, "baseline/matmul.tr", "--size", "64", "--fill", "ones", "--kernel", kernel, "--dtype", dtype, "--repeat", "3")
			if times, _ := got["times_ms"].([]any); got["corner"] != 64.0 || len(times) != 3 || got["kernel"] != kernel || got["dtype"] != dtype {
				t.Errorf("%s %s: %v, want corner 64 and 3 times", kernel, dtype, got)
			}
		}
	}
	if out := programFails(t, "baseline/matmul.tr", "--dtype", "fp8"); !strings.Contains(out, "--dtype is float32 or bf16") {
		t.Errorf("--dtype fp8: %s", out)
	}
}

// 100 Hz döngüsü (S13) 100 tur döner ve tur, yükteki tur ve kaçan tur sayılarını yazar; kaçan turun değeri
// makinenin yüküne bağlı olduğu için denetlenmez.
func TestBaselineLoop100Hz(t *testing.T) {
	got := program(t, "baseline/loop_100hz.tr", "--turns", "100", "--load_s", "0.2", "--size", "64")
	if got["turns"] != 100.0 {
		t.Errorf("turns %v, want 100", got["turns"])
	}
	for _, field := range []string{"missed", "turns_under_load", "missed_under_load", "worst_late_ms", "products"} {
		if _, ok := got[field].(float64); !ok {
			t.Errorf("%s missing: %v", field, got)
		}
	}
	if got["products"] == 0.0 || got["turns_under_load"] == 0.0 {
		t.Errorf("the load did not run: %v", got)
	}
}

// Boş program (S14) hatasız biter ve bir şey yazmaz: run.py yalnız açılıp kapanma süresini ölçer.
func TestBaselineEmpty(t *testing.T) {
	out, err := exec.Command(trlang(t), "baseline/empty.tr").CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Errorf("empty.tr: %v, output %q; want no error and no output", err, out)
	}
}
