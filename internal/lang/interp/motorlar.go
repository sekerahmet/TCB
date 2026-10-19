package interp

import "sync"

// Ek motorlar (tensör, veri, parçalayıcı, JSON/CSV ...) kendi yerleşiklerini her yeni
// VM'ye MotorEkle ile kaydeder. internal/lang/interp bu motorları içe aktarmaz; hepsini doğru sırayla
// tanıtan paket github.com/sekerahmet/TCB/internal/stdlib'dur.
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

// GorevKancasiEkle, bu VM'nin görevlerinden biri (ana program ya da eşzamanlı blok)
// yorumlayıcı kilidini her aldığında, kilidi alan görevin goroutine'inde ve
// görünümüyle çağrılacak bir kanca ekler. Motorlar süreç geneli durumlarını (ör. tensör
// çekirdeğinin türevsiz kipi) kilidi tutan göreve göre ayarlamak için kullanır; kanca
// görev durumunu GorevDegeri ile okur. Yalnızca motor kaydı sırasında (program
// başlamadan) çağrılmalıdır. Kanca kısa olmalıdır: kilidin her el değiştirişinde çalışır.
func (v *VM) GorevKancasiEkle(f func(*VM)) {
	v.gorevKancalari = append(v.gorevKancalari, f)
}
