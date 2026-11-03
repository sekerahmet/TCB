package checkpoint_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// frozenProgram gövdesi donmuş bir modelin noktasını kaydeder (kaydet true) ya da yeni kurulan,
// baştan türev izleyen bir modelle noktadan sürer ve donma durumunu yazdırır.
func frozenProgram(runDir string, save bool) string {
	body := `TR_print("sürüm", kn.TR_resume_step, gövde.TR_requires_grad, baş.TR_requires_grad)`
	if save {
		body = "gövde.TR_requires_grad = TR_false\nkn.TR_update(1)\nkn.TR_save()"
	}
	return fmt.Sprintf(`
TR_var gövde = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var baş = TR_Tensor([3.0], TR_requires_grad: TR_true)
TR_var tablo = TR_Tensor([5.0])
TR_var model = {"gövde": gövde, "baş": baş, "tablo": tablo}
TR_var kn = TR_Checkpoint(%q, {"model": model, "op": TR_AdamW(TR_parameters(model))})
%s
`, trPath(runDir), body)
}

// TestFrozenStateIsSavedAndResumed: sürüm 4 noktası donmuş parametrenin adını yazar ve sürdürme onu
// yeniden dondurur; durum tensörü (tablo) listeye girmez (15.34, 20.4).
func TestFrozenStateIsSavedAndResumed(t *testing.T) {
	runDir := t.TempDir()
	if out, err := run(t, frozenProgram(runDir, true)); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	state, err := os.ReadFile(filepath.Join(runDir, "checkpoints", "step_0000000001", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), `"version": 4`) || !regexp.MustCompile(`"frozen": \[\s*"model\.gövde"|"frozen": \[\s*"gövde"`).Match(state) {
		t.Fatalf("state.json sürüm 4 ve donmuş gövdeyi yazmalı:\n%s", state)
	}
	if strings.Contains(string(state), `"tablo"`) && regexp.MustCompile(`"frozen": \[[^\]]*"tablo"`).Match(state) {
		t.Fatalf("türev izlememiş tablo donmuş sayılmamalı:\n%s", state)
	}
	out, err := run(t, frozenProgram(runDir, false))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "sürüm 1 false true") {
		t.Fatalf("sürdürülen model gövdeyi donmuş başlatmalı: %q", out)
	}
}

// TestVersionThreeReadsWithNothingFrozen: donma durumu olmayan sürüm 3 noktası okunur; hiçbir
// parametre donmuş değildir (sürüm 3 yazılırken dondurma yoktu). Sürüm 5 H7421'dir.
func TestVersionThreeReadsWithNothingFrozen(t *testing.T) {
	runDir := t.TempDir()
	if out, err := run(t, frozenProgram(runDir, true)); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	stepDir := filepath.Join(runDir, "checkpoints", "step_0000000001")
	rewriteVersion(t, stepDir, 3, true)
	out, err := run(t, frozenProgram(runDir, false))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "sürüm 1 true true") {
		t.Fatalf("sürüm 3 noktasında hiçbir parametre donmuş olmamalı: %q", out)
	}
	rewriteVersion(t, stepDir, 5, false)
	if out, err := run(t, frozenProgram(runDir, false)); err == nil || !messages.Has(err, messages.H7421) {
		t.Fatalf("sürüm 5 H7421 vermeli: %v\n%s", err, out)
	}
}

// TestFrozenNameMustBeAParameter: donmuş diye kaydedilen ad modelin parametresi değilse (durum
// tensörü ya da olmayan ad) kayıt bozuktur (H6993).
func TestFrozenNameMustBeAParameter(t *testing.T) {
	for _, name := range []string{"tablo", "yok"} {
		t.Run(name, func(t *testing.T) {
			runDir := t.TempDir()
			if out, err := run(t, frozenProgram(runDir, true)); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			stepDir := filepath.Join(runDir, "checkpoints", "step_0000000001")
			p := filepath.Join(stepDir, "state.json")
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			b = regexp.MustCompile(`"gövde"(\s*\])`).ReplaceAll(b, []byte(`"`+name+`"$1`))
			if err := os.WriteFile(p, b, 0o644); err != nil {
				t.Fatal(err)
			}
			resignCheckpointVersion(t, stepDir, 1, writtenVersion)
			if out, err := run(t, frozenProgram(runDir, false)); err == nil || !messages.Has(err, messages.H6993) {
				t.Fatalf("H6993 bekleniyordu: %v\n%s", err, out)
			}
		})
	}
}

// rewriteVersion noktanın state.json sürümünü değiştirir (dropFrozen ile donma alanını siler) ve
// özeti o sürümle yeniden yazar.
func rewriteVersion(t *testing.T, stepDir string, version int, dropFrozen bool) {
	t.Helper()
	p := filepath.Join(stepDir, "state.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b = regexp.MustCompile(`"version": [0-9]+`).ReplaceAll(b, []byte(fmt.Sprintf(`"version": %d`, version)))
	if dropFrozen {
		b = regexp.MustCompile(`,\s*"frozen": \[[^\]]*\]`).ReplaceAll(b, nil)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	resignCheckpointVersion(t, stepDir, 1, version)
}
