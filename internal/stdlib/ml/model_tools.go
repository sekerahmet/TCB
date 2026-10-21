package ml

import (
	"math"
	"math/rand"
	"sort"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bu dosya model yazmak için gereken yapı taşlarını TR'ye bağlar: tensör birleştirme ve
// bölme, birleşik dikkat ve token örnekleme. Bir modelin parametrelerini toplama ve yükleme:
// parametreler.go.

// Adlı argümanlar (adları kod listesinde, internal/lang/codes T9xx).
var (
	nameKeepDim     = bind.NamedArg(codes.T906)
	nameCausal      = bind.NamedArg(codes.T950)
	nameWindow      = bind.NamedArg(codes.T951)
	nameMask        = bind.NamedArg(codes.T952)
	nameTemperature = bind.NamedArg(codes.T960)
	nameTopK        = bind.NamedArg(codes.T961)
	nameWithNames   = bind.NamedArg(codes.T970)
	nameApproximate = bind.NamedArg(codes.T990)
)

// memberSplit tensörün .böl(parça_sayısı, eksen: -1) metodudur. Üye adları tipten bağımsız
// benzersiz olduğundan Metin'in .böl'üyle aynı koddur (Y103).
const memberSplit = codes.Y103

// namedBool adlı bir Mantık argümanını okur; verilmemişse varsayilan döner.
func namedBool(fn string, named map[string]interp.Value, a bind.Named, defaultVal bool) (bool, error) {
	v, ok := bind.GetNamed(named, a)
	if !ok {
		return defaultVal, nil
	}
	return bind.Bool(fn, a.TR, v)
}

// ----------------------------------------------------------------------------
// birleştir ve .böl
// ----------------------------------------------------------------------------

// concatBuiltin birleştir([a, b, ...], eksen: -1) yerleşiğidir: tensörleri bir eksen
// boyunca birleştirir. Birleşen eksen dışındaki eksenler eşit olmalıdır. Türevlenebilir.
func concatBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T121)
	if len(args) < 1 || len(args) > 2 {
		return nil, messages.New(messages.H6304, fn, nameAxis.TR, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameAxis); err != nil {
		return nil, err
	}
	l, ok := args[0].(*interp.ListVal)
	if !ok {
		return nil, messages.New(messages.H6306, fn, typeName(args[0]))
	}
	if len(l.Elements) == 0 {
		return nil, messages.New(messages.H6307, fn)
	}
	ts := make([]*tensor.Tensor, len(l.Elements))
	ints := true // hepsi tam sayılı tensörse (ör. token kimlikleri) sonuç da tam sayılıdır
	for i, el := range l.Elements {
		t, err := toTensor(messages.Text(messages.H6344, fn, i+1), el)
		if err != nil {
			return nil, err
		}
		ts[i] = t
		tv, ok := el.(*TensorValue)
		ints = ints && ok && tv.hasInts
	}
	e, given, err := axisArg(fn, args, named)
	if err != nil {
		return nil, err
	}
	if !given {
		e = -1
	}
	first := ts[0].Shape()
	if e, err = parseAxis(fn, e, first); err != nil {
		return nil, err
	}
	for i, t := range ts[1:] {
		sh := t.Shape()
		if len(sh) != len(first) {
			return nil, messages.New(messages.H6309, fn, i+2, len(sh), shapeText(sh), len(first), shapeText(first))
		}
		for d := range sh {
			if d != e && sh[d] != first[d] {
				return nil, messages.New(messages.H6311, fn, i+2, shapeText(sh), shapeText(first), e, d, sh[d], first[d])
			}
		}
		if err := checkDevice(fn, ts[0], t); err != nil {
			return nil, err
		}
	}
	r, err := safe(func() *tensor.Tensor { return tensor.Concat(ts, e) })
	if err != nil {
		return nil, err
	}
	result := New(r)
	result.hasInts = ints
	return result, nil
}

// split .böl(parça_sayısı, eksen: -1) metodudur: tensörü verilen eksende eşit parçalara
// böler; parçalar veriyi paylaşan, türev izleyen görünümlerdir.
func (d *TensorValue) split(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.Y103)
	if len(args) < 1 || len(args) > 2 {
		return nil, messages.New(messages.H6315, fn, nameAxis.TR, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameAxis); err != nil {
		return nil, err
	}
	n, ok := intNumber(args[0])
	if !ok || n <= 0 {
		return nil, messages.New(messages.H6317, fn, args[0].Inspect())
	}
	e, given, err := axisArg(fn, args, named)
	if err != nil {
		return nil, err
	}
	if !given {
		e = -1
	}
	sh := d.t.Shape()
	if e, err = parseAxis(fn, e, sh); err != nil {
		return nil, err
	}
	if sh[e]%n != 0 {
		return nil, messages.New(messages.H6318, fn, shapeText(sh), e, sh[e], n)
	}
	parts, err := safe(func() []*tensor.Tensor { return d.t.Split(n, e) })
	if err != nil {
		return nil, err
	}
	el := make([]interp.Value, len(parts))
	for i, p := range parts {
		y := New(p)
		y.hasInts = d.hasInts
		el[i] = y
	}
	return &interp.ListVal{Elements: el}, nil
}

// ----------------------------------------------------------------------------
// dikkat
// ----------------------------------------------------------------------------

// attentionBuiltin dikkat(q, k, v, nedensel: doğru, pencere: 0, maske: yok) yerleşiğidir:
// birleşik ölçekli nokta çarpımı dikkati (bkz. tensor.Attention). Ölçek 1/√dh'dir.
func attentionBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T503)
	if len(args) != 3 {
		return nil, messages.New(messages.H6321, fn, nameCausal.TR, codes.DisplayName(codes.A040), nameWindow.TR, nameMask.TR, codes.DisplayName(codes.A042), len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameCausal, nameWindow, nameMask); err != nil {
		return nil, err
	}
	var qkv [3]*tensor.Tensor
	for i, ad := range []string{"q", "k", "v"} {
		d, ok := args[i].(*TensorValue)
		if !ok {
			return nil, messages.New(messages.H6324, fn, ad, typeName(args[i]))
		}
		qkv[i] = d.t
	}
	q, k, v := qkv[0], qkv[1], qkv[2]
	opt := tensor.AttentionOptions{Causal: true}
	var err error
	if opt.Causal, err = namedBool(fn, named, nameCausal, true); err != nil {
		return nil, err
	}
	if pv, ok := bind.GetNamed(named, nameWindow); ok {
		w, integer := intNumber(pv)
		if !integer || w < 0 {
			return nil, messages.New(messages.H6326, fn, pv.Inspect())
		}
		opt.Window = w
	}
	if err := checkAttentionShapes(fn, q, k, v); err != nil {
		return nil, err
	}
	if mv, ok := bind.GetNamed(named, nameMask); ok && mv != interp.Null {
		mt, err := toTensor(fn+": maske", mv)
		if err != nil {
			return nil, err
		}
		if err := checkMask(fn, mt, q, k); err != nil {
			return nil, err
		}
		opt.Mask = mt
	}
	for _, t := range []*tensor.Tensor{k, v} {
		if err := checkDevice(fn, q, t); err != nil {
			return nil, err
		}
	}
	mixed := mixedDType(m)
	r, err := safe(func() *tensor.Tensor {
		if mixed != tensor.Float32 {
			// Karışık hassasiyet: girdiler blok türüne çevrilir; biriktirme float32, sonuç blok
			// türündedir ('@' ile aynı kural, PyTorch autocast'inde de dikkat 16 bittir).
			return tensor.Attention(q.CastShared(mixed), k.CastShared(mixed), v.CastShared(mixed), opt)
		}
		return tensor.Attention(q, k, v, opt)
	})
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// checkAttentionShapes q [..., H, Tq, dh], k [..., Hkv, Tk, dh], v [..., Hkv, Tk, dv]
// şekillerini denetler ve yol gösteren bir hata verir.
func checkAttentionShapes(fn string, q, k, v *tensor.Tensor) error {
	qs, ks, vs := q.Shape(), k.Shape(), v.Shape()
	n := len(qs)
	if n < 2 {
		return messages.New(messages.H6327, fn, shapeText(qs))
	}
	if len(ks) != n || len(vs) != n {
		return messages.New(messages.H6328, fn, shapeText(qs), shapeText(ks), shapeText(vs))
	}
	on := max(n-3, 0)
	if shapeText(qs[:on]) != shapeText(ks[:on]) || shapeText(qs[:on]) != shapeText(vs[:on]) {
		return messages.New(messages.H6329, fn, shapeText(qs), shapeText(ks), shapeText(vs))
	}
	if ks[n-1] != qs[n-1] {
		return messages.New(messages.H6330, fn, shapeText(qs), shapeText(ks))
	}
	if vs[n-2] != ks[n-2] {
		return messages.New(messages.H6331, fn, shapeText(ks), shapeText(vs))
	}
	if n >= 3 {
		H, Hkv := qs[n-3], ks[n-3]
		if vs[n-3] != Hkv {
			return messages.New(messages.H6332, fn, shapeText(ks), shapeText(vs))
		}
		if Hkv == 0 || H%Hkv != 0 {
			return messages.New(messages.H6333, fn, H, Hkv)
		}
	}
	return nil
}

// checkMask maskenin [..., H, Tq, Tk] şekline yayınlanabildiğini denetler.
func checkMask(fn string, mt, q, k *tensor.Tensor) error {
	qs, ms := q.Shape(), mt.Shape()
	target := append(qs[:len(qs)-1:len(qs)-1], k.Shape()[k.Dim()-2])
	if len(ms) > len(target) {
		return messages.New(messages.H6334, fn, shapeText(ms), shapeText(target))
	}
	for i := 1; i <= len(ms); i++ {
		if d := ms[len(ms)-i]; d != 1 && d != target[len(target)-i] {
			return messages.New(messages.H6335, fn, shapeText(ms), shapeText(target), i, d, target[len(target)-i])
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// örnekle
// ----------------------------------------------------------------------------

// sampleBuiltin örnekle(logit, sıcaklık: 1.0, en_iyi_k: 0, tohum: yok) yerleşiğidir:
// son eksen üzerindeki softmax(logit / sıcaklık) dağılımından bir indeks örnekler.
// en_iyi_k > 0 ise yalnızca en olası k arasından seçer; sıcaklık 0 en olasıyı verir
// (maks_indeks ile aynı). Sonuç son ekseni düşürülmüş, tam sayılı işaretli ve türev
// izlemeyen bir tensördür.
func sampleBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T611)
	if len(args) != 1 {
		return nil, messages.New(messages.H6336, fn, nameTemperature.TR, nameTopK.TR, nameSeed.TR, codes.DisplayName(codes.A042), len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameTemperature, nameTopK, nameSeed); err != nil {
		return nil, err
	}
	d, ok := args[0].(*TensorValue)
	if !ok {
		return nil, messages.New(messages.H6337, fn, typeName(args[0]), codes.DisplayName(codes.T100))
	}
	sh := d.t.Shape()
	if len(sh) == 0 {
		return nil, messages.New(messages.H6338, fn)
	}
	C := sh[len(sh)-1]
	if C == 0 {
		return nil, messages.New(messages.H6339, fn, shapeText(sh))
	}
	temperature := 1.0
	if sv, ok := bind.GetNamed(named, nameTemperature); ok {
		f, number := interp.ToFloat(sv)
		if !number || f < 0 || math.IsNaN(f) {
			return nil, messages.New(messages.H6340, fn, nameTemperature.TR, sv.Inspect())
		}
		temperature = f
	}
	k := 0
	if kv, ok := bind.GetNamed(named, nameTopK); ok {
		n, integer := intNumber(kv)
		if !integer || n < 0 {
			return nil, messages.New(messages.H6341, fn, nameTopK.TR, kv.Inspect())
		}
		k = n
	}
	if k == 0 || k > C {
		k = C
	}
	var rng *rand.Rand
	if tv, ok := bind.GetNamed(named, nameSeed); ok && tv != interp.Null {
		t, integer := tv.(*interp.IntVal)
		if !integer {
			return nil, messages.New(messages.H6342, fn, typeName(tv))
		}
		rng = rand.New(rand.NewSource(t.Val))
	}
	data := d.t.Data()
	line := len(data) / C
	result := make([]float32, line)
	// Rastgele sayılar önce çekilir (paylaşılan üreteç kilit altında, kısa süre).
	u := make([]float64, line)
	if temperature > 0 {
		if rng != nil {
			for i := range u {
				u[i] = rng.Float64()
			}
		} else {
			reportUnseeded()
			isRandom.Lock()
			for i := range u {
				u[i] = randomStream.Float64()
			}
			isRandom.Unlock()
		}
	}
	idx := make([]int, C)
	weight := make([]float64, C)
	for r := 0; r < line; r++ {
		x := data[r*C : (r+1)*C]
		if temperature == 0 {
			en := 0
			for j, v := range x {
				if v > x[en] || (v != v && x[en] == x[en]) {
					en = j
				}
			}
			result[r] = float32(en)
			continue
		}
		candidates := idx[:C]
		for j := range candidates {
			candidates[j] = j
		}
		if k < C {
			// En olası k: büyükten küçüğe, eşitlikte küçük indeks önce.
			sort.SliceStable(candidates, func(a, b int) bool { return x[candidates[a]] > x[candidates[b]] })
			candidates = candidates[:k]
			sort.Ints(candidates)
		}
		largestVal := math.Inf(-1)
		for _, j := range candidates {
			largestVal = math.Max(largestVal, float64(x[j]))
		}
		var top float64
		for i, j := range candidates {
			w := math.Exp((float64(x[j]) - largestVal) / temperature)
			weight[i] = w
			top += w
		}
		if !(top > 0) || math.IsInf(top, 0) {
			// NaN logit, hepsi -∞ ya da +∞: dağılım tanımsız; sessizce son aday seçilmesin.
			return nil, messages.New(messages.H6343, fn, r+1)
		}
		target := u[r] * top
		picked := candidates[len(candidates)-1]
		var accum float64
		for i, j := range candidates {
			accum += weight[i]
			if target < accum {
				picked = j
				break
			}
		}
		result[r] = float32(picked)
	}
	output := sh[:len(sh)-1]
	r, err := safe(func() *tensor.Tensor { return tensor.NewOn(d.t.Device(), result, output...) })
	if err != nil {
		return nil, err
	}
	return NewInt(r), nil
}
