package tensor

import "time"

// Profil sayaçlarının saati: saat() sayaçların tık birimindeki şimdiki zamandır. amd64'te
// işlemci değişmez zaman damgası sayacını bildiriyorsa o (sayac_saat_amd64.go), değilse
// Go'nun monoton saati (nanosaniye) kullanılır. Tıklar görüntü alınırken süreç başından
// beri geçen duvar saati süresiyle ölçeklenir: oran = geçen süre / geçen tık. Süreç uzadıkça
// oran daha kesinleşir (Windows'ta monoton saatin çözünürlüğü ~0,5 ms'dir; 1 sn sonra
// hata binde birin altındadır).

var (
	saatBas    = time.Now()
	saatBasTik = saat()
)

// monotonNs süreç başından beri geçen nanosaniyedir (+1: 0 "ölçülmüyor" demektir).
func monotonNs() int64 { return int64(time.Since(saatBas)) + 1 }

// tikSureOrani bir tıkın nanosaniye karşılığıdır.
func tikSureOrani() float64 {
	if !tscVar {
		return 1
	}
	gecen, tik := time.Since(saatBas), saat()-saatBasTik
	if tik <= 0 || gecen <= 0 {
		return 1
	}
	return float64(gecen) / float64(tik)
}
