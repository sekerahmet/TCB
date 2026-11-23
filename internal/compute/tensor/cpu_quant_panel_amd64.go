package tensor

// Q4_K ve Q6_K ağırlıklarının sekiz satırlı panel çarpımı (quantGemmQ8).
// Sekiz ağırlık satırı işçinin yerel panelinde (pan: blok başına 2048 bayt kod; pmeta: blok başına ölçek ve
// minimumlar) bir kez açılıp çevrilir, ardından bütün etkinleştirme satırlarıyla dörder dörder çarpılır.
// VEX kodlamalı tamsayı nokta çarpımı komutu varsa (hasAVXVNNI) VNNI çekirdeği, yoksa AVX2 çekirdeği koşar;
// her iki yol da dotQ4KQ8Go ve dotQ6KQ8Go ile bit bit aynıdır.

const (
	q4kPanelCodeBytes = 2048
	q4kPanelMetaBytes = 448
	q6kPanelCodeBytes = 2048
	q6kPanelMetaBytes = 800
)

type q8Panel struct {
	dt   DType
	nb   int
	on   bool
	c8   []int32
	bs32 []int16
}

func newQ8Panel(dt DType, rows []*q8kRow, nb int) q8Panel {
	if !q4x8Ready() || len(rows) < 4 {
		return q8Panel{}
	}
	p := q8Panel{dt: dt, nb: nb, on: true}
	switch dt {
	case Q4:
		p.c8 = q4Corrections(rows, nb)
	case Q4K:
		p.bs32 = q4kSubSums(rows, nb)
	}
	return p
}

func (p q8Panel) alloc() (pan, pmeta []byte, ps []float32) {
	if !p.on {
		return nil, nil, nil
	}
	switch p.dt {
	case Q4:
		return make([]byte, p.nb*256), nil, make([]float32, p.nb*8)
	case Q4K:
		return make([]byte, p.nb*q4kPanelCodeBytes), make([]byte, p.nb*q4kPanelMetaBytes), nil
	default:
		return make([]byte, p.nb*q6kPanelCodeBytes), make([]byte, p.nb*q6kPanelMetaBytes), nil
	}
}

func (p q8Panel) run(n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, idx []int, c []float32, pan, pmeta []byte, ps []float32) int {
	if !p.on {
		return n0
	}
	switch p.dt {
	case Q4:
		return q4PanelGemm(n0, n1, N, K, w, rowBytes, all, rows, p.c8, idx, c, pan, ps)
	case Q4K:
		return q4kPanelGemm(n0, n1, N, K, w, rowBytes, all, rows, p.bs32, idx, c, pan, pmeta)
	default:
		return q6kPanelGemm(n0, n1, N, K, w, rowBytes, all, rows, idx, c, pan, pmeta)
	}
}

// q4kSubSums her satırın her bloğundaki sekiz adet 32'lik alt blok kod toplamını (bs[2s] + bs[2s+1]) kurar.
func q4kSubSums(rows []*q8kRow, nb int) []int16 {
	out := make([]int16, len(rows)*nb*8)
	for i, r := range rows {
		dst := out[i*nb*8 : (i+1)*nb*8]
		for b := range nb {
			for s := range 8 {
				dst[b*8+s] = r.bs[b*16+2*s] + r.bs[b*16+2*s+1]
			}
		}
	}
	return out
}

//go:noescape
func packq4kx8(blocks int, w *byte, rowBytes int, pan *byte, pmeta *byte)

//go:noescape
func sdotq4kx8q8x4Vnni(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs32 *int16, out *float32, sq, sd, sbs int)

//go:noescape
func sdotq4kx8q8x4Avx2(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs32 *int16, out *float32, sq, sd, sbs int)

// q4kPanelGemm n0..n1 aralığındaki sekizli Q4_K ağırlık gruplarını panelden çarpar; işlenen son satırın ardını döner.
// Etkinleştirmenin 32'lik alt blok toplamları bs32'dedir (q4kSubSums).
func q4kPanelGemm(n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, bs32 []int16, idx []int, c []float32, pan, pmeta []byte) int {
	k := panelKernel{pack: packq4kx8, dot: sdotq4kx8q8x4Avx2, one: dotQ4KQ8, codeBytes: q4kPanelCodeBytes, metaBytes: q4kPanelMetaBytes, bsPerBlock: 8}
	if hasAVXVNNI {
		k.dot = sdotq4kx8q8x4Vnni
	}
	return panelGemm8(k, bs32, n0, n1, N, K, w, rowBytes, all, rows, idx, c, pan, pmeta)
}

// panelKernel bir türün sekiz satırlı panel çarpımının parçalarıdır: paketleyici, sekiz ağırlık satırı × dört etkinleştirme
// satırı çekirdeği, dörde tamamlanmayan etkinleştirme satırları için tek satırlı çekirdek, blok başına panel baytları ve
// etkinleştirmenin blok başına bs öğesi (Q4_K: 32'lik alt blok toplamları 8, Q6_K: 16'lık toplamlar 16).
type panelKernel struct {
	pack                 func(blocks int, w *byte, rowBytes int, pan *byte, pmeta *byte)
	dot                  func(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs *int16, out *float32, sq, sd, sbs int)
	one                  func(blocks int, w []byte, r *q8kRow, tmp []byte) float32
	codeBytes, metaBytes int
	bsPerBlock           int
}

// panelGemm8 n0..n1 aralığındaki sekizli ağırlık gruplarını k'nin paneliyle çarpar; işlenen son satırın ardını döner. Her grup
// bir kez paketlenir, dörder etkinleştirme satırıyla çarpılır, kalan satırlar tek satırlı çekirdekle; her c öğesi tek çekirdek
// çağrısının sonucunu bir kez alır. Sınırlar Go dilimleriyle denetlenir: kısa tampon assembly'ye girmeden panik verir.
func panelGemm8(k panelKernel, bs []int16, n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, idx []int, c []float32, pan, pmeta []byte) int {
	nb, F := K/kBlock, len(idx)
	sb := nb * k.bsPerBlock
	var out [32]float32
	var tmp [q8Tmp]byte
	n := n0
	for ; n+8 <= n1; n += 8 {
		_, _, _ = w[(n+8)*rowBytes-1], pan[nb*k.codeBytes-1], pmeta[nb*k.metaBytes-1]
		wp := w[n*rowBytes : (n+8)*rowBytes]
		k.pack(nb, &wp[0], rowBytes, &pan[0], &pmeta[0])
		i := 0
		for ; i+4 <= F; i += 4 {
			_, _, _ = all.q[(i+4)*K-1], all.d[(i+4)*nb-1], bs[(i+4)*sb-1]
			q, d, b := all.q[i*K:(i+4)*K], all.d[i*nb:(i+4)*nb], bs[i*sb:(i+4)*sb]
			k.dot(nb, &pan[0], &pmeta[0], &q[0], &d[0], &b[0], &out[0], K, nb*4, sb*2)
			for r := range 4 {
				cr := c[idx[i+r]*N+n : idx[i+r]*N+n+8]
				for j := range 8 {
					cr[j] += out[r*8+j]
				}
			}
		}
		for ; i < F; i++ {
			for j := range 8 {
				c[idx[i]*N+n+j] += k.one(nb, wp[j*rowBytes:(j+1)*rowBytes], rows[i], tmp[:])
			}
		}
	}
	return n
}

//go:noescape
func packq6kx8(blocks int, w *byte, rowBytes int, pan *byte, pmeta *byte)

//go:noescape
func sdotq6kx8q8x4Vnni(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs *int16, out *float32, sq, sd, sbs int)

//go:noescape
func sdotq6kx8q8x4Avx2(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs *int16, out *float32, sq, sd, sbs int)

// q6kPanelGemm n0..n1 aralığındaki sekizli Q6_K ağırlık gruplarını panelden çarpar; işlenen son satırın ardını döner.
// Etkinleştirmenin 16'lık toplamları all.bs'tedir.
func q6kPanelGemm(n0, n1, N, K int, w []byte, rowBytes int, all *q8kRow, rows []*q8kRow, idx []int, c []float32, pan, pmeta []byte) int {
	k := panelKernel{pack: packq6kx8, dot: sdotq6kx8q8x4Avx2, one: dotQ6KQ8, codeBytes: q6kPanelCodeBytes, metaBytes: q6kPanelMetaBytes, bsPerBlock: 16}
	if hasAVXVNNI {
		k.dot = sdotq6kx8q8x4Vnni
	}
	return panelGemm8(k, all.bs, n0, n1, N, K, w, rowBytes, all, rows, idx, c, pan, pmeta)
}
