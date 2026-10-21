package gpu

import (
	"sort"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Gömme (ileri: satır seçme; geri: belirlenimci gruplu toplama)
// ============================================================================
//
// Geri yönde atomik toplama kullanılmaz: indeksler konakta olduğundan konakta tekil
// satırlara göre gruplanır (her grup indeks sırasıyla); her tekil satırı bir blok, grubun
// gradyan satırlarını sırayla ekler. Sonuç CPU arka ucuyla aynı sırada biriktirilir (bit
// düzeyinde aynı) ve her çalıştırmada aynıdır.

const embedBlock = 128

// embedKernel out[r, :] = w[idx[r], :]; satır başına blok. Tablo kendi türünde (16 bitlik
// tablo float32'ye açılmadan) okunur.
func embedKernel(dt tensor.DType) func() *emitter {
	return func() *emitter { return genEmbed(dt) }
}

func genEmbed(dt tensor.DType) *emitter {
	k := newEmitter("gomme_" + dt.String())
	w := k.pointer("w")
	idx := k.pointer("idx")
	out := k.written("cikti")
	D := k.u32p("d")
	r := k.special("%ctaid.x")
	tid := k.special("%tid.x")
	row := k.r()
	k.e("ld.global.u32", row, mem(k.address(idx, r, 4), 0))
	source, target := k.r(), k.r()
	k.e("mul.lo.s32", source, row, D)
	k.e("mul.lo.s32", target, r, D)
	k.rowLoop(tid, D, k.whole(1), k.whole(0), embedBlock, func(ofs, j reg) {
		ks, hs := k.r(), k.r()
		k.e("add.s32", ks, source, j)
		k.e("add.s32", hs, target, j)
		k.store(tensor.Float32, k.address(out, hs, 4), k.load(dt, k.address(w, ks, dtypeSize(dt))))
	})
	return k
}

// embedBackwardKernel gw[tekil[u], :] += Σ_{p ∈ grup u} g[konum[p], :] (sırayla).
func embedBackwardKernel() *emitter {
	k := newEmitter("gomme_geri")
	gw := k.written("gw")
	g := k.pointer("g")
	singular := k.pointer("tekil")
	begin := k.pointer("bas")
	pos := k.pointer("konum")
	D := k.u32p("d")
	u := k.special("%ctaid.x")
	tid := k.special("%tid.x")
	row, p0, p1 := k.r(), k.r(), k.r()
	k.e("ld.global.u32", row, mem(k.address(singular, u, 4), 0))
	ba := k.address(begin, u, 4)
	k.e("ld.global.u32", p0, mem(ba, 0))
	k.e("ld.global.u32", p1, mem(ba, 4))
	target := k.r()
	k.e("mul.lo.s32", target, row, D)
	k.rowLoop(tid, D, k.whole(1), k.whole(0), embedBlock, func(ofs, j reg) {
		hs := k.r()
		k.e("add.s32", hs, target, j)
		ha := k.address(gw, hs, 4)
		acc := k.load(tensor.Float32, ha)
		p := k.r()
		k.e("mov.u32", p, p0)
		loop, done := k.label(), k.label()
		k.put(loop)
		b := k.p()
		k.e("setp.ge.u32", b, p, p1)
		k.jumpIf(b, done)
		kn := k.r()
		k.e("ld.global.u32", kn, mem(k.address(pos, p, 4), 0))
		gs := k.r()
		k.e("mad.lo.s32", gs, kn, D, j)
		k.e("add.rn.f32", acc, acc, k.load(tensor.Float32, k.address(g, gs, 4)))
		k.e("add.s32", p, p, 1)
		k.jump(loop)
		k.put(done)
		k.store(tensor.Float32, ha, acc)
	})
	return k
}

// embedGroups indeksleri tekil satırlara göre gruplar: tekil satırlar (artan),
// grupların başlangıçları (son eleman toplam) ve her gruptaki konumlar (artan).
func embedGroups(idx []int) (singular, begin, pos []int32) {
	num := map[int]int{}
	for _, v := range idx {
		num[v]++
	}
	sorted := make([]int, 0, len(num))
	for v := range num {
		sorted = append(sorted, v)
	}
	sort.Ints(sorted)
	place := make(map[int]int, len(sorted))
	begin = make([]int32, len(sorted)+1)
	for i, v := range sorted {
		place[v] = int(begin[i])
		begin[i+1] = begin[i] + int32(num[v])
		singular = append(singular, int32(v))
	}
	pos = make([]int32, len(idx))
	for p, v := range idx {
		pos[place[v]] = int32(p)
		place[v]++
	}
	return singular, begin, pos
}

// embedChunk ağır bir satırın geçişlerinin bölündüğü parçanın boyudur (tensor.EmbeddingChunk;
// CPU aynı sırayla toplar). Gerçek metinde sık parçalar ("ve", "bir") bir toplu işte binlerce
// kez geçer: satırı tek bir blok sırayla toplarsa öbür bloklar biter, o blok tek başına çalışır
// (Zipf dağılımlı veride adımda 34 ms, tekdüzede 1,7 ms). PyTorch aynı yolu izler (ATen
// EmbeddingBackwardKernel.cu: NROWS_PER_THREAD = 10'luk "partial segment"ler; belirlenimci yolda
// compute_grad_weight + sum_and_scatter, atomik toplama yok).
const embedChunk = tensor.EmbeddingChunk

// embedPlan gradyan toplamanın üç geçişidir; üçü de embedBackwardKernel'dir (bir grup satırı
// sırayla toplar), yalnızca indeks dizileri farklıdır:
//   - hafif satırlar (geçiş ≤ embedChunk): doğrudan gw'ye (bugünkü yol, ek tampon yok);
//   - ağır satırların parçaları: her parça (en çok embedChunk geçiş) ara tamponun bir satırına;
//   - ağır satırlar: kendi parçalarının toplamlarını sırayla gw'ye.
//
// Toplama sırası girdiden belirlenir, her koşuda aynıdır (atomik toplama yok).
type embedPlan struct {
	light   [3][]int32 // tekil, baş, konum (hafif satırlar)
	chunks  [3][]int32 // tampon satırı, baş, konum (parçalar)
	heavy   [3][]int32 // tekil, baş, parça sırası (ağır satırlar)
	nChunks int
}

// planEmbed embedGroups'un gruplarını hafif ve ağır satırlara ayırır.
func planEmbed(singular, begin, pos []int32) embedPlan {
	var p embedPlan
	p.light[1] = []int32{0}
	p.chunks[1] = []int32{0}
	p.heavy[1] = []int32{0}
	for u, row := range singular {
		b, e := begin[u], begin[u+1]
		if e-b <= embedChunk {
			p.light[0] = append(p.light[0], row)
			p.light[2] = append(p.light[2], pos[b:e]...)
			p.light[1] = append(p.light[1], int32(len(p.light[2])))
			continue
		}
		p.heavy[0] = append(p.heavy[0], row)
		for s := b; s < e; s += embedChunk {
			p.chunks[0] = append(p.chunks[0], int32(p.nChunks))
			p.chunks[2] = append(p.chunks[2], pos[s:min(s+embedChunk, e)]...)
			p.chunks[1] = append(p.chunks[1], int32(len(p.chunks[2])))
			p.heavy[2] = append(p.heavy[2], int32(p.nChunks))
			p.nChunks++
		}
		p.heavy[1] = append(p.heavy[1], int32(len(p.heavy[2])))
	}
	return p
}
