package tensor

// Kilidi bırakan tek çekirdekli işlemler (dil tanımı 10.2, S-kilit). Dil katmanı tek çekirdekli
// bir yerleşiği (öğe öğe işlem) Released içinde çağırır; işlemin çekirdeği büyükse ve CPU'daysa
// verilen UnlockRunner'la, yorumlayıcı kilidi bırakılarak çalışır. Kapsam dışında (bileşik
// yerleşikler, geri yayılım, iyileştirici) çekirdekler kilit altında çalışır.

// UnlockRunner hazırlanan çekirdeği kilit bırakılarak çalıştırır ve sonucu verir (dil katmanı).
// Çakışan bir kilitsiz işlem varsa prepare kilit geri alınınca yeniden çağrılabilir.
type UnlockRunner func(prepare func() (*Tensor, *Kernel)) *Tensor

// unlockedElemWork, kilidin bırakıldığı en küçük öğe öğe işlemdir (çıktının öğe sayısı).
// 2^20 öğe bu makinede yaklaşık 1 ms'lik işlemdir [tahmin]; küçüğünde kilidi bırakıp geri
// almanın bedeli kazançtan büyüktür (unlockedMatMulWork'le aynı ölçü).
const unlockedElemWork = 1 << 20

// releaseScope etkin Released kapsamıdır; yalnız yorumlayıcı kilidi tutulurken okunur ve
// yazılır. Kapsam ilk çekirdekte kullanılmış olur (kernels > 0); kilit ancak ondan sonra
// bırakılabildiğinden kilidi o arada alan başka görev kullanılmamış bir kapsam görmez.
type releaseScope struct {
	run     UnlockRunner
	kernels int // kapsamda hazırlanan öğe öğe çekirdek sayısı
}

var activeRelease *releaseScope

// Released f'yi çalıştırır. f tek çekirdekli bir işlem olmalıdır: kapsamda yalnız ilk çekirdek
// bırakılabilir. İki çekirdek arasında kilit bırakılsaydı başka görev girdiyi yerinde
// değiştirebilir ve sonuç iki ayrı girdinin karışığı olurdu; bu yüzden ikinci ve sonraki
// çekirdekler kilit altında çalışır. Kapsamlar iç içe girmez. Kural testi: dil katmanının
// sarmaladığı her yerleşik tek çekirdektir.
func Released(run UnlockRunner, f func() *Tensor) *Tensor {
	activeRelease = &releaseScope{run: run}
	defer func() { activeRelease = nil }()
	return f()
}

// runElem öğe öğe bir işlemin çekirdeğini çalıştırır: Released kapsamında, kapsamın ilk
// çekirdeğiyse, CPU'daysa ve işi büyükse kilit bırakılarak; değilse kilit altında. prepare'in
// çekirdeği nil olabilir (birleştirilecek, sembolik sonuç).
func runElem(be Backend, numel int, prepare func() (*Tensor, *Kernel)) *Tensor {
	s := activeRelease
	if s != nil {
		s.kernels++
	}
	if s != nil && s.kernels == 1 && be.Device() == CPU && numel >= unlockedElemWork {
		return s.run(func() (*Tensor, *Kernel) {
			r, k := prepare()
			if k == nil {
				k = newKernel(be, nil, nil, nil, func() {})
			}
			return r, k
		})
	}
	r, k := prepare()
	if k != nil {
		k.RunLocked()
	}
	return r
}
