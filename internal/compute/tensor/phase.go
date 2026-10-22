package tensor

import (
	"sync/atomic"
	"time"
	"unsafe"
)

// ============================================================================
// Adım evreleri: geri yayılımın süresi ve iyileştiricinin adımı
// ============================================================================
//
// Profil bir eğitim adımını ileri, geri ve sonrası (iyileştirici, kırpma, günlük) diye
// böler. Bunun için iki şey bilir: geri yayılımların toplam süresi ile sonuncusunun bittiği
// an (BackwardTimes) ve, program ilerleme(...).güncelle(...) kullanmıyorsa adım sınırı
// yerine geçen iyileştirici adımı (SetOptimizerStepHook). PyTorch profilinde adım
// kullanıcının prof.step() çağrısıyla, JAX'ta StepTraceAnnotation ile işaretlenir; TR'de açık
// işaret ilerleme(...)'dir, iyileştiricinin adımı yedektir.
//
// Süreler konakta ölçülür (geri yayılım başına iki saat okuması). GPU'da işler eşzamansız
// olduğundan bu, işlerin başlatıldığı süredir; eşzamanlamada beklenen süre beklediği evreye
// düşer.

// phaseStart bitiş anlarının başlangıcıdır.
var phaseStart = time.Now()

// backwardTicks bütün geri yayılımların toplam süresidir (sayaç saatinin tıkı:
// counter_clock*.go; Windows'ta duvar saatinin çözünürlüğü kısa geri yayılımı sıfır
// gösterirdi). backwardEndNs sonuncusunun bittiği andır (phaseStart'tan beri monoton
// nanosaniye: adım sınırlarının saatiyle aynı saat; tıkı ana çevirmek uzun koşuda kayardı;
// 0: henüz yok).
var backwardTicks, backwardEndNs int64

// BackwardTimes geri yayılımların süreç başından beri toplam süresini ve sonuncusunun bittiği
// anı verir (hiç geri yayılım yoksa end sıfır zamandır).
func BackwardTimes() (total time.Duration, end time.Time) {
	total = time.Duration(float64(atomic.LoadInt64(&backwardTicks)) * tickDurationRatio())
	if e := atomic.LoadInt64(&backwardEndNs); e != 0 {
		end = phaseStart.Add(time.Duration(e))
	}
	return total, end
}

// backwardDepth iç içe geri yayılımların derinliğidir (özel geri hesabın içindeki bir geri
// yayılım dıştakinin süresindedir, iki kez sayılmaz).
var backwardDepth int32

// backwardBegin bir geri yayılımın başlangıç tıkıdır (iç içe olanda -1); backwardDone ile
// kapatılır: defer backwardDone(backwardBegin()). Üç saat okuması, kapanış ayrılmaz.
func backwardBegin() int64 {
	if atomic.AddInt32(&backwardDepth, 1) != 1 {
		return -1
	}
	return clock()
}

// backwardDone geri yayılımın süresini ve bitiş anını yazar.
func backwardDone(begin int64) {
	if begin >= 0 {
		atomic.AddInt64(&backwardTicks, max(0, clock()-begin))
		atomic.StoreInt64(&backwardEndNs, int64(time.Since(phaseStart)))
	}
	atomic.AddInt32(&backwardDepth, -1)
}

// optimizerStepHook iyileştirici adımı bitince çağrılır (nil: kanca yok).
var optimizerStepHook atomic.Pointer[func(id uintptr, at time.Time)]

// SetOptimizerStepHook iyileştirici adımının kancasını kurar (nil kaldırır). id iyileştiricinin
// kimliğidir (birden çok iyileştirici varsa profil ilk adım atanı sayar).
func SetOptimizerStepHook(f func(id uintptr, at time.Time)) {
	if f == nil {
		optimizerStepHook.Store(nil)
		return
	}
	optimizerStepHook.Store(&f)
}

// optimizerStepped iyileştiricinin adımı bittiğinde kancayı çağırır.
func optimizerStepped(o unsafe.Pointer) {
	if f := optimizerStepHook.Load(); f != nil {
		(*f)(uintptr(o), time.Now())
	}
}
