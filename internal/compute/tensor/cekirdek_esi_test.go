package tensor

import (
	ikili "encoding/binary"
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
	cekirdekBoylari = []int{0, 1, 7, 8, 15, 16, 17, 31, 33, 1031}
	hizaKaymalari   = []int{0, 1, 3, 7}
)

// koruyucu dilimin dışına yazılmadığını görmek için çevreye konan değerdir.
const koruyucu = float32(-12345.5)

// kayikDilim n elemanlık, başı kayma eleman kaymış ve iki yanı koruyucuyla dolu bir dilim verir;
// tam dilimi (koruyucuları denetlemek için) de döndürür.
func kayikDilim(n, kayma int) (dilim, tam []float32) {
	tam = make([]float32, kayma+n+9)
	for i := range tam {
		tam[i] = koruyucu
	}
	return tam[kayma : kayma+n : kayma+n], tam
}

func koruyucuDenetle(t *testing.T, ad string, tam []float32, kayma, n int) {
	t.Helper()
	for i, v := range tam {
		if (i < kayma || i >= kayma+n) && v != koruyucu {
			t.Fatalf("%s: çekirdek dilimin dışına yazdı ([%d] = %v; kayma %d, boy %d)", ad, i-kayma, v, kayma, n)
		}
	}
}

// ozelDeger çekirdekleri zorlayan değerlerden birini verir: büyük çoğunluk sıradan, kalanı
// NaN, ±sonsuz, ±0, alt-normal, çok büyük ve çok küçük normal değerler.
func ozelDeger(r *rand.Rand, olcek float64, ozel bool) float32 {
	if !ozel || r.Intn(4) != 0 {
		return float32(r.NormFloat64() * olcek)
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

func doldur(r *rand.Rand, d []float32, olcek float64, ozel bool) {
	for i := range d {
		d[i] = ozelDeger(r, olcek, ozel)
	}
}

// ulpIcinde got ile want'ın yakınlığı: ikisi de NaN, ya da sonsuzsa eşit, değilse en çok ulp
// birim (işaretleri farklı ±0/alt-normal komşular mutlak olarak en çok ulp·2⁻¹⁴⁹).
func ulpIcinde(got, want float32, ulp int64) bool {
	gn, wn := got != got, want != want
	if gn || wn {
		return gn && wn
	}
	if math.IsInf(float64(got), 0) || math.IsInf(float64(want), 0) {
		return got == want
	}
	if ulpFark(got, want) <= ulp {
		return true
	}
	return math.Abs(float64(got)-float64(want)) <= float64(ulp)*math.Float64frombits(0x36A0000000000000) // ulp·2⁻¹⁴⁹
}

func yakinDilim(t *testing.T, ad string, got, want []float32, ulp int64) {
	t.Helper()
	for i := range want {
		if !ulpIcinde(got[i], want[i], ulp) {
			t.Fatalf("%s [%d]: çekirdek %v (%#x), başvuru %v (%#x); %d ulp'tan fazla", ad, i, got[i],
				math.Float32bits(got[i]), want[i], math.Float32bits(want[i]), ulp)
		}
	}
}

// vektorCekirdekleriniDenetle bir girdi dizisi için vektör çekirdeklerini (üstel, GELU, çapraz
// entropi) başvurularıyla karşılaştırır.
func vektorCekirdekleriniDenetle(t *testing.T, ad string, x, g []float32, kayma int) {
	t.Helper()
	n := len(x)
	// Üstel: ayrı çıktı ve yerinde.
	y, tam := kayikDilim(n, kayma)
	expVek(y, x)
	koruyucuDenetle(t, ad+" expVek", tam, kayma, n)
	want := make([]float32, n)
	expVekGo(want, x)
	yakinDilim(t, ad+" expVek", y, want, 2)
	copy(y, x)
	expVek(y, y)
	yakinDilim(t, ad+" yerinde expVek", y, want, 2)

	// GELU ileri ve geri.
	y, tam = kayikDilim(n, kayma)
	geluIleriVek(y, x)
	koruyucuDenetle(t, ad+" geluIleri", tam, kayma, n)
	geluIleriGo(want, x)
	yakinDilim(t, ad+" geluIleri", y, want, 4)
	gx, tam := kayikDilim(n, kayma)
	for i := range gx {
		gx[i], want[i] = 0.25, 0.25
	}
	geluGeriVek(gx, g, x)
	koruyucuDenetle(t, ad+" geluGeri", tam, kayma, n)
	geluGeriGo(want, g, x)
	yakinDilim(t, ad+" geluGeri", gx, want, 4)

	// Çapraz entropi: satır işi ve gradyan.
	m1, s1 := ceSatir(x)
	m2, s2 := ceSatirGo(x)
	if !(math.Float32bits(m1) == math.Float32bits(m2) || (m1 != m1 && m2 != m2)) {
		t.Fatalf("%s ceSatir: en büyük %v, başvuru %v", ad, m1, m2)
	}
	if !(s1 == s2 || (s1 != s1 && s2 != s2) || math.Abs(s1-s2) <= 4e-7*math.Abs(s2)) {
		t.Fatalf("%s ceSatir: toplam %v, başvuru %v", ad, s1, s2)
	}
	l := float32(float64(m2) + math.Log(s2))
	h, tam := kayikDilim(n, kayma)
	for i := range h {
		h[i], want[i] = 0.5, 0.5
	}
	ceGeri(h, x, l, 0.375)
	koruyucuDenetle(t, ad+" ceGeri", tam, kayma, n)
	ceGeriGo(want, x, l, 0.375)
	yakinDilim(t, ad+" ceGeri", h, want, 2)
}

// normCekirdekleriniDenetle katman normunu (rms: RMS normu) R×D girdide başvurusuyla bit
// düzeyinde karşılaştırır; g/b ve gradyan biriktiricileri isteğe bağlıdır (secim bitleri).
func normCekirdekleriniDenetle(t *testing.T, ad string, r *rand.Rand, R, D, kayma int, rms bool, secim int, ozel bool) {
	t.Helper()
	n := R * D
	x, _ := kayikDilim(n, kayma)
	doldur(r, x, 1.5, ozel)
	var g, b []float32
	if secim&1 != 0 {
		g, _ = kayikDilim(D, kayma)
		doldur(r, g, 1, ozel)
	}
	if secim&2 != 0 {
		b, _ = kayikDilim(D, kayma)
		doldur(r, b, 1, ozel)
	}
	var mean1, mean2 []float32
	if !rms {
		mean1, mean2 = make([]float32, R), make([]float32, R)
	}
	rstd1, rstd2 := make([]float32, R), make([]float32, R)
	y1, tam := kayikDilim(n, kayma)
	y2 := make([]float32, n)
	lnIleriBlok(y1, x, g, b, mean1, rstd1, D, 1e-5)
	koruyucuDenetle(t, ad+" lnIleri", tam, kayma, n)
	lnIleriBlokGo(y2, x, g, b, mean2, rstd2, D, 1e-5)
	bitEsit32(t, ad+" lnIleri y", y1, y2)
	bitEsit32(t, ad+" lnIleri rstd", rstd1, rstd2)
	bitEsit32(t, ad+" lnIleri mean", mean1, mean2)

	gr, _ := kayikDilim(n, kayma)
	doldur(r, gr, 1, ozel)
	xg1, tam := kayikDilim(n, kayma)
	xg2 := make([]float32, n)
	for i := range xg1 {
		xg1[i], xg2[i] = 0.125, 0.125
	}
	var pg1, pg2, pb1, pb2 []float64
	if secim&4 != 0 {
		pg1, pg2, pb1, pb2 = make([]float64, D), make([]float64, D), make([]float64, D), make([]float64, D)
	}
	lnGeriBlok(xg1, pg1, pb1, x, gr, g, mean2, rstd2, D)
	koruyucuDenetle(t, ad+" lnGeri", tam, kayma, n)
	lnGeriBlokGo(xg2, pg2, pb2, x, gr, g, mean2, rstd2, D)
	bitEsit32(t, ad+" lnGeri xg", xg1, xg2)
	bitEsit64(t, ad+" lnGeri pg", pg1, pg2)
	bitEsit64(t, ad+" lnGeri pb", pb1, pb2)
}

func bitEsit32(t *testing.T, ad string, got, want []float32) {
	t.Helper()
	for i := range want {
		if math.Float32bits(got[i]) != math.Float32bits(want[i]) && !(got[i] != got[i] && want[i] != want[i]) {
			t.Fatalf("%s [%d]: çekirdek %v (%#x), başvuru %v (%#x)", ad, i, got[i], math.Float32bits(got[i]), want[i], math.Float32bits(want[i]))
		}
	}
}

func bitEsit64(t *testing.T, ad string, got, want []float64) {
	t.Helper()
	for i := range want {
		if math.Float64bits(got[i]) != math.Float64bits(want[i]) && !(got[i] != got[i] && want[i] != want[i]) {
			t.Fatalf("%s [%d]: çekirdek %v, başvuru %v", ad, i, got[i], want[i])
		}
	}
}

// mikroDenetle mikroA'yı (v3'te sgemm6x16) mikroAGo ile karşılaştırır; C'nin geçerli bloğu
// dışındaki elemanlar ve dizinin çevresi değişmemelidir.
func mikroDenetle(t *testing.T, ad string, kb int, a []float32, rs, ks int, bp []float32, c0 []float32, ldc, mm, nn, kayma int) {
	t.Helper()
	c, tam := kayikDilim(len(c0), kayma)
	copy(c, c0)
	want := append([]float32(nil), c0...)
	mikroA(kb, a, rs, ks, bp, c, ldc, mm, nn)
	koruyucuDenetle(t, ad, tam, kayma, len(c0))
	mikroAGo(kb, a, rs, ks, bp, want, ldc, mm, nn)
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
		mutlak, tepe, kismi := math.Abs(float64(c0[i])), 0.0, 0.0
		for k := 0; k < kb; k++ {
			p := float64(a[r*rs+k*ks]) * float64(bp[k*nr+j])
			mutlak += math.Abs(p)
			kismi += p
			tepe = math.Max(tepe, math.Max(math.Abs(p), math.Abs(kismi)))
		}
		tepe = math.Max(tepe, math.Abs(kismi+float64(c0[i])))
		got, w := float64(c[i]), float64(want[i])
		if tepe != tepe || tepe >= 0x1p127 || math.IsInf(mutlak, 0) {
			// NaN ya da sonsuz girdi veya taşma bölgesi: yalnızca NaN aynılığı (taşma yoksa).
			if (got != got) != (w != w) && tepe < 0x1p127 {
				t.Fatalf("%s C[%d][%d]: çekirdek %v, başvuru %v", ad, r, j, got, w)
			}
			continue
		}
		pay := 2*float64(kb+1)*u/(1-float64(kb+1)*u)*mutlak + float64(kb+1)*0x1p-149
		if !(math.Abs(got-w) <= pay) {
			t.Fatalf("%s C[%d][%d]: çekirdek %v, başvuru %v (fark %g > pay %g)", ad, r, j, got, w, math.Abs(got-w), pay)
		}
	}
}

// mikroDuzenleri A'nın okunuş düzenleri (rs, ks): paketli, satır öncelikli, devrik.
func mikroDuzenleri(kb int) [][2]int { return [][2]int{{1, mr}, {kb + 3, 1}, {1, mr + 2}} }

func mikroCekirdegiSina(t *testing.T, r *rand.Rand, kb int, ozel bool) {
	t.Helper()
	ldc := nr + 3
	for _, d := range mikroDuzenleri(kb) {
		rs, ks := d[0], d[1]
		for _, kayma := range hizaKaymalari {
			uz := (mr-1)*rs + max(kb-1, 0)*ks + 1
			a, _ := kayikDilim(uz, kayma)
			doldur(r, a, 1, ozel)
			bp, _ := kayikDilim(max(kb, 1)*nr, kayma)
			doldur(r, bp, 1, ozel)
			c0 := make([]float32, mr*ldc)
			doldur(r, c0, 1, ozel)
			for _, mn := range [][2]int{{mr, nr}, {mr - 1, nr}, {mr, nr/2 + 1}, {1, 1}, {mr, 7 % nr}} {
				if mn[1] == 0 {
					continue
				}
				ad := fmt.Sprintf("%s mikroA kb=%d rs=%d ks=%d mm=%d nn=%d kayma=%d özel=%v", mikroCekirdekAdi, kb, rs, ks, mn[0], mn[1], kayma, ozel)
				mikroDenetle(t, ad, kb, a, rs, ks, bp, c0, ldc, mn[0], mn[1], kayma)
			}
		}
	}
}

// Tablo: her boy, kayma ve değer türü (sıradan / uç değerli) için bütün çekirdekler.
func TestAssemblyCekirdekleriGoBasvurusuylaAyni(t *testing.T) {
	r := rand.New(rand.NewSource(2026))
	t.Logf("çekirdek: %s", mikroCekirdekAdi)
	for _, ozel := range []bool{false, true} {
		for _, n := range cekirdekBoylari {
			for _, kayma := range hizaKaymalari {
				ad := fmt.Sprintf("boy=%d kayma=%d özel=%v", n, kayma, ozel)
				x, _ := kayikDilim(n, kayma)
				doldur(r, x, 3, ozel)
				g, _ := kayikDilim(n, kayma)
				doldur(r, g, 1, ozel)
				vektorCekirdekleriniDenetle(t, ad, x, g, kayma)
				if n > 0 {
					for secim := 0; secim < 8; secim++ {
						normCekirdekleriniDenetle(t, ad, r, 1+n%3, n, kayma, secim&1 != 0, secim, ozel)
					}
				}
			}
			mikroCekirdegiSina(t, r, n, ozel)
		}
	}
}

// Bulanıklaştırma: rastgele baytlar float32 değerleri olarak (her bit deseni: NaN yükleri,
// alt-normaller, sonsuzlar) bütün çekirdeklerden geçer. Assembly yalnızca GOAMD64=v3'te
// derlendiği için hedef o ayarla koşulmalıdır; kısa koşu:
// GOAMD64=v3 bash tools/bulaniklastir.sh 10s '^FuzzAssemblyCekirdekleriGoBasvurusu$'.
func FuzzAssemblyCekirdekleriGoBasvurusu(f *testing.F) {
	f.Add([]byte{0, 0, 0x80, 0x7f, 0, 0, 0xc0, 0x7f, 1, 0, 0, 0, 0, 0, 0x80, 0x3f}, uint8(1), uint8(3))
	f.Add(make([]byte, 4*33), uint8(0), uint8(16))
	f.Fuzz(func(t *testing.T, veri []byte, kayma, kb uint8) {
		n := min(len(veri)/4, 2048)
		k := int(kayma % 8)
		x, _ := kayikDilim(n, k)
		for i := range x {
			x[i] = math.Float32frombits(ikili.LittleEndian.Uint32(veri[4*i:]))
		}
		g := make([]float32, n)
		for i := range g {
			g[i] = x[n-1-i]
		}
		vektorCekirdekleriniDenetle(t, "bulanık", x, g, k)
		if n > 0 {
			r := rand.New(rand.NewSource(int64(n)))
			normCekirdekleriniDenetle(t, "bulanık", r, 1, n, k, kayma&8 != 0, int(kayma>>4), true)
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
		mikroDenetle(t, "bulanık mikroA", kbn, a, 1, mr, bp, c0, nr+1, mr, nr, k)
	})
}
