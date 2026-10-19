package tensor

import "fmt"

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
		panic(fmt.Sprintf("tensor: MatMul en az 2 boyutlu girdiler ister, %v @ %v", a.shape, b.shape))
	}
	K := a.shape[len(a.shape)-1]
	if b.shape[len(b.shape)-2] != K {
		panic(fmt.Sprintf("tensor: MatMul iç boyutlar uyuşmuyor: %v @ %v", a.shape, b.shape))
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
			panic(fmt.Sprintf("tensor: toplu MatMul öndeki eksenlerin eşit olmasını ister: %v @ %v", a.shape, b.shape))
		}
		batch = numelOf(a.shape[:len(a.shape)-2])
		M = a.shape[len(a.shape)-2]
	}

	sy := counterStart(be, OpKindMatMul).start()
	av, bv := a.contigView(), b.contigView()
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
	mulFn := func(t *Tensor, M, N, K int, a View, ta bool, b View, tb bool) {
		if c, fresh := t.freshGrad(writer); fresh {
			gs.GemmSet(batch, M, N, K, a, ta, b, tb, c)
		} else {
			be.Gemm(batch, M, N, K, a, ta, b, tb, c)
		}
	}
	r := track(out, "matmul", []*Tensor{a, b}, func() {
		g := out.gradView()
		if a.requiresGrad {
			// dA (M×K) += dC (M×N) · Bᵀ
			mulFn(a, M, K, N, g, false, bv, true)
		}
		if b.requiresGrad {
			// dB (K×N) += Aᵀ · dC
			mulFn(b, K, N, M, av, true, g, false)
		}
	}, a, b)
	sy.finish()
	return r
}

// MatMul yöntem biçimi.
func (t *Tensor) MatMul(o *Tensor) *Tensor { return MatMul(t, o) }
