package tensor

import (
	"math"
	"slices"
	"testing"
)

// Birleştirme girdilerin bitlerini korur: işaretli sıfır (-0) +0'a dönmez; 16 bitlik çıktıda
// da (float32 tamponda toplanıp bir kez yuvarlanır).
func TestConcatKeepsSignedZero(t *testing.T) {
	neg := float32(math.Copysign(0, -1))
	for _, dt := range []DType{Float32, BFloat16, Float16} {
		a, b := New([]float32{neg, 1}, 2).Cast(dt), New([]float32{neg, 0}, 2).Cast(dt)
		d := Concat([]*Tensor{a, b}, 0).Data()
		for i, want := range []bool{true, false, true, false} {
			if math.Signbit(float64(d[i])) != want {
				t.Fatalf("%s: %v elemanlarının işaretleri yanlış (%d. eleman)", dt, d, i)
			}
		}
	}
}

// Kural (T1 bf16 bellek, A1): CPU'da aynı 16 bitlik türdeki girdilerin birleştirmesi bitleri kopyalar.
// bf16 ve fp16'nın bütün 65 536 deseni düz ve devrik girdiyle birleşir. Sinyal NaN girdinin kendi
// bitleridir; öbür her desen tampon yolunun (float32'ye açıp birleştirip geri çevirme) bitleridir.
func TestConcatSixteenBitCopiesBits(t *testing.T) {
	for _, c := range []struct {
		dt         DType
		exp, quiet uint16
	}{{BFloat16, 0x7f80, 0x0040}, {Float16, 0x7c00, 0x0200}} {
		bits := make([]uint16, 1<<16)
		for i := range bits {
			bits[i] = uint16(i)
		}
		a := FromBits16On(CPU, c.dt, bits, 256, 256)
		got := Concat([]*Tensor{a, a.T()}, 1).Bits16s()
		old := Concat([]*Tensor{a.Cast(Float32), a.T().Cast(Float32)}, 1).Cast(c.dt).Bits16s()
		mant := ^c.exp & 0x7fff
		signaling := 0
		for i, g := range got {
			r, col := i/512, i%512
			var in uint16
			if col < 256 {
				in = bits[r*256+col]
			} else {
				in = bits[(col-256)*256+r]
			}
			want := old[i]
			if in&c.exp == c.exp && in&mant != 0 && in&c.quiet == 0 {
				want = in
				signaling++
			}
			if g != want {
				t.Fatalf("%s: %#04x deseni %#04x oldu, %#04x bekleniyordu (%d. öğe)", c.dt, in, g, want, i)
			}
		}
		if signaling == 0 {
			t.Fatalf("%s: sinyal NaN deseni sınanmadı", c.dt)
		}
	}
}

// Kural (T1 bf16 bellek, A2): blokta duran tensörün Clone'u ve 16 bitlik birleştirmesi kendi bloğuna
// yazılır, Contiguous öbekte kalır; bütçe dolunca hepsi öbekte sürer. Her yolda bitler öbekteki
// kaynağın sonucuyla aynıdır.
func TestHostBlockCopiesStayOffHeap(t *testing.T) {
	b := needBlock(t, hostSize*4)
	bits := b.Uint16s()
	for i := range bits {
		bits[i] = uint16(i * 7919)
	}
	const rows = 512
	src := FromHostBlock(b, BFloat16, rows, hostSize*2/rows)
	heap := FromBits16On(CPU, BFloat16, slices.Clone(bits), rows, hostSize*2/rows)
	ops := []struct {
		name  string
		block bool
		f     func(*Tensor) *Tensor
	}{
		{"Clone", true, (*Tensor).Clone},
		{"Concat", true, func(x *Tensor) *Tensor { return Concat([]*Tensor{x, x}, 1) }},
		{"Contiguous", false, func(x *Tensor) *Tensor { return x.T().Contiguous() }},
	}
	check := func(t *testing.T, budget bool) {
		for _, o := range ops {
			got := o.f(src)
			s := got.storage.(*cpuStorage)
			if inBlock := s.block != nil && s.block != b; inBlock != (o.block && budget) {
				t.Fatalf("%s: sonuç blokta mı: %v", o.name, inBlock)
			}
			if !slices.Equal(got.Bits16s(), o.f(heap).Bits16s()) {
				t.Fatalf("%s: bitler öbekteki kaynağın sonucundan farklı", o.name)
			}
		}
	}
	t.Run("bütçe var", func(t *testing.T) { check(t, true) })
	t.Run("bütçe dolu", func(t *testing.T) {
		withHostLimits(t, hostMemory.blocks.Load(), hostAddressSpace)
		check(t, false)
	})
}

// Blokta duran ağırlığı döngüde kopyalayan program bütçeyi aşmaz (DENETİM 236): bütçe dolunca kopya
// öbekte sürer, bitler değişmez.
func TestHostBlockCloneLoopStaysInTheBudget(t *testing.T) {
	b := needBlock(t, hostSize*4)
	bits := b.Uint16s()
	for i := range bits {
		bits[i] = uint16(i * 31)
	}
	src := FromHostBlock(b, BFloat16, hostSize*2)
	limit := hostMemory.blocks.Load() + 3
	withHostLimits(t, limit, hostAddressSpace)
	var last *Tensor
	for range 10 {
		last = src.Clone()
		if n := hostMemory.blocks.Load(); n > limit {
			t.Fatalf("harcanan blok %d, bütçe %d", n, limit)
		}
		if !slices.Equal(last.Bits16s(), bits) {
			t.Fatal("kopyanın bitleri farklı")
		}
	}
	if last.storage.(*cpuStorage).block != nil {
		t.Fatal("bütçe dolduktan sonra kopya bloğa gitti")
	}
}

// Fill(-0) işaret bitini korur (float32 ve 16 bitlik depolamada aynı).
func TestFillNegativeZero(t *testing.T) {
	for _, dt := range []DType{Float32, BFloat16} {
		s := defaultBackend().Alloc(3, dt)
		defaultBackend().Fill(s, float32(math.Copysign(0, -1)))
		for _, v := range fromStorage(defaultBackend(), s, []int{3}).Data() {
			if !math.Signbit(float64(v)) {
				t.Fatalf("%s: Fill(-0) işareti kaybetti", dt)
			}
		}
	}
}

// ============================================================================
// Gradyan kapısı: birleştirme
// ============================================================================

func init() {
	registerGrad("concat",
		gradSpec{name: "concat eksen 1", ops: []string{"concat"},
			build:  func(xs []*Tensor) *Tensor { return Concat(xs, 1) },
			inputs: genInputs(gen(-1, 1, 0, 2, 3), gen(-1, 1, 0, 2, 2), gen(-1, 1, 0, 2, 1)),
			ref: func(in [][]float64) []float64 {
				var res []float64
				for r := 0; r < 2; r++ {
					res = append(res, in[0][r*3:r*3+3]...)
					res = append(res, in[1][r*2:r*2+2]...)
					res = append(res, in[2][r:r+1]...)
				}
				return res
			}},
		gradSpec{name: "concat eksen 0", ops: []string{"concat"},
			build:  func(xs []*Tensor) *Tensor { return Concat(xs, 0) },
			inputs: genInputs(gen(-1, 1, 0, 1, 3), gen(-1, 1, 0, 2, 3)),
			ref: func(in [][]float64) []float64 {
				return append(append([]float64(nil), in[0]...), in[1]...)
			}})
}

func TestGradConcat(t *testing.T) { runGradGroup(t, "concat") }
