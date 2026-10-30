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
	deep := strings.Repeat("[", 22) + "1.0" + strings.Repeat("]", 22)
	if out := mustRun(t, "TR_print(TR_Tensor("+deep+").TR_shape.TR_length)"); strings.TrimSpace(out) != "22" {
		t.Fatalf("derin ama düzgün liste: %q", out)
	}
}
