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
