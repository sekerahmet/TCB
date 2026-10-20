package tensor

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Genişletilmiş eleman bazlı işlemler (cihazdan bağımsız katman): etkinleştirme (SiLU),
// ters trigonometri, hiperbolik, kararlı logaritma/üstel, yuvarlama, sınırlama, iki
// argümanlı arktanjant, karşılaştırma ve koşullu seçim. Hesap arka uçta yapılır; 16
// bitlik girdiler float32'ye açılarak hesaplanır, sonuç girdinin türündedir.

// Silu x·σ(x) (SiLU, "swish"); türevi σ(x)·(1 + x·(1 − σ(x))). Tek geçişli çekirdektir.
func (t *Tensor) Silu() *Tensor { return unary(t, OpSilu, 0) }

// Tan eleman bazlı tanjant (radyan); türevi 1 + tan²(x).
func (t *Tensor) Tan() *Tensor { return unary(t, OpTan, 0) }

// Asin eleman bazlı arksinüs; [-1, 1] dışında NaN. Türevi 1/√(1 − x²).
func (t *Tensor) Asin() *Tensor { return unary(t, OpAsin, 0) }

// Acos eleman bazlı arkkosinüs; [-1, 1] dışında NaN. Türevi −1/√(1 − x²).
func (t *Tensor) Acos() *Tensor { return unary(t, OpAcos, 0) }

// Atan eleman bazlı arktanjant; türevi 1/(1 + x²).
func (t *Tensor) Atan() *Tensor { return unary(t, OpAtan, 0) }

// Sinh hiperbolik sinüs; türevi cosh(x).
func (t *Tensor) Sinh() *Tensor { return unary(t, OpSinh, 0) }

// Cosh hiperbolik kosinüs; türevi sinh(x).
func (t *Tensor) Cosh() *Tensor { return unary(t, OpCosh, 0) }

// Log1p log(1 + x); küçük x'te 1 + x yuvarlamasından etkilenmez. Türevi 1/(1 + x).
func (t *Tensor) Log1p() *Tensor { return unary(t, OpLog1p, 0) }

// Log2 2 tabanında logaritma; ikinin kuvvetlerinde tam sonuç. Türevi 1/(x·ln 2).
func (t *Tensor) Log2() *Tensor { return unary(t, OpLog2, 0) }

// Log10 10 tabanında logaritma; türevi 1/(x·ln 10).
func (t *Tensor) Log10() *Tensor { return unary(t, OpLog10, 0) }

// Expm1 e^x − 1; küçük x'te kararlı. Türevi e^x.
func (t *Tensor) Expm1() *Tensor { return unary(t, OpExpm1, 0) }

// Floor aşağı yuvarlama (taban); türevi 0.
func (t *Tensor) Floor() *Tensor { return unary(t, OpFloor, 0) }

// Ceil yukarı yuvarlama (tavan); türevi 0.
func (t *Tensor) Ceil() *Tensor { return unary(t, OpCeil, 0) }

// Sign işaret: pozitifte 1, negatifte −1, sıfırda 0; NaN korunur. Türevi 0.
func (t *Tensor) Sign() *Tensor { return unary(t, OpSign, 0) }

// Abs mutlak değer |x|: işaret biti silinir (−0 → 0, −sonsuz → sonsuz), NaN korunur. Türevi
// işaret(x)'tir: pozitifte 1, negatifte −1, 0'da 0 (alt türevlerden 0 seçilir).
func (t *Tensor) Abs() *Tensor { return unary(t, OpAbs, 0) }

// Clamp her elemanı [lo, hi] aralığına sınırlar; sınırın yalnızca biri isteniyorsa diğeri
// ±sonsuz verilir. NaN eleman NaN kalır. Türev aralığın içinde (sınırlar dahil) 1, dışında
// 0'dır. lo > hi ya da NaN sınır hatadır.
func (t *Tensor) Clamp(lo, hi float32) *Tensor {
	if lo != lo || hi != hi || lo > hi {
		panic(messages.New(messages.H3040, lo, hi))
	}
	sy := counterStart(t.be, OpKindClamp).start()
	out := emptyOf(isElementwise, t.be, t.shape, t.dtyp)
	t.be.Clamp(out.storage, t.view(), lo, hi)
	r := track(out, "clamp", []*Tensor{t}, func() {
		t.be.ClampGrad(t.accGrad(), out.grad, t.view(), lo, hi)
	}, t)
	sy.finish()
	return r
}

// ClampMin yalnızca alt sınırla Clamp.
func (t *Tensor) ClampMin(lo float32) *Tensor { return t.Clamp(lo, float32(math.Inf(1))) }

// ClampMax yalnızca üst sınırla Clamp.
func (t *Tensor) ClampMax(hi float32) *Tensor { return t.Clamp(float32(math.Inf(-1)), hi) }

// Atan2 eleman bazlı iki argümanlı arktanjant atan2(y, x) (yayınlamalı): (x, y)
// noktasının açısı, (−π, π] aralığında; işaretli sıfırlar ve sonsuzlar IEEE kuralıyla.
// Türevler ∂/∂y = x/(x²+y²), ∂/∂x = −y/(x²+y²); (0, 0)'da ikisi de 0 alınır.
func Atan2(y, x *Tensor) *Tensor {
	be := sameDevice("atan2", y, x)
	sy := counterStart(be, OpKindAtan2).start()
	outShape := broadcastShapes(y.shape, x.shape)
	out := emptyOf(isBroadcastElem, be, outShape, resultDType(y.dtyp, x.dtyp))
	be.Binary(OpAtan2, out.storage, outShape, y.view(), x.view())
	r := track(out, "atan2", []*Tensor{y, x}, func() {
		var gy, gx Storage
		if y.requiresGrad {
			gy = y.accGrad()
		}
		if x.requiresGrad {
			gx = x.accGrad()
		}
		be.Atan2Grad(gy, gx, y.shape, x.shape, out.shape, out.grad, y.view(), x.view())
	}, y, x)
	sy.finish()
	return r
}

// Compare eleman bazlı karşılaştırmanın (op: OpLt, OpGt, OpLe, OpGe) 0/1 maskesini verir
// (yayınlamalı; NaN içeren karşılaştırma 0). Sonuç float32'dir ve türev izlemez.
func Compare(a, b *Tensor, op BinOp) *Tensor {
	if op < OpLt || op > OpGe {
		panic(messages.Internal(fmt.Sprintf("Compare needs a comparison op, got %d", op)))
	}
	be := sameDevice("karşılaştırma", a, b)
	sy := counterStart(be, OpKindAdd+OpKind(op)).start()
	outShape := broadcastShapes(a.shape, b.shape)
	out := emptyOf(isCompare, be, outShape, Float32)
	be.Binary(op, out.storage, outShape, a.view(), b.view())
	sy.finish()
	return out
}

// CompareScalar tensörü bir sayıyla karşılaştırır (Compare(t, s)); sagda doğruysa sayı
// solda durur (s op t).
func (t *Tensor) CompareScalar(s float32, op BinOp, onRight bool) *Tensor {
	st := fromStorage(t.be, t.be.FromHost([]float32{s}, Float32), nil)
	if onRight {
		return Compare(st, t, op)
	}
	return Compare(t, st, op)
}

// Where eleman bazlı seçim: c ≠ 0 olan yerlerde a, diğerlerinde b (üçü de yayınlamalı;
// NaN koşul doğru sayılır). Sonucun türü a ile b'nin ortak türüdür. Gradyan a'ya
// koşulun doğru, b'ye yanlış olduğu yerlerden akar (diğer yerlerin terimi tam 0'dır,
// gradyandaki NaN öbür dala sızmaz); koşul türev almaz.
func Where(c, a, b *Tensor) *Tensor {
	be := sameDevice("koşullu seçim", c, a, b)
	sy := counterStart(be, OpKindWhere).start()
	outShape := broadcastShapes(broadcastShapes(c.shape, a.shape), b.shape)
	out := emptyOf(isSelect, be, outShape, resultDType(a.dtyp, b.dtyp))
	be.Where(out.storage, outShape, c.view(), a.view(), b.view())
	r := track(out, "where", []*Tensor{a, b}, func() {
		if a.requiresGrad {
			be.WhereGrad(a.accGrad(), a.shape, out.shape, out.grad, c.view(), true)
		}
		if b.requiresGrad {
			be.WhereGrad(b.accGrad(), b.shape, out.shape, out.grad, c.view(), false)
		}
	}, c)
	sy.finish()
	return r
}
