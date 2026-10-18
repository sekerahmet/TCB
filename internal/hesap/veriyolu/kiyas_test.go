package veriyolu

import (
	"context"
	"testing"
)

// kiyasKumesi, 4 parça × 4M tokenlık sentetik '<u2' küme kurar.
func kiyasKumesi(b *testing.B, esleme bool) *Kume {
	b.Helper()
	d := b.TempDir()
	for i := 0; i < 4; i++ {
		u16Yaz(b, d, parcaAdi(i), sozdeU16(4<<20, uint64(i)))
	}
	k, err := KumeAc(d, KumeSecenekleri{Parca: ParcaSecenekleri{EslemeKapali: !esleme}})
	if err != nil {
		b.Fatal(err)
	}
	return k
}

func kiyasOrnekleyici(b *testing.B, esleme bool) {
	const B, T = 32, 1024
	k := kiyasKumesi(b, esleme)
	o, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: B, T: T, Tohum: 1})
	if err != nil {
		b.Fatal(err)
	}
	defer o.Kapat()
	var ti TopluIs
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := o.Sonraki(&ti); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N)*B*T/b.Elapsed().Seconds(), "token/s")
}

func BenchmarkOrnekleyiciEsleme(b *testing.B) { kiyasOrnekleyici(b, true) }
func BenchmarkOrnekleyiciReadAt(b *testing.B) { kiyasOrnekleyici(b, false) }

func BenchmarkOnYukleyici(b *testing.B) {
	const B, T = 32, 1024
	k := kiyasKumesi(b, true)
	o, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: B, T: T, Tohum: 1})
	if err != nil {
		b.Fatal(err)
	}
	defer o.Kapat()
	y := OnYukle(context.Background(), o, 4)
	defer y.Kapat()
	var ti TopluIs
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := y.Al(&ti); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N)*B*T/b.Elapsed().Seconds(), "token/s")
}
