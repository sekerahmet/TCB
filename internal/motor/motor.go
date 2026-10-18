// Package motor, çekirdek yorumlayıcının (internal/dil/vm) içe aktarmadığı parçaları ona bağlar:
// tensör ve yapay zekâ, veri ve parçalayıcı, JSON/CSV motorlarının yerleşikleri ile koddaki
// kesme_noktası deyiminin açtığı hata ayıklayıcı. Tam dili çalıştıran her program bu
// paketi içe aktarmalıdır:
//
//	import _ "tr_dili/internal/motor"
package motor

import (
	"time"

	"tr_dili/internal/arac/hataayiklama"
	"tr_dili/internal/dil/vm"
	"tr_dili/internal/motor/parcalayici"
	"tr_dili/internal/motor/tensor"
	"tr_dili/internal/motor/veri"
	"tr_dili/internal/motor/veribicim"
)

func init() {
	vm.MotorEkle(veribicim.Kaydet)
	// Tensör motoru (tensör, otomatik türev, katmanlar) çekirdek yerleşiklerini (toplam,
	// ortalama, min, maks, karekök) sarar; bu yüzden onlardan SONRA kaydedilmelidir.
	vm.MotorEkle(tensor.Kaydet)
	// Parçalayıcı ve veri motorları tensör değerleri üretir; yeni adlar tanımlar.
	vm.MotorEkle(parcalayici.Kaydet)
	vm.MotorEkle(veri.Kaydet)

	vm.YeniHataAyiklayici = func(sourceFile, sourceCode string) vm.HataAyiklayici {
		return hataayiklama.NewDebugger(sourceFile, sourceCode, true)
	}
}

// Kapat motorların program boyunca açık kalmış kaynaklarını (veri kümesi okuyucuları,
// önden yükleme gorutinleri, önbellekler) kapatır. Program bitince çağrılır; takılan bir
// okumada en çok birkaç saniye bekler. Birden çok kez çağrılabilir.
func Kapat() {
	veri.TumunuKapat(3 * time.Second)
}
