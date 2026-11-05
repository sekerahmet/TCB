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

// quantRowCounts çarpımın sınanan satır sayılarıdır.
var quantRowCounts = []int{1}

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

// quantWeights rastgele ağırlık ve aynı ağırlığın iki cihazdaki hali; K elems'in katıdır.
func quantWeights(t *testing.T, rng *rand.Rand, dt tensor.DType, N, K int) (host, card *tensor.Tensor) {
	t.Helper()
	elems, _ := dt.QuantBlock()
	return quantBoth(t, dt, quantBlockBytes(rng, dt, N*K/elems), N, K)
}

// Tek satırlık çarpım açılmış hesapla aynıdır (15.41'in toleransıyla): aynı açılmış ağırlık, aynı
// etkinleştirme, CPU'nun float32 çarpımı. N warp sınırını, K tek blok ve şeritleri aşmayan boyu da
// geçer.
func TestQuantGemvMatchesOpened(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for _, dt := range quantTypes {
		elems, _ := dt.QuantBlock()
		for _, c := range [][2]int{{1, elems}, {7, elems}, {9, 3 * elems}, {16, 5 * elems}, {33, 11 * elems}} {
			N, K := c[0], c[1]
			host, card := quantWeights(t, rng, dt, N, K)
			x := twin(random(rng, K, 1), false, 1, K)
			want := tensor.MatMul(x[0], host.Open(tensor.Float32).T()).Data()
			got := tensor.MatMul(x[1], card.T())
			if got.Device() != gpuDevice {
				t.Fatalf("%v: cihaz %v", dt, got.Device())
			}
			checkDiff(t, dt.Name(), got.Data(), want, 1e-5*math.Sqrt(float64(K)))
			// CPU'nun kendi çekirdeğiyle de aynı sınırda.
			checkDiff(t, dt.Name()+" CPU", got.Data(), tensor.MatMul(x[0], host.T()).Data(), 1e-5*math.Sqrt(float64(K)))
		}
	}
}

// 15.22a: aynı kartta aynı program aynı biti verir; atomik toplama yoktur.
func TestQuantGemvIsDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	for _, dt := range quantTypes {
		elems, _ := dt.QuantBlock()
		K := 4 * elems
		_, card := quantWeights(t, rng, dt, 40, K)
		x := twin(random(rng, K, 1), false, 1, K)[1]
		first := tensor.MatMul(x, card.T()).Data()
		for range 3 {
			sameFloatBits(t, dt.Name(), tensor.MatMul(x, card.T()).Data(), first)
		}
	}
}

// Sonuç x'in hassasiyetindedir (15.53); öndeki eksenler satıra katlanır; dilim ağırlık blok sınırından
// okunur; ofsetli görünüm hizasız olabilir.
func TestQuantGemvShapesAndPrecisions(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	for _, dt := range quantTypes {
		elems, _ := dt.QuantBlock()
		N, K := 6, 2*elems
		host, card := quantWeights(t, rng, dt, N, K)
		tol := 1e-5 * math.Sqrt(float64(K))
		data := random(rng, K+1, 1)
		x := twin(data[:K], false, 1, K)
		for _, p := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
			got := tensor.MatMul(x[1].Cast(p), card.T())
			if got.DType() != p {
				t.Fatalf("%v: sonuç %v, x %v", dt, got.DType(), p)
			}
			checkDiff(t, dt.Name()+" 16 bit", got.Cast(tensor.Float32).Data(), tensor.MatMul(x[0].Cast(p), host.T()).Cast(tensor.Float32).Data(), 1e-2)
		}
		folded := tensor.MatMul(x[1].Reshape(1, 1, K), card.T())
		if s := folded.Shape(); len(s) != 3 || s[2] != N {
			t.Fatalf("%v: şekil %v", dt, s)
		}
		checkDiff(t, dt.Name()+" katlanmış", folded.Data(), tensor.MatMul(x[0], host.T()).Data(), tol)
		rows := tensor.MatMul(x[1], card.Slice(0, 2, 5).T())
		checkDiff(t, dt.Name()+" dilim", rows.Data(), tensor.MatMul(x[0], host.Slice(0, 2, 5).T()).Data(), tol)
		shifted := twin(data, false, K+1)
		got := tensor.MatMul(shifted[1].Slice(0, 1, K+1).Reshape(1, K), card.T())
		checkDiff(t, dt.Name()+" hizasız", got.Data(), tensor.MatMul(shifted[0].Slice(0, 1, K+1).Reshape(1, K), host.T()).Data(), tol)
	}
}

// Gemm c'ye ekler, GemmSet üzerine yazar: iki yol da açılmış hesapla aynıdır.
func TestQuantGemmAccumulates(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	g := gpuBackend()
	for _, dt := range quantTypes {
		elems, _ := dt.QuantBlock()
		for _, M := range quantRowCounts {
			N, K := 5, 2*elems
			raw := quantBlockBytes(rng, dt, N*K/elems)
			xs, base := random(rng, M*K, 1), random(rng, M*N, 1)
			as, ws, cs := g.FromHost(xs, tensor.Float32), g.FromRaw(raw, dt), g.FromHost(base, tensor.Float32)
			w := tensor.View{S: ws, Shape: []int{N, K}, Strides: []int{K, 1}}
			a := tensor.View{S: as, Shape: []int{M, K}, Strides: []int{K, 1}}
			g.Gemm(1, M, N, K, a, false, w, true, cs)
			host := tensor.FromQuantized(dt, raw, N, K)
			want := tensor.MatMul(tensor.NewOn(tensor.CPU, xs, M, K), host.Open(tensor.Float32).T()).Data()
			for i := range want {
				want[i] += base[i]
			}
			checkDiff(t, dt.Name(), fromDevice(g, cs), want, 1e-5*math.Sqrt(float64(K)))
			for _, s := range []tensor.Storage{as, ws, cs} {
				g.Release(s)
			}
		}
	}
}

// Gömme seçilen satırları kartta açar; değer CPU'nunkiyle bit bit aynıdır (15.53).
func TestQuantEmbeddingOnTheCard(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	for _, dt := range quantTypes {
		elems, _ := dt.QuantBlock()
		host, card := quantWeights(t, rng, dt, 6, 2*elems)
		idx := []int{5, 0, 3, 3, 1, 2}
		want := tensor.Embedding(host, idx, 2, 3)
		got := tensor.Embedding(card, idx, 2, 3)
		if got.DType() != tensor.Float32 || got.Device() != gpuDevice || got.Dim() != 3 {
			t.Fatalf("%v: tür %v, cihaz %v, şekil %v", dt, got.DType(), got.Device(), got.Shape())
		}
		sameFloatBits(t, dt.Name(), got.Data(), want.Data())
		if rows := tensor.Embedding(card.Slice(0, 1, 4), []int{2}); rows.Numel() != 2*elems {
			t.Fatalf("%v: dilim %v", dt, rows.Shape())
		} else {
			sameFloatBits(t, dt.Name()+" dilim", rows.Data(), tensor.Embedding(host, []int{3}).Data())
		}
		for _, it := range []*tensor.Tensor{tensor.NewOn(gpuDevice, []float32{5, 0}, 2), tensor.Int64sOn(gpuDevice, tensor.Int64, []int64{5, 0}, 2)} {
			at := tensor.EmbeddingAt(card, it)
			sameFloatBits(t, dt.Name()+" cihaz indeksi", at.Data(), tensor.Embedding(host, []int{5, 0}).Data())
		}
		if n := tensor.Embedding(card, nil, 0).Numel(); n != 0 {
			t.Errorf("%v: boş indeks %d öğe", dt, n)
		}
		func() {
			defer func() {
				if err, _ := recover().(error); !messages.Has(err, messages.H3043) {
					t.Errorf("%v: hata %v; H3043 olmalı", dt, err)
				}
			}()
			tensor.Embedding(card, []int{6})
		}()
	}
}

// Boş toplam float çarpımıyla aynıdır; türev izleyen girdi kartta da H6696'dır.
func TestQuantGemvEdges(t *testing.T) {
	var empty *tensor.Tensor
	onCard(t, func() { empty = tensor.FromQuantized(tensor.Q8, nil, 3, 0) })
	x := tensor.NewOn(gpuDevice, nil, 1, 0)
	if got := tensor.MatMul(x, empty.T()).Data(); len(got) != 3 || got[0] != 0 || got[2] != 0 {
		t.Errorf("boş toplam: %v", got)
	}
	rng := rand.New(rand.NewSource(10))
	_, card := quantWeights(t, rng, tensor.Q8, 2, 32)
	func() {
		defer func() {
			if err, _ := recover().(error); !messages.Has(err, messages.H6696) {
				t.Errorf("hata %v; H6696 olmalı", err)
			}
		}()
		tensor.MatMul(tensor.NewOn(gpuDevice, make([]float32, 32), 1, 32).SetRequiresGrad(true), card.T())
	}()
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
