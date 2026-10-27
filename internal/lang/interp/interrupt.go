package interp

import (
	"fmt"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kesilme (nazik çıkış): kullanıcı Ctrl+C'ye bastığında ya da süreç sonlandırma isteği
// (SIGTERM; Windows'ta konsol penceresinin kapanması) aldığında komut satırı Kes'i
// çağırır. VM programı bir sonraki güvenli noktada (döngü turu, fonksiyon çağrısı) durdurur:
// yığın çözülürken bütün 'ertele' blokları çalışır, 'dene/yakala' kesilmeyi yakalayamaz ve
// Run ErrKesildi döndürür (komut satırı 3 koduyla çıkar). Bekleme (bekle) hemen uyanır.
//
// Sıcak yola ek denetim girmez: Kes, kilit bekleyen görev sayacını (bekleyen) artırır;
// döngüler ve çağrılar zaten bu sayaca baktığından kilit devri yoluna (kilidiDevret) girer ve
// kesilme orada fark edilir.

// ErrInterrupted reports that the program was stopped by an interrupt signal; dene/yakala does
// not catch it.
var ErrInterrupted error = messages.New(messages.H2461)

// interruptPanic yığını çözmek için atılan paniğin değeridir. Bir motorun panik kurtarıcısı
// bunu yakalarsa KesilmeMi ile tanıyıp yeniden atmalıdır (hataya çevirmemelidir).
type interruptPanic struct{}

// IsInterrupt reports whether a recovered panic value is an interruption. If so the caller must
// re-panic: an interruption is not caught like an error.
func IsInterrupt(r any) bool {
	_, ok := r.(interruptPanic)
	return ok
}

// Interrupt asks the program to stop gracefully; it may be called from any goroutine. It
// returns true on the first call (later calls do nothing).
func (v *VM) Interrupt() bool {
	if !v.interrupted.CompareAndSwap(false, true) {
		return false
	}
	close(v.interruptChan)
	v.pending.Add(1) // döngüler ve çağrılar kilit devri yoluna girsin (bkz. kilidiDevret)
	return true
}

// Interrupted reports whether an interrupt was requested for the program.
func (v *VM) Interrupted() bool { return v.interrupted.Load() }

// Ertelenen kesme (kontrol noktası): bir kontrol noktası kurulunca (KesmeyiErtele) ilk
// kesme isteği programı hemen durdurmaz, yalnızca bayrak koyar (DurdurmaIstendi). Eğitim
// döngüsü sıradaki adım sınırında bayrağı görür, kontrol noktasını kaydeder ve programı
// SimdiKes ile durdurur; böylece yarım adım hiç kaydedilmez ve sürdürülen eğitim kesintisizle
// aynı sonucu verir. İkinci istek (komut satırında) süreci hemen bitirir.

// DeferInterrupts asks that later interrupt requests be deferred to the step boundary; called
// when a checkpoint is set up. It may be called from any goroutine.
func (v *VM) DeferInterrupts() { v.interruptDeferrers.Add(1) }

// RequestInterrupt delivers an interrupt request (Ctrl+C, a termination signal, a background
// job's stop request): if a checkpoint defers interrupts it only sets a flag and returns true
// (the program saves and stops at the step boundary); otherwise it stops the program like
// Interrupt and returns false. It may be called from any goroutine.
func (v *VM) RequestInterrupt() (deferred bool) {
	if v.interruptDeferrers.Load() > 0 && !v.interrupted.Load() {
		v.stopRequested.Store(true)
		return true
	}
	v.Interrupt()
	return false
}

// StopRequested reports whether a deferred interrupt request is pending. The checkpoint checks
// it at the step boundary.
func (v *VM) StopRequested() bool { return v.stopRequested.Load() }

// InterruptNow stops the program gracefully at this point: it calls Interrupt and starts
// unwinding the stack right away (the calling builtin does not return; deferred work runs,
// dene/yakala does not catch it, Run returns ErrInterrupted). It is called from inside a builtin,
// on the task holding the interpreter lock.
func (v *VM) InterruptNow() {
	v.Interrupt()
	v.checkInterrupt()
}

// checkInterrupt kilit devrinden sonra çağrılır: kesme istendiyse ve bu görev henüz
// çözülmeye başlamadıysa yığını çözen paniği atar. Çözülme sırasında çalışan 'ertele'
// blokları yeniden kesilmez (görev başına bir kez).
func (v *VM) checkInterrupt() {
	if v.interrupted.Load() && !v.interruptThrown {
		v.interruptThrown = true
		panic(interruptPanic{})
	}
}

// panicError kurtarılan bir paniği hataya çevirir; kesilme ErrKesildi olur.
func panicError(r any) error {
	if IsInterrupt(r) {
		return ErrInterrupted
	}
	return messages.Internal(fmt.Sprint(r))
}

// interruptibleSleep süre boyunca ya da kesme istenene kadar bekler.
func (v *VM) interruptibleSleep(duration time.Duration) {
	if duration <= 0 {
		return
	}
	z := time.NewTimer(duration)
	defer z.Stop()
	select {
	case <-z.C:
	case <-v.interruptChan:
	}
}
