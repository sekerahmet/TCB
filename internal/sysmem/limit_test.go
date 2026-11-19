package sysmem

import (
	"math"
	"runtime/debug"
	"testing"
)

// resetLimit tavanı kaldırır ve öbek dışı sayımı sıfırlar; test bitince de aynısını yapar.
func resetLimit(t *testing.T) {
	t.Helper()
	clear := func() {
		SetLimit(0)
		limit.Lock()
		limit.outside = 0
		limit.Unlock()
	}
	clear()
	t.Cleanup(clear)
}

// Kural (19.12a): tavan çöp toplayıcının öbeğini ve öbek dışı blokları birlikte sınırlar: Go'nun sınırı
// tavandan öbek dışının düşüldüğü kadardır, öbek dışı tavanı aşınca sıfırdır; blok bırakılınca geri gelir.
func TestLimitSubtractsMemoryOutsideTheHeap(t *testing.T) {
	resetLimit(t)
	const gib, mib = int64(1) << 30, int64(1) << 20
	SetLimit(gib)
	if got := debug.SetMemoryLimit(-1); got != gib {
		t.Fatalf("SetLimit(1 GiB): Go'nun sınırı %d, %d bekleniyordu", got, gib)
	}
	if n, ok := Limit(); !ok || n != gib {
		t.Errorf("Limit() = %d, %v; %d, true bekleniyordu", n, ok, gib)
	}
	for _, s := range []struct {
		add, want, outside int64
	}{
		{300 * mib, gib - 300*mib, 300 * mib},
		{100 * mib, gib - 400*mib, 400 * mib},
		{-300 * mib, gib - 100*mib, 100 * mib},
		{2 * gib, 0, 2*gib + 100*mib},
		{-2*gib - 100*mib, gib, 0},
	} {
		AddExternal(s.add)
		if got := debug.SetMemoryLimit(-1); got != s.want {
			t.Errorf("AddExternal(%d): Go'nun sınırı %d, %d bekleniyordu", s.add, got, s.want)
		}
		// Toplayıcının uyarısı öbek dışını Outside'dan yazar.
		if got := Outside(); got != s.outside {
			t.Errorf("AddExternal(%d): Outside() = %d, %d bekleniyordu", s.add, got, s.outside)
		}
	}
}

// Kural (19.12a): tavan verilmezse öbek dışı sayılır ama Go'nun sınırı değişmez; sonradan verilen
// tavan o ana kadarki öbek dışını düşer.
func TestNoLimitLeavesTheGoLimit(t *testing.T) {
	resetLimit(t)
	before := debug.SetMemoryLimit(math.MaxInt64)
	t.Cleanup(func() { debug.SetMemoryLimit(before) })
	AddExternal(500 << 20)
	if got := debug.SetMemoryLimit(-1); got != math.MaxInt64 {
		t.Fatalf("tavansız AddExternal Go'nun sınırını %d yaptı", got)
	}
	if n, ok := Limit(); ok || n != 0 {
		t.Errorf("Limit() = %d, %v; 0, false bekleniyordu", n, ok)
	}
	SetLimit(1 << 30)
	if got, want := debug.SetMemoryLimit(-1), int64(1<<30-500<<20); got != want {
		t.Errorf("SetLimit sonrası Go'nun sınırı %d, %d bekleniyordu", got, want)
	}
	SetLimit(0)
	if got := debug.SetMemoryLimit(-1); got != math.MaxInt64 {
		t.Errorf("tavan kalkınca Go'nun sınırı %d, eski değer %d bekleniyordu", got, int64(math.MaxInt64))
	}
	if _, ok := Limit(); ok {
		t.Error("SetLimit(0) tavanı kaldırmadı")
	}
}
