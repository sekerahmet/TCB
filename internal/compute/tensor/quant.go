package tensor

import (
	"bytes"
	le "encoding/binary"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Nicemlenmiş saklama (15.51–15.55): "q8", "q4_k", "q6_k"
// ============================================================================
//
// Depolama dosyanın bloklarıdır: cpuStorage.raw GGUF'un bayt düzenini olduğu gibi tutar, n öğe
// sayısıdır. Blok son eksendedir ve son eksenin uzunluğu bloğun katıdır: bir satır tam blok
// sayısıdır. Görünümün Offset'i öğe cinsindendir, satır başına düşer. Hesap float32'dedir:
// açılan değer llama.cpp'nin dequantize_row_*'uyla bit bit aynıdır (llama.cpp cd959bae,
// ggml-quants.c:556, :1539, :1960).

// Blok düzenleri (ggml-common.h): öğe sayısı ve bayt.
const (
	q8Block, q8Bytes = 32, 34
	kBlock           = 256
	q4kBytes         = 144
	q6kBytes         = 210
)

// IsQuantized blok blok saklanan nicemlenmiş tür mü. "q4" ayrılmış addır, deposu yoktur.
func (d DType) IsQuantized() bool { return d == Q8 || d == Q4K || d == Q6K }

// QuantBlock bloğun öğe sayısı ve bayt boyudur; nicemlenmiş olmayan türde 0, 0.
func (d DType) QuantBlock() (elems, size int) {
	switch d {
	case Q8:
		return q8Block, q8Bytes
	case Q4K:
		return kBlock, q4kBytes
	case Q6K:
		return kBlock, q6kBytes
	}
	return 0, 0
}

// checkQuantShape kurulacak nicemlenmiş tensörün cihazını ve son eksenini denetler (15.51).
func checkQuantShape(be Backend, dt DType, shape []int) {
	if be.Device().Kind != KindCPU {
		panic(messages.New(messages.H6689, dt.Name(), be.Device()))
	}
	elems, _ := dt.QuantBlock()
	if len(shape) == 0 {
		panic(messages.New(messages.H6692, dt.Name(), 0, elems))
	}
	if last := shape[len(shape)-1]; last%elems != 0 {
		panic(messages.New(messages.H6692, dt.Name(), last, elems))
	}
}

// FromQuantized dosyanın bloklarından tensör kurar; raw tensörün deposu olur (çağıran bırakır).
// Son eksen bloğun katı olmalıdır; ölçekler CheckQuantScales'tan geçmiş olmalıdır.
func FromQuantized(dt DType, raw []byte, shape ...int) *Tensor {
	be := defaultBackend()
	checkQuantShape(be, dt, shape)
	n := allocSizeOn(be, isCreate, shape, dt)
	elems, size := dt.QuantBlock()
	if len(raw) != n/elems*size {
		panic(messages.New(messages.H3035, len(raw)/size*elems, shape))
	}
	return fromStorage(be, &cpuStorage{dtyp: dt, raw: raw, n: n}, shape)
}

// CheckQuantScales ilk NaN ya da sonsuz ölçekli bloğun indeksini verir, yoksa -1 (15.55).
func CheckQuantScales(dt DType, raw []byte) int {
	_, size := dt.QuantBlock()
	for b := 0; b*size < len(raw); b++ {
		block := raw[b*size : (b+1)*size]
		switch dt {
		case Q8:
			if badScale(block[0:]) {
				return b
			}
		case Q4K:
			if badScale(block[0:]) || badScale(block[2:]) {
				return b
			}
		case Q6K:
			if badScale(block[208:]) {
				return b
			}
		}
	}
	return -1
}

// badScale fp16 ölçek NaN ya da sonsuz mu (üs bitlerinin hepsi 1).
func badScale(p []byte) bool {
	return le.LittleEndian.Uint16(p)&0x7C00 == 0x7C00
}

// fp16 küçük uçlu fp16 ölçeği float32'ye açar.
func fp16(p []byte) float32 { return FromBits16(Float16, le.LittleEndian.Uint16(p)) }

// dequantQ8 bir bloğu açar: y = d · kod; çarpım float32'de kesindir.
func dequantQ8(y []float32, b []byte) {
	d := fp16(b)
	for j := range q8Block {
		y[j] = float32(int8(b[2+j])) * d
	}
}

// q4kScale j. alt bloğun 6 bitlik ölçeği ve minimumudur (get_scale_min_k4).
func q4kScale(j int, q []byte) (sc, m byte) {
	if j < 4 {
		return q[j] & 63, q[j+4] & 63
	}
	return q[j+4]&0xF | (q[j-4]>>6)<<4, q[j+4]>>4 | (q[j]>>6)<<4
}

// dequantQ4K bir bloğu açar: y = (d·a)·kod − (m·b). Çarpımlar float32'de kesindir, tek yuvarlama
// çıkarmadadır: kaynaştırılmış çıkarma da aynı biti verir.
func dequantQ4K(y []float32, b []byte) {
	d, minimum := fp16(b), fp16(b[2:])
	scales, q := b[4:16], b[16:]
	for g := range 4 {
		sc, m := q4kScale(2*g, scales)
		d1, m1 := d*float32(sc), minimum*float32(m)
		sc, m = q4kScale(2*g+1, scales)
		d2, m2 := d*float32(sc), minimum*float32(m)
		row := y[g*64 : g*64+64]
		for l := range 32 {
			row[l] = float32(d1*float32(q[l]&0xF)) - m1
			row[l+32] = float32(d2*float32(q[l]>>4)) - m2
		}
		q = q[32:]
	}
}

// dequantQ6K bir bloğu açar: y = (d·a)·(kod − 32); çarpımlar float32'de kesindir.
func dequantQ6K(y []float32, b []byte) {
	ql, qh, sc := b[0:128], b[128:192], b[192:208]
	d := fp16(b[208:])
	for n := range 2 {
		row := y[n*128 : n*128+128]
		for l := range 32 {
			is := l / 16
			h := qh[l]
			q1 := int(ql[l]&0xF|(h&3)<<4) - 32
			q2 := int(ql[l+32]&0xF|((h>>2)&3)<<4) - 32
			q3 := int(ql[l]>>4|((h>>4)&3)<<4) - 32
			q4 := int(ql[l+32]>>4|((h>>6)&3)<<4) - 32
			row[l] = d * float32(int8(sc[is])) * float32(q1)
			row[l+32] = d * float32(int8(sc[is+2])) * float32(q2)
			row[l+64] = d * float32(int8(sc[is+4])) * float32(q3)
			row[l+96] = d * float32(int8(sc[is+6])) * float32(q4)
		}
		ql, qh, sc = ql[64:], qh[32:], sc[8:]
	}
}

// dequantBlocks raw'daki tam blokları y'ye açar.
func dequantBlocks(dt DType, y []float32, raw []byte) {
	elems, size := dt.QuantBlock()
	open := dequantQ8
	switch dt {
	case Q4K:
		open = dequantQ4K
	case Q6K:
		open = dequantQ6K
	}
	for i, o := 0, 0; o < len(raw); i, o = i+elems, o+size {
		open(y[i:i+elems], raw[o:o+size])
	}
}

// quantBytes görünümün öğe aralığının depodaki baytlarıdır; aralık blok sınırındadır.
func quantBytes(s *cpuStorage, offset, n int) []byte {
	elems, size := s.dtyp.QuantBlock()
	return s.raw[offset/elems*size : (offset+n)/elems*size]
}

// checkNotQuantized öğe öğe kopyanın kapısıdır: nicemlenmiş depoda öğe yoktur, blok vardır.
func checkNotQuantized(types ...DType) {
	for _, dt := range types {
		if dt.IsQuantized() {
			panic(typedComputeError(dt))
		}
	}
}

// CheckQuantWeight x @ w'nin nicemlenmiş sağ işlenenini denetler (15.53): w [N, K] deposunun
// devriğidir; geri yön yoktur, x türev izliyorsa çarpım hatadır.
func CheckQuantWeight(x, w *Tensor) {
	if len(w.shape) != 2 || w.strides[0] != 1 {
		panic(messages.New(messages.H6695, w.dtyp.Name()))
	}
	if GradEnabled() && x.RequiresGrad() {
		panic(messages.New(messages.H6696, w.dtyp.Name()))
	}
}

// refuseTransposed nicemlenmiş devrik görünümün okuma dışı işlemlerini durdurur (15.53).
func (t *Tensor) refuseTransposed() {
	if !t.IsContiguous() {
		panic(messages.New(messages.H6688, t.dtyp.Name()))
	}
}

// refuseQuantView sayılı olmayan görünümü durdurur (15.53): .TR_reshape, .TR_permute ve eksen
// ekleyip silenler nicemlenmiş tensörde yoktur.
func (t *Tensor) refuseQuantView() {
	if t.dtyp.IsQuantized() {
		t.refuseTransposed()
		panic(typedComputeError(t.dtyp))
	}
}

// quantRowSlice nicemlenmiş dilimin kapısıdır: bloklar bölünmez, yalnız ilk eksende ve adımsız.
func (t *Tensor) quantRowSlice(axis, step int) {
	t.refuseTransposed()
	if axis != 0 || step != 1 || len(t.shape) < 2 {
		panic(typedComputeError(t.dtyp))
	}
}

// Open nicemlenmiş tensörü açar: .TR_to_dtype'ın tek yoludur. Cast açmaz: hesabın iç dönüşümleri
// (TR_mixed_precision bloğu) ağırlığı sessizce açmasın (15.53).
func (t *Tensor) Open(dt DType) *Tensor {
	if dt == Float64 {
		return t.dequantize64()
	}
	return t.dequantize(dt)
}

// dequantize tensörü dt hassasiyetine açar (15.52): bitişik görünüm blok blok float32'ye açılır, bf16
// ve fp16 sonra hedefe yuvarlanır. Devrik görünüm açılmaz, ileti devriği geri almayı söyler
// (15.53). Nicemlenmiş tensör türev izlemez: sonuç yapraktır.
func (t *Tensor) dequantize(dt DType) *Tensor {
	t.refuseTransposed()
	s := cpuStore(t.storage)
	out := emptyOf(isCast, t.be, t.shape, Float32)
	elems, size := s.dtyp.QuantBlock()
	blocks := quantBytes(s, t.offset, t.Numel())
	y := cd(out.storage)
	parallelFor(len(blocks)/size, elems*4, func(lo, hi int) {
		dequantBlocks(s.dtyp, y[lo*elems:hi*elems], blocks[lo*size:hi*size])
	})
	if dt == Float32 {
		return out
	}
	return out.Cast(dt)
}

// dequantize64 değerleri float64'te açar (15.52: float64'te kesindir). Kodlar ve ölçekler float32'de
// zaten kesin çarpılır; "q4_k"nin çıkarması burada yuvarlanmaz.
func (t *Tensor) dequantize64() *Tensor {
	t.refuseTransposed()
	s := cpuStore(t.storage)
	out := emptyOf(isCast, t.be, t.shape, Float64)
	y := cpuStore(out.storage).raw
	elems, size := s.dtyp.QuantBlock()
	blocks := quantBytes(s, t.offset, t.Numel())
	parallelFor(len(blocks)/size, elems*8, func(lo, hi int) {
		row, wide := make([]float32, elems), make([]float64, elems)
		for b := lo; b < hi; b++ {
			block := blocks[b*size : (b+1)*size]
			if s.dtyp == Q4K {
				dequantQ4K64(wide, block)
			} else {
				dequantBlocks(s.dtyp, row, block)
				for j, v := range row {
					wide[j] = float64(v)
				}
			}
			for j, v := range wide {
				PutRawFloat64(y[(b*elems+j)*8:], v)
			}
		}
	})
	return out
}

// dequantQ4K64 dequantQ4K'nın float64 eşidir.
func dequantQ4K64(y []float64, b []byte) {
	d, minimum := float64(fp16(b)), float64(fp16(b[2:]))
	scales, q := b[4:16], b[16:]
	for g := range 4 {
		for half := range 2 {
			sc, m := q4kScale(2*g+half, scales)
			for l := range 32 {
				code := q[l] & 0xF
				if half == 1 {
					code = q[l] >> 4
				}
				y[g*64+half*32+l] = d*float64(sc)*float64(code) - minimum*float64(m)
			}
		}
		q = q[32:]
	}
}

// OpenAt idx'teki öğenin açılmış değeridir; yalnız o öğenin bloğu açılır (yazdırma, 15.53: büyük
// ağırlığın bütünü açılmaz). Devrik görünümü de okur: adımlar öğeyi depoda bulur.
func (t *Tensor) OpenAt(idx ...int) float32 {
	s := cpuStore(t.storage)
	at := t.offset
	for i, x := range idx {
		at += x * t.strides[i]
	}
	elems, size := s.dtyp.QuantBlock()
	var block [kBlock]float32
	b := at / elems
	dequantBlocks(s.dtyp, block[:elems], s.raw[b*size:(b+1)*size])
	return block[at%elems]
}

// quantCopy aynı hassasiyette yeni bir depodur (15.54): kodlar ve ölçekler kopyalanır.
func (t *Tensor) quantCopy(is opKind) *Tensor {
	t.refuseTransposed()
	allocSizeOn(t.be, is, t.shape, t.dtyp)
	s := cpuStore(t.storage)
	raw := bytes.Clone(quantBytes(s, t.offset, t.Numel()))
	return fromStorage(t.be, &cpuStorage{dtyp: t.dtyp, raw: raw, n: t.Numel()}, t.shape)
}

// quantRows ilk eksenin idx satırlarını sırayla kopyalar (15.53, tensörle indeks): bloklar bölünmez,
// kodlar ve ölçekler değişmez. Sınır dışı indeks çağıranın denetimindedir.
func (t *Tensor) quantRows(idx, shape []int) *Tensor {
	t.quantRowSlice(0, 1)
	s := cpuStore(t.storage)
	elems, size := s.dtyp.QuantBlock()
	rowBytes := t.Numel() / max(t.shape[0], 1) / elems * size
	n := allocSizeOn(t.be, isCopy, shape, t.dtyp)
	out := newQuantStorage(s.dtyp, n)
	base := t.offset / elems * size
	for i, r := range idx {
		copy(out.raw[i*rowBytes:(i+1)*rowBytes], s.raw[base+r*rowBytes:base+(r+1)*rowBytes])
	}
	return fromStorage(t.be, out, shape)
}

// newQuantStorage n öğelik sıfırlı depodur: bütün ölçekler ve kodlar 0, değeri 0 ("q6_k"de
// kod 0 değeri −32·d·a'dır, d 0 olduğu için 0).
func newQuantStorage(dt DType, n int) *cpuStorage {
	elems, size := dt.QuantBlock()
	return &cpuStorage{dtyp: dt, raw: make([]byte, n/elems*size), n: n}
}
