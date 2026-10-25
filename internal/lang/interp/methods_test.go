package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (B2b): Liste ve Metin'de .dilim yoktur (Tensör'ün metodu); ileti dilimin köşeli
// parantezle alındığını söyler (H2526).
func TestSliceMethodOnListGetsRangeHint(t *testing.T) {
	for _, code := range []string{"[1, 2, 3].dilim(0, 1)", `"abc".dilim(0, 1)`} {
		_, err := New().Run(mustParseProgram(t, code+"\n"))
		if !messages.Has(err, messages.H2526) {
			t.Errorf("%s: %v", code, err)
		}
	}
}
