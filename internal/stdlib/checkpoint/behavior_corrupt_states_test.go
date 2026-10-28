package checkpoint_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (dil tanımı 20.7): tür ve parametre sayısı tutan iyileştiricinin bilinmeyen durum
// anahtarı, hiçbir okuyucunun yazmadığı okuyucu türü ve pozitif olmayan toplu/bağlam kullanıcının
// programındaki fark değil kaydın bozukluğudur: önceki geçerli nokta önerilir. (Gemini gerileme
// denetimi, Bulgu 5.)
func TestCorruptOptimizerAndReaderStatesSuggestPrevious(t *testing.T) {
	dataDir := trPath(filepath.Join(reporoot.Root(), "tests", "data", "text"))
	wantHint := messages.Text(messages.H7471, "step_0000000001", 1)

	build := func(t *testing.T) string {
		t.Helper()
		runDir := t.TempDir()
		out, err := run(t, fmt.Sprintf(`
TR_var w = TR_tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var ö = TR_dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_var kn = TR_checkpoint(%q, {w: w, op: op, ö: ö})
ö.TR_next()
kn.TR_update(1)
kn.TR_save()
ö.TR_next()
kn.TR_update(2)
kn.TR_save()
`, dataDir, trPath(runDir)))
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return runDir
	}

	cases := []struct {
		name   string
		tamper func(t *testing.T, step2Dir string)
	}{
		{
			name: "iyilestirici_gecersiz_anahtar_adi",
			tamper: func(t *testing.T, step2Dir string) {
				trtPath := filepath.Join(step2Dir, "tensors.trt")
				if _, err := run(t, fmt.Sprintf(`
TR_save_tensors(%q, {
    "w/0": TR_tensor([1.0, 2.0, 3.0]),
    "op/step": TR_tensor([0.0, 0.0]),
    "op/bozuk": TR_tensor([1.0, 2.0, 3.0])
})
`, trPath(trtPath))); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "okuyucu_gecersiz_tur_adi",
			tamper: func(t *testing.T, step2Dir string) {
				durumPath := filepath.Join(step2Dir, "state.json")
				b, err := os.ReadFile(durumPath)
				if err != nil {
					t.Fatal(err)
				}
				corrupted := strings.Replace(string(b), `"sampler"`, `"bozuk_tür"`, 1)
				if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "okuyucu_sifir_toplu_boyutu",
			tamper: func(t *testing.T, step2Dir string) {
				durumPath := filepath.Join(step2Dir, "state.json")
				b, err := os.ReadFile(durumPath)
				if err != nil {
					t.Fatal(err)
				}
				corrupted := strings.Replace(string(b), "\"batch\",\n            2", "\"batch\",\n            0", 1)
				if err := os.WriteFile(durumPath, []byte(corrupted), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runDir := build(t)
			step2Dir := filepath.Join(runDir, "checkpoints", "step_0000000002")
			tc.tamper(t, step2Dir)
			resignCheckpoint(t, step2Dir, 2)

			_, err := run(t, fmt.Sprintf(`
TR_var w = TR_tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var ö = TR_dataset(%q).TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_checkpoint(%q, {w: w, op: op, ö: ö})
`, dataDir, trPath(runDir)))
			if err == nil {
				t.Fatal("bozuk kontrol noktası reddedilmeliydi")
			}
			msg := strings.ReplaceAll(err.Error(), trPath(runDir), "<KOŞU>")
			if !strings.Contains(msg, wantHint) {
				t.Errorf("%s: bozuk son noktada önceki geçerli nokta (%s) önerilmeliydi, alınan hata:\n%s", tc.name, wantHint, msg)
			}
		})
	}
}
