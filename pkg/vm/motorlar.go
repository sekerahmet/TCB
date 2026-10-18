package vm

import "sync"

// Ek motorlar (yapay zeka, web/görsel arayüz, veritabanı...) kendi yerleşiklerini her yeni
// VM'ye MotorEkle ile kaydeder. pkg/vm bu motorları içe aktarmaz; hepsini doğru sırayla
// tanıtan paket tr_dili/pkg/motor'dur.
var (
	motorMu  sync.Mutex
	motorlar []func(*VM)
)

// MotorEkle, New'in her VM için çekirdek yerleşiklerden sonra çağıracağı bir kayıt
// fonksiyonu ekler. Kayıtlar eklenme sırasıyla çalışır.
func MotorEkle(kaydet func(v *VM)) {
	motorMu.Lock()
	defer motorMu.Unlock()
	motorlar = append(motorlar, kaydet)
}

func (v *VM) registerMotors() {
	motorMu.Lock()
	kayitlar := make([]func(*VM), len(motorlar))
	copy(kayitlar, motorlar)
	motorMu.Unlock()
	for _, kaydet := range kayitlar {
		kaydet(v)
	}
}

// GorevKancasiEkle, bu VM'nin görevlerinden biri (ana program, eşzamanlı blok, web/GUI
// isteği) yorumlayıcı kilidini her aldığında, kilidi alan görevin goroutine'inde ve
// görünümüyle çağrılacak bir kanca ekler. Motorlar süreç geneli durumlarını (ör. tensör
// çekirdeğinin türevsiz kipi) kilidi tutan göreve göre ayarlamak için kullanır; kanca
// görev durumunu GorevDegeri ile okur. Yalnızca motor kaydı sırasında (program
// başlamadan) çağrılmalıdır. Kanca kısa olmalıdır: kilidin her el değiştirişinde çalışır.
func (v *VM) GorevKancasiEkle(f func(*VM)) {
	v.gorevKancalari = append(v.gorevKancalari, f)
}
