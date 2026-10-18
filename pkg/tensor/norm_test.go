package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// Vektör çekirdekleri (GOAMD64=v3'te AVX2, aksi halde Go eşleri) taşınabilir Go
// eşleriyle bit düzeyinde aynı sonucu vermeli: v1 ve v3 derlemeleri aynı
// sonucu üretir. Kuyruklu (D mod 8 ≠ 0) ve seçeneksiz (γ/β/gradyan yok) yollar
// dahil.

func esitBit32(t *testing.T, ad string, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: uzunluk %d, beklenen %d", ad, len(got), len(want))
	}
	for i := range got {
		if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
			t.Fatalf("%s [%d]: %v (%#x), beklenen %v (%#x)", ad, i, got[i], math.Float32bits(got[i]), want[i], math.Float32bits(want[i]))
		}
	}
}

func esitBit64(t *testing.T, ad string, got, want []float64) {
	t.Helper()
	for i := range got {
		if math.Float64bits(got[i]) != math.Float64bits(want[i]) {
			t.Fatalf("%s [%d]: %v, beklenen %v", ad, i, got[i], want[i])
		}
	}
}

func TestNormVektorGoEsi(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	dilim := func(n int, kayma float64) []float32 {
		d := make([]float32, n)
		for i := range d {
			d[i] = float32(rng.NormFloat64()*1.5 + kayma)
		}
		return d
	}
	t.Logf("çekirdek: %s", mikroCekirdekAdi)
	for _, D := range []int{1, 3, 7, 8, 9, 15, 16, 17, 31, 64, 67, 131, 512} {
		for _, R := range []int{1, 2, 5} {
			for secim := 0; secim < 8; secim++ {
				rms := secim&4 != 0
				x := dilim(R*D, 0.7)
				var g, b []float32
				if secim&1 != 0 {
					g = dilim(D, 1)
				}
				if secim&2 != 0 && !rms {
					b = dilim(D, 0)
				}
				y1, y2 := make([]float32, R*D), make([]float32, R*D)
				rs1, rs2 := make([]float32, R), make([]float32, R)
				var m1, m2 []float32
				if !rms {
					m1, m2 = make([]float32, R), make([]float32, R)
				}
				lnIleriBlok(y1, x, g, b, m1, rs1, D, 1e-5)
				lnIleriBlokGo(y2, x, g, b, m2, rs2, D, 1e-5)
				esitBit32(t, "ileri y", y1, y2)
				esitBit32(t, "ileri rstd", rs1, rs2)
				esitBit32(t, "ileri mean", m1, m2)

				gr := dilim(R*D, 0)
				for xgVar := 0; xgVar < 2; xgVar++ {
					var xg1, xg2 []float32
					if xgVar == 1 {
						xg1, xg2 = dilim(R*D, 0), make([]float32, R*D)
						copy(xg2, xg1)
					}
					pg1, pg2 := make([]float64, D), make([]float64, D)
					pb1, pb2 := make([]float64, D), make([]float64, D)
					for i := range pg1 {
						pg1[i] = rng.Float64()
						pg2[i], pb1[i], pb2[i] = pg1[i], -pg1[i], -pg1[i]
					}
					if secim&2 != 0 {
						pb1, pb2 = nil, nil
					}
					lnGeriBlok(xg1, pg1, pb1, x, gr, g, m1, rs1, D)
					lnGeriBlokGo(xg2, pg2, pb2, x, gr, g, m2, rs2, D)
					esitBit32(t, "geri xg", xg1, xg2)
					esitBit64(t, "geri pg", pg1, pg2)
					esitBit64(t, "geri pb", pb1, pb2)
				}
			}
		}
	}
}

// normOzeti sabit girdilerle katman/RMS normu ileri ve geri sonuçlarının bit özeti.
func normOzeti() uint64 {
	rng := rand.New(rand.NewSource(5))
	h := uint64(14695981039346656037)
	ekle := func(b uint64) { h = (h ^ b) * 1099511628211 }
	for _, D := range []int{5, 64, 67} {
		R := 9
		x, gr, g, b := make([]float32, R*D), make([]float32, R*D), make([]float32, D), make([]float32, D)
		for _, s := range [][]float32{x, gr, g, b} {
			for i := range s {
				s[i] = float32(rng.NormFloat64())
			}
		}
		for _, rms := range []bool{false, true} {
			y, rs, xg := make([]float32, R*D), make([]float32, R), make([]float32, R*D)
			var mean []float32
			if !rms {
				mean = make([]float32, R)
			}
			pg, pb := make([]float64, D), make([]float64, D)
			lnIleriBlok(y, x, g, b, mean, rs, D, 1e-5)
			lnGeriBlok(xg, pg, pb, x, gr, g, mean, rs, D)
			for _, s := range [][]float32{y, rs, mean, xg} {
				for _, v := range s {
					ekle(uint64(math.Float32bits(v)))
				}
			}
			for _, s := range [][]float64{pg, pb} {
				for _, v := range s {
					ekle(math.Float64bits(v))
				}
			}
		}
	}
	return h
}

// GOAMD64=v1 (Go eşleri) ve v3 (AVX2) derlemeleri aynı bitleri üretmeli; özet iki
// derlemede de ölçülüp sabitlenmiştir.
func TestNormDerlemelerArasiAyni(t *testing.T) {
	const beklenen = 0xfe0296651a382150
	if h := normOzeti(); h != beklenen {
		t.Fatalf("norm çekirdeklerinin bit özeti %#x, beklenen %#x (%s)", h, uint64(beklenen), mikroCekirdekAdi)
	}
}

// Vektör çekirdeği satır dışına (komşu bellek) yazmamalı: kuyruk maskeli saklanır.
func TestNormVektorTasmaYok(t *testing.T) {
	for _, D := range []int{1, 5, 13, 64, 67} {
		R := 3
		tampon := make([]float32, R*D+16)
		for i := range tampon {
			tampon[i] = 42
		}
		x := make([]float32, R*D)
		for i := range x {
			x[i] = float32(i%7) - 3
		}
		y := tampon[:R*D]
		lnIleriBlok(y, x, nil, nil, make([]float32, R), make([]float32, R), D, 1e-5)
		for i := R * D; i < len(tampon); i++ {
			if tampon[i] != 42 {
				t.Fatalf("D=%d: ileri yön satır dışına yazdı ([%d] = %v)", D, i, tampon[i])
			}
		}
		pg := make([]float64, D+8)
		for i := range pg {
			pg[i] = 7
		}
		xg := make([]float32, R*D+16)
		lnGeriBlok(xg[:R*D], pg[:D], nil, x, x, nil, make([]float32, R), make([]float32, R), D)
		for i := D; i < len(pg); i++ {
			if pg[i] != 7 {
				t.Fatalf("D=%d: geri yön ölçek gradyanını taşırdı ([%d] = %v)", D, i, pg[i])
			}
		}
		for i := R * D; i < len(xg); i++ {
			if xg[i] != 0 {
				t.Fatalf("D=%d: geri yön girdi gradyanını taşırdı ([%d] = %v)", D, i, xg[i])
			}
		}
	}
}
