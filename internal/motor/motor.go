// Package motor, çekirdek yorumlayıcının (internal/dil/yurut) içe aktarmadığı parçaları ona bağlar:
// tensör ve yapay zekâ, veri ve parçalayıcı, JSON/CSV motorlarının yerleşikleri ile koddaki
// kesme_noktası deyiminin açtığı hata ayıklayıcı. Tam dili çalıştıran her program bu
// paketi içe aktarmalıdır:
//
//	import _ "tr_dili/internal/motor"
package motor

import (
	"time"

	"tr_dili/internal/arac/ayikla"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/ogrenme"
	"tr_dili/internal/motor/parcalayici"
	"tr_dili/internal/motor/veri"
	"tr_dili/internal/motor/veribicim"
)

func init() {
	yurut.MotorEkle(veribicim.Kaydet)
	// Tensör motoru (tensör, otomatik türev, katmanlar) çekirdek yerleşiklerini (toplam,
	// ortalama, min, maks, karekök) sarar; bu yüzden onlardan SONRA kaydedilmelidir.
	yurut.MotorEkle(ogrenme.Kaydet)
	// Parçalayıcı ve veri motorları tensör değerleri üretir; yeni adlar tanımlar.
	yurut.MotorEkle(parcalayici.Kaydet)
	yurut.MotorEkle(veri.Kaydet)

	yurut.YeniHataAyiklayici = func(sourceFile, sourceCode string) yurut.HataAyiklayici {
		return ayikla.NewDebugger(sourceFile, sourceCode, true)
	}
}

// Kapat motorların program boyunca açık kalmış kaynaklarını (veri kümesi okuyucuları,
// önden yükleme gorutinleri, önbellekler) kapatır. Program bitince çağrılır; takılan bir
// okumada en çok birkaç saniye bekler. Birden çok kez çağrılabilir.
func Kapat() {
	veri.TumunuKapat(3 * time.Second)
}
