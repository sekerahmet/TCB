package profiler

import "runtime/metrics"

// Çöp toplayıcının işlemci payı (dil tanımı 19.12a): toplayıcının süresi, işlemcinin boşta olmayan
// süresine bölünür. Toplam işlemciye (GOMAXPROCS × süre) bölmek çok işlemcili makinede payı seyreltirdi:
// tek iş parçacığıyla çalışan programda boştaki işlemciler paydaya girerdi. Go bu ölçüleri her toplamanın
// sonunda günceller (go1.27.4 src/runtime/metrics.go): toplama olmayan aralıkta pay 0'dır. Profil ve
// trlang'ın tavan uyarısı (cmd/trlang/collector_share.go) aynı ölçüyü kullanır.

// CollectorTime süreç başından beri toplayıcının ve işlemcinin kullanılan süresidir (işlemci-saniye).
type CollectorTime struct {
	GC   float64 // /cpu/classes/gc/total
	Used float64 // /cpu/classes/total − /cpu/classes/idle
}

// collectorMetrics ReadCollectorTime'ın okuduğu ölçülerdir, bu sırayla.
var collectorMetrics = [...]string{
	"/cpu/classes/gc/total:cpu-seconds",
	"/cpu/classes/total:cpu-seconds",
	"/cpu/classes/idle:cpu-seconds",
}

// ReadCollectorTime çalışma zamanının işlemci ölçülerini okur.
func ReadCollectorTime() CollectorTime {
	var s [len(collectorMetrics)]metrics.Sample
	for i, name := range collectorMetrics {
		s[i].Name = name
	}
	metrics.Read(s[:])
	v := func(i int) float64 {
		if s[i].Value.Kind() == metrics.KindFloat64 {
			return s[i].Value.Float64()
		}
		return 0
	}
	return CollectorTime{GC: v(0), Used: v(1) - v(2)}
}

// CollectorShare a'dan b'ye toplayıcının işlemci payıdır; aralıkta işlemci kullanılmadıysa 0.
func CollectorShare(a, b CollectorTime) float64 { return ratio(b.GC-a.GC, b.Used-a.Used) }

// Percent payı profilin yüzdesiyle yazar (H8293): trlang'ın toplayıcı uyarısı da aynı biçimi kullanır.
func Percent(f float64) string { return percent(f) }
