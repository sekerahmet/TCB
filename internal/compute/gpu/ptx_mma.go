package gpu

import "fmt"

// ============================================================================
// PTX yazıcısı: tensör çekirdeği (mma.sync) ve warp içi değiş tokuş
// ============================================================================
//
// Bu komutlar sm_80 ister (çekirdeğin hedefi 80 olmalıdır) ve warp düzeyidir: warp'ın 32 iş
// parçacığının hepsi aynı komuta gelmelidir. Parça düzeni (m16n8k16) sim_warp.go'da.

// mmaParts mma.sync.m16n8k16 parçaları: A dört, B iki 32 bitlik (16 bitlik çift) yazmaç, C/D
// dört float32 yazmaç.
type mmaParts struct {
	a [4]reg
	b [2]reg
	c [4]reg
}

// mma D = A·B + C (16 bitlik girdiler f16 ya da bf16, float32 birikim); d ve c aynı yazmaçlar
// olabilir (yerinde biriktirme).
func (k *emitter) mma(d, c [4]reg, a [4]reg, b [2]reg, f16 bool) {
	t := "bf16"
	if f16 {
		t = "f16"
	}
	list := func(r ...reg) string {
		s := "{"
		for i, x := range r {
			if i > 0 {
				s += ", "
			}
			s += string(x)
		}
		return s + "}"
	}
	k.e(fmt.Sprintf("mma.sync.aligned.m16n8k16.row.col.f32.%s.%s.f32", t, t),
		list(d[:]...), list(a[:]...), list(b[:]...), list(c[:]...))
}

// shflXor warp içinde şerit ^ maske şeridinin 32 bitlik değerini verir (v ile aynı türde).
func (k *emitter) shflXor(v reg, mask int) reg {
	r := k.fresh2(regClassOf(v))
	k.e("shfl.sync.bfly.b32", r, v, mask, 31, -1)
	return r
}

// pack2 iki float32'yi en yakına-çifte yuvarlayıp 16 bitlik çift olarak bir 32 bitlik yazmaca
// koyar: alt yarı lo, üst yarı hi.
func (k *emitter) pack2(lo, hi reg, f16 bool) reg {
	r := k.r()
	op := "cvt.rn.bf16x2.f32"
	if f16 {
		op = "cvt.rn.f16x2.f32"
	}
	k.e(op, r, hi, lo)
	return r
}

// regClassOf yazmaç adının türü (yalnızca %f ve %r).
func regClassOf(v reg) int {
	if len(v) > 1 && v[1] == 'f' {
		return yzF
	}
	return yzR
}
