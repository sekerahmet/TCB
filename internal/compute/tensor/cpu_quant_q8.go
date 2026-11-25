package tensor

import (
	"math"
	"sync"
)

// Etkinleştirmenin nicemlendiği çarpım (15.53a'nın TR_quantized_activations bloğu): Q4_0, Q4_K ve Q6_K ağırlıklı
// çarpım ağırlığı açmaz: x'in her satırı ağırlığın bloğu kadar (Q4_0'da 32, Q4_K ve Q6_K'da 256 öğe) bloklarla
// int8'e nicemlenir (quantizeQ8), nokta çarpımı tamsayıdadır, blok başına bir kez float32'ye geçilir (Go eşleri
// dotQ4Q8Go, dotQ4KQ8Go ve dotQ6KQ8Go; AVX2 eşleri cpu_quant_q8_amd64.s ve cpu_quant_q4q8_amd64.s'de). Bitler
// float yolundan farklıdır; sonuç iş parçacığı sayısından ve satırların gruplanmasından bağımsızdır (her çıktı
// öğesi tek bir çekirdek çağrısıdır, blok sırası sabittir). Yolu yalnız blok seçer (MatMulActQuant).

// q8kRow bir satırın nicemlenmiş hâli: q kodlar, d blok ölçekleri (blk öğede bir), bs 16'lık alt blokların kod
// toplamları (Q4_K ve Q6_K'nın çekirdekleri okur).
type q8kRow struct {
	q   []int8
	d   []float32
	bs  []int16
	blk int
}

var q8RowPool = sync.Pool{New: func() any { return &q8kRow{} }}

func takeQ8Row(K, blk int) *q8kRow {
	r := q8RowPool.Get().(*q8kRow)
	nb, nbs := K/blk, K/16
	if cap(r.q) < K {
		r.q = make([]int8, K)
	} else {
		r.q = r.q[:K]
	}
	if cap(r.d) < nb {
		r.d = make([]float32, nb)
	} else {
		r.d = r.d[:nb]
	}
	if cap(r.bs) < nbs {
		r.bs = make([]int16, nbs)
	} else {
		r.bs = r.bs[:nbs]
	}
	r.blk = blk
	return r
}

func putQ8Row(r *q8kRow) { q8RowPool.Put(r) }

// newQ8Row K öğelik, blk'lık bloklu boş satırdır; newQ8KRow 256'lıktır.
func newQ8Row(K, blk int) *q8kRow {
	return &q8kRow{q: make([]int8, K), d: make([]float32, K/blk), bs: make([]int16, K/16), blk: blk}
}

func newQ8KRow(K int) *q8kRow { return newQ8Row(K, kBlock) }

// actBlock dt ağırlıklı çarpımda etkinleştirmenin blok boyudur: ağırlığın bloğu (15.53a).
func actBlock(dt DType) int {
	elems, _ := dt.QuantBlock()
	return elems
}

// quantizeQ8Go x'i r.blk'lık bloklarla nicemler (15.53 taslağının formülü): blokta a = max|x|; a = 0 ise ya da
// 127 / a float32'de sonlu değilse (a < ~3,7e−37) bloğun kodları 0, ölçeği 0; değilse kod = x · float32(127 / a)'nın
// float32 sonucu, eşitte çifte yuvarlanır, ±127'ye kırpılır; ölçek = float32(a / 127). x sonludur (q8Finite).
// Taşan ölçekte blok sıfıra iner: katkısı ~1e−37 mertebesindedir; oran taşsaydı kodlar işarete inerdi, sıfır öğe
// 0·∞ = NaN olurdu (int8(NaN) Go'da uygulamaya bağlıdır).
func quantizeQ8Go(x []float32, r *q8kRow) {
	for b := 0; b < len(x)/r.blk; b++ {
		blk := x[b*r.blk : (b+1)*r.blk]
		q := r.q[b*r.blk : (b+1)*r.blk]
		var amax float32
		for _, v := range blk {
			if a := float32(math.Abs(float64(v))); a > amax {
				amax = a
			}
		}
		switch id := float32(127 / amax); {
		case !(amax > 0) || id-id != 0:
			clear(q)
			r.d[b] = 0
		default:
			r.d[b] = float32(amax / 127)
			for j, v := range blk {
				t := math.RoundToEven(float64(float32(v * id)))
				q[j] = int8(max(-127, min(127, t)))
			}
		}
		for s := 0; s < r.blk/16; s++ {
			var t int16
			for _, c := range q[s*16 : s*16+16] {
				t += int16(c)
			}
			r.bs[b*r.blk/16+s] = t
		}
	}
}

// dotQ4Q8Go x·aç(w)'nin nicemli eşidir (Q4_0 ağırlık, 32'lik Q8 satır; 15.53a): blokta tamsayı toplam
// isum = Σ (kod_w − 8)·kod_x (|isum| ≤ 32·8·127, float32'de kesin), blok başına bir kez float32'ye geçilir:
// t = d_w·d_x, acc += t·isum (çarpım ve toplama ayrı yuvarlanır, bloklar sırayla). sdotq4q8 bununla bit bit
// aynıdır. j. baytın alt yarısı j., üst yarısı j+16. öğedir (dequantQ4).
func dotQ4Q8Go(blocks int, w []byte, r *q8kRow) float32 {
	var acc float32
	for i := range blocks {
		b := w[i*18 : (i+1)*18]
		a := r.q[i*32 : (i+1)*32]
		var isum int32
		for j := range 16 {
			isum += (int32(b[2+j]&0xF)-8)*int32(a[j]) + (int32(b[2+j]>>4)-8)*int32(a[j+16])
		}
		t := float32(fp16(b) * r.d[i])
		acc = float32(acc + float32(t*float32(isum)))
	}
	return acc
}

// dotQ4KQ8Go x·aç(w)'nin nicemli eşidir (Q4_K ağırlık, Q8 satır): alt blok j'de tamsayı toplam
// sc_j·Σ q4·q8 ve m_j·Σ q8; blokta d8 · (d·isum − dmin·msum). sdotq4kq8 bununla bit bit aynıdır.
func dotQ4KQ8Go(blocks int, w []byte, r *q8kRow) float32 {
	var acc float32
	for i := range blocks {
		b := w[i*144 : (i+1)*144]
		d, dmin := fp16(b), fp16(b[2:])
		scales, q := b[4:16], b[16:]
		a := r.q[i*kBlock : (i+1)*kBlock]
		var isum, msum int32
		for g := range 4 {
			sc1, m1 := q4kScale(2*g, scales)
			sc2, m2 := q4kScale(2*g+1, scales)
			var s1, s2 int32
			for l := range 32 {
				s1 += int32(q[l]&0xF) * int32(a[g*64+l])
				s2 += int32(q[l]>>4) * int32(a[g*64+32+l])
			}
			isum += int32(sc1)*s1 + int32(sc2)*s2
			bs := r.bs[i*16+g*4 : i*16+g*4+4]
			msum += int32(m1)*int32(bs[0]+bs[1]) + int32(m2)*int32(bs[2]+bs[3])
			q = q[32:]
		}
		t := float32(float32(d*float32(isum)) - float32(dmin*float32(msum)))
		acc = float32(acc + float32(t*r.d[i]))
	}
	return acc
}

// QuantActSupported dt'nin ağırlığı etkinleştirmesi nicemlenen çarpıma girer mi (15.53a'nın biçim tablosu: q4,
// q4_k ve q6_k). Sonlu olmayan değer içeren satırın kararı satır satır verilir (quantGemvQ8, quantGemmQ8): o satır
// float yoluyla (quantGemv) hesaplanır, NaN ve sonsuz sonuca taşınır; nicemleme onları sessizce bir sayıya
// çevirirdi. Başka satırın verisi bir satırın sonucunu değiştirmez.
func QuantActSupported(dt DType) bool { return dt == Q4 || dt == Q4K || dt == Q6K }

// quantGemmAct quantGemm'in etkinleştirmesi nicemlenen eşidir: c[m, n] += Σ_k nicem(a)[m, k]·aç(w)[n, k].
func quantGemmAct(M, N, K int, a []float32, w []byte, dt DType, c []float32) {
	if K == 0 {
		return // boş toplam: c olduğu gibi kalır (GemmActQuant sıfırlamıştır)
	}
	if M == 1 {
		quantGemvQ8(N, K, a, w, dt, c)
		return
	}
	quantGemmQ8(M, N, K, a, w, dt, c)
}

// q8FiniteGo a'nın hepsi sonlu mu (v − v NaN'da ve sonsuzda NaN'dır).
func q8FiniteGo(a []float32) bool {
	for _, v := range a {
		if v-v != 0 {
			return false
		}
	}
	return true
}

// dotQ8 dt'nin tek satırlı nicemli çekirdeğidir.
func dotQ8(dt DType, blocks int, w []byte, r *q8kRow, tmp []byte) float32 {
	switch dt {
	case Q4:
		return dotQ4Q8(blocks, w, r)
	case Q6K:
		return dotQ6KQ8(blocks, w, r, tmp)
	}
	return dotQ4KQ8(blocks, w, r, tmp)
}

// dotQ8W4Go dört ağırlık satırlı tek-etkinleştirme çekirdeklerinin Go eşidir.
func dotQ8W4Go(dot func(int, []byte, *q8kRow) float32, blocks, blockBytes int, w []byte, rowBytes int, r *q8kRow, out []float32) {
	_ = out[:4]
	if blocks > 0 {
		_ = w[:3*rowBytes+blocks*blockBytes]
	}
	for j := range 4 {
		out[j] = dot(blocks, w[j*rowBytes:j*rowBytes+blocks*blockBytes], r)
	}
}

// dotQ8W4 dt'nin dört ağırlık satırlı nicemli çekirdeğidir (Q4_0 tekli yoldan gider). AVX-VNNI'siz işlemcide yoktur:
// orada tek satırlı AVX2 çekirdeği daha az işlemci zamanı harcar (Qwen3-4B Q4_K_M, 128 adımlık üretim, AMD EPYC 7B12,
// 8 iş parçacığı: dörtlüler 54,5, tek satırlılar 44,1 CPU-sn; üretim 11 670 → 10 956 ms; T1, 2026-11-23, profil ve
// commit'siz deneme). İkisi de Go eşiyle bit bit aynıdır; seçim yalnız hızı değiştirir.
func dotQ8W4(dt DType) func(blocks int, w []byte, rowBytes int, r *q8kRow, tmp []byte, out []float32) {
	if hasAVX2 && !hasAVXVNNI {
		return nil
	}
	switch dt {
	case Q4K:
		return dotQ4KQ8W4
	case Q6K:
		return dotQ6KQ8W4
	}
	return nil
}

// q8GemvBlock tek satırlı nicemli çarpımın en küçük ağırlık satırı karosudur: bir işçinin bir parçada yazdığı
// c 16 float32'dir (64 bayt; c 64 bayta hizalı değilse iki komşu parça bir önbellek satırını paylaşabilir).
// q8GemvMaxWorkers bellek bant genişliği sınırlı tek satırlı çarpımda kardeş iş parçacıklarının çekişmesini
// sınırlayan katılımcı tavanıdır (20 iş parçacıklı i7-13700H'ye göre; Gemini, 2026-11-23). Karo ve tavan yalnız
// işin bölünüşünü değiştirir: her c öğesi tek çekirdek çağrısının sonucunu bir kez alır, bitler işçi sayısından
// bağımsızdır.
const (
	q8GemvBlock      = 16
	q8GemvMaxWorkers = 16
)

// quantGemvQ8 quantGemv'in nicemli yoludur: x bir kez nicemlenir, ağırlık satırları dörder (Q4_K ve Q6_K) ya da tek tek
// tamsayı çarpılır. x'te sonlu olmayan değer varsa float yolu (quantGemv).
func quantGemvQ8(N, K int, a []float32, w []byte, dt DType, c []float32) {
	if !q8Finite(a[:K]) {
		quantGemv(N, K, a, w, dt, c)
		return
	}
	r := takeQ8Row(K, actBlock(dt))
	defer putQ8Row(r)
	quantizeQ8(a[:K], r)
	rowBytes := quantRowBytes(dt, K)
	nb := K / r.blk
	fourW := dotQ8W4(dt)
	tile := q8GemvBlock
	if N >= 8192 {
		tile = 4 * q8GemvBlock
	}
	nTiles := (N + tile - 1) / tile
	holdWorkers(quantHold)
	workersN := 1
	if N*K >= 2*parallelThreshold && nTiles > 1 {
		workersN = min(workerCount(N*K, parallelThreshold, elemBigJob), q8GemvMaxWorkers)
	}
	distribute(workersN, nTiles, func(_, t int) {
		var tmp [q8Tmp]byte
		var out [4]float32
		n := t * tile
		nEnd := min(N, n+tile)
		for ; fourW != nil && n+4 <= nEnd; n += 4 {
			fourW(nb, w[n*rowBytes:(n+4)*rowBytes], rowBytes, r, tmp[:], out[:])
			for j := range 4 {
				c[n+j] += out[j]
			}
		}
		for ; n < nEnd; n++ {
			c[n] += dotQ8(dt, nb, w[n*rowBytes:(n+1)*rowBytes], r, tmp[:])
		}
	})
	holdWorkers(quantHold)
}

// q8Tmp çekirdeklerin çalışma alanının baytıdır: en çoğu Q4_K'nın dört ağırlık satırlısı (4 × 16 bayt ölçek).
const q8Tmp = 64

// quantGemmQ8 çok satırlı nicemli çarpımdır: satırlar bir kez paralel nicemlenir; iş birimi q8BlockN ağırlık satırıdır.
// AVX2/F16C açık ve F >= 4 iken sekizli ağırlık satırları işçinin yerel panelinde bir kez paketlenip bütün etkinleştirme
// satırlarıyla dörder dörder çarpılır (q8Panel: Q4_0, Q4_K ve Q6_K). Sekizliye tamamlanmayan ağırlık satırları (ya da
// panelsiz yol) Q4_K'da sekizli, ardından dörderli ve tekli çekirdeklerle hesaplanır; her c öğesi tek çekirdek çağrısı
// alır ve quantGemvQ8 ile bit bit aynıdır. Sonlu olmayan değer içeren satır float yoluyla (quantGemv) hesaplanır.
func quantGemmQ8(M, N, K int, a []float32, w []byte, dt DType, c []float32) {
	idx := make([]int, 0, M)
	for m := range M {
		if q8Finite(a[m*K : (m+1)*K]) {
			idx = append(idx, m)
		} else {
			quantGemv(N, K, a[m*K:(m+1)*K], w, dt, c[m*N:(m+1)*N])
		}
	}
	F := len(idx)
	if F == 0 {
		return
	}
	blk := actBlock(dt)
	nb := K / blk
	all := newQ8Row(F*K, blk)
	rows := make([]*q8kRow, F)
	for i := range F {
		rows[i] = &q8kRow{q: all.q[i*K : (i+1)*K], d: all.d[i*nb : (i+1)*nb], bs: all.bs[i*K/16 : (i+1)*K/16], blk: blk}
	}
	// Satırlar birbirinden bağımsız nicemlenir: paralel nicemleme bitleri değiştirmez. Tek iş parçacığında 512
	// satırlık okumada duvar süresinin ~%14'üydü (profil, 2026-11-10, Qwen3-4B Q4_K_M, i7-13700H: 5 CPU-sn, seri).
	parallelFor(F, 4*K, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			quantizeQ8(a[idx[i]*K:(idx[i]+1)*K], rows[i])
		}
	})
	rowBytes := quantRowBytes(dt, K)
	four := dotQ8x4(dt)
	// Bloklama: iş birimi q8BlockN ağırlık satırıdır. Panelsiz yolda her etkinleştirme dörtlüsü (4·K bayt) bloğun
	// bütün ağırlık satırlarıyla art arda çarpılır, birinci düzey önbellekte kalır; blok ağırlığı ikinci düzeyde kalır.
	// Bloksuz sürümde her ağırlık satırı bütün etkinleştirmeyi (512 satırda 1,3–5 MB) baştan okuyordu; 512 satırlık
	// okumada float yolundan yavaştı (profil, 2026-11-10, Qwen3-4B Q4_K_M, i7-13700H: süre çekirdeklerde, ~17 GOPS).
	// Panel sekizli ağırlık satırlarını bir kez paketler, bütün etkinleştirme satırlarıyla çarpar. N sekizin katıyken
	// bloklar da sekizin katıdır (q8BlockN, derleme anında denetlenir): panel bütün satırları alır, eights kurulmaz.
	panel := newQ8Panel(dt, rows, nb)
	var eights *q8x8
	if !panel.on || N%8 != 0 {
		eights = newQ8x8(dt, rows, nb, K, rowBytes)
	}
	nBlocks := (N + q8BlockN - 1) / q8BlockN
	parallelFor(nBlocks, q8BlockN*F*K, func(lo, hi int) {
		var tmp [q8Tmp]byte
		var out [4]float32
		pan, pmeta, ps := panel.alloc()
		for jb := lo; jb < hi; jb++ {
			n0, n1 := jb*q8BlockN, min(N, (jb+1)*q8BlockN)
			n0 = panel.run(n0, n1, N, K, w, rowBytes, all, rows, idx, c, pan, pmeta, ps)
			if n0 == n1 {
				continue
			}
			i := eights.into(n0, n1, N, w, rows, idx, c, tmp[:])
			for ; !(panel.on && dt == Q4) && i+4 <= F; i += 4 {
				for n := n0; n < n1; n++ {
					four(nb, w[n*rowBytes:(n+1)*rowBytes], rows[i], K, tmp[:], out[:])
					for r := range 4 {
						c[idx[i+r]*N+n] += out[r]
					}
				}
			}
			for ; i < F; i++ {
				for n := n0; n < n1; n++ {
					c[idx[i]*N+n] += dotQ8(dt, nb, w[n*rowBytes:(n+1)*rowBytes], rows[i], tmp[:])
				}
			}
		}
	})
}

// q8x8 quantGemmQ8'in Q4_K'daki sekizli gruplarıdır: G grup (q4x8Ready ise len(rows)/8, değilse 0) ve sekiz satırlı
// çekirdeğin çevrik tabloları. Gruplar nicemlemeden sonra bir kez kurulur. Q4_0'ın okuması paneldedir (q4PanelGemm).
type q8x8 struct {
	G, nb, K, rowBytes int
	dT                 []float32
	ps                 []int16
}

func newQ8x8(dt DType, rows []*q8kRow, nb, K, rowBytes int) *q8x8 {
	e := &q8x8{nb: nb, K: K, rowBytes: rowBytes}
	if dt == Q4K && q4x8Ready() {
		e.G = len(rows) / 8
	}
	e.dT, e.ps = q8x8Tables(rows[:e.G*8], nb)
	return e
}

// into n0..n1 ağırlık satırlarını her sekizli grupla çarpar ve c'ye ekler (c'nin her öğesi yine tek bir çekirdek
// çağrısının sonucunu bir kez alır); sekizlilerin kapladığı satır sayısını (8·G) verir.
func (e *q8x8) into(n0, n1, N int, w []byte, rows []*q8kRow, idx []int, c []float32, tmp []byte) int {
	var out [8]float32
	for g := range e.G {
		for n := n0; n < n1; n++ {
			dotQ4KQ8x8(e.nb, w[n*e.rowBytes:(n+1)*e.rowBytes], rows[8*g], e.K, e.dT[g*e.nb*8:], e.ps[g*e.nb*64:], tmp, out[:])
			for r := range 8 {
				c[idx[8*g+r]*N+n] += out[r]
			}
		}
	}
	return 8 * e.G
}

// q8x8Tables sekiz satırlı çekirdeğin tablolarıdır (len(rows) sekizin katı): g'inci sekizlinin i'inci bloğunda
// dT[(g·nb+i)·8 + r] = d8, ps[(g·nb+i)·64 + p·16 + r·2 + k] = bs[2j] + bs[2j+1] (j = 2p + k, 32'lik alt blok j'nin
// kod toplamı; int16'da taşmaz: |bs| ≤ 16·127).
func q8x8Tables(rows []*q8kRow, nb int) ([]float32, []int16) {
	G := len(rows) / 8
	dT, ps := q8x8Scales(rows, nb), make([]int16, G*nb*64)
	for g := range G {
		for i := range nb {
			for r := range 8 {
				row := rows[g*8+r]
				for j := range 8 {
					ps[(g*nb+i)*64+(j/2)*16+r*2+j%2] = row.bs[i*16+2*j] + row.bs[i*16+2*j+1]
				}
			}
		}
	}
	return dT, ps
}

// q4Corrections panel çekirdeğinin düzeltmesidir: c8[i·nb + b] = −8·Σx, i'inci satırın b'inci 32'lik bloğunun kod
// toplamıyla (bs'nin iki 16'lığı). Panel kodları 0–15 tutar; isum = Σ kod·x + c8 = Σ (kod − 8)·x, tamsayıda kesin.
func q4Corrections(rows []*q8kRow, nb int) []int32 {
	c8 := make([]int32, len(rows)*nb)
	for i, r := range rows {
		for b := range nb {
			c8[i*nb+b] = -8 * (int32(r.bs[2*b]) + int32(r.bs[2*b+1]))
		}
	}
	return c8
}

// q8x8Scales sekizlilerin çevrik ölçek tablosudur (len(rows) sekizin katı): dT[(g·nb+i)·8 + r] = g'inci sekizlinin
// r'inci satırının i'inci bloğunun d8'i.
func q8x8Scales(rows []*q8kRow, nb int) []float32 {
	G := len(rows) / 8
	dT := make([]float32, G*nb*8)
	for g := range G {
		for i := range nb {
			for r := range 8 {
				dT[(g*nb+i)*8+r] = rows[g*8+r].d[i]
			}
		}
	}
	return dT
}

// q8BlockN quantGemmQ8'in bir iş biriminin ağırlık satırı sayısıdır: K = 9728'de Q4_K 16 satır 87 KB,
// ikinci düzey önbelleğe (i7-13700H'nin P çekirdeğinde 1,25 MB) sığar [hesap]. Sekizin katı olmalıdır: panel
// sekizli ağırlık satırlarını alır, N sekizin katıyken artık kalmaz ve quantGemmQ8 eights'i kurmaz; sekizin katı
// değilse bir sonraki satır derlenmez (sabit dizin sınır dışı).
const q8BlockN = 16

var _ = [1]int{}[q8BlockN%8]

// dotQ8x4 dt'nin dört satırlı nicemli çekirdeğidir: out[r] = r0'dan başlayan r'inci satırın sonucu.
func dotQ8x4(dt DType) func(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	switch dt {
	case Q4:
		return dotQ4Q8x4
	case Q6K:
		return dotQ6KQ8x4
	}
	return dotQ4KQ8x4
}

// dotQ4Q8x4 Q4_0'ın dört satırlısıdır: Go eşi (dotQ8x4Go). AVX2'li yolda Q4_0'ın okuması paneldedir (q4PanelGemm);
// buraya yalnız panelsiz durum (AVX2 yok ya da dörtten az satır) gelir.
func dotQ4Q8x4(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	dotQ8x4Go(dotQ4Q8Go, blocks, w, r0, K, out)
}

// dotQ8x4Go dört satırlı çekirdeklerin Go eşidir (dotQ8RowsGo, n = 4).
func dotQ8x4Go(dot func(int, []byte, *q8kRow) float32, blocks int, w []byte, r0 *q8kRow, K int, out []float32) {
	dotQ8RowsGo(4, dot, blocks, w, r0, K, out)
}

// dotQ8RowsGo n satırlı çekirdeklerin Go eşidir: r0'dan başlayan n satır (r0'ın dilimleri K arayla devam eder;
// bitişik tampon), out[r] = dot(r'inci satır). Tampon ya da out kısaysa panik verir.
func dotQ8RowsGo(n int, dot func(int, []byte, *q8kRow) float32, blocks int, w []byte, r0 *q8kRow, K int, out []float32) {
	nb := K / r0.blk
	for r := range n {
		row := &q8kRow{q: r0.q[r*K : r*K+K : r*K+K], d: r0.d[r*nb : r*nb+nb], bs: r0.bs[r*K/16 : r*K/16+K/16], blk: r0.blk}
		out[r] = dot(blocks, w, row)
	}
}

// dotQ6KQ8Go x·aç(w)'nin nicemli eşidir (Q6_K ağırlık, Q8 satır): 16'lık alt blok j'de sc_j·Σ (q6 − 32)·q8;
// tamsayıda isum = Σ sc_j·Σ q6·q8 ve msum = Σ sc_j·bs_j; blokta t = d·(isum − 32·msum), acc += t·d8.
// sdotq6kq8 bununla bit bit aynıdır.
func dotQ6KQ8Go(blocks int, w []byte, r *q8kRow) float32 {
	var acc float32
	for i := range blocks {
		b := w[i*210 : (i+1)*210]
		ql, qh, sc := b[0:128], b[128:192], b[192:208]
		d := fp16(b[208:])
		a := r.q[i*kBlock : (i+1)*kBlock]
		var isum, msum int32
		for n := range 2 {
			for l := range 32 {
				h := qh[l]
				q1 := int32(ql[l]&0xF | (h&3)<<4)
				q2 := int32(ql[l+32]&0xF | ((h>>2)&3)<<4)
				q3 := int32(ql[l]>>4 | ((h>>4)&3)<<4)
				q4 := int32(ql[l+32]>>4 | ((h>>6)&3)<<4)
				is := l / 16
				isum += int32(int8(sc[is]))*q1*int32(a[n*128+l]) + int32(int8(sc[is+2]))*q2*int32(a[n*128+l+32]) +
					int32(int8(sc[is+4]))*q3*int32(a[n*128+l+64]) + int32(int8(sc[is+6]))*q4*int32(a[n*128+l+96])
			}
			ql, qh, sc = ql[64:], qh[32:], sc[8:]
		}
		sc = b[192:208]
		for j := range 16 {
			msum += int32(int8(sc[j])) * int32(r.bs[i*16+j])
		}
		t := float32(d * float32(isum-32*msum))
		acc = float32(acc + float32(t*r.d[i]))
	}
	return acc
}
