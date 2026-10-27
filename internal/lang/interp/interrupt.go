package interp

import (
	"fmt"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Durdurma (nazik çıkış, 1.5-1.7 ve 1.15): komut satırı Ctrl+C'yi ve sonlandırma isteklerini
// RequestStop ile iletir. İstek dört yoldan karşılanır:
//
//   - Uygulanır (StopApplied): görevler bir sonraki güvenli noktada (döngü turu, fonksiyon
//     çağrısı) durur. Yığın çözülürken bütün 'ertele' blokları çalışır, 'dene/yakala' isteği
//     yakalayamaz ve Run ErrInterrupted döndürür. Programın durumunu değiştirmeyen bekleme
//     (bekle, girdi, dosya okuma) bırakılır; durum değiştiren yerleşik biter.
//   - Üstlenilmiş programda (StopHandled, TR_handle_stop) yalnız kaydedilir; yeni bir şey bekleyen
//     yerleşik "durdurma istendi" hatasıyla döner, program kendi sınırında biter (1.6).
//   - Kontrol noktası kuruluysa adım sınırına ertelenir (StopDeferred): sınırlı bir süre içinde
//     .güncelle ya da .kaydet noktayı kaydeder ve durur; süre dolarsa istek uygulanır (1.6).
//   - Program biterken (StopFinishing) ertelenen işleri kesmez (1.15).
//
// Sıcak yola ek denetim girmez: Interrupt kilit bekleyen görev sayacını (pending) artırır;
// döngüler ve çağrılar zaten bu sayaca baktığından kilit devri yoluna (handOffLock) girer ve
// durdurma orada fark edilir.

// ErrInterrupted reports that the program was stopped by an interrupt signal; dene/yakala does
// not catch it.
var ErrInterrupted error = messages.New(messages.H2461)

// interruptPanic yığını çözmek için atılan paniğin değeridir. Bir motorun panik kurtarıcısı
// bunu yakalarsa IsInterrupt ile tanıyıp yeniden atmalıdır (hataya çevirmemelidir).
type interruptPanic struct{}

// IsInterrupt reports whether a recovered panic value is an interruption. If so the caller must
// re-panic: an interruption is not caught like an error.
func IsInterrupt(r any) bool {
	_, ok := r.(interruptPanic)
	return ok
}

// StopDisposition bir durdurma isteğinin nasıl karşılandığını söyler.
type StopDisposition int

const (
	// StopApplied: istek uygulandı, program durduruluyor (1.5).
	StopApplied StopDisposition = iota
	// StopHandled: program durdurmayı üstlenmişti, istek kaydedildi (1.6).
	StopHandled
	// StopDeferred: kontrol noktası isteği adım sınırına erteledi (1.6).
	StopDeferred
	// StopFinishing: program bitiyordu, ertelenen işler kesilmeyecek (1.15).
	StopFinishing
	// StopRepeated: ilk istek zaten gelmişti; bu istekle bir şey değişmedi.
	StopRepeated
)

// RequestStop bir durdurma isteğini iletir; signal isteğin sinyal numarasıdır (sıfırdan büyük;
// süreç ilk isteğin sinyaliyle biter, 1.9). Herhangi bir goroutine'den çağrılabilir.
func (v *VM) RequestStop(signal int) StopDisposition {
	v.stopMu.Lock()
	if !v.stopSignal.CompareAndSwap(0, int32(signal)) {
		v.stopMu.Unlock()
		return StopRepeated
	}
	switch {
	case v.finishing.Load():
		v.stopMu.Unlock()
		return StopFinishing
	case v.interrupted.Load():
		v.stopMu.Unlock()
		return StopApplied
	case v.stopHandled.Load():
		v.stopRequested.Store(true)
		close(v.stopChan)
		v.stopMu.Unlock()
		return StopHandled
	}
	if window, ok := v.deferralWindow(); ok {
		v.stopRequested.Store(true)
		v.stopTimer = time.AfterFunc(window, func() { v.Interrupt() })
		v.stopMu.Unlock()
		return StopDeferred
	}
	v.stopMu.Unlock()
	v.Interrupt()
	return StopApplied
}

// deferralWindow isteğin ertelenebileceği en uzun süredir: her kontrol noktası bir süre
// söylemelidir (ikinci .güncelle'den önce gelen istek ertelenmez), en kısası geçerlidir.
// stopMu tutulurken çağrılır.
func (v *VM) deferralWindow() (time.Duration, bool) {
	if len(v.stopWindows) == 0 {
		return 0, false
	}
	var shortest time.Duration
	for i, window := range v.stopWindows {
		d, ok := window()
		if !ok {
			return 0, false
		}
		if i == 0 || d < shortest {
			shortest = d
		}
	}
	return shortest, true
}

// DeferStops, bir kontrol noktası kurulduğunu bildirir: sonraki ilk istek programı hemen
// durdurmaz, window'un söylediği süre içinde noktanın kaydedilmesini bekler. window herhangi bir
// goroutine'den çağrılır; ikinci .güncelle'den önce ya da adım süresi bilinmiyorsa false verir
// (istek 1.5'teki gibi uygulanır).
func (v *VM) DeferStops(window func() (time.Duration, bool)) {
	v.stopMu.Lock()
	v.stopWindows = append(v.stopWindows, window)
	v.stopMu.Unlock()
}

// HandleStop programın durdurmayı üstlendiğini bildirir (TR_handle_stop, 1.6): ondan sonra gelen
// ilk istek programı durdurmaz, kaydedilir.
func (v *VM) HandleStop() {
	v.stopMu.Lock()
	v.stopHandled.Store(true)
	v.stopMu.Unlock()
}

// StopHandled program durdurmayı üstlendi mi?
func (v *VM) StopHandled() bool { return v.stopHandled.Load() }

// StopRequested, uygulanmayıp kaydedilen (üstlenilmiş ya da ertelenmiş) bir isteğin geldiğini
// söyler. Kontrol noktası adım sınırında bakar.
func (v *VM) StopRequested() bool { return v.stopRequested.Load() }

// StopSignal ilk isteğin sinyal numarasıdır (0: istek gelmedi).
func (v *VM) StopSignal() int { return int(v.stopSignal.Load()) }

// Interrupt applies a stop to every task: they unwind at their next safe point and abandonable
// waits end. It may be called from any goroutine and returns true on the first call (later
// calls do nothing).
func (v *VM) Interrupt() bool {
	if !v.interrupted.CompareAndSwap(false, true) {
		return false
	}
	v.stopMu.Lock()
	if v.stopTimer != nil {
		v.stopTimer.Stop()
	}
	v.stopMu.Unlock()
	close(v.interruptChan)
	v.pending.Add(1) // döngüler ve çağrılar kilit devri yoluna girsin (bkz. handOffLock)
	return true
}

// Interrupted reports whether a stop was applied to the program.
func (v *VM) Interrupted() bool { return v.interrupted.Load() }

// InterruptNow stops the program gracefully at this point: it calls Interrupt and starts
// unwinding the stack right away (the calling builtin does not return; deferred work runs,
// dene/yakala does not catch it, Run returns ErrInterrupted). It is called from inside a builtin,
// on the task holding the interpreter lock.
func (v *VM) InterruptNow() {
	v.Interrupt()
	v.checkInterrupt()
}

// checkInterrupt kilit devrinden sonra çağrılır: durdurma uygulandıysa ve bu görev henüz
// çözülmeye başlamadıysa yığını çözen paniği atar. Çözülme sırasında ve temizlik işlerinin
// (ertele blokları) içinde yeniden kesilmez.
func (v *VM) checkInterrupt() {
	if v.interrupted.Load() && !v.interruptThrown && v.cleanupDepth == 0 {
		v.interruptThrown = true
		panic(interruptPanic{})
	}
}

// panicError kurtarılan bir paniği hataya çevirir; kesilme ErrInterrupted olur.
func panicError(r any) error {
	if IsInterrupt(r) {
		return ErrInterrupted
	}
	return messages.Internal(fmt.Sprint(r))
}

// runDefers e'nin ertelenen işlerini çalıştırır. Ertelenen işlerin içinde beklemeler bırakılmaz
// ve durdurma hatası vermez (1.5, 1.6): görev o sürece temizlik içindedir.
func (v *VM) runDefers(e *Environment) error {
	v.cleanupDepth++
	defer func() { v.cleanupDepth-- }()
	return e.RunDefers()
}

// stopWaitError, üstlenilmiş bir programda istek geldikten sonra yeni bir şey bekleyen
// yerleşiğin hatasıdır (1.6). Temizlik işlerinde (ertele blokları, bu hatayı yakalayan yakala
// gövdesi) ve istek gelmediyse nil.
func (v *VM) stopWaitError(fn any) error {
	if v.cleanupDepth > 0 || !v.stopHandled.Load() || !v.stopRequested.Load() {
		return nil
	}
	return messages.New(messages.H2530, fn, codes.NameRef(codes.FnStopRequested), codes.NameRef(codes.KwDefer), codes.NameRef(codes.KwCatch))
}

// IsStopRequestedError hatanın "durdurma istendi" hatası (1.6) olup olmadığını söyler. Onu
// yakalayan yakala gövdesi bir temizlik işidir: orada beklemeler normal çalışır.
func IsStopRequestedError(err error) bool { return messages.Has(err, messages.H2530) }

// wakeChannels bu görevin beklemesini bitirecek olayların kanallarıdır: applied uygulanan bir
// durdurma, requested üstlenilmiş programa gelen istek (yalnız yeni bir şey bekleyen yerleşik
// için). Temizlik işinde ve yığın çözülürken ikisi de nil'dir: bekleme kesilmez.
func (v *VM) wakeChannels(forNew bool) (applied, requested <-chan struct{}) {
	if v.cleanupDepth > 0 || v.interruptThrown {
		return nil, nil
	}
	applied = v.interruptChan
	if forNew && v.stopHandled.Load() {
		requested = v.stopChan
	}
	return applied, requested
}

// awaitReady, ready kapanana kadar yorumlayıcı kilidi bırakılarak bekler. Uygulanan bir
// durdurma beklemeyi bırakır: görev burada yığını çözer (bekleyenin sonucu atılır). forNew
// yeni bir şey bekleyen yerleşik içindir (bekle, girdi): üstlenilmiş programa istek gelince ya
// da gelmişken "durdurma istendi" hatasıyla döner (1.6). Bekleme tamamlandıysa nil döner.
func (v *VM) awaitReady(fn any, ready <-chan struct{}, forNew bool) error {
	if forNew {
		if err := v.stopWaitError(fn); err != nil {
			return err
		}
	}
	applied, requested := v.wakeChannels(forNew)
	v.Blocking(func() {
		select {
		case <-ready:
		case <-applied:
		case <-requested:
		}
	})
	select {
	case <-ready:
		return nil
	default:
	}
	v.checkInterrupt() // kilit devri yolu atmadıysa (Exclusive) burada atılır
	if forNew {
		if err := v.stopWaitError(fn); err != nil {
			return err
		}
	}
	return ErrInterrupted
}

// Abandonable, programın durumunu değiştirmeyen bir beklemeyi (dosya okuma, veri yükleme)
// yapar: work kendi goroutine'inde, yorumlayıcı kilidi bırakılarak koşar. Uygulanan bir
// durdurma beklemeyi bırakır; work arka planda biter, sonucu atılır (1.5). Bitiren iş dönüşte
// nil verir. work kilidi gerektiren hiçbir şeye dokunmamalı ve bırakılırsa yan etkisi
// olmamalıdır.
func (v *VM) Abandonable(work func()) error {
	return v.await(nil, work, false)
}

// AwaitNew Abandonable gibi bekler, ama yeni bir şey bekleyen yerleşik içindir (girdi, kamera
// karesi, ileti): üstlenilmiş programda istek gelince fn adıyla "durdurma istendi" hatası
// verir (1.6).
func (v *VM) AwaitNew(fn any, work func()) error {
	return v.await(fn, work, true)
}

func (v *VM) await(fn any, work func(), forNew bool) error {
	if forNew {
		if err := v.stopWaitError(fn); err != nil {
			return err
		}
	}
	done := make(chan struct{})
	var crashed any
	go func() {
		defer close(done)
		defer func() { crashed = recover() }()
		work()
	}()
	err := v.awaitReady(fn, done, forNew)
	if err == nil && crashed != nil {
		panic(crashed) // yerleşiğin beklenmedik paniği, kendi goroutine'inde olsaydı olacağı gibi
	}
	return err
}

// sleep bekle() için süre kadar bekler (bkz. awaitReady).
func (v *VM) sleep(fn any, d time.Duration) error {
	if err := v.stopWaitError(fn); err != nil {
		return err
	}
	if d <= 0 {
		return nil
	}
	ready := make(chan struct{})
	timer := time.AfterFunc(d, func() { close(ready) })
	defer timer.Stop()
	return v.awaitReady(fn, ready, true)
}

// handleStopBuiltin durdurmayı_üstlen() yerleşiğidir (1.6).
func handleStopBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 0 {
		return nil, ArgCountError(codes.FnHandleStop, 0, len(args))
	}
	y.HandleStop()
	return Null, nil
}

// stopRequestedBuiltin durdurma_istendi() yerleşiğidir (1.6): hiçbir şeyi değiştirmez.
func stopRequestedBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 0 {
		return nil, ArgCountError(codes.FnStopRequested, 0, len(args))
	}
	if y.StopSignal() != 0 {
		return True, nil
	}
	return False, nil
}
