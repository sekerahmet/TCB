package gpu

import (
	"encoding/binary"
	"slices"
	"testing"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (15.47a "bit bit aynıdır"): 16 bitlik tensörün bütün bit desenleri karta gider, kartta
// adımlı kopyalanır ve karttan gelir; float32'den geçmez, sinyal NaN'ı susmaz. CPU'daki eşi
// TestSixteenBitStridedCopyKeepsBits. Neden (gerçek kart, DENETİM istek 134): kartta yükleyip
// yazınca bf16'da 254 NaN deseninin 126'sı, fp16'da 2 046'nın 2 045'i değişiyordu.

var sixteenBitTypes = []tensor.DType{tensor.BFloat16, tensor.Float16}

// everyBits16 bütün 16 bitlik desenler, sırayla (256×256'lık kare de olur).
func everyBits16() []uint16 {
	bits := make([]uint16, 1<<16)
	for i := range bits {
		bits[i] = uint16(i)
	}
	return bits
}

// bitBytes desenlerin küçük uçlu baytları.
func bitBytes(bits []uint16) []byte {
	raw := make([]byte, 0, 2*len(bits))
	for _, h := range bits {
		raw = binary.LittleEndian.AppendUint16(raw, h)
	}
	return raw
}

// sameBits got'la want'ı karşılaştırır, ayrılan desen sayısını ve ilkini söyler.
func sameBits(t *testing.T, name string, dt tensor.DType, got, want []uint16) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%v %s: %d öğe, beklenen %d", dt, name, len(got), len(want))
	}
	changed, first := 0, -1
	for i := range got {
		if got[i] != want[i] {
			if changed == 0 {
				first = i
			}
			changed++
		}
	}
	if changed > 0 {
		t.Errorf("%v %s: %d/%d desen değişti; ilki %d. öğe %#04x, beklenen %#04x", dt, name, changed, len(want), first, got[first], want[first])
	}
}

// bytesBits küçük uçlu baytların desenleri.
func bytesBits(raw []byte) []uint16 {
	bits := make([]uint16, len(raw)/2)
	for i := range bits {
		bits[i] = binary.LittleEndian.Uint16(raw[2*i:])
	}
	return bits
}

// Yükleme ve indirme ayrı ölçülür: biri öbürünün hatasını örtmesin. Yükleme ToRaw'dan geçmeden
// cihazın baytlarıyla, indirme de FromRaw'dan geçmeden doğrudan yazılmış baytlarla sınanır.
func TestSixteenBitUploadAndDownloadKeepBits(t *testing.T) {
	g := gpuBackend()
	bits := everyBits16()
	raw := bitBytes(bits)
	for _, dt := range sixteenBitTypes {
		s := g.FromRaw(raw, dt).(*storage)
		onCard := make([]byte, len(raw))
		g.enter()
		g.download(unsafe.Pointer(&onCard[0]), s.addr, len(onCard))
		g.exit()
		sameBits(t, "yükleme", dt, bytesBits(onCard), bits)
		g.Release(s)

		g.enter()
		d := g.allocate(len(bits), dt, false)
		g.loadHost(d, unsafe.Pointer(&raw[0]), len(raw))
		g.exit()
		sameBits(t, "indirme", dt, bytesBits(g.ToRaw(flatView(d, len(bits)))), bits)
		g.Release(d)
	}
}

// Tensör yolları: FromBits16On ve Bits16s, .TR_to_device'ın iki yönü.
func TestSixteenBitTensorCrossesTheCardWithItsBits(t *testing.T) {
	bits := everyBits16()
	for _, dt := range sixteenBitTypes {
		card := tensor.FromBits16On(gpuDevice, dt, slices.Clone(bits), 256, 256)
		if card.DType() != dt {
			t.Fatalf("%v: kartta tür %v", dt, card.DType())
		}
		sameBits(t, "FromBits16On", dt, card.Bits16s(), bits)
		sameBits(t, "kartta -> CPU", dt, card.To(tensor.CPU).Bits16s(), bits)
		host := tensor.FromBits16On(tensor.CPU, dt, slices.Clone(bits), 256, 256)
		moved := host.To(gpuDevice)
		if moved.DType() != dt {
			t.Fatalf("%v: taşınınca tür %v", dt, moved.DType())
		}
		sameBits(t, "CPU -> kartta", dt, moved.Bits16s(), bits)
		sameBits(t, "gidiş dönüş", dt, moved.To(tensor.CPU).Bits16s(), bits)
	}
}

// Kartta aynı türde adımlı kopya bitleri taşır: dilim, devrik ve onları kopyalayan yollar CPU'yla
// aynı deseni verir (CPU'nun aynı işlemi bitleri koruduğunu TestSixteenBitStridedCopyKeepsBits sınar).
func TestSixteenBitViewsOnTheCardKeepBits(t *testing.T) {
	into := func(dt tensor.DType, x *tensor.Tensor) *tensor.Tensor {
		return tensor.ZerosOf(dt, 256, 256).To(x.Device())
	}
	for _, dt := range sixteenBitTypes {
		bits := everyBits16()
		host := tensor.FromBits16On(tensor.CPU, dt, slices.Clone(bits), 256, 256)
		card := tensor.FromBits16On(gpuDevice, dt, slices.Clone(bits), 256, 256)
		for _, c := range []struct {
			name string
			of   func(x *tensor.Tensor) *tensor.Tensor
		}{
			{"devrik", func(x *tensor.Tensor) *tensor.Tensor { return x.T() }},
			{"satır dilimi", func(x *tensor.Tensor) *tensor.Tensor { return x.Slice(0, 1, 255) }},
			{"sütun dilimi", func(x *tensor.Tensor) *tensor.Tensor { return x.Slice(1, 3, 200) }},
			{"devrik kopya", func(x *tensor.Tensor) *tensor.Tensor { return x.T().Clone() }},
			{"bitişik devrik", func(x *tensor.Tensor) *tensor.Tensor { return x.T().Contiguous() }},
			{"dilim kopyası", func(x *tensor.Tensor) *tensor.Tensor { return x.Slice(1, 3, 200).Clone() }},
			{"CopyFrom", func(x *tensor.Tensor) *tensor.Tensor {
				y := into(dt, x)
				y.CopyFrom(x.T())
				return y
			}},
			{"görünüme CopyFrom", func(x *tensor.Tensor) *tensor.Tensor {
				y := into(dt, x)
				y.T().CopyFrom(x)
				return y
			}},
		} {
			sameBits(t, c.name, dt, c.of(card).Bits16s(), c.of(host).Bits16s())
			sameBits(t, c.name+" -> CPU", dt, c.of(card).To(tensor.CPU).Bits16s(), c.of(host).Bits16s())
		}
	}
}

// Ham yol türevi koparmaz: 16 bitlik tensör taşınınca gradyan iki yönde de kaynak cihaza gelir.
// Değerler iki türde de tam: 2·w yuvarlanmaz.
func TestSixteenBitToDeviceKeepsGradient(t *testing.T) {
	values := []float32{1, 2, -3, 0.5}
	want := []float32{2, 4, -6, 1}
	for _, dt := range sixteenBitTypes {
		for _, c := range []struct {
			name     string
			from, to tensor.Device
		}{{"CPU -> kartta", tensor.CPU, gpuDevice}, {"kartta -> CPU", gpuDevice, tensor.CPU}} {
			w := tensor.NewOf(dt, values, 4).To(c.from).SetRequiresGrad(true)
			y := w.To(c.to)
			y.Mul(y).SumAll().Backward()
			if w.Grad() == nil {
				t.Fatalf("%v %s: gradyan gelmedi", dt, c.name)
			}
			if w.Grad().Device() != c.from {
				t.Errorf("%v %s: gradyan %v cihazında", dt, c.name, w.Grad().Device())
			}
			if got := w.Grad().Data(); !slices.Equal(got, want) {
				t.Errorf("%v %s: gradyan %v, beklenen %v", dt, c.name, got, want)
			}
		}
	}
}
