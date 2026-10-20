package dataset

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// sentinelDetail önbellek iletilerinde nöbetçi hatadan sonraki ayrıntıyı iletinin
// parametrelerinden ayırır; nöbetçi tek başınaysa metnin kendisini verir.
func TestSentinelDetail(t *testing.T) {
	m := messages.New(messages.H5012, dataload.ErrCacheInsufficient, "a.npy", int64(10), int64(5), int64(1), int64(0))
	want := clean(strings.TrimPrefix(m.Error(), dataload.ErrCacheInsufficient.Error()+": "))
	if got := sentinelDetail(m); got != want {
		t.Errorf("H5012 ayrıntısı %q, beklenen %q", got, want)
	}
	if got := sentinelDetail(dataload.ErrCacheInsufficient); got != clean(dataload.ErrCacheInsufficient.Error()) {
		t.Errorf("yalın nöbetçi: %q", got)
	}
}
