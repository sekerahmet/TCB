package yurut

import (
	"errors"
	"fmt"
	"time"
)

// Kesilme (nazik çıkış): kullanıcı Ctrl+C'ye bastığında ya da süreç sonlandırma isteği
// (SIGTERM; Windows'ta konsol kapatma/oturum kapatma olayı) aldığında komut satırı Kes'i
// çağırır. VM programı bir sonraki güvenli noktada (döngü turu, fonksiyon çağrısı) durdurur:
// yığın çözülürken bütün 'ertele' blokları çalışır, 'dene/yakala' kesilmeyi yakalayamaz ve
// Run ErrKesildi döndürür (komut satırı 3 koduyla çıkar). Bekleme (bekle) hemen uyanır.
//
// Sıcak yola ek denetim girmez: Kes, kilit bekleyen görev sayacını (bekleyen) artırır;
// döngüler ve çağrılar zaten bu sayaca baktığından kilit devri yoluna (kilidiDevret) girer ve
// kesilme orada fark edilir.

// ErrKesildi programın kesme sinyaliyle durdurulduğunu söyler; dene/yakala bunu yakalamaz.
var ErrKesildi = errors.New("program durduruldu (kesme isteği: Ctrl+C ya da sonlandırma sinyali)")

// kesilmePanigi yığını çözmek için atılan paniğin değeridir. Bir motorun panik kurtarıcısı
// bunu yakalarsa KesilmeMi ile tanıyıp yeniden atmalıdır (hataya çevirmemelidir).
type kesilmePanigi struct{}

// KesilmeMi kurtarılan panik değeri (recover) bir kesilme mi? Öyleyse çağıran paniği
// yeniden atmalıdır: kesilme bir hata gibi yakalanmaz.
func KesilmeMi(r any) bool {
	_, ok := r.(kesilmePanigi)
	return ok
}

// Kes programın nazikçe durdurulmasını ister; herhangi bir goroutine'den çağrılabilir. İlk
// çağrıda doğru döner (sonrakiler bir şey yapmaz).
func (v *VM) Kes() bool {
	if !v.kesildi.CompareAndSwap(false, true) {
		return false
	}
	close(v.kesilmeKanali)
	v.bekleyen.Add(1) // döngüler ve çağrılar kilit devri yoluna girsin (bkz. kilidiDevret)
	return true
}

// Kesildi program için kesme istendi mi?
func (v *VM) Kesildi() bool { return v.kesildi.Load() }

// KesilmeKanali kesme istenince kapanan kanaldır; uzun beklemeler (bekle, motorların
// beklemeleri) bununla erken uyanır.
func (v *VM) KesilmeKanali() <-chan struct{} { return v.kesilmeKanali }

// kesilmeyiDenetle kilit devrinden sonra çağrılır: kesme istendiyse ve bu görev henüz
// çözülmeye başlamadıysa yığını çözen paniği atar. Çözülme sırasında çalışan 'ertele'
// blokları yeniden kesilmez (görev başına bir kez).
func (v *VM) kesilmeyiDenetle() {
	if v.kesildi.Load() && !v.kesmeAtildi {
		v.kesmeAtildi = true
		panic(kesilmePanigi{})
	}
}

// panikHatasi kurtarılan bir paniği hataya çevirir; kesilme ErrKesildi olur.
func panikHatasi(r any) error {
	if KesilmeMi(r) {
		return ErrKesildi
	}
	return fmt.Errorf("İç hata (çökme önlendi): %v", r)
}

// kesilebilirUyu süre boyunca ya da kesme istenene kadar bekler.
func (v *VM) kesilebilirUyu(sure time.Duration) {
	if sure <= 0 {
		return
	}
	z := time.NewTimer(sure)
	defer z.Stop()
	select {
	case <-z.C:
	case <-v.kesilmeKanali:
	}
}
