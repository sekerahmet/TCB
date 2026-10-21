package tensor

import (
	"math/rand"
	"testing"
)

// Tensör ömrünün kuralları (lifetime.go, dil tanımı 15.44): depolama bütün sahipleri
// bırakılınca havuza döner; bırakılmamış bir sahip (görünüm, kayıt, sarıcı) onu tutar.

// returnLog iade edilen depolamaları kaydeder (havuz onları hemen yeniden verebilir; iade
// anında kaydedilir).
type returnLog map[Storage]bool

// watchReturns testin sonuna kadar iadeleri kaydeder.
func watchReturns(t *testing.T) returnLog {
	log := returnLog{}
	storageReturned = func(s Storage) { log[s] = true }
	t.Cleanup(func() { storageReturned = nil })
	return log
}

func (l returnLog) returned(x *Tensor) bool { return l[x.storage] }

// big havuza girecek kadar büyük bir yaprak.
func big(rng *rand.Rand) *Tensor { return Randn(rng, 64, 64) }

func TestDropReturnsStorageOfSoleHeader(t *testing.T) {
	log := watchReturns(t)
	x := big(rand.New(rand.NewSource(1)))
	x.Drop()
	if !log.returned(x) {
		t.Fatal("tek başlığı bırakılan depolama havuza dönmedi")
	}
	x.Drop() // ikinci çağrı etkisizdir
}

func TestViewKeepsStorage(t *testing.T) {
	log := watchReturns(t)
	x := big(rand.New(rand.NewSource(1)))
	v := x.Transpose(0, 1)
	x.Drop()
	if log.returned(x) {
		t.Fatal("görünümü canlıyken depolama iade edildi")
	}
	if got := v.Data()[1]; got != x.storage.(*cpuStorage).d[64] {
		t.Fatalf("görünümün verisi bozuldu: %v", got)
	}
	v.Drop()
	if !log.returned(x) {
		t.Fatal("son görünüm bırakılınca depolama dönmedi")
	}
}

func TestWrapCountsValues(t *testing.T) {
	log := watchReturns(t)
	x := big(rand.New(rand.NewSource(1)))
	x.Wrap()
	x.Wrap()
	x.Unwrap()
	if x.IsDropped() {
		t.Fatal("iki sarıcıdan biri bırakılınca başlık düştü")
	}
	x.Unwrap()
	if !x.IsDropped() || !log.returned(x) {
		t.Fatal("son sarıcı bırakılınca başlık düşmedi")
	}
	expectPanic(t, "fazla Unwrap", x.Unwrap)
}

func TestSavedTensorReleasedAfterItsBackward(t *testing.T) {
	log := watchReturns(t)
	rng := rand.New(rand.NewSource(1))
	w := big(rng).SetRequiresGrad(true)
	h := w.MulScalar(2) // ara sonuç: yalnızca h·h'nin geri hesabı kaydeder
	z := h.Mul(h).SumAll()
	h.Drop()
	if log.returned(h) {
		t.Fatal("geri yayılımın kaydettiği tensör geri yayılımdan önce iade edildi")
	}
	z.Backward()
	if !log.returned(h) {
		t.Fatal("kaydeden geri hesap bitince depolama dönmedi")
	}
	if g := w.Grad().Data()[0]; g != 8*w.Data()[0] {
		t.Fatalf("gradyan yanlış: %v, beklenen %v", g, 8*w.Data()[0])
	}
}

func TestUndroppedHeaderIsNotReturned(t *testing.T) {
	log := watchReturns(t)
	rng := rand.New(rand.NewSource(1))
	w := big(rng).SetRequiresGrad(true)
	h := w.MulScalar(2)
	h.Mul(h).SumAll().Backward()
	if log.returned(h) {
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

func TestAuditNonContiguousReshapeReturnsStorageOnDrop(t *testing.T) {
	log := watchReturns(t)
	x := big(rand.New(rand.NewSource(1)))
	v := x.Transpose(0, 1)
	r := v.Reshape(64 * 64)
	v.Drop()
	x.Drop()
	r.Drop()
	if !log.returned(r) {
		t.Fatal("bitişik olmayan tensörün Reshape çıktısı bırakılınca depolaması havuza dönmedi")
	}
}

func TestAuditMultiOutputCustomOpBackwardReturnsTempGradStorages(t *testing.T) {
	x := big(rand.New(rand.NewSource(1))).SetRequiresGrad(true)
	var g0, g1, gx *Tensor
	outs := CustomOp("test_iki_cikti", []*Tensor{x}, []*Tensor{x.MulScalar(2), x.MulScalar(3)}, nil, func(_ any, grads []*Tensor) []*Tensor {
		g0, g1 = grads[0], grads[1]
		gx = grads[0].Add(grads[1])
		return []*Tensor{gx}
	})
	loss := outs[0].Add(outs[1]).SumAll()
	log := watchReturns(t)
	loss.Backward()
	if !log.returned(g0) || !log.returned(g1) || !log.returned(gx) {
		t.Fatalf("CustomOp geri yayılım ara gradyan depoları havuza dönmedi: g0=%v g1=%v gx=%v",
			log.returned(g0), log.returned(g1), log.returned(gx))
	}
}

