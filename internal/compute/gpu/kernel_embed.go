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
