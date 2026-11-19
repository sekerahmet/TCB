package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// Tam GELU çekirdeklerinin ölçütü (dil tanımı 15.17, "Hesap yolu"): başvuru bugüne kadarki float64
// yoldur, float32'ye bir kez yuvarlanmış; ileri her sonlu float32'de en çok 1 ulp, türev en çok
// 1 ulp, x ∈ [−1, −0,5]'te (türevin sıfırı x ≈ −0,7518 çevresinde, d iki terimin farkıdır) 1 ulp
// ya da 2^-28 mutlak. Uçlar bit bit; ileri yönde x < −1'de sıfıra inen sonuç +0'dır, öbür sıfırlar
// x·Φ'nin işaretini taşır (GPU ile aynı, 15.50); türev çarpanında negatif sonlu x'te sıfır −0'dır. Bütün
// float32'lerin taraması testte değil, Colab'da bir kez koşar; burada seyrek bir örneklem ve
// zor bölgeler sınanır.

// geluExactValue başvuru: x·Φ(x), Φ(x) = ½·erfc(−x/√2). erfc kullanılır, 1+erf değil: x çok
// negatifken Φ(x) kuyruk olarak göreli doğrulukla bulunur. Φ(x) = 0 olan uçta sonuç 0'dır
// (−∞·0 NaN üretirdi).
func geluExactValue(x float64) float64 {
	phi := 0.5 * math.Erfc(-x*invSqrt2)
	if phi == 0 {
		return 0
	}
	return x * phi
}

// geluExactSlope başvuru: x·Φ(x)'in türevi Φ(x) + x·φ(x), φ standart normal yoğunluğu. φ(x) = 0
// iken (|x| büyük ya da sonsuz) türev Φ(x)'tir: x·0 sonsuzda NaN üretirdi.
func geluExactSlope(x float64) float64 {
	phi := 0.5 * math.Erfc(-x*invSqrt2)
	density := invSqrt2Pi * math.Exp(-0.5*x*x)
	if density == 0 {
		return phi
	}
	return phi + x*density
}

// geluOrd float32'yi sıralı tamsayıya çevirir; +0 ile −0 aynı yere düşer.
func geluOrd(f float32) int64 {
	b := int64(math.Float32bits(f))
	if b&(1<<31) != 0 {
		return -(b &^ (1 << 31))
	}
	return b
}

// geluUlp iki sonlu ya da sonsuz float32 arasındaki ulp sayısı.
func geluUlp(a, b float32) int64 {
	d := geluOrd(a) - geluOrd(b)
	if d < 0 {
		return -d
	}
	return d
}

// geluZeroSign sıfıra inen ileri sonucun beklenen işareti: −0 girdide ve −1 ≤ x < 0'da (çok küçük
// |x|, x·Φ'nin işareti) −0; x < −1'de ve öbürlerinde +0.
func geluZeroSign(x float32) bool { return math.Signbit(float64(x)) && x >= -1 }

// geluSlopeZeroSign sıfıra inen türev çarpanının beklenen işareti: negatif sonlu x'te −0, öbürlerinde
// (−∞ dahil) +0.
func geluSlopeZeroSign(x float32) bool { return x < 0 && !math.IsInf(float64(x), -1) }

// geluRun ileri değeri ve türev çarpanını verir. Türev geri yönden okunur: gx −0'dan başlar,
// −0 + d = d olduğu için sıfırın işareti de korunur.
func geluRun(xs []float32) (y, d []float32) {
	y, d = make([]float32, len(xs)), make([]float32, len(xs))
	one := make([]float32, len(xs))
	negZero := float32(math.Copysign(0, -1))
	for i := range one {
		one[i], d[i] = 1, negZero
	}
	geluExactForwardVec(y, xs)
	geluExactBackwardVec(d, one, xs)
	return y, d
}

// geluCheck x'ler için ileri ve türevi başvuruyla ölçer; türevin mutlak tabanını kullananları sayar.
func geluCheck(t *testing.T, xs []float32) (tabanli int) {
	t.Helper()
	y, d := geluRun(xs)
	for i, x := range xs {
		if x != x {
			// İleri yön girdinin bitlerini verir; türev NaN'dır (çarpımda sessizleşebilir).
			if math.Float32bits(y[i]) != math.Float32bits(x) || d[i] == d[i] {
				t.Fatalf("NaN girdi (%08x) olduğu gibi çıkmadı: ileri %08x, türev %08x", math.Float32bits(x), math.Float32bits(y[i]), math.Float32bits(d[i]))
			}
			continue
		}
		ry := float32(geluExactValue(float64(x)))
		if u := geluUlp(y[i], ry); u > 1 {
			t.Fatalf("gelu_exact(%g) = %g, başvuru %g (%d ulp)", x, y[i], ry, u)
		}
		rd := float32(geluExactSlope(float64(x)))
		if u := geluUlp(d[i], rd); u > 1 {
			m := math.Abs(float64(d[i]) - float64(rd))
			if x < -1 || x > -0.5 || m > 0x1p-28 {
				t.Fatalf("gelu_exact'(%g) = %g, başvuru %g (%d ulp, mutlak %g)", x, d[i], rd, u, m)
			}
			tabanli++
		}
		for _, c := range []struct {
			ad  string
			v   float32
			neg bool
		}{{"ileri", y[i], geluZeroSign(x)}, {"türev", d[i], geluSlopeZeroSign(x)}} {
			if c.v == 0 && math.Signbit(float64(c.v)) != c.neg {
				t.Fatalf("gelu_exact %s(%g): sıfırın işareti yanlış (%08x)", c.ad, x, math.Float32bits(c.v))
			}
		}
	}
	return tabanli
}

// TestGeluExactKernelMatchesReference ölçütü uçlarda, zor bölgelerde ve bütün float32'lerin seyrek
// bir örnekleminde sınar (AVX2'li işlemcide çekirdek, değilse Go eşi).
func TestGeluExactKernelMatchesReference(t *testing.T) {
	nan := float32(math.NaN())
	edges := []float32{0, float32(math.Copysign(0, -1)), posInf32, negInf32, nan, -nan,
		math.Float32frombits(0x7fc00001), math.Float32frombits(0xffa00000),
		1e-45, -1e-45, 1e-38, -1e-38, 0.5, -0.5, 0.7071067, -0.7071068, -0.7518, 1, -1,
		12, -12, 13, -13, -15, -1e-45, 19.999998, -19.999998, 20, -20, 20.000002, -20.000002, -38, -40, 40,
		-1e30, 1e30, math.MaxFloat32, -math.MaxFloat32}
	geluCheck(t, edges)
	y, d := geluRun(edges)
	for _, c := range []struct {
		x, y, d float32
	}{{posInf32, posInf32, 1}, {negInf32, 0, 0}, {0, 0, 0.5}} {
		for i, x := range edges {
			if math.Float32bits(x) == math.Float32bits(c.x) && (math.Float32bits(y[i]) != math.Float32bits(c.y) || d[i] != c.d) {
				t.Errorf("gelu_exact(%g) = %g, türev %g; beklenen %g ve %g", x, y[i], d[i], c.y, c.d)
			}
		}
	}
	// −0 korunur; x < −1'de sıfıra inen ileri sonuç +0, türev çarpanı negatif sonlu x'te −0.
	for i, x := range edges {
		if math.Float32bits(x) == 0x80000000 && math.Float32bits(y[i]) != 0x80000000 {
			t.Errorf("gelu_exact(−0) = %08x, beklenen −0", math.Float32bits(y[i]))
		}
		if (x == -20.000002 || x == -38 || x == -40) && (math.Float32bits(y[i]) != 0 || math.Float32bits(d[i]) != 0x80000000) {
			t.Errorf("gelu_exact(%g) = %08x, türev %08x; beklenen +0 ve −0", x, math.Float32bits(y[i]), math.Float32bits(d[i]))
		}
	}

	// Türevin sıfırının çevresi yoğun: tabanı kullananlar yalnız burada.
	var dense []float32
	for v := float32(-1); v <= -0.5; v = math.Nextafter32(v, 1) {
		if math.Float32bits(v)%61 == 0 {
			dense = append(dense, v)
		}
	}
	if n := geluCheck(t, dense); n == 0 {
		t.Log("seyrek örneklemde türevin tabanını kullanan x yok")
	}
	// İki bölgenin eşiği (t = ½, |x| ≈ 0,7071) ve |x| = 20 çevresi her bit.
	var near []float32
	for _, c := range []float32{0.70710677, -0.70710677, 20, -20} {
		v := c
		for k := 0; k < 4096; k++ {
			v = math.Nextafter32(v, float32(math.Inf(-1)))
		}
		for k := 0; k < 8192; k++ {
			near = append(near, v)
			v = math.Nextafter32(v, float32(math.Inf(1)))
		}
	}
	geluCheck(t, near)
	// Bütün bit örüntülerinin seyrek örneklemi (NaN'lar dahil).
	sample := make([]float32, 0, 1<<20)
	for b := uint64(0); b < 1<<32; b += 4093 {
		sample = append(sample, math.Float32frombits(uint32(b)))
	}
	geluCheck(t, sample)
}

// TestGeluExactKernelMatchesGo AVX2 çekirdeği ile taşınabilir Go eşi her girdide, her kuyruk
// uzunluğunda bit bit aynıdır; çekirdek dilimin dışına yazmaz. AVX2'siz işlemcide iki yol da Go eşidir.
func TestGeluExactKernelMatchesGo(t *testing.T) {
	if !hasAVX2 {
		t.Log("AVX2 yok (ya da TR_CPU_AVX2=0): iki yol da Go eşi")
	}
	rng := rand.New(rand.NewSource(17))
	for _, n := range []int{0, 1, 3, 7, 8, 9, 15, 16, 17, 31, 64, 1000} {
		x := make([]float32, n)
		for i := range x {
			switch i % 4 {
			case 0:
				x[i] = float32(rng.NormFloat64() * 3)
			case 1:
				x[i] = float32(rng.NormFloat64() * 15)
			case 2:
				x[i] = math.Float32frombits(rng.Uint32())
			default:
				x[i] = float32(rng.Float64()*0.5 - 1)
			}
		}
		if n >= 9 {
			x[0], x[1], x[2], x[3] = float32(math.NaN()), posInf32, negInf32, float32(math.Copysign(0, -1))
			x[4], x[5], x[6], x[7] = -20.000002, 20.000002, -40, 1e-45
		}
		y1, y2 := make([]float32, n+8), make([]float32, n)
		for i := range y1 {
			y1[i] = 42
		}
		geluExactForwardVec(y1[:n], x)
		geluExactForwardGo(y2, x)
		g := make([]float32, n)
		for i := range g {
			g[i] = float32(rng.NormFloat64())
		}
		gx1, gx2 := make([]float32, n+8), make([]float32, n)
		for i := range gx1 {
			gx1[i] = 42
		}
		for i := range gx2 {
			gx1[i], gx2[i] = 0.25, 0.25
		}
		geluExactBackwardVec(gx1[:n], g, x)
		geluExactBackwardGo(gx2, g, x)
		for i := n; i < n+8; i++ {
			if y1[i] != 42 || gx1[i] != 42 {
				t.Fatalf("n=%d: çekirdek dilimin dışına yazdı ([%d])", n, i)
			}
		}
		// Türev çarpanı ayrıca: birikimde (0,25 + g·d) sıfırın işareti görünmez.
		_, d1 := geluRun(x)
		for i, v := range x {
			d2 := geluExactSlopeGo(v)
			if math.Float32bits(d1[i]) != math.Float32bits(d2) && !(d1[i] != d1[i] && d2 != d2) {
				t.Fatalf("n=%d x=%g (%08x): türev %08x/%08x", n, v, math.Float32bits(v), math.Float32bits(d1[i]), math.Float32bits(d2))
			}
		}
		for i := range x {
			if math.Float32bits(y1[i]) != math.Float32bits(y2[i]) || math.Float32bits(gx1[i]) != math.Float32bits(gx2[i]) {
				t.Fatalf("n=%d x=%g (%08x): ileri %08x/%08x, geri %08x/%08x", n, x[i], math.Float32bits(x[i]),
					math.Float32bits(y1[i]), math.Float32bits(y2[i]), math.Float32bits(gx1[i]), math.Float32bits(gx2[i]))
			}
		}
	}
}
