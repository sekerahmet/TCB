package tensor

import (
	binaryArg "encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// Assembly çekirdekleri (GOAMD64=v3: cpu_mikro_amd64.s, cpu_vektor_amd64.s) ile taşınabilir Go
// başvuruları (cpu_basvuru.go, cpu_arkauc.go, cpu_sinir.go) AYNI programda, rastgele ve uç
// girdilerle karşılaştırılır: kenar boylar (0, 1, 7, 8, 15, 16, 17, 31, 33, büyük; 8'li ve
// 16'lı yazmaç bloklarının ve maskeli kuyruğun her durumu), hizasız başlangıçlar (dilim 32
// baytlık sınırdan 1, 3, 7 eleman kaymış), NaN, ±sonsuz, ±0 ve alt-normal değerler. Her
// çekirdeğin dilimin dışına yazmadığı da denetlenir. v3 dışındaki derlemelerde çekirdek
// başvurunun kendisidir; test orada da koşar ve sonuçların bit düzeyinde aynı olmasını görür.
//
// Toleranslar (gerekçeleriyle):
//   - Mikro çekirdek (mikroA/sgemm6x16): iki taraf da her C elemanı için Σ_k a_k·b_k toplamını
//     sıfırdan k sırasıyla biriktirip C'ye ekler; fark yalnızca yuvarlamadadır: assembly her
//     adımda kaynaştırılmış çarpma-toplama (FMA, tek yuvarlama) yapar, Go başvurusu çarpımı
//     ve toplamı ayrı yuvarlar (v3'te derleyici onu da FMA'ya çevirebilir). Ardışık toplamanın
//     hata sınırı |fl(S) − S| ≤ γ_n·Σ|a_k·b_k|, γ_n = n·u/(1 − n·u), u = 2⁻²⁴ olduğundan iki
//     sonucun farkı en çok 2·γ_(kb+1)·(Σ|a_k·b_k| + |c|) olabilir; alt-normal bölgede her adım
//     en çok bir alt-normal ulp (2⁻¹⁴⁹) yitirebildiği için (kb+1)·2⁻¹⁴⁹ mutlak pay eklenir.
//     Kısmi toplamı ya da çarpımı float32 sınırına (2¹²⁷) yaklaşan elemanlarda yalnızca
//     sınıf (NaN/sonsuz) aynılığı aranmaz: FMA çarpımı taşırmadan toplayabildiği için biri
//     sonsuz, öteki sonlu olabilir (iki sonuç da IEEE kurallarına uygundur).
//   - expVek: 2 ulp. Çekirdek ile expf32 aynı aralık indirgemesini ve polinomu kullanır;
//     polinomun Horner adımları assembly'de FMA'dır, Go'da (v1) ayrı yuvarlanır.
//   - GELU ileri/geri: 4 ulp; işlem sırası aynı, yalnızca içerideki üstel 1-2 ulp farklıdır ve
//     bölme/çarpma bu farkı en çok iki katına çıkarır.
//   - Çapraz entropi: en büyük bit düzeyinde aynı; Σe^{x−m} (float64, aynı 8 kulvar) göreli
//     4e-7 (her terimin üsteli 2 ulp ≈ 2.4e-7 farklı olabilir); gradyan 2 ulp.
//   - Katman/RMS normu: bit düzeyinde aynı (çekirdek Go başvurusunun işlem sırasını ve float64
//     ara toplamlarını birebir izler; FMA kullanmaz).
// NaN için yükün (payload) aynılığı aranmaz: iki tarafta da NaN olması yeter.

var (
	kernelSizes  = []int{0, 1, 7, 8, 15, 16, 17, 31, 33, 1031}
	alignOffsets = []int{0, 1, 3, 7}
)

// koruyucu dilimin dışına yazılmadığını görmek için çevreye konan değerdir.
const guard = float32(-12345.5)

// kayikDilim n elemanlık, başı kayma eleman kaymış ve iki yanı koruyucuyla dolu bir dilim verir;
// tam dilimi (koruyucuları denetlemek için) de döndürür.
func shiftedSlice(n, shift int) (slice, whole []float32) {
	whole = make([]float32, shift+n+9)
	for i := range whole {
		whole[i] = guard
	}
	return whole[shift : shift+n : shift+n], whole
}

func checkGuard(t *testing.T, ad string, whole []float32, shift, n int) {
	t.Helper()
	for i, v := range whole {
		if (i < shift || i >= shift+n) && v != guard {
			t.Fatalf("%s: çekirdek dilimin dışına yazdı ([%d] = %v; kayma %d, boy %d)", ad, i-shift, v, shift, n)
		}
	}
}

// ozelDeger çekirdekleri zorlayan değerlerden birini verir: büyük çoğunluk sıradan, kalanı
// NaN, ±sonsuz, ±0, alt-normal, çok büyük ve çok küçük normal değerler.
func customValue(r *rand.Rand, scaleF float64, custom bool) float32 {
	if !custom || r.Intn(4) != 0 {
		return float32(r.NormFloat64() * scaleF)
	}
	switch r.Intn(9) {
	case 0:
		return float32(math.NaN())
	case 1:
		return float32(math.Inf(1))
	case 2:
		return float32(math.Inf(-1))
	case 3:
		return float32(math.Copysign(0, -1))
	case 4:
		return 0
	case 5, 6: // alt-normal
		v := math.Float32frombits(uint32(1 + r.Intn(0x7FFFFF)))
		if r.Intn(2) == 0 {
			v = -v
		}
		return v
	case 7:
		return float32(math.Copysign(1e30, r.NormFloat64()))
	default:
		return float32(math.Copysign(1e-37, r.NormFloat64())) // en küçük normallerin yanı
	}
}

func fill(r *rand.Rand, d []float32, scaleF float64, custom bool) {
	for i := range d {
		d[i] = customValue(r, scaleF, custom)
	}
}

// ulpIcinde got ile want'ın yakınlığı: ikisi de NaN, ya da sonsuzsa eşit, değilse en çok ulp
// birim (işaretleri farklı ±0/alt-normal komşular mutlak olarak en çok ulp·2⁻¹⁴⁹).
func withinULP(got, want float32, ulp int64) bool {
	gn, wn := got != got, want != want
	if gn || wn {
		return gn && wn
	}
	if math.IsInf(float64(got), 0) || math.IsInf(float64(want), 0) {
		return got == want
	}
	if ulpDiff(got, want) <= ulp {
		return true
	}
	return math.Abs(float64(got)-float64(want)) <= float64(ulp)*math.Float64frombits(0x36A0000000000000) // ulp·2⁻¹⁴⁹
}

func nearSlice(t *testing.T, ad string, got, want []float32, ulp int64) {
	t.Helper()
	for i := range want {
		if !withinULP(got[i], want[i], ulp) {
			t.Fatalf("%s [%d]: çekirdek %v (%#x), başvuru %v (%#x); %d ulp'tan fazla", ad, i, got[i],
				math.Float32bits(got[i]), want[i], math.Float32bits(want[i]), ulp)
		}
	}
}

// vektorCekirdekleriniDenetle bir girdi dizisi için vektör çekirdeklerini (üstel, GELU, çapraz
// entropi) başvurularıyla karşılaştırır.
func checkVectorKernels(t *testing.T, ad string, x, g []float32, shift int) {
	t.Helper()
	n := len(x)
	// Üstel: ayrı çıktı ve yerinde.
	y, whole := shiftedSlice(n, shift)
	expVec(y, x)
	checkGuard(t, ad+" expVek", whole, shift, n)
	want := make([]float32, n)
	expVecGo(want, x)
	nearSlice(t, ad+" expVek", y, want, 2)
	copy(y, x)
	expVec(y, y)
	nearSlice(t, ad+" yerinde expVek", y, want, 2)

	// GELU ileri ve geri.
	y, whole = shiftedSlice(n, shift)
	geluForwardVec(y, x)
	checkGuard(t, ad+" geluIleri", whole, shift, n)
	geluForwardGo(want, x)
	nearSlice(t, ad+" geluIleri", y, want, 4)
	gx, whole := shiftedSlice(n, shift)
	for i := range gx {
		gx[i], want[i] = 0.25, 0.25
	}
	geluBackwardVec(gx, g, x)
	checkGuard(t, ad+" geluGeri", whole, shift, n)
	geluBackwardGo(want, g, x)
	nearSlice(t, ad+" geluGeri", gx, want, 4)

	// Çapraz entropi: satır işi ve gradyan.
	m1, s1 := ceRow(x)
	m2, s2 := ceRowGo(x)
	if !(math.Float32bits(m1) == math.Float32bits(m2) || (m1 != m1 && m2 != m2)) {
		t.Fatalf("%s ceSatir: en büyük %v, başvuru %v", ad, m1, m2)
	}
	if !(s1 == s2 || (s1 != s1 && s2 != s2) || math.Abs(s1-s2) <= 4e-7*math.Abs(s2)) {
		t.Fatalf("%s ceSatir: toplam %v, başvuru %v", ad, s1, s2)
	}
	l := float32(float64(m2) + math.Log(s2))
	h, whole := shiftedSlice(n, shift)
	for i := range h {
		h[i], want[i] = 0.5, 0.5
	}
	ceBackward(h, x, l, 0.375)
	checkGuard(t, ad+" ceGeri", whole, shift, n)
	ceBackwardGo(want, x, l, 0.375)
	nearSlice(t, ad+" ceGeri", h, want, 2)
}

// normCekirdekleriniDenetle katman normunu (rms: RMS normu) R×D girdide başvurusuyla bit
// düzeyinde karşılaştırır; g/b ve gradyan biriktiricileri isteğe bağlıdır (secim bitleri).
func checkNormKernels(t *testing.T, ad string, r *rand.Rand, R, D, shift int, rms bool, choice int, custom bool) {
	t.Helper()
	n := R * D
	x, _ := shiftedSlice(n, shift)
	fill(r, x, 1.5, custom)
	var g, b []float32
	if choice&1 != 0 {
		g, _ = shiftedSlice(D, shift)
		fill(r, g, 1, custom)
	}
	if choice&2 != 0 {
		b, _ = shiftedSlice(D, shift)
		fill(r, b, 1, custom)
	}
	var mean1, mean2 []float32
	if !rms {
		mean1, mean2 = make([]float32, R), make([]float32, R)
	}
	rstd1, rstd2 := make([]float32, R), make([]float32, R)
	y1, whole := shiftedSlice(n, shift)
	y2 := make([]float32, n)
	lnForwardBlock(y1, x, g, b, mean1, rstd1, D, 1e-5)
	checkGuard(t, ad+" lnIleri", whole, shift, n)
	lnForwardBlockGo(y2, x, g, b, mean2, rstd2, D, 1e-5)
	bitEqual32(t, ad+" lnIleri y", y1, y2)
	bitEqual32(t, ad+" lnIleri rstd", rstd1, rstd2)
	bitEqual32(t, ad+" lnIleri mean", mean1, mean2)

	gr, _ := shiftedSlice(n, shift)
	fill(r, gr, 1, custom)
	xg1, whole := shiftedSlice(n, shift)
	xg2 := make([]float32, n)
	for i := range xg1 {
		xg1[i], xg2[i] = 0.125, 0.125
	}
	var pg1, pg2, pb1, pb2 []float64
	if choice&4 != 0 {
		pg1, pg2, pb1, pb2 = make([]float64, D), make([]float64, D), make([]float64, D), make([]float64, D)
	}
	lnBackwardBlock(xg1, pg1, pb1, x, gr, g, mean2, rstd2, D)
	checkGuard(t, ad+" lnGeri", whole, shift, n)
	lnBackwardBlockGo(xg2, pg2, pb2, x, gr, g, mean2, rstd2, D)
	bitEqual32(t, ad+" lnGeri xg", xg1, xg2)
	bitEqual64(t, ad+" lnGeri pg", pg1, pg2)
	bitEqual64(t, ad+" lnGeri pb", pb1, pb2)
}

func bitEqual32(t *testing.T, ad string, got, want []float32) {
	t.Helper()
	for i := range want {
		if math.Float32bits(got[i]) != math.Float32bits(want[i]) && !(got[i] != got[i] && want[i] != want[i]) {
			t.Fatalf("%s [%d]: çekirdek %v (%#x), başvuru %v (%#x)", ad, i, got[i], math.Float32bits(got[i]), want[i], math.Float32bits(want[i]))
		}
	}
}

func bitEqual64(t *testing.T, ad string, got, want []float64) {
	t.Helper()
	for i := range want {
		if math.Float64bits(got[i]) != math.Float64bits(want[i]) && !(got[i] != got[i] && want[i] != want[i]) {
			t.Fatalf("%s [%d]: çekirdek %v, başvuru %v", ad, i, got[i], want[i])
		}
	}
}

// mikroDenetle mikroA'yı (v3'te sgemm6x16) mikroAGo ile karşılaştırır; C'nin geçerli bloğu
// dışındaki elemanlar ve dizinin çevresi değişmemelidir.
func checkMicro(t *testing.T, ad string, kb int, a []float32, rs, ks int, bp []float32, c0 []float32, ldc, mm, nn, shift int) {
	t.Helper()
	c, whole := shiftedSlice(len(c0), shift)
	copy(c, c0)
	want := append([]float32(nil), c0...)
	microA(kb, a, rs, ks, bp, c, ldc, mm, nn)
	checkGuard(t, ad, whole, shift, len(c0))
	microAGo(kb, a, rs, ks, bp, want, ldc, mm, nn)
	const u = 1.0 / (1 << 24)
	for i := range want {
		r, j := i/ldc, i%ldc
		if r >= mm || j >= nn {
			if math.Float32bits(c[i]) != math.Float32bits(c0[i]) {
				t.Fatalf("%s: geçerli bloğun dışındaki C[%d][%d] değişti: %v → %v", ad, r, j, c0[i], c[i])
			}
			continue
		}
		// Σ|a·b| ve kısmi toplamların en büyüğü float64'te (float32 çarpımları orada tamdır).
		absolute, peak, partial := math.Abs(float64(c0[i])), 0.0, 0.0
		for k := 0; k < kb; k++ {
			p := float64(a[r*rs+k*ks]) * float64(bp[k*nr+j])
			absolute += math.Abs(p)
			partial += p
			peak = math.Max(peak, math.Max(math.Abs(p), math.Abs(partial)))
		}
		peak = math.Max(peak, math.Abs(partial+float64(c0[i])))
		got, w := float64(c[i]), float64(want[i])
		if peak != peak || peak >= 0x1p127 || math.IsInf(absolute, 0) {
			// NaN ya da sonsuz girdi veya taşma bölgesi: yalnızca NaN aynılığı (taşma yoksa).
			if (got != got) != (w != w) && peak < 0x1p127 {
				t.Fatalf("%s C[%d][%d]: çekirdek %v, başvuru %v", ad, r, j, got, w)
			}
			continue
		}
		numer := 2*float64(kb+1)*u/(1-float64(kb+1)*u)*absolute + float64(kb+1)*0x1p-149
		if !(math.Abs(got-w) <= numer) {
			t.Fatalf("%s C[%d][%d]: çekirdek %v, başvuru %v (fark %g > pay %g)", ad, r, j, got, w, math.Abs(got-w), numer)
		}
	}
}

// mikroDuzenleri A'nın okunuş düzenleri (rs, ks): paketli, satır öncelikli, devrik.
func microLayouts(kb int) [][2]int { return [][2]int{{1, mr}, {kb + 3, 1}, {1, mr + 2}} }

func testMicroKernel(t *testing.T, r *rand.Rand, kb int, custom bool) {
	t.Helper()
	ldc := nr + 3
	for _, d := range microLayouts(kb) {
		rs, ks := d[0], d[1]
		for _, shift := range alignOffsets {
			uz := (mr-1)*rs + max(kb-1, 0)*ks + 1
			a, _ := shiftedSlice(uz, shift)
			fill(r, a, 1, custom)
			bp, _ := shiftedSlice(max(kb, 1)*nr, shift)
			fill(r, bp, 1, custom)
			c0 := make([]float32, mr*ldc)
			fill(r, c0, 1, custom)
			for _, mn := range [][2]int{{mr, nr}, {mr - 1, nr}, {mr, nr/2 + 1}, {1, 1}, {mr, 7 % nr}} {
				if mn[1] == 0 {
					continue
				}
				ad := fmt.Sprintf("%s mikroA kb=%d rs=%d ks=%d mm=%d nn=%d kayma=%d özel=%v", microKernelName, kb, rs, ks, mn[0], mn[1], shift, custom)
				checkMicro(t, ad, kb, a, rs, ks, bp, c0, ldc, mn[0], mn[1], shift)
			}
		}
	}
}

// Tablo: her boy, kayma ve değer türü (sıradan / uç değerli) için bütün çekirdekler.
func TestAssemblyKernelsMatchGo(t *testing.T) {
	r := rand.New(rand.NewSource(2026))
	t.Logf("çekirdek: %s", microKernelName)
	for _, custom := range []bool{false, true} {
		for _, n := range kernelSizes {
			for _, shift := range alignOffsets {
				ad := fmt.Sprintf("boy=%d kayma=%d özel=%v", n, shift, custom)
				x, _ := shiftedSlice(n, shift)
				fill(r, x, 3, custom)
				g, _ := shiftedSlice(n, shift)
				fill(r, g, 1, custom)
				checkVectorKernels(t, ad, x, g, shift)
				if n > 0 {
					for choice := 0; choice < 8; choice++ {
						checkNormKernels(t, ad, r, 1+n%3, n, shift, choice&1 != 0, choice, custom)
					}
				}
			}
			testMicroKernel(t, r, n, custom)
		}
	}
}

// Bulanıklaştırma: rastgele baytlar float32 değerleri olarak (her bit deseni: NaN yükleri,
// alt-normaller, sonsuzlar) bütün çekirdeklerden geçer. Assembly yalnızca GOAMD64=v3'te
// derlendiği için hedef o ayarla koşulmalıdır; kısa koşu:
// GOAMD64=v3 bash tools/bulaniklastir.sh 10s '^FuzzAssemblyCekirdekleriGoBasvurusu$'.
func FuzzAssemblyKernelsMatchGo(f *testing.F) {
	f.Add([]byte{0, 0, 0x80, 0x7f, 0, 0, 0xc0, 0x7f, 1, 0, 0, 0, 0, 0, 0x80, 0x3f}, uint8(1), uint8(3))
	f.Add(make([]byte, 4*33), uint8(0), uint8(16))
	f.Fuzz(func(t *testing.T, data []byte, shift, kb uint8) {
		n := min(len(data)/4, 2048)
		k := int(shift % 8)
		x, _ := shiftedSlice(n, k)
		for i := range x {
			x[i] = math.Float32frombits(binaryArg.LittleEndian.Uint32(data[4*i:]))
		}
		g := make([]float32, n)
		for i := range g {
			g[i] = x[n-1-i]
		}
		checkVectorKernels(t, "bulanık", x, g, k)
		if n > 0 {
			r := rand.New(rand.NewSource(int64(n)))
			checkNormKernels(t, "bulanık", r, 1, n, k, shift&8 != 0, int(shift>>4), true)
		}
		// Mikro çekirdek: A ve B baytlardan (döngüsel), kb en çok 64.
		kbn := int(kb % 65)
		if n == 0 {
			return
		}
		al := func(i int) float32 { return x[i%n] }
		a := make([]float32, (mr-1)*1+max(kbn-1, 0)*mr+1)
		for i := range a {
			a[i] = al(i)
		}
		bp := make([]float32, max(kbn, 1)*nr)
		for i := range bp {
			bp[i] = al(i + len(a))
		}
		c0 := make([]float32, mr*(nr+1))
		for i := range c0 {
			c0[i] = al(i + len(a) + len(bp))
		}
		checkMicro(t, "bulanık mikroA", kbn, a, 1, mr, bp, c0, nr+1, mr, nr, k)
	})
}
