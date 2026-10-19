// Package stdlib, çekirdek yorumlayıcının (internal/lang/interp) içe aktarmadığı parçaları ona bağlar:
// tensör ve yapay zekâ, veri ve parçalayıcı, JSON/CSV, dosya ve yol motorlarının yerleşikleri. Tam dili
// çalıştıran her program bu paketi içe aktarmalıdır:
//
//	import _ "github.com/sekerahmet/TCB/internal/stdlib"
//
// Koddaki kesme_noktası deyiminin açtığı hata ayıklayıcı araç katmanındadır; onu isteyen
// program ayrıca internal/devtools/debugger paketini içe aktarır (motor katmanı araç katmanına
// bağlı değildir; bkz. internal/layers_test.go).
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package stdlib

import (
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
	"github.com/sekerahmet/TCB/internal/stdlib/dataset"
	"github.com/sekerahmet/TCB/internal/stdlib/files"
	"github.com/sekerahmet/TCB/internal/stdlib/formats"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
	"github.com/sekerahmet/TCB/internal/stdlib/tokenizer"
)

func init() {
	interp.AddEngine(formats.Register)
	interp.AddEngine(files.Kaydet)
	// Tensör motoru (tensör, otomatik türev, katmanlar) çekirdek yerleşiklerini (toplam,
	// ortalama, min, maks, karekök) sarar; bu yüzden onlardan SONRA kaydedilmelidir.
	interp.AddEngine(ml.Kaydet)
	// Parçalayıcı ve veri motorları tensör değerleri üretir; yeni adlar tanımlar.
	interp.AddEngine(tokenizer.Register)
	interp.AddEngine(dataset.Register)
	// Süreç (ortam, komut satırı), kayıt (günlük, ilerleme) ve kontrol noktası: günlüğün
	// başlangıç kaydı ve kontrol noktası programın komut satırı ayarlarını yazar.
	interp.AddEngine(func(v *interp.VM) {
		s := process.Register(v)
		logging.Register(v, s)
		checkpoint.Kaydet(v, s)
	})
}

// Kapat motorların program boyunca açık kalmış kaynaklarını (veri kümesi okuyucuları,
// önden yükleme gorutinleri, önbellekler, satır okuyucularının ve günlüklerin dosyaları) kapatır,
// bitmemiş ilerleme çubuklarını bitirir ve geçici_klasör() ile oluşturulan klasörleri siler
// (dosyalar kapandıktan sonra). Program bitince çağrılır; takılan bir okumada en çok birkaç
// saniye bekler. Birden çok kez çağrılabilir.
func Kapat() {
	logging.CloseAll()
	dataset.CloseAll(3 * time.Second)
	files.TumunuKapat()
	ml.Kapat()
}
