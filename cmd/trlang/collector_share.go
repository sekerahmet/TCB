package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"runtime/metrics"
	"time"

	"github.com/sekerahmet/TCB/internal/devtools/profiler"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/sysmem"
)

// Toplayıcının payı uyarısı (dil tanımı 19.12a: "Süreç tavana yaklaştıkça çöp toplayıcı sıklaşır"):
// tavan verilince bir izleyici saniyede bir örnekler ve H8894'ü bir kez, iki koşul birlikte tutunca
// yazar: son 10 saniyede toplayıcının kullanılan işlemcideki payı %15'i geçer ve o pencerede Go'nun
// belleği en az bir örnekte Go'nun sınırının (tavan − öbek dışı) 0,95'ine varır. İkinci koşul bekleyen
// süreci ayırır: Go en geç iki dakikada bir toplama zorlar, işlemcinin az kullanıldığı pencerede bu
// toplama payı büyütebilir ama bellek sınırdan uzaktır. Eşikler bir denemenin 10 saniyelik
// pencerelerinden gelir (bellek tavanı tasarımı, D2): gevşek tavanlarda pay en çok %10, programı
// yavaşlatan tavanda %18–20; payın %15'i geçtiği pencerelerde bellek sınırın 0,99'undaydı.
const (
	shareWindow   = 10   // saniye: payın penceresi (örnek sayısı shareWindow+1)
	shareLimit    = 0.15 // pay bunu geçerse
	memoryNearCap = 0.95 // Go'nun belleği Go'nun sınırının bu katına varırsa
)

// shareSample izleyicinin bir örneğidir.
type shareSample struct {
	cpu      profiler.CollectorTime
	goMemory uint64 // Go'nun belleği: /memory/classes/total − /memory/classes/heap/released (Go'nun sınırının saydığı)
	goLimit  int64  // Go'nun sınırı
	outside  int64  // öbek dışı bloklar (sysmem)
}

// collectorWatch son pencerenin örneklerini tutar.
type collectorWatch struct {
	samples []shareSample
	warned  bool
}

// add örneği pencereye ekler; uyarı yazılacaksa payı ve true döner. Uyarı bir kezdir.
func (w *collectorWatch) add(s shareSample) (float64, bool) {
	if w.warned {
		return 0, false
	}
	w.samples = append(w.samples, s)
	if len(w.samples) > shareWindow+1 {
		w.samples = w.samples[1:]
	}
	if len(w.samples) < shareWindow+1 {
		return 0, false
	}
	share := profiler.CollectorShare(w.samples[0].cpu, s.cpu)
	if share <= shareLimit {
		return share, false
	}
	for _, x := range w.samples {
		if float64(x.goMemory) >= memoryNearCap*float64(x.goLimit) {
			w.warned = true
			return share, true
		}
	}
	return share, false
}

// memoryMetrics Go'nun belleğinin ölçüleridir, bu sırayla (go1.27.4 runtime/debug.SetMemoryLimit).
var memoryMetrics = [...]string{"/memory/classes/total:bytes", "/memory/classes/heap/released:bytes"}

// takeShareSample o anın örneğidir.
func takeShareSample() shareSample {
	var m [len(memoryMetrics)]metrics.Sample
	for i, name := range memoryMetrics {
		m[i].Name = name
	}
	metrics.Read(m[:])
	var goMemory uint64
	if m[0].Value.Kind() == metrics.KindUint64 && m[1].Value.Kind() == metrics.KindUint64 {
		goMemory = m[0].Value.Uint64() - m[1].Value.Uint64()
	}
	return shareSample{cpu: profiler.ReadCollectorTime(), goMemory: goMemory,
		goLimit: debug.SetMemoryLimit(-1), outside: sysmem.Outside()}
}

// watchCollectorShare izleyiciyi başlatır: memCap tavandır. Uyarı stderr'e tek satırdır; ondan sonra
// izleyici durur. Program bitince izleyici de biter.
func watchCollectorShare(stderr io.Writer, memCap int64) {
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		var w collectorWatch
		for range tick.C {
			s := takeShareSample()
			if share, ok := w.add(s); ok {
				fmt.Fprintln(stderr, "⚠️ "+collectorShareWarning(share, memCap, s))
				return
			}
		}
	}()
}

// collectorShareWarning H8894'ün metnidir: bayt sayıları düz yazılır, tavan kopyalanınca çalışır.
func collectorShareWarning(share float64, memCap int64, s shareSample) string {
	return messages.Text(messages.H8894, profiler.Percent(share), shareWindow,
		codes.NameRef(codes.EnvMemoryLimit), memCap, s.goMemory, s.outside)
}
