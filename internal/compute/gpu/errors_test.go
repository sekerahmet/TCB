package gpu

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestInternalfIsInternalError(t *testing.T) {
	err := internalf("detail %d", 7)
	if !messages.Has(err, messages.H9000) {
		t.Fatalf("iç hata kodu bekleniyordu: %v", err)
	}
	if got := internalDetail(err); got != "detail 7" {
		t.Fatalf("ayrıntı = %q", got)
	}
	if !strings.Contains(err.Error(), "detail 7") {
		t.Fatalf("ileti ayrıntıyı içermeli: %v", err)
	}
}
