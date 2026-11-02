package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- GÖREVİN İPTALİ (10.9) ---
//
// g.TR_cancel() 1.5'in durdurmasını tek göreve uygular: görev bir sonraki güvenli noktada
// (kullanıcı fonksiyonu çağrısı, döngü turu, süren bir yerleşiğin beklemesi) yığınını çözer;
// durum değiştiren yerleşik biter, beklemeler bırakılır. İptal görevin içinde TR_catch ile
// yakalanmaz (durdurmanın paniği ve ErrInterrupted gibi). Görevin başlattığı ve bitmemiş görevler de
// iptal edilir. Görev durunca (alt görevleri bitip ertelenenleri çalışınca) onu bekleyen iptal
// hatasını (H2920) alır. İptal bir hata değildir: görevin bitişinde yazılmaz (10.6).
//
// Güvenli noktalar durdurmanınkilerdir (checkInterrupt, wakeChannels): görev görünümü kendi
// görevinin bayrağına ve kanalına da bakar. Fonksiyon girişi yalnız pending sıfır değilken yavaş
// yola girer; iptal görev durana kadar pending'i bir artırır (Interrupt gibi).

// cancel iptali görevde ve bitmemiş alt görevlerinde başlatır; bitmiş görevde ya da ikinci kez
// etkisizdir. Kilidi tutan herhangi bir görev çağırabilir.
func (t *TaskVal) cancel(y *VM) {
	t.mu.Lock()
	if t.ended || t.cancelled.Load() {
		t.mu.Unlock()
		return
	}
	t.cancelled.Store(true)
	y.pending.Add(1) // fonksiyon girişleri iptale baksın (finish geri alır)
	close(t.cancelCh)
	children := t.children
	t.mu.Unlock()
	for _, c := range children {
		c.cancel(y)
	}
}

// addChild child'ı görevin alt görevi olarak kaydeder: görev bitmeden önce beklenir. İptal
// edilmiş görevin yeni alt görevi de iptal edilir.
func (t *TaskVal) addChild(y *VM, child *TaskVal) {
	t.mu.Lock()
	t.children = append(t.children, child)
	cancelled := t.cancelled.Load()
	t.mu.Unlock()
	if cancelled {
		child.cancel(y)
	}
}

// endChildren görev gövdesi bitince alt görevleri bekler (10.9: görev program gibi biter); görev
// hatayla ya da iptalle bittiyse (failed) önce onları iptal eder. Bekleme bir temizlik işidir:
// durdurma ya da iptal onu kesmez. y görevin kendi görünümüdür.
func (t *TaskVal) endChildren(y *VM, failed bool) {
	t.mu.Lock()
	children := t.children
	t.children = nil
	t.mu.Unlock()
	if failed {
		for _, c := range children {
			c.cancel(y)
		}
	}
	y.cleanupDepth++
	defer func() { y.cleanupDepth-- }()
	for _, c := range children {
		_ = y.awaitReady(nil, c.done, false) // temizlikte bırakılmaz, hep nil döner
	}
}

// cancelError iptal edilen görevin bitiş hatasıdır: iptalin kendisi (görevin durması) iptal
// hatasına (H2920) döner; ertelenen işlerin hataları ona eklenir (9.4). Program durdurulduysa
// (1.5) ya da görev iptalden önce kendi hatasıyla bittiyse hata olduğu gibi kalır.
func (t *TaskVal) cancelError(y *VM, err error) error {
	deferred, main := splitProgramErrors(err)
	if main != nil || y.interrupted.Load() {
		return err
	}
	var out error = messages.New(messages.H2920, t.line)
	for _, d := range deferred {
		out = addDeferError(out, d)
	}
	return out
}
