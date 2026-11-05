package gpu

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (15.53): nicemlenmiş tensör kartta nicemli durur; bloklar ham baytlarıyla gider, bit bit
// döner; kartta açılan değer CPU'nunkiyle bit bit aynıdır; sayılı olmayan işlem kartta da hatadır.

var quantTypes = []tensor.DType{tensor.Q8, tensor.Q4K, tensor.Q6K}

// quantBlockBytes rastgele bloklar; ölçekler sonlu, küçük.
func quantBlockBytes(rng *rand.Rand, dt tensor.DType, blocks int) []byte {
	_, size := dt.QuantBlock()
	raw := make([]byte, blocks*size)
	rng.Read(raw)
	for b := range blocks {
		block := raw[b*size : (b+1)*size]
		for _, at := range quantScaleOffsets(dt) {
			// 2⁻¹⁰ ile 2⁻⁴ arası: çarpım sonlu, değerler makul.
			binary.LittleEndian.PutUint16(block[at:], uint16(rng.Intn(6)+5)<<10|uint16(rng.Intn(1024))|uint16(rng.Intn(2))<<15)
		}
	}
	return raw
}

// quantScaleOffsets bloktaki fp16 ölçeklerin yerleridir.
func quantScaleOffsets(dt tensor.DType) []int {
	switch dt {
	case tensor.Q8:
		return []int{0}
	case tensor.Q4K:
		return []int{0, 2}
	}
	return []int{208}
}

// onCard f'yi varsayılan cihaz kartken çalıştırır.
func onCard(t *testing.T, f func()) {
	t.Helper()
	old := tensor.DefaultDevice()
	tensor.SetDefaultDevice(gpuDevice)
	defer tensor.SetDefaultDevice(old)
	f()
}

// sameFloatBits iki float32 dilimini bit bit karşılaştırır.
func sameFloatBits(t *testing.T, name string, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d öğe, beklenen %d", name, len(got), len(want))
	}
	changed, first := 0, -1
	for i := range got {
		if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
			if changed == 0 {
				first = i
			}
			changed++
		}
	}
	if changed > 0 {
		t.Errorf("%s: %d/%d öğe ayrıldı; ilki %d. öğe %#08x, beklenen %#08x", name, changed, len(want), first,
			math.Float32bits(got[first]), math.Float32bits(want[first]))
	}
}

// quantBoth aynı bloklarla CPU'da ve kartta birer tensör kurar.
func quantBoth(t *testing.T, dt tensor.DType, raw []byte, shape ...int) (host, card *tensor.Tensor) {
	t.Helper()
	host = tensor.FromQuantized(dt, bytes.Clone(raw), shape...)
	onCard(t, func() { card = tensor.FromQuantized(dt, bytes.Clone(raw), shape...) })
	return host, card
}

// Taşıma iki yönde de bloklara dokunmaz; kartta kurmak taşımayla aynıdır.
func TestQuantCrossesTheCardWithItsBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, dt := range quantTypes {
		elems, size := dt.QuantBlock()
		raw := quantBlockBytes(rng, dt, 6)
		host, built := quantBoth(t, dt, raw, 3, 2*elems)
		moved := host.To(gpuDevice)
		for name, c := range map[string]*tensor.Tensor{"taşınan": moved, "kurulan": built} {
			if c.DType() != dt || c.Device() != gpuDevice {
				t.Fatalf("%v %s: tür %v, cihaz %v", dt, name, c.DType(), c.Device())
			}
			if back := c.To(tensor.CPU).Raw(); !bytes.Equal(back, raw) {
				t.Errorf("%v %s: bloklar değişti", dt, name)
			}
		}
		// İlk eksende dilim bloğun sınırında başlar; yalnız o satır gider.
		if back := moved.Slice(0, 1, 2).To(tensor.CPU).Raw(); !bytes.Equal(back, raw[2*size:4*size]) {
			t.Errorf("%v: dilim bloklar değişti", dt)
		}
		if back := moved.Clone().To(tensor.CPU).Raw(); !bytes.Equal(back, raw) {
			t.Errorf("%v: kopya bloklar değişti", dt)
		}
		rows := tensor.TakeRows(moved, []int{2, 0}, []int{2}).To(tensor.CPU).Raw()
		if !bytes.Equal(rows, append(bytes.Clone(raw[4*size:6*size]), raw[:2*size]...)) {
			t.Errorf("%v: tensörle indeks bloklar değişti", dt)
		}
	}
}

// scalePatterns sınanacak fp16 desenleridir: kartta 65 536'sı da; benzeticide alt normaller, sonsuzlar
// ve NaN'lar, normallerden seyrek bir örnek (bütünü tek başına dakikalar sürer).
func scalePatterns() []uint16 {
	var out []uint16
	for s := range 1 << 16 {
		exp := s >> 10 & 0x1F
		if !inSim() || s%977 == 0 || (exp == 0 || exp == 0x1F) && s%7 == 0 {
			out = append(out, uint16(s))
		}
	}
	return out
}

// fp16 ölçeğin desenleri: alt normal, sonsuz, NaN dahil. Kodlar da her değerini alır.
func TestQuantOpenMatchesCPUForEveryScale(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	patterns := scalePatterns()
	for _, dt := range quantTypes {
		elems, size := dt.QuantBlock()
		for _, at := range quantScaleOffsets(dt) {
			raw := quantBlockBytes(rng, dt, len(patterns))
			for b, s := range patterns {
				binary.LittleEndian.PutUint16(raw[b*size+at:], s)
			}
			host, card := quantBoth(t, dt, raw, len(patterns), elems)
			sameFloatBits(t, dt.Name(), card.Open(tensor.Float32).Data(), host.Open(tensor.Float32).Data())
		}
	}
}

// q4_k'nın iki ölçeği aynı anda özel olduğunda çıkarmanın NaN kuralı da işler.
func TestQuantOpenQ4KBothScalesSpecial(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	special := []uint16{0, 0x8000, 1, 0x8001, 0x03FF, 0x7BFF, 0x7C00, 0xFC00, 0x7C01, 0xFC01, 0x7E00, 0xFE00, 0x7FFF, 0xFFFF}
	raw := quantBlockBytes(rng, tensor.Q4K, len(special)*len(special))
	_, size := tensor.Q4K.QuantBlock()
	for i, a := range special {
		for j, b := range special {
			block := raw[(i*len(special)+j)*size:]
			binary.LittleEndian.PutUint16(block[0:], a)
			binary.LittleEndian.PutUint16(block[2:], b)
		}
	}
	host, card := quantBoth(t, tensor.Q4K, raw, len(special)*len(special), 256)
	sameFloatBits(t, "q4_k", card.Open(tensor.Float32).Data(), host.Open(tensor.Float32).Data())
}

// Öbür hedefler de CPU'nun bitlerini verir: 16 bitlikte ve float64'te açmak aynı yoldandır.
func TestQuantOpenToOtherPrecisions(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for _, dt := range quantTypes {
		elems, _ := dt.QuantBlock()
		raw := quantBlockBytes(rng, dt, 8)
		host, card := quantBoth(t, dt, raw, 4, 2*elems)
		for _, to := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
			got, want := card.Open(to), host.Open(to)
			if got.Device() != gpuDevice {
				t.Errorf("%v → %v: cihaz %v", dt, to, got.Device())
			}
			sameBits(t, dt.Name(), to, got.Bits16s(), want.Bits16s())
		}
		if got, want := card.Open(tensor.Float64).To(tensor.CPU).Raw(), host.Open(tensor.Float64).Raw(); !bytes.Equal(got, want) {
			t.Errorf("%v → float64: bitler ayrıldı", dt)
		}
	}
}

// Yazdırma yalnız gösterilen bloğu okur; değer CPU'nunkidir.
func TestQuantOpenAtOnTheCard(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for _, dt := range quantTypes {
		elems, _ := dt.QuantBlock()
		raw := quantBlockBytes(rng, dt, 6)
		host, card := quantBoth(t, dt, raw, 3, 2*elems)
		for _, at := range [][]int{{0, 0}, {1, elems + 3}, {2, 2*elems - 1}} {
			if got, want := card.OpenAt(at...), host.OpenAt(at...); math.Float32bits(got) != math.Float32bits(want) {
				t.Errorf("%v %v: %v, beklenen %v", dt, at, got, want)
			}
		}
	}
}

// tensor paketinin TestQuantRefusesEverythingElse tablosunun kartta koşan eşi: nicemli bayt
// ondalık okunmaz, ileti CPU'nunkidir.
func TestQuantRefusesEverythingElseOnTheCard(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	raw := quantBlockBytes(rng, tensor.Q8, 4)
	var q func() *tensor.Tensor
	var f *tensor.Tensor
	onCard(t, func() {
		card := tensor.FromQuantized(tensor.Q8, bytes.Clone(raw), 2, 64)
		q = func() *tensor.Tensor { return card }
		f = tensor.New(make([]float32, 128), 2, 64)
	})
	left := f.Reshape(64, 2)
	for _, c := range []struct {
		name string
		code messages.Code
		run  func()
	}{
		{"toplama", messages.H6687, func() { tensor.Add(q(), f) }},
		{"q + q", messages.H6687, func() { tensor.Add(q(), q()) }},
		{"çarpma", messages.H6687, func() { tensor.Mul(f, q()) }},
		{"toplam", messages.H6687, func() { q().SumAll() }},
		{"softmax", messages.H6687, func() { q().Softmax(-1) }},
		{"bitiştirme", messages.H6687, func() { tensor.Concat([]*tensor.Tensor{q(), q()}, 0) }},
		{"sol işlenen", messages.H6687, func() { tensor.MatMul(q(), left) }},
		{"veri", messages.H6687, func() { q().Data() }},
		{"öğe", messages.H6687, func() { q().At(0, 0) }},
		{"yeni şekil", messages.H6687, func() { q().Reshape(64, 2) }},
		{"yeni eksen", messages.H6687, func() { q().Unsqueeze(0) }},
		{"eksen sırası", messages.H6687, func() { q().Permute(1, 0) }},
		{"son eksende dilim", messages.H6687, func() { q().Slice(1, 0, 32) }},
		{"adımlı dilim", messages.H6687, func() { q().SliceStep(0, 0, 2, 2) }},
		{"float32'ye kopya", messages.H6687, func() { f.CopyFrom(q()) }},
		{"q'ya kopya", messages.H6687, func() { q().CopyFrom(f) }},
		{"sessiz açma", messages.H6687, func() { q().Cast(tensor.Float32) }},
		{"devrik açılmaz", messages.H6688, func() { q().T().Open(tensor.Float32) }},
		{"devrik dilim", messages.H6688, func() { q().T().Slice(0, 0, 1) }},
		{"devrik kopya", messages.H6688, func() { q().T().Clone() }},
		{"devrikten satır", messages.H6688, func() { tensor.TakeRows(q().T(), []int{0}, []int{1}) }},
		{"devrik şekil", messages.H6688, func() { q().T().Reshape(128) }},
		{"devrik taşınmaz", messages.H6688, func() { q().T().To(tensor.CPU) }},
		{"tek satırın devriği açılmaz", messages.H6688, func() { q().Slice(0, 0, 1).T().Open(tensor.Float32) }},
		{"türev", messages.H6690, func() { q().SetRequiresGrad(true) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if err, _ := recover().(error); !messages.Has(err, c.code) {
					t.Errorf("hata %v; %s olmalı", err, c.code)
				}
			}()
			c.run()
		})
	}
}
