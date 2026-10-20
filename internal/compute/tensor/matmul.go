package tensor

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// MatMul matris çarpımı (cihazdan bağımsız katman):
//   - (M,K) @ (K,N) -> (M,N)
//   - (..., M, K) @ (K, N) -> (..., M, N)   (ör. doğrusal katman, öndeki eksenler satıra katlanır)
//   - (..., M, K) @ (..., K, N) -> (..., M, N)  (toplu; öndeki eksenler eşit olmalı)
//
// Bitişik olmayan girdiler (ör. devrik görünümler) arka uca verilmeden önce
// bitişik hale getirilir. Geri yayılım devrikleri kopyalamaz; arka ucun
// transA/transB bayraklarını kullanır.
func MatMul(a, b *Tensor) *Tensor { return MatMulOut(a, b, resultDType(a.dtyp, b.dtyp)) }

// MatMulOut çıktı türü açıkça verilen matris çarpımı (karışık hassasiyet: ör.
// bfloat16 girdiler, float32 çıktı). Arka uç iç toplamayı en az float32'de
// yapar ve sonucu çıktı türüne bir kez yuvarlar.
func MatMulOut(a, b *Tensor, outType DType) *Tensor {
	be := sameDevice("matmul", a, b)
	if len(a.shape) < 2 || len(b.shape) < 2 {
		panic(messages.New(messages.H3050, a.shape, b.shape))
	}
	K := a.shape[len(a.shape)-1]
	if b.shape[len(b.shape)-2] != K {
		panic(messages.New(messages.H3051, a.shape, b.shape))
	}
	N := b.shape[len(b.shape)-1]

	var batch, M int
	outShape := copyInts(a.shape)
	outShape[len(outShape)-1] = N
	if len(b.shape) == 2 {
		// Sağ taraf tek matris: soldaki öndeki eksenler satırlara katlanır.
		batch = 1
		M = numelOf(a.shape[:len(a.shape)-1])
	} else {
		if len(a.shape) != len(b.shape) || !sameShape(a.shape[:len(a.shape)-2], b.shape[:len(b.shape)-2]) {
			panic(messages.New(messages.H3052, a.shape, b.shape))
		}
		batch = numelOf(a.shape[:len(a.shape)-2])
		M = a.shape[len(a.shape)-2]
	}

	sy := counterStart(be, OpKindMatMul).start()
	av, bv := a.contigView(), b.contigView()
	// Bitişik kopyalar geri yönde de okunur; geri yön bitince (türev izlenmiyorsa hemen) iade edilir.
	ra, rb := copyRelease(a, av), copyRelease(b, bv)
	release := func() { ra(); rb() }
	ad, bd := a.dtyp, b.dtyp
	gs, writer := be.(GemmSetter)
	var out *Tensor
	if writer {
		out = emptyOf(isMatMul, be, outShape, outType)
		gs.GemmSet(batch, M, N, K, av, false, bv, false, out.storage)
	} else {
		out = zerosOf(isMatMul, be, outShape, outType)
		be.Gemm(batch, M, N, K, av, false, bv, false, out.storage)
	}

	// carp gradyanın ilkiyse (taze) üzerine yazar, değilse ekler.
	mulFn := func(t *node, M, N, K int, a View, ta bool, b View, tb bool) {
		if c, fresh := t.freshGrad(writer); fresh {
			gs.GemmSet(batch, M, N, K, a, ta, b, tb, c)
		} else {
			be.Gemm(batch, M, N, K, a, ta, b, tb, c)
		}
	}
	r := track(out, "matmul", []*Tensor{a, b}, func(o *node) {
		defer release()
		an, bn := o.parents[0], o.parents[1]
		g := o.gradView()
		// Karışık hassasiyet: girdiler aynı 16 bitlik türdeyse (bf16 ya da f16) gelen
		// gradyan da o türe yuvarlanır; böylece geri yöndeki iki çarpım da 16 bitlik girdiyle
		// (GPU'da tensör çekirdekleri) yapılır. Biriktirme yine en az float32'dedir.
		if ad == bd && ad != Float32 && g.DType() != ad {
			s := allocUninit(be, numelOf(g.Shape), ad)
			be.Copy(s, 0, g)
			defer giveBack(be, s)
			g = View{S: s, Shape: g.Shape, Strides: g.Strides}
		}
		if an.wants() {
			// dA (M×K) += dC (M×N) · Bᵀ
			mulFn(an, M, K, N, g, false, bv, true)
		}
		if bn.wants() {
			// dB (K×N) += Aᵀ · dC
			mulFn(bn, K, N, M, av, true, g, false)
		}
	}, a, b)
	if r.IsLeaf() {
		release()
	}
	sy.finish()
	return r
}

// MatMul yöntem biçimi.
func (t *Tensor) MatMul(o *Tensor) *Tensor { return MatMul(t, o) }
