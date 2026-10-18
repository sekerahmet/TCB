// Package motor, çekirdek yorumlayıcının (pkg/vm) içe aktarmadığı parçaları ona bağlar:
// yapay zeka, JSON/CSV, web/görsel arayüz ve veritabanı motorlarının yerleşikleri ile koddaki
// kesme_noktası deyiminin açtığı hata ayıklayıcı. Tam dili çalıştıran her program bu
// paketi içe aktarmalıdır:
//
//	import _ "tr_dili/pkg/motor"
package motor

import (
	"time"

	"tr_dili/pkg/hataayiklama"
	"tr_dili/pkg/motor/parcalayici"
	"tr_dili/pkg/motor/tensor"
	"tr_dili/pkg/motor/veri"
	"tr_dili/pkg/motor/veribicim"
	"tr_dili/pkg/motor/veritabani"
	"tr_dili/pkg/motor/web"
	"tr_dili/pkg/motor/yz"
	"tr_dili/pkg/vm"
)

func init() {
	// Sıra, yerleşiklerin VM'ye eskiden kaydedildiği sırayla aynıdır.
	// JSON/CSV (veribicim) ve web_al/web_gönder (web) eskiden yz.Kaydet içindeydi; yz'nin
	// hemen ardından kaydedilerek eski sıra korunur.
	vm.MotorEkle(yz.Kaydet)
	vm.MotorEkle(veribicim.Kaydet)
	vm.MotorEkle(web.Kaydet)
	vm.MotorEkle(veritabani.Kaydet)
	// Tensör motoru eski yerleşikleri (toplam, relu, sıfırlar, tensör...) sarar; bu yüzden
	// çekirdek yerleşiklerden ve yapay zekâ motorundan SONRA kaydedilmelidir.
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
