package tensor

import (
	"math"
	"os"
)

// H1 hız prototipi (15.53, etkinleştirmenin nicemlenmesi; öntanımlı kapalı). TR_NICEM_ETKIN=1 iken Q4_K ve Q6_K
// ağırlıklı çarpım ağırlığı açmaz: x'in her satırı 256'lık bloklarla int8'e nicemlenir (quantizeQ8K), nokta
// çarpımı tamsayıdadır, blok başına bir kez float32'ye geçilir (Go eşleri dotQ4KQ8Go ve dotQ6KQ8Go; AVX2 eşleri
// cpu_quant_q8_amd64.s'de). Bitler float yolundan farklıdır; sonuç iş parçacığı sayısından ve satırların
// gruplanmasından bağımsızdır (her çıktı öğesi tek bir çekirdek çağrısıdır, blok sırası sabittir).
var actQuant = os.Getenv("TR_NICEM_ETKIN") == "1"

// q8kRow bir satırın nicemlenmiş hâli: q kodlar, d blok ölçekleri, bs 16'lık alt blokların kod toplamları.
type q8kRow struct {
	q  []int8
	d  []float32
	bs []int16
}

func newQ8KRow(K int) *q8kRow {
	return &q8kRow{q: make([]int8, K), d: make([]float32, K/kBlock), bs: make([]int16, K/16)}
}

// quantizeQ8K x'i nicemler: blokta d = max|x| / 127, kod = yuvarla(x · 127 / max|x|) (eşitte çifte), ±127.
// x sonludur (quantQ8Ready). max|x| < 127 / float32'nin en büyüğü (~3,7e−37) iken 127 / max|x| float32'de
// sonsuza taşar: kodlar işarete inerdi, sıfır öğe 0·∞ = NaN olurdu (int8(NaN) Go'da uygulamaya bağlıdır).
// O blokta oran float64'te hesaplanır (en küçük alt normalde bile sonludur); öbür bloklarda yol değişmez.
func quantizeQ8K(x []float32, r *q8kRow) {
	for b := 0; b < len(x)/kBlock; b++ {
		blk := x[b*kBlock : (b+1)*kBlock]
		q := r.q[b*kBlock : (b+1)*kBlock]
		var amax float32
		for _, v := range blk {
			if a := float32(math.Abs(float64(v))); a > amax {
				amax = a
			}
		}
		switch id := float32(127 / amax); {
		case !(amax > 0):
			clear(q)
			r.d[b] = 0
		case id-id != 0:
			r.d[b] = float32(amax / 127)
			id64 := 127 / float64(amax)
			for j, v := range blk {
				q[j] = int8(max(-127, min(127, math.RoundToEven(float64(v)*id64))))
			}
		default:
			r.d[b] = float32(amax / 127)
			for j, v := range blk {
				t := math.RoundToEven(float64(float32(v * id)))
				q[j] = int8(max(-127, min(127, t)))
			}
		}
		for s := 0; s < kBlock/16; s++ {
			var t int16
			for _, c := range q[s*16 : s*16+16] {
				t += int16(c)
			}
			r.bs[b*kBlock/16+s] = t
		}
	}
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

// quantQ8Ready dt'nin nicemli yolu bu girdide kullanılır mı: prototip açık, Q4_K ya da Q6_K ve girdinin hepsi
// sonlu. NaN ya da sonsuz varsa float yolu kullanılır: nicemleme onları sessizce bir sayıya çevirirdi (int8(NaN)
// tanımsızdır, sonsuz ölçeği 0 yapar); float yolu onları sonuca taşır.
func quantQ8Ready(dt DType, a []float32) bool {
	return actQuant && (dt == Q4K || dt == Q6K) && q8Finite(a)
}

// q8Finite a'nın hepsi sonlu mu (v − v NaN'da ve sonsuzda NaN'dır).
func q8Finite(a []float32) bool {
	for _, v := range a {
		if v-v != 0 {
			return false
		}
	}
	return true
}

// dotQ8 dt'nin tek satırlı nicemli çekirdeğidir.
func dotQ8(dt DType, blocks int, w []byte, r *q8kRow, tmp []byte) float32 {
	if dt == Q6K {
		return dotQ6KQ8(blocks, w, r, tmp)
	}
	return dotQ4KQ8(blocks, w, r, tmp)
}

// quantGemvQ8 quantGemv'in nicemli yoludur: x bir kez nicemlenir, her ağırlık satırı tamsayı çarpılır.
func quantGemvQ8(N, K int, a []float32, w []byte, dt DType, c []float32) {
	r := newQ8KRow(K)
	quantizeQ8K(a, r)
	rowBytes := quantRowBytes(dt, K)
	parallelFor(N, K, func(lo, hi int) {
		var tmp [q8Tmp]byte
		for n := lo; n < hi; n++ {
			c[n] += dotQ8(dt, K/kBlock, w[n*rowBytes:(n+1)*rowBytes], r, tmp[:])
		}
	})
	holdWorkers(quantHold)
}

// q8Tmp çekirdeklerin çalışma alanının baytıdır: en çoğu Q6_K'nın dört satırlısı (16 ölçek ve dört msum).
const q8Tmp = 48

// quantGemmQ8 çok satırlı nicemli çarpımdır: satırlar bir kez nicemlenir; her ağırlık satırı dört etkinleştirme
// satırıyla birlikte çarpılır (dotQ4KQ8x4, dotQ6KQ8x4: kodlar bir kez açılır), kalan satırlar tek tek. Her
// satırın sonucu tek satırlı yolunkiyle (quantGemvQ8) bit bit aynıdır.
func quantGemmQ8(M, N, K int, a []float32, w []byte, dt DType, c []float32) {
	nb := K / kBlock
	all := &q8kRow{q: make([]int8, M*K), d: make([]float32, M*nb), bs: make([]int16, M*K/16)}
	rows := make([]*q8kRow, M)
	for m := range M {
		rows[m] = &q8kRow{q: all.q[m*K : (m+1)*K], d: all.d[m*nb : (m+1)*nb], bs: all.bs[m*K/16 : (m+1)*K/16]}
	}
	// Satırlar birbirinden bağımsız nicemlenir: paralel nicemleme bitleri değiştirmez. Tek iş parçacığında 512
	// satırlık okumada duvar süresinin ~%14'üydü (profil, 2026-11-10, Qwen3-4B Q4_K_M, i7-13700H: 5 CPU-sn, seri).
	parallelFor(M, 4*K, func(lo, hi int) {
		for m := lo; m < hi; m++ {
			quantizeQ8K(a[m*K:(m+1)*K], rows[m])
		}
	})
	rowBytes := quantRowBytes(dt, K)
	four := dotQ8x4(dt)
	// Bloklama: iş birimi q8BlockN ağırlık satırıdır. Her etkinleştirme dörtlüsü (4·K bayt) bloğun bütün
	// ağırlık satırlarıyla art arda çarpılır, birinci düzey önbellekte kalır; blok ağırlığı (q8BlockN satır)
	// ikinci düzeyde kalır. Bloksuz sürümde her ağırlık satırı bütün etkinleştirmeyi (512 satırda 1,3–5 MB)
	// baştan okuyordu; 512 satırlık okumada float yolundan yavaştı (profil, 2026-11-10, Qwen3-4B Q4_K_M,
	// i7-13700H: süre çekirdeklerde, ~17 GOPS). Her c öğesi yine tek bir çekirdek çağrısının sonucunu bir kez
	// alır: bitler bloksuz sürümle aynıdır.
	nBlocks := (N + q8BlockN - 1) / q8BlockN
	parallelFor(nBlocks, q8BlockN*M*K, func(lo, hi int) {
		var tmp [q8Tmp]byte
		var out [4]float32
		for jb := lo; jb < hi; jb++ {
			n0, n1 := jb*q8BlockN, min(N, (jb+1)*q8BlockN)
			m := 0
			for ; m+4 <= M; m += 4 {
				for n := n0; n < n1; n++ {
					four(nb, w[n*rowBytes:(n+1)*rowBytes], rows[m], K, tmp[:], out[:])
					for r := range 4 {
						c[(m+r)*N+n] += out[r]
					}
				}
			}
			for ; m < M; m++ {
				for n := n0; n < n1; n++ {
					c[m*N+n] += dotQ8(dt, nb, w[n*rowBytes:(n+1)*rowBytes], rows[m], tmp[:])
				}
			}
		}
	})
}

// q8BlockN quantGemmQ8'in bir iş biriminin ağırlık satırı sayısıdır: K = 9728'de Q4_K 16 satır 87 KB,
// ikinci düzey önbelleğe (i7-13700H'nin P çekirdeğinde 1,25 MB) sığar [hesap].
const q8BlockN = 16

// dotQ8x4 dt'nin dört satırlı nicemli çekirdeğidir: out[r] = r0'dan başlayan r'inci satırın sonucu.
func dotQ8x4(dt DType) func(blocks int, w []byte, r0 *q8kRow, K int, tmp []byte, out []float32) {
	if dt == Q6K {
		return dotQ6KQ8x4
	}
	return dotQ4KQ8x4
}

// dotQ8x4Go dört satırlı çekirdeklerin Go eşidir: r0'dan başlayan dört satır (r0'ın dilimleri K arayla devam
// eder; bitişik tampon), out[r] = dot(r'inci satır).
func dotQ8x4Go(dot func(int, []byte, *q8kRow) float32, blocks int, w []byte, r0 *q8kRow, K int, out []float32) {
	nb := K / kBlock
	for r := range 4 {
		row := &q8kRow{q: r0.q[r*K : r*K+K : r*K+K], d: r0.d[r*nb : r*nb+nb], bs: r0.bs[r*K/16 : r*K/16+K/16]}
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
