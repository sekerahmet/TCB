package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (10.5): TR_wait_all() argüman almaz; görev listesi biçimi görev değeriyle (10.1) gelir. O
// gelene dek verilen argüman sessizce yok sayılmaz (eskiden liste kabul edilip TR_true dönüyordu):
// H2014, kopyalanınca çalışan yazım TR_wait_all().
func TestWaitAllTakesNoArguments(t *testing.T) {
	for _, source := range []string{"TR_print(TR_wait_all([1, 2]))", "TR_wait_all(1, 2)"} {
		if _, err := runTR(t, source+"\n"); !messages.Has(err, messages.H2014) {
			t.Errorf("%s: H2014 beklenirdi, gelen %v", source, err)
		}
	}
	if _, err := runTR(t, "TR_wait_all()\n"); err != nil {
		t.Errorf("TR_wait_all(): %v", err)
	}
}
