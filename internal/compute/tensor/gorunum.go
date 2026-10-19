package tensor

import "fmt"

// Görünüm (view) işlemleri: veri kopyalanmaz, yalnızca ofset/şekil/adım
// değişir. Geri yayılımda gradyan, ebeveynin bitişik gradyan tamponuna aynı
// eşleme ile (bitişik adımlar üzerinden) geri dağıtılır.

// viewOf ebeveynle depoyu paylaşan yeni bir tensör başlığı üretir.
func viewOf(t *Tensor, offset int, shape, strides []int) *Tensor {
	return &Tensor{storage: t.storage, be: t.be, offset: offset, shape: shape, strides: strides, tur: t.tur}
}

// scatterBackward, görünümün bitişik gradyanını ebeveynin gradyan tamponuna
// gOff ofseti ve gStrides adımlarıyla ekleyen geri yayılım işlevini üretir.
func scatterBackward(parent, out *Tensor, gOff int, gStrides []int) func() {
	return func() {
		parent.be.ScatterAdd(parent.accGrad(), gOff, gStrides, out.grad, out.shape)
	}
}

// gradDevret, düzeni ebeveyninkiyle aynı olan (aynı elemanlar, aynı satır
// öncelikli sıra) bir çıktının gradyanını ebeveyne aktarır. Ebeveynin henüz
// gradyanı yoksa depolama kopyalanmadan devredilir ve çıktıdan alınır; böylece
// her gradyan depolamasının tek bir sahibi olur (geri yayılım, sahibi kalmayan
// ara gradyanı ayrıca bırakmaz/iade etmez). Aksi halde üzerine eklenir.
func gradDevret(t, out *Tensor) {
	g := out.grad
	if t.grad == nil && g.Len() == t.Numel() && g.DType() == gradTuru(t.tur) {
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
	bilinmeyen := -1
	for i, d := range sh {
		if d == -1 {
			if bilinmeyen >= 0 {
				panic("tensor: Reshape içinde yalnızca bir eksen -1 olabilir")
			}
			bilinmeyen = i
			sh[i] = 1
			continue
		}
		if d < 0 {
			panic(fmt.Sprintf("tensor: Reshape için geçersiz şekil %v", shape))
		}
	}
	// Çarpım taşma denetimlidir: taşan bir şekil sessizce n'ye eşit görünmesin.
	carpim, tasma := elemanSayisi(sh)
	if tasma {
		panic(sinirHatasi(isYenidenSekil, sh, t.tur, true))
	}
	if bilinmeyen >= 0 {
		if carpim == 0 || n%carpim != 0 {
			panic(fmt.Sprintf("tensor: %v şekli %d elemana uydurulamaz", shape, n))
		}
		sh[bilinmeyen] = n / carpim
		carpim = n
	}
	if carpim != n {
		panic(fmt.Sprintf("tensor: %v şeklindeki tensör %v şekline sokulamaz", t.shape, shape))
	}
	var st []int
	if t.IsContiguous() {
		st = contiguousStrides(sh)
	} else if g, ok := gorunumAdimlari(t.shape, t.strides, sh); ok {
		st = g
	} else {
		return t.Contiguous().Reshape(sh...)
	}
	return yenidenSekil(t, sh, st)
}

// yenidenSekil t'nin elemanlarına aynı mantıksal sırayla sh şekli ve st adımlarıyla bakan
// görünümü kurar. Mantıksal sıra değişmediğinden gradyan ebeveyne olduğu gibi devredilir.
func yenidenSekil(t *Tensor, sh, st []int) *Tensor {
	sy := sayacBasla(t.be, IslemGorunum).baslat()
	out := viewOf(t, t.offset, sh, st)
	r := track(out, "reshape", []*Tensor{t}, func() { gradDevret(t, out) })
	sy.bitir()
	return r
}

// gorunumAdimlari, eski şekil ve adımlarla bakılan elemanlara (satır öncelikli mantıksal
// sırayı koruyarak) yeni şekille kopyasız bakılabiliyorsa yeni adımları verir. Boyutu 1
// olan eksenler atlanır; kalan eski eksenler, depoda düzenli adımlı ardışık gruplara
// (adım[i] = adım[i+1]·boyut[i+1]) ayrılır. Her grup, yeni şeklin sondan ardışık
// eksenleriyle tam olarak örtüşmelidir; grup içinde yeni eksenlerin adımları grubun en
// küçük adımından türetilir.
func gorunumAdimlari(eski, eskiAdim, yeni []int) ([]int, bool) {
	st := make([]int, len(yeni))
	if numelOf(eski) == 0 {
		return contiguousStrides(yeni), true
	}
	var boy, adim []int
	for i, d := range eski {
		if d != 1 {
			boy = append(boy, d)
			adim = append(adim, eskiAdim[i])
		}
	}
	yi := len(yeni) - 1
	for k := len(boy) - 1; k >= 0; {
		// Grup: k'den geriye düzenli adımlı ardışık eksenler.
		taban, toplam := adim[k], boy[k]
		j := k - 1
		for j >= 0 && adim[j] == adim[j+1]*boy[j+1] {
			toplam *= boy[j]
			j--
		}
		carpim := 1
		for yi >= 0 && carpim < toplam {
			st[yi] = taban * carpim
			carpim *= yeni[yi]
			yi--
		}
		if carpim != toplam {
			return nil, false
		}
		k = j
	}
	// Kalan yeni eksenlerin hepsi 1 olmalıdır (eleman sayısı eşit olduğu için öyledir).
	for ; yi >= 0; yi-- {
		if yeni[yi] != 1 {
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
	adim := 1
	if axis < len(t.shape) {
		adim = t.strides[axis] * t.shape[axis]
	}
	st := make([]int, 0, nd)
	st = append(st, t.strides[:axis]...)
	st = append(st, adim)
	st = append(st, t.strides[axis:]...)
	return yenidenSekil(t, sh, st)
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
	return yenidenSekil(t, sh, st)
}

// Permute eksenleri verilen sıraya dizer (kopyasız).
func (t *Tensor) Permute(dims ...int) *Tensor {
	sy := sayacBasla(t.be, IslemGorunum).baslat()
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
	r := track(out, "permute", []*Tensor{t}, scatterBackward(t, out, 0, gst))
	sy.bitir()
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
	sy := sayacBasla(t.be, IslemGorunum).baslat()
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
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if end < start {
		end = start
	}
	m := (end - start + step - 1) / step
	sh := copyInts(t.shape)
	sh[axis] = m
	st := copyInts(t.strides)
	st[axis] *= step
	gst := contiguousStrides(t.shape)
	gOff := start * gst[axis]
	gst[axis] *= step
	out := viewOf(t, t.offset+start*t.strides[axis], sh, st)
	r := track(out, "slice", []*Tensor{t}, scatterBackward(t, out, gOff, gst))
	sy.bitir()
	return r
}

// Contiguous tensör bitişikse kendisini, değilse bitişik bir kopyasını verir.
func (t *Tensor) Contiguous() *Tensor {
	if t.IsContiguous() {
		return t
	}
	sy := sayacBasla(t.be, IslemKopya).baslat()
	out := emptyOf(isKopya, t.be, t.shape, t.tur)
	t.be.Copy(out.storage, 0, t.view())
	r := track(out, "contiguous", []*Tensor{t}, func() { gradDevret(t, out) })
	sy.bitir()
	return r
}

// Clone verinin bitişik bir kopyasını üretir (türevlenebilir).
func (t *Tensor) Clone() *Tensor {
	sy := sayacBasla(t.be, IslemKopya).baslat()
	out := emptyOf(isKopya, t.be, t.shape, t.tur)
	t.be.Copy(out.storage, 0, t.view())
	r := track(out, "clone", []*Tensor{t}, func() { gradDevret(t, out) })
	sy.bitir()
	return r
}
