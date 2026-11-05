package tensor

import (
	le "encoding/binary"
	"hash/fnv"
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// quantTestBlocks altın değerlerin blok sayısıdır.
const quantTestBlocks = 512

// quantBlocks altın değerlerin girdisidir: sabit tohumlu üreteçle bayt doldurulur, ölçeklerin NaN
// ve sonsuz olanı sonluya çekilir. Üreteç llama.cpp tarafındaki C koşumuyla aynıdır.
func quantBlocks(dt DType, seed uint32) []byte { return randomQuant(dt, seed, quantTestBlocks) }

// randomQuant blocks bloğu üretir.
func randomQuant(dt DType, seed uint32, blocks int) []byte {
	_, size := dt.QuantBlock()
	raw := make([]byte, blocks*size)
	state := seed
	for i := range raw {
		state = state*1664525 + 1013904223
		raw[i] = byte(state >> 24)
	}
	fix := func(p []byte) {
		if v := le.LittleEndian.Uint16(p); v&0x7C00 == 0x7C00 {
			le.LittleEndian.PutUint16(p, v&^0x4000)
		}
	}
	for b := range blocks {
		block := raw[b*size : (b+1)*size]
		switch dt {
		case Q8:
			fix(block)
		case Q4K:
			fix(block)
			fix(block[2:])
		case Q6K:
			fix(block[208:])
		}
	}
	return raw
}

// TestDequantMatchesLlamaCpp: açma llama.cpp'nin dequantize_row_*'uyla bit bit aynıdır (15.52). Sayılar
// gerçek C kodundan: ggml-quants.c'nin üç fonksiyonu (cd959bae) değiştirilmeden derlendi, aynı
// baytlar açıldı, float32 bitlerinin FNV-1a özeti ve bir örnek yazıldı. Ölçekler bütün fp16 aralığını
// tarar (alt normaller ve eksiler dahil); "q8"de −128 kodu da geçer.
func TestDequantMatchesLlamaCpp(t *testing.T) {
	for _, c := range []struct {
		dt     DType
		seed   uint32
		hash   uint64
		at     int
		sample uint32
	}{
		{Q8, 1, 0x9994d70c2568f054, 33, 0x3a0c6000},
		{Q4K, 2, 0x9aea8c1a1a939707, 300, 0xc8f6bd40},
		{Q6K, 3, 0xf982bdb0f96cd2fc, 300, 0xc02c0000},
	} {
		t.Run(c.dt.Name(), func(t *testing.T) {
			elems, _ := c.dt.QuantBlock()
			q := FromQuantized(c.dt, quantBlocks(c.dt, c.seed), quantTestBlocks, elems)
			y := q.Open(Float32).Data()
			h := fnv.New64a()
			var word [4]byte
			for _, v := range y {
				le.LittleEndian.PutUint32(word[:], math.Float32bits(v))
				_, _ = h.Write(word[:])
			}
			if got := h.Sum64(); got != c.hash {
				t.Errorf("özet %016x; beklenen %016x", got, c.hash)
			}
			if got := math.Float32bits(y[c.at]); got != c.sample {
				t.Errorf("öğe %d: %08x; beklenen %08x", c.at, got, c.sample)
			}
		})
	}
}

// TestDequantKnownValues elle hesaplanmış bloklar: düzen ve işaretler okunur biçimde sabit.
func TestDequantKnownValues(t *testing.T) {
	const two, half = 0x4000, 0x3800 // fp16 2.0 ve 0.5
	q8 := make([]byte, q8Bytes)
	le.LittleEndian.PutUint16(q8, two)
	q8[2], q8[3], q8[4] = 3, 0xFF, 0x80 // 3, −1, −128
	y := FromQuantized(Q8, q8, 32).Open(Float32).Data()
	if y[0] != 6 || y[1] != -2 || y[2] != -256 || y[3] != 0 {
		t.Errorf("q8: %v", y[:4])
	}

	q4 := make([]byte, q4kBytes)
	le.LittleEndian.PutUint16(q4, two)
	le.LittleEndian.PutUint16(q4[2:], half)
	q4[4], q4[8] = 3, 4 // alt blok 0: a = 3, b = 4
	q4[5], q4[9] = 1, 2 // alt blok 1: a = 1, b = 2
	q4[16] = 0x57       // kod 7 ve üst yarıda 5
	y = FromQuantized(Q4K, q4, 256).Open(Float32).Data()
	if y[0] != 2*3*7-0.5*4 || y[32] != 2*1*5-0.5*2 || y[1] != -2 {
		t.Errorf("q4_k: %v %v %v", y[0], y[32], y[1])
	}

	q6 := make([]byte, q6kBytes)
	le.LittleEndian.PutUint16(q6[208:], two)
	q6[192] = 0xFD              // a = −3
	q6[0], q6[128] = 0x05, 0x02 // kod 5 | (2 << 4) = 37
	y = FromQuantized(Q6K, q6, 256).Open(Float32).Data()
	if y[0] != 2*-3*(37-32) {
		t.Errorf("q6_k: %v", y[0])
	}
}

// TestDequantZeroScaleGivesZero: sıfır ölçek geçerlidir, bloğun değeri 0'dır (15.55).
func TestDequantZeroScaleGivesZero(t *testing.T) {
	for _, dt := range []DType{Q8, Q4K, Q6K} {
		elems, size := dt.QuantBlock()
		raw := make([]byte, size)
		for i := range raw {
			raw[i] = 0x11
		}
		switch dt {
		case Q8:
			raw[0], raw[1] = 0, 0
		case Q4K:
			copy(raw, []byte{0, 0, 0, 0})
		case Q6K:
			raw[208], raw[209] = 0, 0
		}
		for i, v := range FromQuantized(dt, raw, elems).Open(Float32).Data() {
			if v != 0 {
				t.Fatalf("%s: öğe %d = %v", dt.Name(), i, v)
			}
		}
	}
}

func TestCheckQuantScales(t *testing.T) {
	const nan, inf, negInf = 0x7E00, 0x7C00, 0xFC00
	for _, c := range []struct {
		dt DType
		at int
	}{{Q8, 0}, {Q4K, 0}, {Q4K, 2}, {Q6K, 208}} {
		_, size := c.dt.QuantBlock()
		raw := make([]byte, 3*size)
		if got := CheckQuantScales(c.dt, raw); got != -1 {
			t.Errorf("%s: sıfır ölçek %d", c.dt.Name(), got)
		}
		for _, bad := range []uint16{nan, inf, negInf} {
			raw := make([]byte, 3*size)
			le.LittleEndian.PutUint16(raw[size+c.at:], bad)
			if got := CheckQuantScales(c.dt, raw); got != 1 {
				t.Errorf("%s +%d %#x: blok %d", c.dt.Name(), c.at, bad, got)
			}
		}
	}
}

// TestDequantToOtherPrecisions: float64 kesindir, bf16 ve fp16 float32'den bir kez yuvarlanır (15.52).
func TestDequantToOtherPrecisions(t *testing.T) {
	q := FromQuantized(Q4K, quantBlocks(Q4K, 2), quantTestBlocks, 256)
	f32 := q.Open(Float32).Data()
	wide, fault := Convert(q, Float64)
	if fault != nil {
		t.Fatal(fault)
	}
	exact := wide.Float64s()
	for i := range f32 {
		// Float32 sonuç kesin değerin en yakın float32'sidir.
		if float32(exact[i]) != f32[i] {
			t.Fatalf("öğe %d: float64 %v, float32 %v", i, exact[i], f32[i])
		}
	}
	if wide.Shape()[0] != quantTestBlocks {
		t.Errorf("şekil %v", wide.Shape())
	}
	for _, dt := range []DType{BFloat16, Float16} {
		got := q.Open(dt).Data()
		want := NewOf(dt, f32, len(f32)).Data()
		for i := range got {
			if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
				t.Fatalf("%s öğe %d: %v, %v", dt.Name(), i, got[i], want[i])
			}
		}
	}
}

func TestQuantStorageSizes(t *testing.T) {
	for _, c := range []struct {
		dt          DType
		n, bytes    int
		name        string
		elems, size int
	}{
		{Q8, 64, 68, "q8", 32, 34}, {Q4K, 512, 288, "q4_k", 256, 144}, {Q6K, 256, 210, "q6_k", 256, 210},
	} {
		if got := c.dt.StorageBytes(c.n); got != c.bytes {
			t.Errorf("%s: %d bayt", c.name, got)
		}
		if e, s := c.dt.QuantBlock(); e != c.elems || s != c.size || c.dt.Name() != c.name {
			t.Errorf("%s: blok %d %d, ad %s", c.name, e, s, c.dt.Name())
		}
		if !c.dt.IsQuantized() || c.dt.IsFloat() || c.dt.IsTyped() || c.dt.Elem() != ElemFloat {
			t.Errorf("%s: sınıf", c.name)
		}
	}
	if Q4.IsQuantized() {
		t.Error("q4'ün deposu yok")
	}
}

// TestQuantMemoryLimitCountsBlocks: bellek sınırı öğe başına bit değil blok baytı sayar.
func TestQuantMemoryLimitCountsBlocks(t *testing.T) {
	old := allocLimit()
	limitBytes.Store(1 << 20)
	defer limitBytes.Store(old)
	if !sigar(1<<20/144*256, Q4K) {
		t.Error("1 MiB'e sığan q4_k reddedildi")
	}
	if sigar(1<<21/34*32, Q8) {
		t.Error("2 MiB'lik q8 kabul edildi")
	}
	if sigar(math.MaxInt, Q8) {
		t.Error("taşan boy kabul edildi")
	}
}

func TestQuantShapeGate(t *testing.T) {
	expect := func(code messages.Code, f func()) {
		t.Helper()
		defer func() {
			if err, _ := recover().(error); !messages.Has(err, code) {
				t.Errorf("hata %v; %s olmalı", err, code)
			}
		}()
		f()
	}
	expect(messages.H6692, func() { ZerosOf(Q8, 2, 33) })
	expect(messages.H6692, func() { ZerosOf(Q4K, 32) })
	expect(messages.H6692, func() { ZerosOf(Q8) })
	expect(messages.H3035, func() { FromQuantized(Q8, make([]byte, 33), 32) })
}

// TestQuantViews: bitişik tensörde satırlar, devrik görünüm ve kopya; bloklar bölünmez (15.53).
func TestQuantViews(t *testing.T) {
	q := FromQuantized(Q8, quantBlocks(Q8, 1)[:6*q8Bytes], 3, 64)
	all := q.Open(Float32).Data()

	row := q.Slice(0, 1, 2).Squeeze(0)
	if s := row.Shape(); len(s) != 1 || s[0] != 64 {
		t.Fatalf("satır şekli %v", s)
	}
	got := row.Open(Float32).Data()
	for i := range got {
		if got[i] != all[64+i] {
			t.Fatalf("satır öğesi %d: %v", i, got[i])
		}
	}

	if got := q.T().T().Open(Float32).Data(); got[70] != all[70] {
		t.Errorf("iki kez devrik: %v", got[70])
	}
	if got := q.T().OpenAt(5, 2); got != all[2*64+5] {
		t.Errorf("devrikte öğe: %v", got)
	}

	c := q.Slice(0, 1, 3).Clone()
	if c.DType() != Q8 || c.storage == q.storage || c.Open(Float32).Data()[0] != all[64] {
		t.Errorf("kopya: %v", c.DType())
	}

	rows := TakeRows(q, []int{2, 0, 2, 1}, []int{2, 2})
	if s := rows.Shape(); len(s) != 3 || s[0] != 2 || s[1] != 2 || s[2] != 64 || rows.DType() != Q8 {
		t.Fatalf("satır seçimi: %v %v", s, rows.DType())
	}
	picked := rows.Open(Float32).Data()
	for k, r := range []int{2, 0, 2, 1} {
		for i := range 64 {
			if picked[k*64+i] != all[r*64+i] {
				t.Fatalf("seçim %d öğe %d", k, i)
			}
		}
	}
}

// TestTakeRows: ondalıkta türev izlenir, ham bayt türünde satırlar kopyalanır, sınır dışı hata.
func TestTakeRows(t *testing.T) {
	x := New([]float32{1, 2, 3, 4, 5, 6}, 3, 2).SetRequiresGrad(true)
	y := TakeRows(x, []int{2, 0, 2}, []int{3})
	if got := y.Data(); got[0] != 5 || got[1] != 6 || got[2] != 1 || got[4] != 5 {
		t.Errorf("seçim %v", got)
	}
	y.SumAll().Backward()
	if g := x.Grad().Data(); g[0] != 1 || g[2] != 0 || g[4] != 2 {
		t.Errorf("türev %v", g)
	}
	v := New([]float32{7, 8, 9}, 3)
	if got := TakeRows(v, []int{1}, nil); got.Dim() != 0 || got.Item() != 8 {
		t.Errorf("sıfır boyutlu indeks: %v", got.Shape())
	}
	ints := FromRawOn(CPU, Int32, []byte{1, 0, 0, 0, 2, 0, 0, 0}, 2)
	if got := TakeRows(ints, []int{1, 1, 0}, []int{3}).Int64s(); got[0] != 2 || got[2] != 1 {
		t.Errorf("tam sayı %v", got)
	}
	for _, bad := range []int{-1, 3} {
		func() {
			defer func() {
				if err, _ := recover().(error); !messages.Has(err, messages.H6694) {
					t.Errorf("indeks %d: %v", bad, err)
				}
			}()
			TakeRows(v, []int{0, bad}, []int{2})
		}()
	}
}

// TestQuantRefusesEverythingElse: sayılı olmayan her işlem hatadır, sessizce açılmaz (15.53).
func TestQuantRefusesEverythingElse(t *testing.T) {
	q := func() *Tensor { return FromQuantized(Q8, quantBlocks(Q8, 1)[:4*q8Bytes], 2, 64) }
	f := New(make([]float32, 128), 2, 64)
	for _, c := range []struct {
		name string
		code messages.Code
		run  func()
	}{
		{"toplama", messages.H6687, func() { Add(q(), f) }},
		{"q + q", messages.H6687, func() { Add(q(), q()) }},
		{"çarpma", messages.H6687, func() { Mul(f, q()) }},
		{"toplam", messages.H6687, func() { q().SumAll() }},
		{"softmax", messages.H6687, func() { q().Softmax(-1) }},
		{"bitiştirme", messages.H6687, func() { Concat([]*Tensor{q(), q()}, 0) }},
		{"sol işlenen", messages.H6687, func() { MatMul(q(), New(make([]float32, 128), 64, 2)) }},
		{"veri", messages.H6687, func() { q().Data() }},
		{"öğe", messages.H6687, func() { q().At(0, 0) }},
		{"yeni şekil", messages.H6687, func() { q().Reshape(64, 2) }},
		{"yeni eksen", messages.H6687, func() { q().Unsqueeze(0) }},
		{"eksen sırası", messages.H6687, func() { q().Permute(1, 0) }},
		{"son eksende dilim", messages.H6687, func() { q().Slice(1, 0, 32) }},
		{"adımlı dilim", messages.H6687, func() { q().SliceStep(0, 0, 2, 2) }},
		{"tek eksende dilim", messages.H6687, func() { q().Slice(0, 1, 2).Squeeze(0).Slice(0, 0, 32) }},
		{"float32'ye kopya", messages.H6687, func() { f.CopyFrom(q()) }},
		{"q'ya kopya", messages.H6687, func() { q().CopyFrom(f) }},
		{"sessiz açma", messages.H6687, func() { q().Cast(Float32) }},
		{"devrik açılmaz", messages.H6688, func() { q().T().Open(Float32) }},
		{"devrik dilim", messages.H6688, func() { q().T().Slice(0, 0, 1) }},
		{"devrik kopya", messages.H6688, func() { q().T().Clone() }},
		{"devrikten satır", messages.H6688, func() { TakeRows(q().T(), []int{0}, []int{1}) }},
		{"devrik şekil", messages.H6688, func() { q().T().Reshape(128) }},
		{"türev", messages.H6690, func() { q().SetRequiresGrad(true) }},
		{"GPU", messages.H6689, func() { q().To(Device{Kind: KindGPU}) }},
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
