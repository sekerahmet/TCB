package checkpoint_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (dil tanımı 20.7): örnekleyici durumundaki pencere, gruptaki pencere sayısını aşamaz;
// aşan konum bu veride geçersizdir (bozuk kayıt), sessizce grup atlanmaz. (Gemini gerileme
// denetimi, Bulgu 6.)
func TestSamplerRejectsWindowBeyondGroup(t *testing.T) {
	dataDir := trPath(filepath.Join(reporoot.Root(), "tests", "data", "text"))
	runDir := t.TempDir()

	out, err := run(t, fmt.Sprintf(`
TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var ö = TR_Dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_var kn = TR_Checkpoint(%q, {"w": w, "ö": ö})
ö.TR_next()
kn.TR_update(1)
kn.TR_save()
`, dataDir, trPath(runDir)))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	step1Dir := filepath.Join(runDir, "checkpoints", "step_0000000001")
	durumPath := filepath.Join(step1Dir, "state.json")
	b, err := os.ReadFile(durumPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := strings.Replace(string(b), "\"window\",\n            2", "\"window\",\n            999999", 1)
	if corrupted == string(b) {
		t.Fatalf("pencere alanı state.json içinde bulunamadı:\n%s", string(b))
	}
	if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}

	resignCheckpoint(t, step1Dir, 1)

	loadOut, err := run(t, fmt.Sprintf(`
TR_var w = TR_Tensor([0.0, 0.0], TR_requires_grad: TR_true)
TR_var ö = TR_Dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_var kn = TR_Checkpoint(%q, {"w": w, "ö": ö})
ö.TR_next()
TR_print("DURUM:", ö.TR_state)
`, dataDir, trPath(runDir)))
	loadOut = strings.ReplaceAll(loadOut, trPath(runDir), "<KOŞU>")
	if err == nil {
		t.Fatalf("veri kümesi sınırını aşan pencere konumu (999999) reddedilmeliydi; hatasız yüklendi ve grubu atladı:\n%s", loadOut)
	}
}
