package tensor

import "testing"

// Kural (15.3): kopya olan .TR_reshape'in deposu ve görünümleri kilitlidir; havuza iade edilen
// depo yeniden kullanılınca kilidi taşımaz (başka bir tensörün, örneğin bir gradyanın deposudur).
// Bozma: Release'te kilit silinmezse havuzdan gelen depo kilitli kalır (2026-11-08, hedef
// modelin gradyan sıfırlaması H6041 verdi).
func TestWriteLockSharedByViewsAndClearedInThePool(t *testing.T) {
	be := cpuBackend{}
	s := be.Alloc(poolMin, Float32)
	x := fromStorage(be, s, []int{poolMin})
	if x.WriteLock() != nil {
		t.Fatal("yeni depo kilitli")
	}
	if !x.LockWrites([]int{2, 3}, []int{1, 2}) {
		t.Fatal("CPU deposu kilit tutmuyor")
	}
	v := x.Reshape(2, poolMin/2)
	if l := v.WriteLock(); l == nil || len(l.Shape) != 2 || l.Strides[1] != 2 {
		t.Fatalf("görünüm kilidi paylaşmıyor: %+v", l)
	}
	be.Release(s)
	for i := 0; i < 3; i++ {
		y := be.Alloc(poolMin, Float32)
		if fromStorage(be, y, []int{poolMin}).WriteLock() != nil {
			t.Fatal("havuzdan gelen depo eski kilidi taşıyor")
		}
		be.Release(y)
	}
}
