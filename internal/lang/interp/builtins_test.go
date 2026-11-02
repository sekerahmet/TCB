package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (10.5): TR_wait_all() argümansız ya da bir görev listesiyle çağrılır. Görev olmayan öğe H2919,
// ikiden çok argüman H2012; argüman sessizce yok sayılmaz (eskiden liste kabul edilip TR_true dönüyordu).
func TestWaitAllTakesNoneOrATaskList(t *testing.T) {
	for source, code := range map[string]messages.Code{"TR_print(TR_wait_all([1, 2]))": messages.H2919, "TR_wait_all(1, 2)": messages.H2012} {
		if _, err := runTR(t, source+"\n"); !messages.Has(err, code) {
			t.Errorf("%s: %s beklenirdi, gelen %v", source, code, err)
		}
	}
	if _, err := runTR(t, "TR_wait_all()\n"); err != nil {
		t.Errorf("TR_wait_all(): %v", err)
	}
}
