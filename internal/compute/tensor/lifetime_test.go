package tensor

import (
	"math/rand"
	"testing"
)

// Tensör ömrünün kuralları (lifetime.go, dil tanımı 15.44): depolama bütün sahipleri
// bırakılınca havuza döner; bırakılmamış bir sahip (görünüm, kayıt, sarıcı) onu tutar.

// returned depolamanın CPU havuzuna iade edilip edilmediği (poolMin'den büyük depolamalar).
func returned(x *Tensor) bool { return x.storage.(*cpuStorage).release }

// big havuza girecek kadar büyük bir yaprak.
func big(rng *rand.Rand) *Tensor { return Randn(rng, 64, 64) }

func TestDropReturnsStorageOfSoleHeader(t *testing.T) {
	x := big(rand.New(rand.NewSource(1)))
	x.Drop()
	if !returned(x) {
		t.Fatal("tek başlığı bırakılan depolama havuza dönmedi")
	}
	x.Drop() // ikinci çağrı etkisizdir
}

func TestViewKeepsStorage(t *testing.T) {
	x := big(rand.New(rand.NewSource(1)))
	v := x.Transpose(0, 1)
	x.Drop()
	if returned(x) {
		t.Fatal("görünümü canlıyken depolama iade edildi")
	}
	if got := v.Data()[1]; got != x.storage.(*cpuStorage).d[64] {
		t.Fatalf("görünümün verisi bozuldu: %v", got)
	}
	v.Drop()
	if !returned(x) {
		t.Fatal("son görünüm bırakılınca depolama dönmedi")
	}
}

func TestWrapCountsValues(t *testing.T) {
	x := big(rand.New(rand.NewSource(1)))
	x.Wrap()
	x.Wrap()
	x.Unwrap()
	if x.IsDropped() {
		t.Fatal("iki sarıcıdan biri bırakılınca başlık düştü")
	}
	x.Unwrap()
	if !x.IsDropped() || !returned(x) {
		t.Fatal("son sarıcı bırakılınca başlık düşmedi")
	}
	expectPanic(t, "fazla Unwrap", x.Unwrap)
}

func TestSavedTensorReleasedAfterItsBackward(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	w := big(rng).SetRequiresGrad(true)
	h := w.MulScalar(2) // ara sonuç: yalnızca h·h'nin geri hesabı kaydeder
	z := h.Mul(h).SumAll()
	h.Drop()
	if returned(h) {
		t.Fatal("geri yayılımın kaydettiği tensör geri yayılımdan önce iade edildi")
	}
	z.Backward()
	if !returned(h) {
		t.Fatal("kaydeden geri hesap bitince depolama dönmedi")
	}
	if g := w.Grad().Data()[0]; g != 8*w.Data()[0] {
		t.Fatalf("gradyan yanlış: %v, beklenen %v", g, 8*w.Data()[0])
	}
}

func TestUndroppedHeaderIsNotReturned(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	w := big(rng).SetRequiresGrad(true)
	h := w.MulScalar(2)
	h.Mul(h).SumAll().Backward()
	if returned(h) {
		t.Fatal("bırakılmamış başlığın depolaması iade edildi")
	}
}

func TestDroppedHeaderUseIsInternalError(t *testing.T) {
	x := big(rand.New(rand.NewSource(1)))
	x.Drop()
	expectPanic(t, "bırakılmış tensörün verisi", func() { x.Data() })
	expectPanic(t, "bırakılmış tensörle işlem", func() { x.Add(x) })
}

func TestCastMemoSkipsDroppedEntry(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	w := big(rng).SetRequiresGrad(true)
	h := w.MulScalar(2)
	a := h.CastShared(BFloat16)
	if b := h.CastShared(BFloat16); b != a {
		t.Fatal("paylaşılan dönüşüm yeniden kullanılmadı")
	}
	a.Drop()
	if b := h.CastShared(BFloat16); b == a || b.IsDropped() {
		t.Fatal("bırakılmış dönüşüm önbellekten verildi")
	}
}
