package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Kilidi bırakan tensör yerleşikleri (dil tanımı 10.2, S-kilit). Uzun süren bir işlemin
// hazırlığı yorumlayıcı kilidi altında yapılır (şekil denetimi, çıktı, hesap çizgesine kayıt,
// görevin türev bağlamıyla); sayısal çekirdek kilit bırakılmışken, depo izinleriyle çalışır
// (tensor/access.go). Bu arada başka görevler ilerler.

// unlockedMatMulWork, kilidin bırakıldığı en küçük çarpım işidir (M·N·K, toplu işlerde toplam).
// 2^27 bu makinede yaklaşık 1 ms'lik CPU çarpımıdır (1024×1024×1024 ≈ 8 ms, 2026-11-03); daha
// küçüğünde kilidi bırakıp geri almanın bedeli (bekleyen varsa kilidi en çok 5 ms ona vermek)
// kazançtan büyüktür.
const unlockedMatMulWork = 1 << 27

// runUnlocked hazırlanan çekirdeği kilit bırakılmışken çalıştırır ve sonucu verir. İzinler kilit
// altında alınır; alınamazsa kilit bırakılarak çakışan işlemin bitmesi beklenir ve hazırlık kilit
// geri alınınca yeniden yapılır (girdiler bu arada değişmiş olabilir). m nil ise ya da kilit
// tutulmuyorsa (Exclusive, tek görev) çekirdek olduğu yerde çalışır.
func runUnlocked(m *interp.VM, prepare func() (*tensor.Tensor, *tensor.Kernel)) *tensor.Tensor {
	for {
		r, k := prepare()
		if m == nil {
			k.RunLocked()
			return r
		}
		if m.OtherTasksLive() {
			k.ShareProcessors()
		}
		if k.TryAcquire() {
			m.Blocking(k.Run)
			k.End()
			return r
		}
		m.Blocking(k.WaitFree)
	}
}

// released tek çekirdekli bir tensör yerleşiğini (öğe öğe işlem) çalıştıran işlevi verir:
// çekirdeği büyükse ve CPU'daysa kilit bırakılarak (tensor.Released). Bileşik yerleşikler
// (geri yayılım, iyileştirici adımı) buradan geçmez: ortalarında kilit bırakılmaz.
func released(m *interp.VM, f func() *tensor.Tensor) func() *tensor.Tensor {
	if m == nil {
		return f
	}
	return func() *tensor.Tensor {
		return tensor.Released(func(prepare func() (*tensor.Tensor, *tensor.Kernel)) *tensor.Tensor {
			return runUnlocked(m, prepare)
		}, f)
	}
}

// stepOn iyileştiricinin adımıdır: parametreler CPU'daysa ve güncellenecek öğe sayısı en az
// unlockedStepWork ise kilit bırakılarak, tek çekirdek olarak (bütün izinler birlikte; öbür
// görevler yarım güncellenmiş parametre görmez, tensor/optimizer_step.go).
func stepOn(m *interp.VM, o *Optimizer) {
	var kernel func() *tensor.Kernel
	switch {
	case o.adamw != nil:
		kernel = o.adamw.StepKernel
	case o.sgd != nil:
		kernel = o.sgd.StepKernel
	}
	if m == nil || kernel == nil || !onCPU(o.params) || tensor.StepWork(o.params) < unlockedStepWork {
		o.opt.Step()
		return
	}
	runUnlocked(m, func() (*tensor.Tensor, *tensor.Kernel) { return nil, kernel() })
}

// unlockedStepWork, adımın kilidi bıraktığı en küçük güncellemedir (öğe sayısı; öğe öğe işlemin
// eşiğiyle aynı, ~1 ms [tahmin]).
const unlockedStepWork = 1 << 20

// onCPU bütün tensörlerin CPU'da olup olmadığıdır.
func onCPU(ts []*tensor.Tensor) bool {
	for _, t := range ts {
		if t.Device() != tensor.CPU {
			return false
		}
	}
	return true
}

// matMulOn a @ b'dir; iş büyükse ve CPU'daysa kilit bırakılarak.
func matMulOn(m *interp.VM, a, b *tensor.Tensor, outType tensor.DType) *tensor.Tensor {
	if m == nil || a.Device() != tensor.CPU || matMulWork(a.Shape(), b.Shape()) < unlockedMatMulWork {
		return tensor.MatMulOut(a, b, outType)
	}
	return runUnlocked(m, func() (*tensor.Tensor, *tensor.Kernel) { return tensor.MatMulKernel(a, b, outType) })
}

// matMulWork çarpımın M·N·K'sidir (en az iki boyutlu, denetlenmiş şekiller).
func matMulWork(as, bs []int) int {
	work := bs[len(bs)-1]
	for _, d := range as {
		work *= d
	}
	return work
}
