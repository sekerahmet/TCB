package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestReplayKeyIncludesVendorMatmul: TR_replay'in kaydı TR_vendor_matmul bloğunda ve dışında ayrıdır;
// anahtar 15.53a'nın durumunu da ayrı tutar (15.53b).
func TestReplayKeyIncludesVendorMatmul(t *testing.T) {
	m := interp.New()
	outside := replayKeyPrefix(m)
	m.SetTaskValue(vendorMatmulKey{}, true)
	inside := replayKeyPrefix(m)
	if inside == outside {
		t.Fatalf("blokta ve dışında aynı anahtar: %q", inside)
	}
	m.SetTaskValue(vendorMatmulKey{}, nil)
	m.SetTaskValue(actQuantKey{}, true)
	if q := replayKeyPrefix(m); q == inside || q == outside {
		t.Fatalf("iki bloğun anahtarı karışıyor: %q, %q, %q", outside, inside, q)
	}
}

// TestVendorMatmulArguments: blok yalnız bir fonksiyon alır; sorgu argümansızdır ve blok dışında yanlıştır.
func TestVendorMatmulArguments(t *testing.T) {
	m := interp.New()
	_, err := m.Builtin(codes.FnVendorMatmul).Fn(m, nil, nil)
	wantMessage(t, err, messages.H6725)
	_, err = m.Builtin(codes.FnVendorMatmul).Fn(m, []interp.Value{interp.True}, nil)
	wantMessage(t, err, messages.H6726)
	v, err := m.Builtin(codes.FnVendorMatmulOn).Fn(m, nil, nil)
	if err != nil || v != interp.False {
		t.Fatalf("blok dışında sorgu: %v, %v", v, err)
	}
}
