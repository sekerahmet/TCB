package tensor

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// MatMul matris çarpımı (cihazdan bağımsız katman):
//   - (M,K) @ (K,N) -> (M,N)
//   - (..., M, K) @ (K, N) -> (..., M, N)   (ör. doğrusal katman, öndeki eksenler satıra katlanır)
//   - (..., M, K) @ (..., K, N) -> (..., M, N)  (toplu; öndeki eksenler eşit olmalı)
//
// Bitişik olmayan girdiler arka uca verilmeden önce bitişik hale getirilir; yalnız sağ taraf
// bitişik bir matrisin devriği olan tek matrisse (paylaşımlı gömme: x @ tablo.devrik()) kopya
// çıkarılmaz, arka uca devrik bayrağıyla verilir (dil modelinin çıkış katmanında her parçada
// 113 MB'lık kopya kalkar). Geri yayılım devrikleri kopyalamaz; arka ucun transA/transB
// bayraklarını kullanır.
func MatMul(a, b *Tensor) *Tensor { return MatMulOut(a, b, MatMulType(a, b)) }

// MatMulType MatMul'ün çıktı türüdür: türler aynıysa o tür, değilse float32.
func MatMulType(a, b *Tensor) DType {
	if b.dtyp.IsQuantized() {
		return a.dtyp // nicemlenmiş ağırlıkta sonuç x'in hassasiyetindedir (15.53)
	}
	return resultDType(a.dtyp, b.dtyp)
}

// MatMulOut çıktı türü açıkça verilen matris çarpımı (karışık hassasiyet: ör.
// bfloat16 girdiler, float32 çıktı). Arka uç iç toplamayı en az float32'de
// yapar ve sonucu çıktı türüne bir kez yuvarlar.
func MatMulOut(a, b *Tensor, outType DType) *Tensor {
	r, k := MatMulKernel(a, b, outType)
	k.RunLocked()
	return r
}

// MatMulKernel MatMulOut'un hazırlığıdır (kilit altında: şekil denetimi, bitişik kopyalar, çıktı
// ve hesap çizgesine kayıt); çarpımın kendisi dönen çekirdektir. Çekirdek çalışmadan çıktı
// okunmamalıdır: k.Run (izinler alınmışsa, kilitsiz) ya da k.RunLocked.
func MatMulKernel(a, b *Tensor, outType DType) (*Tensor, *Kernel) {
	return matMulKernel(a, b, outType, false, false)
}

// MatMulVendorKernel MatMulKernel'in TR_vendor_matmul bloğundaki eşidir (15.53b): arka uç
// VendorGemmer'sa çarpım üreticinin kitaplığına verilebilir (değilse, ör. CPU'da, blok etkisizdir).
// Durum düğüme kaydedilir: geri yön de aynı yoldan yapılır, bloğun dışında çağrılsa da.
func MatMulVendorKernel(a, b *Tensor, outType DType) (*Tensor, *Kernel) {
	return matMulKernel(a, b, outType, false, true)
}

// MatMulVendor MatMulOut'un TR_vendor_matmul bloğundaki eşidir (MatMulVendorKernel).
func MatMulVendor(a, b *Tensor, outType DType) *Tensor {
	r, k := MatMulVendorKernel(a, b, outType)
	k.RunLocked()
	return r
}

// ActQuantGemmer etkinleştirmesi nicemlenen çarpımı yapan arka uçtur (15.53a'nın TR_quantized_activations bloğu):
// c += nicem(a) · aç(bv); bv nicemlenmiş ağırlığın devrik görünümüdür. Yalnız CPU'da vardır.
type ActQuantGemmer interface {
	GemmActQuant(M, N, K int, a View, bv View, c Storage)
}

// MatMulActQuant MatMulOut'un etkinleştirmesi nicemlenen eşidir (15.53a): b q4_k ya da q6_k ağırlığın devriğidir
// (QuantActSupported), arka uç ActQuantGemmer'dır. Türün ve cihazın uygunluğunu çağıran denetler (iletiler
// stdlib'de); burada uymazsa iç hatadır.
func MatMulActQuant(a, b *Tensor, outType DType) *Tensor {
	r, k := MatMulActQuantKernel(a, b, outType)
	k.RunLocked()
	return r
}

// MatMulActQuantKernel MatMulActQuant'ın hazırlığıdır (MatMulKernel gibi): çekirdek dönen değerdir.
func MatMulActQuantKernel(a, b *Tensor, outType DType) (*Tensor, *Kernel) {
	return matMulKernel(a, b, outType, true, false)
}

func matMulKernel(a, b *Tensor, outType DType, actQuant, vendor bool) (*Tensor, *Kernel) {
	be := sameDevice("matmul", a, b)
	if b.dtyp.IsQuantized() {
		CheckQuantWeight(a, b)
	}
	var aq ActQuantGemmer
	if actQuant {
		var ok bool
		if aq, ok = be.(ActQuantGemmer); !ok || !QuantActSupported(b.dtyp) || len(b.shape) != 2 {
			// İç hata: kapı stdlib'dedir (checkActQuant, H6723 ve H6724; 15.53a); buraya uygun olmayan girdi gelmez.
			panic(messages.Internal("int8 activation product is not available for this backend or weight"))
		}
	}
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
		M = NumelOf(a.shape[:len(a.shape)-1])
	} else {
		if len(a.shape) != len(b.shape) || !sameShape(a.shape[:len(a.shape)-2], b.shape[:len(b.shape)-2]) {
			panic(messages.New(messages.H3052, a.shape, b.shape))
		}
		batch = NumelOf(a.shape[:len(a.shape)-2])
		M = a.shape[len(a.shape)-2]
	}

	// Sayaç hazırlıkta başlar (düğüm işlemin adını kayıtta ondan alır), çarpım bitince durur.
	sy := counterStart(be, OpKindMatMul).start()
	av := a.contigView()
	bv, tb := b.rightView()
	// Bitişik kopyalar geri yönde de okunur; geri yön bitince iade edilir. Türev izlenmiyorsa
	// çekirdek bitince iade edilir: gövde izin tutarken iade kendi iznini bekler (Kernel.end).
	ra, rb := copyRelease(a, av), copyRelease(b, bv)
	release := func() { ra(); rb() }
	gs, writer := be.(GemmSetter)
	writer = writer && !actQuant // nicemli yol c'ye ekler: çıktı sıfırla başlar
	vg, _ := be.(VendorGemmer)
	if !vendor {
		vg = nil
	}
	var out *Tensor
	if writer {
		out = emptyOf(isMatMul, be, outShape, outType)
	} else {
		out = zerosOf(isMatMul, be, outShape, outType)
	}
	c := out.storage // gövde yalnız hazırlıkta kopyalanan görünüm ve depolarla çalışır
	k := newKernel(be, []Storage{c}, []Storage{av.S, bv.S}, []*Tensor{a, b, out}, func() {
		if actQuant {
			aq.GemmActQuant(M, N, K, av, bv, c)
		} else if vg != nil {
			vg.GemmVendor(batch, M, N, K, av, false, bv, tb, c, writer)
		} else if writer {
			gs.GemmSet(batch, M, N, K, av, false, bv, tb, c)
		} else {
			be.Gemm(batch, M, N, K, av, false, bv, tb, c)
		}
		sy.finish()
	})

	r := track(out, "matmul", []*Tensor{a, b}, matMulGrad(be, vg, batch, M, N, K, av, bv, tb, a.dtyp, b.dtyp, release), a, b)
	if r.IsLeaf() {
		k.end = release
	}
	return r, k
}

// matMulGrad çarpımın geri hesabıdır: dA (M×K) += dC·Bᵀ, dB (K×N) += Aᵀ·dC. release bitişik
// kopyaları geri hesap bitince iade eder. vg nil değilse (ileri yön TR_vendor_matmul bloğunda
// kaydedildi, 15.53b) geri yönün iki çarpımı da onunla yapılır: geri yön kaydedilen durumu
// kullanır, arka ucun o anki durumunu değil.
func matMulGrad(be Backend, vg VendorGemmer, batch, M, N, K int, av, bv View, tb bool, ad, bd DType, release func()) func(o *node) {
	gs, writer := be.(GemmSetter)
	// carp gradyanın ilkiyse (taze) üzerine yazar, değilse ekler.
	mulFn := func(t *node, M, N, K int, a View, ta bool, b View, tb bool) {
		if vg != nil {
			c, fresh := t.freshGrad(true)
			gateGemmVendor(vg, batch, M, N, K, a, ta, b, tb, c, fresh)
			return
		}
		if c, fresh := t.freshGrad(writer); fresh {
			gateGemmSet(gs, batch, M, N, K, a, ta, b, tb, c)
		} else {
			gateGemm(be, batch, M, N, K, a, ta, b, tb, c)
		}
	}
	return func(o *node) {
		defer release()
		an, bn := o.parents[0], o.parents[1]
		g := o.gradView()
		// Karışık hassasiyet: girdiler aynı 16 bitlik türdeyse (bf16 ya da f16) gelen
		// gradyan da o türe yuvarlanır; böylece geri yöndeki iki çarpım da 16 bitlik girdiyle
		// (GPU'da tensör çekirdekleri) yapılır. Biriktirme yine en az float32'dedir.
		if ad == bd && ad != Float32 && g.DType() != ad {
			s := allocUninit(be, NumelOf(g.Shape), ad)
			gateCopy(be, s, 0, g)
			defer giveBack(be, s)
			g = View{S: s, Shape: g.Shape, Strides: g.Strides}
		}
		if an.wants() {
			// dA (M×K) += dC (M×N) · Bᵀ (bv devrik saklıysa Bᵀ'nin kendisidir)
			mulFn(an, M, K, N, g, false, bv, !tb)
		}
		if bn.wants() {
			// dB (K×N) += Aᵀ · dC
			mulFn(bn, K, N, M, av, true, g, false)
		}
	}
}

// MatMul yöntem biçimi.
func (t *Tensor) MatMul(o *Tensor) *Tensor { return MatMul(t, o) }

// rightView çarpımın sağ işlenenidir: nicemlenmiş ağırlık her zaman deposuyla, devrik bayrağıyla
// verilir (tek satırlı ağırlıkta [K, 1] görünümü bitişik görünür, adımlar yine devriktir).
func (t *Tensor) rightView() (View, bool) {
	if t.dtyp.IsQuantized() {
		return View{S: t.storage, Offset: t.offset, Shape: []int{t.shape[1], t.shape[0]}, Strides: []int{t.shape[0], 1}}, true
	}
	return t.contigView2DTransposed()
}

// contigView2DTransposed sağ taraf için görünümü verir: bitişik bir [N, K] matrisin devriği olan
// [K, N] görünümse (adımlar [1, K]) depolamayı kopyasız, devrik bayrağıyla (true) verir; değilse
// contigView'dür.
func (t *Tensor) contigView2DTransposed() (View, bool) {
	if len(t.shape) == 2 && !t.IsContiguous() && t.strides[0] == 1 && t.strides[1] == t.shape[0] && t.shape[0] > 1 && t.shape[1] > 1 {
		return View{S: t.storage, Offset: t.offset, Shape: []int{t.shape[1], t.shape[0]}, Strides: []int{t.shape[0], 1}}, true
	}
	return t.contigView(), false
}
