// Package motor, çekirdek yorumlayıcının (internal/dil/yurut) içe aktarmadığı parçaları ona bağlar:
// tensör ve yapay zekâ, veri ve parçalayıcı, JSON/CSV, dosya ve yol motorlarının yerleşikleri. Tam dili
// çalıştıran her program bu paketi içe aktarmalıdır:
//
//	import _ "tr_dili/internal/motor"
//
// Koddaki kesme_noktası deyiminin açtığı hata ayıklayıcı araç katmanındadır; onu isteyen
// program ayrıca internal/arac/ayikla paketini içe aktarır (motor katmanı araç katmanına
// bağlı değildir; bkz. internal/katman_test.go).
//
// Katman: motor (internal/motor); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package motor

import (
	"time"

	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/dosya"
	"tr_dili/internal/motor/gunluk"
	"tr_dili/internal/motor/ogrenme"
	"tr_dili/internal/motor/parcalayici"
	"tr_dili/internal/motor/surec"
	"tr_dili/internal/motor/veri"
	"tr_dili/internal/motor/veribicim"
)

func init() {
	yurut.MotorEkle(veribicim.Kaydet)
	yurut.MotorEkle(dosya.Kaydet)
	// Tensör motoru (tensör, otomatik türev, katmanlar) çekirdek yerleşiklerini (toplam,
	// ortalama, min, maks, karekök) sarar; bu yüzden onlardan SONRA kaydedilmelidir.
	yurut.MotorEkle(ogrenme.Kaydet)
	// Parçalayıcı ve veri motorları tensör değerleri üretir; yeni adlar tanımlar.
	yurut.MotorEkle(parcalayici.Kaydet)
	yurut.MotorEkle(veri.Kaydet)
	// Süreç (ortam, komut satırı) ve kayıt (günlük, ilerleme): günlüğün başlangıç kaydı
	// programın komut satırı ayarlarını yazar.
	yurut.MotorEkle(func(v *yurut.VM) { gunluk.Kaydet(v, surec.Kaydet(v)) })
}

// Kapat motorların program boyunca açık kalmış kaynaklarını (veri kümesi okuyucuları,
// önden yükleme gorutinleri, önbellekler, satır okuyucularının ve günlüklerin dosyaları) kapatır,
// bitmemiş ilerleme çubuklarını bitirir ve geçici_klasör() ile oluşturulan klasörleri siler
// (dosyalar kapandıktan sonra). Program bitince çağrılır; takılan bir okumada en çok birkaç
// saniye bekler. Birden çok kez çağrılabilir.
func Kapat() {
	gunluk.TumunuKapat()
	veri.TumunuKapat(3 * time.Second)
	dosya.TumunuKapat()
	ogrenme.Kapat()
}
