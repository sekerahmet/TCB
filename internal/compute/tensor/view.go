package tensor

import "fmt"

// Görünüm (view) işlemleri: veri kopyalanmaz, yalnızca ofset/şekil/adım
// değişir. Geri yayılımda gradyan, ebeveynin bitişik gradyan tamponuna aynı
// eşleme ile (bitişik adımlar üzerinden) geri dağıtılır.

// viewOf ebeveynle depoyu paylaşan yeni bir tensör başlığı üretir.
func viewOf(t *Tensor, offset int, shape, strides []int) *Tensor {
	return &Tensor{storage: t.storage, be: t.be, offset: offset, shape: shape, strides: strides, dtyp: t.dtyp}
}

// scatterBackward, görünümün bitişik gradyanını ebeveynin gradyan tamponuna
// gOff ofseti ve gStrides adımlarıyla ekleyen geri yayılım işlevini üretir.
func scatterBackward(parent, out *Tensor, gOff int, gStrides []int) func() {
	return func() {
		parent.be.ScatterAdd(parent.accGrad(), gOff, gStrides, out.grad, out.shape)
	}
}

// handOffGrad, düzeni ebeveyninkiyle aynı olan (aynı elemanlar, aynı satır
// öncelikli sıra) bir çıktının gradyanını ebeveyne aktarır. Ebeveynin henüz
// gradyanı yoksa depolama kopyalanmadan devredilir ve çıktıdan alınır; böylece
// her gradyan depolamasının tek bir sahibi olur (geri yayılım, sahibi kalmayan
// ara gradyanı ayrıca bırakmaz/iade etmez). Aksi halde üzerine eklenir.
func handOffGrad(t, out *Tensor) {
	g := out.grad
	if t.grad == nil && g.Len() == t.Numel() && g.DType() == t.gradDType() {
		t.grad = g
		out.grad = nil
		return
	}
	t.be.AddTo(t.accGrad(), g, 1)
}

// Reshape tensörü yeni şekle sokar; bir eksen -1 olabilir (çıkarsanır).
// Kopyasızdır (görünüm) ancak yeni şekil eldeki adımlarla ifade edilebiliyorsa:
// bitişik tensörlerde her zaman; bitişik olmayanlarda yalnızca birleştirilen her
// eksen grubu depoda düzenli adımlıysa (ör. boyut 1 eksen ekleme/silme, düzenli
// adımlı bir ekseni bölme). Aksi halde önce Contiguous ile kopyalanır.
func (t *Tensor) Reshape(shape ...int) *Tensor {
	sh := copyInts(shape)
	n := t.Numel()
	unknown := -1
	for i, d := range sh {
		if d == -1 {
			if unknown >= 0 {
				panic("tensor: Reshape içinde yalnızca bir eksen -1 olabilir")
			}
			unknown = i
			sh[i] = 1
			continue
		}
		if d < 0 {
			panic(fmt.Sprintf("tensor: Reshape için geçersiz şekil %v", shape))
		}
	}
	// Çarpım taşma denetimlidir: taşan bir şekil sessizce n'ye eşit görünmesin.
	product, overflow := elemCount(sh)
	if overflow {
		panic(limitError(isReshape, sh, t.dtyp, true))
	}
	if unknown >= 0 {
		if product == 0 || n%product != 0 {
			panic(fmt.Sprintf("tensor: %v şekli %d elemana uydurulamaz", shape, n))
		}
		sh[unknown] = n / product
		product = n
	}
	if product != n {
		panic(fmt.Sprintf("tensor: %v şeklindeki tensör %v şekline sokulamaz", t.shape, shape))
	}
	var st []int
	if t.IsContiguous() {
		st = contiguousStrides(sh)
	} else if g, ok := viewStrides(t.shape, t.strides, sh); ok {
		st = g
	} else {
		return t.Contiguous().Reshape(sh...)
	}
	return reshaped(t, sh, st)
}

// reshaped t'nin elemanlarına aynı mantıksal sırayla sh şekli ve st adımlarıyla bakan
// görünümü kurar. Mantıksal sıra değişmediğinden gradyan ebeveyne olduğu gibi devredilir.
func reshaped(t *Tensor, sh, st []int) *Tensor {
	sy := counterStart(t.be, OpKindView).start()
	out := viewOf(t, t.offset, sh, st)
	r := trackPass(out, "reshape", t, func() { handOffGrad(t, out) })
	sy.finish()
	return r
}

// viewStrides, eski şekil ve adımlarla bakılan elemanlara (satır öncelikli mantıksal
// sırayı koruyarak) yeni şekille kopyasız bakılabiliyorsa yeni adımları verir. Boyutu 1
// olan eksenler atlanır; kalan eski eksenler, depoda düzenli adımlı ardışık gruplara
// (adım[i] = adım[i+1]·boyut[i+1]) ayrılır. Her grup, yeni şeklin sondan ardışık
// eksenleriyle tam olarak örtüşmelidir; grup içinde yeni eksenlerin adımları grubun en
// küçük adımından türetilir.
func viewStrides(old, oldStep, freshT []int) ([]int, bool) {
	st := make([]int, len(freshT))
	if numelOf(old) == 0 {
		return contiguousStrides(freshT), true
	}
	var size, step []int
	for i, d := range old {
		if d != 1 {
			size = append(size, d)
			step = append(step, oldStep[i])
		}
	}
	yi := len(freshT) - 1
	for k := len(size) - 1; k >= 0; {
		// Grup: k'den geriye düzenli adımlı ardışık eksenler.
		floorV, total := step[k], size[k]
		j := k - 1
		for j >= 0 && step[j] == step[j+1]*size[j+1] {
			total *= size[j]
			j--
		}
		product := 1
		for yi >= 0 && product < total {
			st[yi] = floorV * product
			product *= freshT[yi]
			yi--
		}
		if product != total {
			return nil, false
		}
		k = j
	}
	// Kalan yeni eksenlerin hepsi 1 olmalıdır (eleman sayısı eşit olduğu için öyledir).
	for ; yi >= 0; yi-- {
		if freshT[yi] != 1 {
			return nil, false
		}
		st[yi] = 1
	}
	return st, true
}

// Flatten tensörü tek boyuta indirir.
func (t *Tensor) Flatten() *Tensor { return t.Reshape(-1) }

// Unsqueeze verilen konuma uzunluğu 1 olan bir eksen ekler (kopyasız görünüm).
func (t *Tensor) Unsqueeze(axis int) *Tensor {
	nd := len(t.shape) + 1
	if axis < 0 {
		axis += nd
	}
	if axis < 0 || axis >= nd {
		panic(fmt.Sprintf("tensor: Unsqueeze ekseni %d geçersiz", axis))
	}
	sh := make([]int, 0, nd)
	sh = append(sh, t.shape[:axis]...)
	sh = append(sh, 1)
	sh = append(sh, t.shape[axis:]...)
	// Saf görünüm: bitişik olmayan tensörde de kopya yok. Yeni eksenin adımı önemsizdir
	// (tek eleman); sağındaki eksenin kapladığı aralık seçilir.
	step := 1
	if axis < len(t.shape) {
		step = t.strides[axis] * t.shape[axis]
	}
	st := make([]int, 0, nd)
	st = append(st, t.strides[:axis]...)
	st = append(st, step)
	st = append(st, t.strides[axis:]...)
	return reshaped(t, sh, st)
}

// Squeeze uzunluğu 1 olan ekseni kaldırır (kopyasız görünüm).
func (t *Tensor) Squeeze(axis int) *Tensor {
	axis = normAxis(axis, len(t.shape))
	if t.shape[axis] != 1 {
		panic(fmt.Sprintf("tensor: Squeeze ekseni %d uzunluğu 1 değil (%v)", axis, t.shape))
	}
	sh := make([]int, 0, len(t.shape)-1)
	sh = append(sh, t.shape[:axis]...)
	sh = append(sh, t.shape[axis+1:]...)
	// Saf görünüm: boyutu 1 olan eksen ve adımı düşürülür; bitişik olmayan tensörde de
	// kopya yok (t[i] ve döngü öğeleri buradan geçer).
	st := make([]int, 0, len(t.shape)-1)
	st = append(st, t.strides[:axis]...)
	st = append(st, t.strides[axis+1:]...)
	return reshaped(t, sh, st)
}

// Permute eksenleri verilen sıraya dizer (kopyasız).
func (t *Tensor) Permute(dims ...int) *Tensor {
	sy := counterStart(t.be, OpKindView).start()
	nd := len(t.shape)
	if len(dims) != nd {
		panic(fmt.Sprintf("tensor: Permute %d eksen bekler, %d verildi", nd, len(dims)))
	}
	seen := make([]bool, nd)
	sh := make([]int, nd)
	st := make([]int, nd)
	gst := make([]int, nd)
	cst := contiguousStrides(t.shape)
	for i, d := range dims {
		d = normAxis(d, nd)
		if seen[d] {
			panic(fmt.Sprintf("tensor: Permute eksenleri yinelenmiş %v", dims))
		}
		seen[d] = true
		sh[i] = t.shape[d]
		st[i] = t.strides[d]
		gst[i] = cst[d]
	}
	out := viewOf(t, t.offset, sh, st)
	r := trackPass(out, "permute", t, scatterBackward(t, out, 0, gst))
	sy.finish()
	return r
}

// Transpose iki ekseni yer değiştirir (kopyasız).
func (t *Tensor) Transpose(a, b int) *Tensor {
	nd := len(t.shape)
	a, b = normAxis(a, nd), normAxis(b, nd)
	dims := make([]int, nd)
	for i := range dims {
		dims[i] = i
	}
	dims[a], dims[b] = dims[b], dims[a]
	return t.Permute(dims...)
}

// T 2 boyutlu tensörün devriği (son iki eksenin yer değiştirmesi).
func (t *Tensor) T() *Tensor {
	if len(t.shape) < 2 {
		panic("tensor: T en az 2 boyut ister")
	}
	return t.Transpose(-2, -1)
}

// Slice verilen eksende [start, end) aralığını seçer (kopyasız). Negatif
// indeksler sondan sayar.
func (t *Tensor) Slice(axis, start, end int) *Tensor {
	return t.SliceStep(axis, start, end, 1)
}

// SliceStep verilen eksende start'tan end'e (hariç) step adımlarla seçer
// (kopyasız). step pozitif olmalıdır.
func (t *Tensor) SliceStep(axis, start, end, step int) *Tensor {
	sy := counterStart(t.be, OpKindView).start()
	nd := len(t.shape)
	axis = normAxis(axis, nd)
	n := t.shape[axis]
	if step <= 0 {
		panic("tensor: Slice adımı pozitif olmalıdır")
	}
	if start < 0 {
		start += n
	}
	if end < 0 {
		end += n
	}
	start = min(max(start, 0), n)
	end = min(end, n)
	if end < start {
		end = start
	}
	// Eksen dışındaki başlangıç boş dilimdir (ofseti depolamanın dışına taşmaz); eleman sayısı
	// büyük adımda taşmadan hesaplanır.
	m := 0
	if end > start {
		m = 1 + (end-start-1)/step
	}
	sh := copyInts(t.shape)
	sh[axis] = m
	st := copyInts(t.strides)
	st[axis] *= step
	gst := contiguousStrides(t.shape)
	gOff := start * gst[axis]
	gst[axis] *= step
	out := viewOf(t, t.offset+start*t.strides[axis], sh, st)
	r := trackPass(out, "slice", t, scatterBackward(t, out, gOff, gst))
	sy.finish()
	return r
}

// Contiguous tensör bitişikse kendisini, değilse bitişik bir kopyasını verir.
func (t *Tensor) Contiguous() *Tensor {
	if t.IsContiguous() {
		return t
	}
	sy := counterStart(t.be, OpKindCopy).start()
	out := emptyOf(isCopy, t.be, t.shape, t.dtyp)
	t.be.Copy(out.storage, 0, t.view())
	r := trackPass(out, "contiguous", t, func() { handOffGrad(t, out) })
	sy.finish()
	return r
}

// Clone verinin bitişik bir kopyasını üretir (türevlenebilir).
func (t *Tensor) Clone() *Tensor {
	sy := counterStart(t.be, OpKindCopy).start()
	out := emptyOf(isCopy, t.be, t.shape, t.dtyp)
	t.be.Copy(out.storage, 0, t.view())
	r := trackPass(out, "clone", t, func() { handOffGrad(t, out) })
	sy.finish()
	return r
}
