// Package motor, çekirdek yorumlayıcının (pkg/vm) içe aktarmadığı parçaları ona bağlar:
// yapay zeka, web/görsel arayüz ve veritabanı motorlarının yerleşikleri ile koddaki
// kesme_noktası deyiminin açtığı hata ayıklayıcı. Tam dili çalıştıran her program bu
// paketi içe aktarmalıdır:
//
//	import _ "tr_dili/pkg/motor"
package motor

import (
	"tr_dili/pkg/hataayiklama"
	"tr_dili/pkg/motor/tensor"
	"tr_dili/pkg/motor/veritabani"
	"tr_dili/pkg/motor/web"
	"tr_dili/pkg/motor/yz"
	"tr_dili/pkg/vm"
)

func init() {
	// Sıra, yerleşiklerin VM'ye eskiden kaydedildiği sırayla aynıdır.
	vm.MotorEkle(yz.Kaydet)
	vm.MotorEkle(web.Kaydet)
	vm.MotorEkle(veritabani.Kaydet)
	// Tensör motoru eski yerleşikleri (toplam, relu, sıfırlar, tensör...) sarar; bu yüzden
	// çekirdek yerleşiklerden ve yapay zekâ motorundan SONRA kaydedilmelidir.
	vm.MotorEkle(tensor.Kaydet)

	vm.YeniHataAyiklayici = func(sourceFile, sourceCode string) vm.HataAyiklayici {
		return hataayiklama.NewDebugger(sourceFile, sourceCode, true)
	}
}
