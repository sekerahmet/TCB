package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kendini içeren liste tensöre çevrilirken sonsuz şekil üretmez (eskiden bellek bitene dek
// eksen ekleniyordu); açık bir hata verir.
func TestTensorFromCyclicList(t *testing.T) {
	_, err := run(t, "TR_var a = []\na.TR_append(a)\nTR_Tensor(a)")
	if err == nil || !messages.Has(err, messages.H6018) {
		t.Fatalf("döngüsel liste hatası beklenirdi: %v", err)
	}
	if out := mustRun(t, "TR_var a = [[1.0]]\nTR_for i TR_in 0..20 { a = [a] }\nTR_print(TR_Tensor(a).TR_shape.TR_length)"); strings.TrimSpace(out) != "22" {
		t.Fatalf("derin ama düzgün liste: %q", out)
	}
}
