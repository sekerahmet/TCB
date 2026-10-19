package tensor

import "time"

// Profil sayaçlarının saati: saat() sayaçların tık birimindeki şimdiki zamandır. amd64'te
// işlemci değişmez zaman damgası sayacını bildiriyorsa o (sayac_saat_amd64.go), değilse
// Go'nun monoton saati (nanosaniye) kullanılır. Tıklar görüntü alınırken süreç başından
// beri geçen duvar saati süresiyle ölçeklenir: oran = geçen süre / geçen tık. Süreç uzadıkça
// oran daha kesinleşir (Windows'ta monoton saatin çözünürlüğü ~0,5 ms'dir; 1 sn sonra
// hata binde birin altındadır).

var (
	clockStart     = time.Now()
	clockStartTick = clock()
)

// monotonicNs süreç başından beri geçen nanosaniyedir (+1: 0 "ölçülmüyor" demektir).
func monotonicNs() int64 { return int64(time.Since(clockStart)) + 1 }

// tickDurationRatio bir tıkın nanosaniye karşılığıdır.
func tickDurationRatio() float64 {
	if !hasTSC {
		return 1
	}
	elapsed, tick := time.Since(clockStart), clock()-clockStartTick
	if tick <= 0 || elapsed <= 0 {
		return 1
	}
	return float64(elapsed) / float64(tick)
}
