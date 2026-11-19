package profiler

import (
	"runtime"
	"strings"
	"testing"
)

// Toplayıcının payı kullanılan işlemciye bölünür; aralıkta işlemci kullanılmadıysa 0'dır.
func TestCollectorShareOverUsedCPU(t *testing.T) {
	a := CollectorTime{GC: 1, Used: 10}
	for _, c := range []struct {
		b    CollectorTime
		want float64
	}{
		{CollectorTime{GC: 1.5, Used: 12}, 0.25},
		{CollectorTime{GC: 1, Used: 14}, 0},
		{a, 0},
	} {
		if got := CollectorShare(a, c.b); got != c.want {
			t.Errorf("CollectorShare(%v, %v) = %v, %v bekleniyordu", a, c.b, got, c.want)
		}
	}
	runtime.GC()
	if c := ReadCollectorTime(); c.GC <= 0 || c.Used < c.GC {
		t.Errorf("toplamadan sonra ölçü: %+v; toplayıcının süresi pozitif ve kullanılan işlemcinin içinde olmalı", c)
	}
}

// Kural (19.12a, "Süreç tavana yaklaştıkça çöp toplayıcı sıklaşır"): rapor, canlı tablo ve tek satırlık
// özet toplayıcının işlemci payını yazar; canlı tablo pencerenin payını da. Bellek satırı Go'nun
// öbeğini "çöp toplayıcının öbeği" diye adlandırır.
func TestReportShowsTheCollectorShare(t *testing.T) {
	g := sampleSnapshot()
	g.GC = GCInfo{Count: 423, PauseSec: 0.113, CPUShare: 0.04}
	g.Window.GCCount, g.Window.PauseSec, g.Window.GCCPUShare = 165, 0.0547, 0.05
	screen := strings.Join(ScreenLines(g, Screen{Width: 200}), "\n")
	for _, c := range []struct{ where, text, want string }{
		{"rapor", ReportText(g), "Çöp toplama: 423 kez, toplam duraklama 113 ms, işlemci payı %4,0"},
		{"rapor", ReportText(g), "Bellek: çöp toplayıcının öbeği 200,0 MB"},
		{"canlı tablo", screen, "Çöp top.   423 kez, toplam duraklama 113 ms, pay %4,0 (son 2 sn: 165 kez, 54,7 ms, pay %5,0)"},
		{"özet", SummaryLine(g), "çöp toplayıcının öbeği 200,0 MB · çöp toplama 423 (113 ms, pay %4,0)"},
	} {
		if !strings.Contains(c.text, c.want) {
			t.Errorf("%s %q içermiyor:\n%s", c.where, c.want, c.text)
		}
	}
}
