package main

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/profiler"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// samplesOf saniyede bir örneklerdir: toplayıcı her saniye kullanılan işlemcinin share kadarını alır,
// Go'nun belleği memory[i], Go'nun sınırı 1000.
func samplesOf(share float64, memory ...uint64) []shareSample {
	var out []shareSample
	for i, m := range memory {
		out = append(out, shareSample{cpu: profiler.CollectorTime{GC: share * float64(i), Used: float64(i)},
			goMemory: m, goLimit: 1000, outside: 300})
	}
	return out
}

// repeat n tane m'dir.
func repeat(m uint64, n int) []uint64 {
	out := make([]uint64, n)
	for i := range out {
		out[i] = m
	}
	return out
}

// lateShare ilk örnekte belleği sınıra yakın, sonra uzak 20 örnektir; toplayıcı 10. saniyeye kadar
// çalışmaz, sonra her saniye yarım işlemci-saniye alır: pay %15'i 14. örnekte geçer, o zaman ilk
// örnek pencerenin dışındadır.
func lateShare(near, far uint64) []shareSample {
	var out []shareSample
	for i := range 20 {
		s := shareSample{cpu: profiler.CollectorTime{Used: float64(i)}, goMemory: far, goLimit: 1000}
		if i == 0 {
			s.goMemory = near
		}
		if i > 10 {
			s.cpu.GC = 0.5 * float64(i-10)
		}
		out = append(out, s)
	}
	return out
}

// Kural (19.12a, tasarımın C "Uyarı"sı): uyarı, son 10 saniyenin payı %15'i geçtiğinde ve o pencerede Go'nun
// belleği en az bir örnekte Go'nun sınırının 0,95'ine vardığında bir kez çıkar; yalnız pay ya da yalnız
// bellek yetmez, pencere dolmadan uyarı yoktur.
func TestCollectorShareWarnsOnce(t *testing.T) {
	near, far := uint64(960), uint64(500)
	for _, c := range []struct {
		name    string
		samples []shareSample
		warnAt  int // uyarının çıktığı örnek (0'dan), -1: çıkmaz
	}{
		{"ikisi birlikte", samplesOf(0.30, repeat(near, 15)...), 10},
		{"yalnız pay", samplesOf(0.30, repeat(far, 15)...), -1},
		{"yalnız bellek", samplesOf(0.05, repeat(near, 15)...), -1},
		{"pay tam eşikte", samplesOf(0.15, repeat(near, 15)...), -1},
		{"bellek pencerenin bir örneğinde", samplesOf(0.30, append(append(repeat(far, 5), near), repeat(far, 9)...)...), 10},
		{"bellek pencereden çıkmış", lateShare(near, far), -1},
		{"bellek sonradan yaklaşıyor", samplesOf(0.30, append(repeat(far, 12), near, near)...), 12},
	} {
		var w collectorWatch
		got := -1
		for i, s := range c.samples {
			share, ok := w.add(s)
			if !ok {
				continue
			}
			if got >= 0 {
				t.Errorf("%s: uyarı ikinci kez çıktı (örnek %d)", c.name, i)
			}
			got = i
			if share < 0.29 || share > 0.31 {
				t.Errorf("%s: pay %v, 0,30 bekleniyordu", c.name, share)
			}
		}
		if got != c.warnAt {
			t.Errorf("%s: uyarı örnek %d'de, %d bekleniyordu", c.name, got, c.warnAt)
		}
	}
}

// Uyarının metni tavanı, Go'nun belleğini ve öbek dışını bayt olarak, tavanı kopyalanınca çalışan
// biçimde yazar.
func TestCollectorShareWarningText(t *testing.T) {
	got := collectorShareWarning(0.234, 6442450944, shareSample{goMemory: 2900000000, outside: 3500000000})
	want := messages.Text(messages.H8894, "%23", 10, "TR_MEMORY_LIMIT", int64(6442450944), uint64(2900000000), int64(3500000000))
	if got != want {
		t.Errorf("uyarı:\n%s\nbeklenen:\n%s", got, want)
	}
	for _, s := range []string{"TR_MEMORY_LIMIT=6442450944", "2900000000", "3500000000", "%23"} {
		if !strings.Contains(got, s) {
			t.Errorf("uyarıda %q yok: %s", s, got)
		}
	}
}
