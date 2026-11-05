package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Nicemlenmiş ağırlıkla tek satırlık çarpım (15.53; üretimde bir parça)
// ============================================================================
//
// c[n] (+)= Σ_k a[k]·aç(w)[n, k]. Her warp bir çıktıyı hesaplar: ağırlığın satırı K/blok bloktur,
// warp'ın şeritleri bloklara bölünür ("q8"de blok başına 4, K türlerinde 8 şerit), şerit kendi
// parçasını yazmaçta açar, etkinleştirmeyle çarpıp dört toplayıcıda biriktirir; bellekte açılmış
// değer yoktur. Toplama sırası yalnız şekle bağlıdır: toplayıcılar sabit sırayla, sonra warp içi
// kelebek; atomik toplama yoktur. Açılan değer sonlu ölçekte quantElem'inkiyle aynıdır (bu yolda
// NaN'ın bitleri korunmaz: sonuç zaten NaN'dır).

const (
	quantWarps = 8 // bloktaki warp sayısı: blok başına çıktı
	quantAccs  = 4 // şeridin bağımsız toplayıcı sayısı
)

// quantLanes bir bloğu okuyan şerit sayısıdır: "q8"de şerit 8 öğe, K türlerinde 32 öğe alır.
func quantLanes(dt tensor.DType) int {
	if dt == tensor.Q8 {
		return 4
	}
	return 8
}

func quantGemvKernel(dt tensor.DType, over bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("nicem_gemv_%s_%v", dt.Name(), over))
		pa, pw := k.pointer("a"), k.pointer("w")
		pc := k.written("c")
		N, count := k.u32p("n"), k.u32p("bloklar")
		tid := k.special("%tid.x")
		lane, n := k.r(), k.r()
		k.e("and.b32", lane, tid, 31)
		k.e("shr.u32", n, tid, 5)
		k.e("mad.lo.s32", n, k.special("%ctaid.x"), quantWarps, n)
		end := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, n, N) // bütün warp aynı çıktıda: warp birlikte çıkar
		k.jumpIf(p, end)
		_, size := dt.QuantBlock()
		rowBytes, row := k.r(), k.rd()
		k.e("mul.lo.s32", rowBytes, count, size)
		k.e("mul.wide.u32", row, n, rowBytes)
		k.e("add.s64", row, row, pw)
		lanes := quantLanes(dt)
		sub, b := k.r(), k.r()
		k.e("and.b32", sub, lane, lanes-1)
		k.e("shr.u32", b, lane, log2(lanes))
		acc := make([]reg, quantAccs)
		for i := range acc {
			acc[i] = k.constant(0)
		}
		k.whileLess("u32", b, count, func() {
			switch dt {
			case tensor.Q8:
				k.dotQ8(pa, row, b, sub, acc)
			case tensor.Q4K:
				k.dotQ4K(pa, row, b, sub, acc)
			default:
				k.dotQ6K(pa, row, b, sub, acc)
			}
			k.e("add.s32", b, b, 32/lanes)
		})
		total := k.sum(k.sum(acc[0], acc[1]), k.sum(acc[2], acc[3]))
		for m := 16; m > 0; m /= 2 {
			total = k.sum(total, k.shflXor(total, m))
		}
		k.e("setp.ne.u32", p, lane, 0)
		k.jumpIf(p, end)
		k.gemvWrite(pc, n, total, over)
		k.put(end)
		return k
	}
}

// half adresteki fp16 ölçeği float32'ye açar (sonlu değerde kesin).
func (k *emitter) half(addr reg, offset int) reg {
	h, v := k.h(), k.f()
	k.e("ld.global.u16", h, mem(addr, offset))
	k.e("cvt.f32.f16", v, h)
	return v
}

// words adresten n adet 16 bitlik sözcük okur: "q8" ve "q6_k" blokları yalnız 2 bayta hizalıdır.
func (k *emitter) words(addr reg, offset, n int) []reg {
	w := make([]reg, n)
	for i := range w {
		w[i] = k.r()
		k.e("ld.global.u16", w[i], mem(addr, offset+2*i))
	}
	return w
}

// byteOf 16 bitlik sözcüklerin i. baytıdır.
func (k *emitter) byteOf(w []reg, i int) reg {
	r := k.r()
	if i%2 == 0 {
		k.e("and.b32", r, w[i/2], 0xFF)
	} else {
		k.e("shr.u32", r, w[i/2], 8)
	}
	return r
}

// activations a[at..at+n)'yi okur; at float sayısıdır, a 16 bayta hizalıdır, n 4'ün katıdır.
func (k *emitter) activations(a reg, at reg, offset, n int) []reg {
	addr := k.address(a, at, 4)
	var out []reg
	for i := 0; i < n; i += 4 {
		out = append(out, k.loadV("global", "", addr, 4*(offset+i), 4)...)
	}
	return out
}

// dotQ8 şerit bloğun 8 öğesini alır: 8·sub'dan başlayan kodlar.
func (k *emitter) dotQ8(pa, row, b, sub reg, acc []reg) {
	block := k.address(row, b, q8BlockBytes)
	d := k.half(block, 0)
	codes := k.words(k.address(block, sub, 8), 2, 4)
	at := k.r()
	k.e("shl.b32", at, b, 5)
	k.e("mad.lo.s32", at, sub, 8, at)
	x := k.activations(pa, at, 0, 8)
	for i := range 8 {
		c := k.r()
		k.e("shl.b32", c, codes[i/2], 24-8*(i%2))
		k.e("shr.s32", c, c, 24)
		y := k.f()
		k.e("mul.rn.f32", y, k.toFloat(c), d)
		k.e("fma.rn.f32", acc[i%quantAccs], x[i], y, acc[i%quantAccs])
	}
}

// dotQ4K şerit 16 baytlık kodu alır: g = sub/2. 64'lüğün alt yarım baytları (alt blok 2g) ve üst
// yarım baytları (alt blok 2g+1), her birinden 16 öğe. Çarpımlar kesin, tek yuvarlama çıkarmadadır:
// fma dequantQ4K'yla aynı biti verir.
func (k *emitter) dotQ4K(pa, row, b, sub reg, acc []reg) {
	block := k.address(row, b, q4kBlockBytes)
	d, minimum := k.half(block, 0), k.half(block, 2)
	s0, s1 := k.r(), k.r()
	k.e("and.b32", s0, sub, 6)
	k.e("or.b32", s1, s0, 1)
	scale := func(s reg) (reg, reg) {
		sc, m := k.q4kScale(block, s)
		ds, dm := k.f(), k.f()
		k.e("mul.rn.f32", ds, d, k.toFloat(sc))
		k.e("mul.rn.f32", dm, minimum, k.toFloat(m))
		k.e("neg.f32", dm, dm)
		return ds, dm
	}
	d1, m1 := scale(s0)
	d2, m2 := scale(s1)
	codes := k.r4(k.address(block, sub, 16), 16)
	// Etkinleştirme: b·256 + (sub/2)·64 + (sub%2)·16; üst yarım baytlar 32 sonrasıdır.
	at, t := k.r(), k.r()
	k.e("shl.b32", at, b, 8)
	k.e("shr.u32", t, sub, 1)
	k.e("mad.lo.s32", at, t, 64, at)
	k.e("and.b32", t, sub, 1)
	k.e("mad.lo.s32", at, t, 16, at)
	lo, hi := k.activations(pa, at, 0, 16), k.activations(pa, at, 32, 16)
	for i := range 16 {
		w := codes[i/4]
		for half, x := range [][]reg{lo, hi} {
			c := k.r()
			k.e("shr.u32", c, w, 8*(i%4)+4*half)
			k.e("and.b32", c, c, 0xF)
			ds, dm := d1, m1
			if half == 1 {
				ds, dm = d2, m2
			}
			y := k.f()
			k.e("fma.rn.f32", y, ds, k.toFloat(c), dm)
			a := acc[(2*i+half)%quantAccs]
			k.e("fma.rn.f32", a, x[i], y, a)
		}
	}
}

// r4 adresteki 16 hizalı 16 baytı dört 32 bitlik sözcük olarak okur.
func (k *emitter) r4(addr reg, offset int) []reg {
	w := []reg{k.r(), k.r(), k.r(), k.r()}
	k.e("ld.global.v4.u32", vlist(w), mem(addr, offset))
	return w
}

// dotQ6K şerit bir 128'liğin dört çeyreğinden 8'er öğe alır: n = sub/4, l = (sub%4)·8.
func (k *emitter) dotQ6K(pa, row, b, sub reg, acc []reg) {
	block := k.address(row, b, q6kBlockBytes)
	d := k.half(block, 208)
	n, l := k.r(), k.r()
	k.e("shr.u32", n, sub, 2)
	k.e("and.b32", l, sub, 3)
	// Ölçek: sc[n·8 + l/16 + 2·çeyrek]; l/16 şerit boyunca sabittir (8 öğe 16'yı geçmez).
	at := k.r()
	k.e("shr.u32", at, l, 1)
	k.e("mad.lo.s32", at, n, 8, at)
	ds := make([]reg, 4)
	for q := range ds {
		ds[q] = k.f()
		k.e("mul.rn.f32", ds[q], d, k.toFloat(k.signedByte(k.byteAt(block, at, 192+2*q))))
	}
	k.e("shl.b32", l, l, 3)
	qlAt, qhAt := k.r(), k.r()
	k.e("mad.lo.s32", qlAt, n, 64, l)
	k.e("mad.lo.s32", qhAt, n, 32, l)
	ql, qh := k.address(block, qlAt, 1), k.address(block, qhAt, 1)
	low, high, top := k.words(ql, 0, 4), k.words(ql, 32, 4), k.words(qh, 128, 4)
	xAt := k.r()
	k.e("shl.b32", xAt, b, 8)
	k.e("mad.lo.s32", xAt, n, 128, xAt)
	k.e("add.s32", xAt, xAt, l)
	x := make([][]reg, 4)
	for q := range x {
		x[q] = k.activations(pa, xAt, 32*q, 8)
	}
	for i := range 8 {
		lo, hi, h := k.byteOf(low, i), k.byteOf(high, i), k.byteOf(top, i)
		for q := range 4 {
			source := lo
			if q%2 == 1 {
				source = hi
			}
			c, t := k.r(), k.r()
			k.e("shr.u32", c, source, 4*(q/2))
			k.e("and.b32", c, c, 0xF)
			k.e("shr.u32", t, h, 2*q)
			k.e("and.b32", t, t, 3)
			k.e("shl.b32", t, t, 4)
			k.e("or.b32", c, c, t)
			k.e("add.s32", c, c, -32)
			y := k.f()
			k.e("mul.rn.f32", y, ds[q], k.toFloat(c))
			k.e("fma.rn.f32", acc[q], x[q][i], y, acc[q])
		}
	}
}
