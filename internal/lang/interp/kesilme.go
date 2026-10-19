package interp

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

// Ertelenen kesme (kontrol noktası): bir kontrol noktası kurulunca (KesmeyiErtele) ilk
// kesme isteği programı hemen durdurmaz, yalnızca bayrak koyar (DurdurmaIstendi). Eğitim
// döngüsü sıradaki adım sınırında bayrağı görür, kontrol noktasını kaydeder ve programı
// SimdiKes ile durdurur; böylece yarım adım hiç kaydedilmez ve sürdürülen eğitim kesintisizle
// aynı sonucu verir. İkinci istek (komut satırında) süreci hemen bitirir.

// KesmeyiErtele bundan sonraki kesme isteklerinin adım sınırına ertelenmesini ister; bir
// kontrol noktası kurulunca çağrılır. Herhangi bir goroutine'den çağrılabilir.
func (v *VM) KesmeyiErtele() { v.kesmeErteleyen.Add(1) }

// KesmeIste bir kesme isteğini (Ctrl+C, sonlandırma sinyali, arka plan işinin durdurma
// isteği) iletir: kesmeyi erteleyen bir kontrol noktası varsa yalnızca bayrak koyar ve doğru
// döner (program adım sınırında kaydedip duracak); yoksa Kes gibi programı durdurur ve yanlış
// döner. Herhangi bir goroutine'den çağrılabilir.
func (v *VM) KesmeIste() (ertelendi bool) {
	if v.kesmeErteleyen.Load() > 0 && !v.kesildi.Load() {
		v.durdurmaIstendi.Store(true)
		return true
	}
	v.Kes()
	return false
}

// DurdurmaIstendi ertelenmiş bir kesme isteği var mı? Kontrol noktası adım sınırında bakar.
func (v *VM) DurdurmaIstendi() bool { return v.durdurmaIstendi.Load() }

// SimdiKes programı bu noktada nazikçe durdurur: Kes'i çağırır ve yığını hemen çözmeye
// başlar (çağıran yerleşik dönmez; ertelenen işler çalışır, dene/yakala yakalamaz, Run
// ErrKesildi döndürür). Yorumlayıcı kilidini tutan görevden, bir yerleşiğin içinden çağrılır.
func (v *VM) SimdiKes() {
	v.Kes()
	v.kesilmeyiDenetle()
}

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
