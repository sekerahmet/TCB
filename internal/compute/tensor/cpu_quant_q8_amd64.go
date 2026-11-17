package tensor

// Sarmalayıcıların sınır denetimleri: assembly dilimlerin dışına çıkmaz, çünkü burada çekirdeğin okuyacağı her
// aralık (ağırlık, kodlar, ölçekler, alt toplamlar, tmp, out) Go'da bir kez dilimlenir; çağıranın sözü yetmezse
// assembly sessizce okumak yerine Go panik verir.

// sdotq4kq8 dotQ4KQ8Go'nun AVX2 eşidir (blocks blok; tmp 16 baytlık çalışma alanı). Assembly: cpu_quant_q8_amd64.s.
//
//go:noescape
func sdotq4kq8(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte) float32

func dotQ4KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		return dotQ4KQ8Go(blocks, w, r)
	}
	_, _, _, _, _ = w[:blocks*144], r.q[:blocks*kBlock], r.d[:blocks], r.bs[:blocks*16], tmp[:16]
	return sdotq4kq8(blocks, &w[0], &r.q[0], &r.d[0], &r.bs[0], &tmp[0])
}

// sdotq4kq8x4 dört Q8 satırını aynı Q4_K satırıyla çarpar (satırlar sq, sd, sb bayt arayla; tmp 32 bayt);
// her bloğun sonucunu out[r]'ye ekler.
//
//go:noescape
func sdotq4kq8x4(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte, out *float32, sq, sd, sb int)

// dotQ4KQ8x4 r0'dan başlayan dört bitişik satırı (aynı tampondan, K arayla) çarpar: out[r] = r'inci satırın sonucu.
func dotQ4KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		dotQ8x4Go(dotQ4KQ8Go, blocks, w, r0, K, out)
		return
	}
	checkQ8x4(blocks, 144, w, r0, K, tmp[:32], out)
	clear(out[:4])
	sdotq4kq8x4(blocks, &w[0], &r0.q[0], &r0.d[0], &r0.bs[0], &tmp[0], &out[0], K, K/kBlock*4, K/16*2)
}

// sdotq4kq8x8 sekiz Q8 satırını (kodlar sq bayt arayla) aynı Q4_K satırıyla çarpar; dT blok başına sekiz d8, ps
// blok başına 64 alt blok çifti toplamı (q8x8Tables); her bloğun sonucunu out[r]'ye ekler (tmp 32 bayt).
//
//go:noescape
func sdotq4kq8x8(blocks int, w *byte, q *int8, dT *float32, ps *int16, tmp *byte, out *float32, sq int)

// q4x8Ready quantGemmQ8 Q4_K'da sekizli grupları sekiz satırlı çekirdeğe, Q4_0'da ağırlık satırlarını panele
// (q4PanelGemm) verir mi (AVX2 ve F16C).
func q4x8Ready() bool { return hasAVX2 && hasF16C }

// dotQ4KQ8x8 r0'dan başlayan sekiz bitişik satırı (aynı tampondan, K arayla) çarpar: out[r] = r'inci satırın
// sonucu. dT ve ps sekizlinin çevrik tablolarıdır (q8x8Tables); Go eşi onları kullanmaz.
func dotQ4KQ8x8(blocks int, w []byte, r0 *q8kRow, K int, dT []float32, ps []int16, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		dotQ8RowsGo(8, dotQ4KQ8Go, blocks, w, r0, K, out)
		return
	}
	_, _, _, _, _, _ = w[:blocks*144], r0.q[:7*K+blocks*kBlock], dT[:blocks*8], ps[:blocks*64], tmp[:32], out[:8]
	clear(out[:8])
	sdotq4kq8x8(blocks, &w[0], &r0.q[0], &dT[0], &ps[0], &tmp[0], &out[0], K)
}

// sdotq6kq8 dotQ6KQ8Go'nun AVX2 eşidir (tmp 32 baytlık çalışma alanı).
//
//go:noescape
func sdotq6kq8(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte) float32

func dotQ6KQ8(blocks int, w []byte, r *q8kRow, tmp []byte) float32 {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		return dotQ6KQ8Go(blocks, w, r)
	}
	_, _, _, _, _ = w[:blocks*210], r.q[:blocks*kBlock], r.d[:blocks], r.bs[:blocks*16], tmp[:32]
	return sdotq6kq8(blocks, &w[0], &r.q[0], &r.d[0], &r.bs[0], &tmp[0])
}

// sdotq6kq8x4 dört Q8 satırını aynı Q6_K satırıyla çarpar (satırlar sq, sd, sb bayt arayla; tmp 48 bayt);
// her bloğun sonucunu out[r]'ye ekler.
//
//go:noescape
func sdotq6kq8x4(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte, out *float32, sq, sd, sb int)

// dotQ6KQ8x4 r0'dan başlayan dört bitişik satırı (aynı tampondan, K arayla) çarpar: out[r] = r'inci satırın sonucu.
func dotQ6KQ8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		dotQ8x4Go(dotQ6KQ8Go, blocks, w, r0, K, out)
		return
	}
	checkQ8x4(blocks, 210, w, r0, K, tmp[:48], out)
	clear(out[:4])
	sdotq6kq8x4(blocks, &w[0], &r0.q[0], &r0.d[0], &r0.bs[0], &tmp[0], &out[0], K, K/kBlock*4, K/16*2)
}

// checkQ8x4 dört satırlı çekirdeğin okuyacağı aralıkları Go'da dilimler: ağırlık satırı, r0'ın arkasındaki üç
// satırın kodları, ölçekleri ve alt toplamları (bitişik tamponun kapasitesinden; blok boyu r0.blk), out'un dört öğesi.
func checkQ8x4(blocks, blockBytes int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	nb := K / r0.blk
	_, _, _ = w[:blocks*blockBytes], r0.q[:3*K+blocks*r0.blk], r0.d[:3*nb+blocks]
	_, _, _ = r0.bs[:3*K/16+blocks*r0.blk/16], tmp[0], out[:4]
}

// sdotq4q8 dotQ4Q8Go'nun AVX2 eşidir (Q4_0 ağırlık, 32'lik Q8 satır; blocks blok; bs satırın 16'lık kod
// toplamları). Assembly: cpu_quant_q4q8_amd64.s.
//
//go:noescape
func sdotq4q8(blocks int, w *byte, q *int8, d *float32, bs *int16) float32

func dotQ4Q8(blocks int, w []byte, r *q8kRow) float32 {
	if !hasAVX2 || !hasF16C || blocks == 0 {
		return dotQ4Q8Go(blocks, w, r)
	}
	_, _, _, _ = w[:blocks*18], r.q[:blocks*32], r.d[:blocks], r.bs[:blocks*2]
	return sdotq4q8(blocks, &w[0], &r.q[0], &r.d[0], &r.bs[0])
}

// packq4x8 sekiz Q4_0 satırını (w'den rowBytes arayla) sekiz satırlı panele yazar: blok başına 256 bayt kod
// (dword g·8 + r = r'inci satırın 4g..4g+3. öğeleri, 0–15) ve ps'de sekiz ölçek (float32).
//
//go:noescape
func packq4x8(blocks int, w *byte, rowBytes int, p *byte, ps *float32)

// sdotq4x8q8x4 paneli dört Q8 satırıyla çarpar (kodlar sq, d8 ve c8 sd bayt arayla): out[r·8 + j] r'inci
// etkinleştirme satırının j'inci ağırlık satırıyla sonucudur (sdotq4q8'inkiyle bit bit aynı).
//
//go:noescape
func sdotq4x8q8x4(blocks int, p *byte, ps *float32, q *int8, d *float32, c8 *int32, out *float32, sq, sd int)

// q4PanelGemm quantGemmQ8'in Q4_0 okumasıdır (q4x8Ready ve en az dört satır): n0..n1'in sekizli ağırlık satırları
// işçinin panelinde (pan, ps) bir kez paketlenir, panel bütün etkinleştirme satırlarıyla dörder çarpılır; dörde
// tamamlanmayan satırlar tek satırlı çekirdekle. Şerit bir ağırlık satırıdır: 32'lik blok başına yatay toplama yok
// (6764e85a'nın sekiz satırlı sdotq4q8x8'i her blokta sekiz satırın VPHADDD ağacını kuruyordu; okumada CPU'nun
// %82,9'u, profil TG-2026-11-17-HIGHRAM-T1-4). İşlenen son satırın ardını verir (n0 + 8·sekizli sayısı); kalan
// satırlar çağıranın. Sınırlar dilimin boyuyla (kapasitesiyle değil) denetlenir: kısa tampon assembly'den önce panik.
func q4PanelGemm(n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, c8 []int32, idx []int, c []float32, pan []byte, ps []float32) int {
	nb, F := K/32, len(idx)
	var out [32]float32
	n := n0
	for ; n+8 <= n1; n += 8 {
		_, _, _ = w[(n+8)*rowBytes-1], pan[nb*256-1], ps[nb*8-1]
		wp := w[n*rowBytes : (n+8)*rowBytes]
		packq4x8(nb, &wp[0], rowBytes, &pan[0], &ps[0])
		i := 0
		for ; i+4 <= F; i += 4 {
			_, _, _ = all.q[(i+4)*K-1], all.d[(i+4)*nb-1], c8[(i+4)*nb-1]
			q, d, cc := all.q[i*K:(i+4)*K], all.d[i*nb:(i+4)*nb], c8[i*nb:(i+4)*nb]
			sdotq4x8q8x4(nb, &pan[0], &ps[0], &q[0], &d[0], &cc[0], &out[0], K, nb*4)
			for r := range 4 {
				cr := c[idx[i+r]*N+n : idx[i+r]*N+n+8]
				for j := range 8 {
					cr[j] += out[r*8+j]
				}
			}
		}
		for ; i < F; i++ {
			for j := range 8 {
				c[idx[i]*N+n+j] += dotQ4Q8(nb, wp[j*rowBytes:(j+1)*rowBytes], rows[i])
			}
		}
	}
	return n
}
