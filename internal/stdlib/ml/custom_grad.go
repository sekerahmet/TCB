package ml

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bu dosya kullanıcı tanımlı türevi (özel_türev) ve sayısal gradyan denetimini
// (türev_denetle) TR'ye bağlar.
//
//	değişken kare = özel_türev(
//	    ileri: fonksiyon(x) { sonuç [x * x, [x]] },        // [çıktı, saklananlar]
//	    geri: fonksiyon(saklananlar, g) { sonuç [g * 2 * saklananlar[0]] }
//	)
//
// ileri türevsiz çalışır ve [çıktı, saklananlar] döndürür; çıktı bir tensör ya da tensör
// listesidir, saklananlar geri hesabın ihtiyaç duyacağı değerlerin listesidir. Girdilerden
// biri türev izliyorsa çıktı(lar) çekirdekte tek bir dış düğüm olarak kaydedilir (bkz.
// github.com/sekerahmet/TCB/internal/compute/tensor CustomOp); geri yayılımda geri(saklananlar, g) türevsiz çalışır ve her
// konumlu girdi için bir gradyan (ya da yok) döndürür. Çok çıktıda g her çıktı için bir
// gradyan içeren listedir; kayba bağlı olmayan çıktının gradyanı sıfır tensördür (geri
// hesap 'yok' denetimi yapmadan aritmetik kullanabilsin diye).
//
// geri, geri_yayılım'ı çağıran görevin VM'iyle (yorumlayıcı kilidini tutan görev) çalışır:
// görev, çekirdeğe BackwardCtx ile bağlam olarak verilir.

var (
	nameForward   = bind.NamedArg(codes.T980)
	nameBackward  = bind.NamedArg(codes.T981)
	nameStep      = bind.NamedArg(codes.T987)
	nameTolerance = bind.NamedArg(codes.T983)
)

// registerCustomGrad özel_türev ve türev_denetle yerleşiklerini kaydeder (bkz. Kaydet).
func registerCustomGrad(v *interp.VM) {
	wrap(v, codes.T310, newBuiltin(customGrad))
	wrap(v, codes.T311, newBuiltin(checkGrad))
}

// customGradExample hata mesajlarındaki kullanım örneğidir; adlar koddan kurulur.
var customGradExample = fmt.Sprintf("%s(%s: %s(x) { %s [x * x, [x]] }, %s: %s(saklananlar, g) { %s [g * 2 * saklananlar[0]] })",
	codes.Name(codes.T310), nameForward.TR, codes.Name(codes.A020), codes.Name(codes.A021), nameBackward.TR, codes.Name(codes.A020), codes.Name(codes.A021))

// backwardError, geri yayılım sırasında (çekirdeğin içinden) kullanıcının geri hesabında
// oluşan hatayı taşır; backprop paniği yakalayıp hatayı olduğu gibi döndürür.
type backwardError struct{ err error }

// backprop t'den geri yayılım yapar; dış düğümlerin geri hesapları m göreviyle çalışır.
// Çekirdek paniği Türkçe bir hataya, kullanıcının geri hesabındaki hata ise olduğu gibi
// (satır bilgisiyle) döner.
func backprop(m *interp.VM, t, seed *tensor.Tensor) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if interp.IsInterrupt(r) {
				panic(r) // kesilme (Ctrl+C) hata değildir; yığın çözülmeye devam eder
			}
			if g, ok := r.(backwardError); ok {
				err = g.err
				return
			}
			err = panicError(r)
		}
	}()
	t.BackwardCtx(seed, m)
	return nil
}

// isCallable değer bir fonksiyon (TR ya da yerleşik) mi?
func isCallable(v interp.Value) bool {
	switch v.(type) {
	case *interp.FuncVal, *interp.BuiltinVal:
		return true
	}
	return false
}

// customOp özel_türev'in döndürdüğü çağrılabilir değerin durumudur.
type customOp struct {
	forward, backward interp.Value
}

// customGrad özel_türev(ileri:, geri:) yerleşiğidir.
func customGrad(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T310)
	if err := bind.CheckNamed(fn+"()", named, nameForward, nameBackward); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, messages.New(messages.H6405, fn, nameForward.TR, nameBackward.TR, len(args), customGradExample)
	}
	forward, ok1 := bind.GetNamed(named, nameForward)
	backward, ok2 := bind.GetNamed(named, nameBackward)
	if !ok1 || !ok2 {
		if !ok1 && !ok2 {
			return nil, messages.New(messages.H6431, fn, nameForward.TR, nameBackward.TR, customGradExample)
		}
		missing := nameForward.TR
		if ok1 {
			missing = nameBackward.TR
		}
		return nil, messages.New(messages.H6430, fn, missing, customGradExample)
	}
	if !isCallable(forward) {
		return nil, messages.New(messages.H6406, fn, nameForward.TR, typeName(forward))
	}
	if !isCallable(backward) {
		return nil, messages.New(messages.H6406, fn, nameBackward.TR, typeName(backward))
	}
	if f, ok := backward.(*interp.FuncVal); ok {
		required := 0
		for _, p := range f.Parameters {
			if p.DefaultValue == nil {
				required++
			}
		}
		if len(f.Parameters) < 2 || required > 2 {
			return nil, messages.New(messages.H6408, fn, nameBackward.TR, codes.DisplayName(codes.A020), len(f.Parameters))
		}
	}
	o := &customOp{forward: forward, backward: backward}
	return &interp.BuiltinVal{Name: codes.Name(codes.T310), Fn: o.call}, nil
}

// call özel işlemi çalıştırır: ileri türevsiz çalışır, sonuç tek dış düğümle çizgeye
// bağlanır. Adlı argümanlar ileri'ye olduğu gibi geçer.
func (o *customOp) call(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	args = append([]interp.Value(nil), args...)
	inputs := make([]*tensor.Tensor, len(args))
	for i, a := range args {
		if d, ok := a.(*TensorValue); ok {
			inputs[i] = d.t
		}
	}
	var result interp.Value
	var err error
	tensor.NoGrad(func() { result, err = m.ApplyFunction(o.forward, args, named, 0) })
	if err != nil {
		return nil, err
	}
	outputs, many, savedVals, err := forwardResult(result)
	if err != nil {
		return nil, err
	}
	var saved []*tensor.Tensor
	sumTensors(savedVals, &saved, 0)
	outs, err := safe(func() []*tensor.Tensor {
		return tensor.CustomOp(codes.Name(codes.T310), inputs, outputs, saved, o.backwardFn(m, args, savedVals, many))
	})
	if err != nil {
		return nil, err
	}
	return packTensorValues(outs, !many), nil
}

// packTensorValues geri çağrı çıktılarını kullanıcıya paketler: single ise tek tensör, değilse
// tensör listesi.
func packTensorValues(ts []*tensor.Tensor, single bool) interp.Value {
	if single {
		return New(ts[0])
	}
	el := make([]interp.Value, len(ts))
	for i, t := range ts {
		el[i] = New(t)
	}
	return &interp.ListVal{Elements: el}
}

// forwardResult ileri'nin [çıktı, saklananlar] dönüşünü çözer.
func forwardResult(v interp.Value) (outputs []*tensor.Tensor, many bool, savedVals *interp.ListVal, err error) {
	fn, result := codes.DisplayName(codes.T310), codes.DisplayName(codes.A021)
	forwards := nameForward.TR
	l, ok := v.(*interp.ListVal)
	if !ok || len(l.Elements) != 2 {
		return nil, false, nil, messages.New(messages.H6409, fn, nameForward.TR, result, shortForm(v))
	}
	savedVals, ok = l.Elements[1].(*interp.ListVal)
	if !ok {
		return nil, false, nil, messages.New(messages.H6410, fn, forwards, typeName(l.Elements[1]))
	}
	switch c := l.Elements[0].(type) {
	case *TensorValue:
		return []*tensor.Tensor{c.t}, false, savedVals, nil
	case *interp.ListVal:
		if len(c.Elements) == 0 {
			return nil, false, nil, messages.New(messages.H6411, fn, forwards)
		}
		for i, e := range c.Elements {
			d, ok := e.(*TensorValue)
			if !ok {
				return nil, false, nil, messages.New(messages.H6412, fn, forwards, i+1, typeName(e))
			}
			outputs = append(outputs, d.t)
		}
		return outputs, true, savedVals, nil
	}
	return nil, false, nil, messages.New(messages.H6413, fn, forwards, typeName(l.Elements[0]))
}

// shortForm hata mesajı için değerin tipini ve (kısaysa) kendisini yazar.
func shortForm(v interp.Value) string {
	if v == nil {
		return interp.Null.Inspect()
	}
	s := v.Inspect()
	if len(s) > 60 || strings.Contains(s, "\n") {
		return typeName(v)
	}
	return fmt.Sprintf("%s (%s)", s, typeName(v))
}

// sumTensors saklananlar içindeki (iç içe listeler dahil) tensörleri toplar; bunların
// sürümleri kaydedilir ve geri yayılımdan önce yerinde değiştirilmedikleri denetlenir.
func sumTensors(v interp.Value, out *[]*tensor.Tensor, depth int) {
	if depth > 16 {
		return
	}
	switch x := v.(type) {
	case *TensorValue:
		*out = append(*out, x.t)
	case *interp.ListVal:
		for _, e := range x.Elements {
			sumTensors(e, out, depth+1)
		}
	}
}

// backwardFn çekirdeğin dış düğümüne verilen geri çağrıyı kurar. Bağlam (ctx) geri
// yayılımı başlatan görevin VM'idir; verilmemişse (Go'dan doğrudan geri yayılım) ileri
// hesabı yapan görev kullanılır.
func (o *customOp) backwardFn(forwardVM *interp.VM, args []interp.Value, savedVals *interp.ListVal, many bool) tensor.CustomBackward {
	return func(ctx any, grads []*tensor.Tensor) []*tensor.Tensor {
		m, _ := ctx.(*interp.VM)
		if m == nil {
			m = forwardVM
		}
		var g interp.Value
		if many {
			el := make([]interp.Value, len(grads))
			for i, t := range grads {
				el[i] = New(t)
			}
			g = &interp.ListVal{Elements: el}
		} else {
			g = New(grads[0])
		}
		result, err := m.ApplyFunction(o.backward, []interp.Value{savedVals, g}, nil, 0)
		if err != nil {
			panic(backwardError{err})
		}
		gs, err := backwardResult(result, args)
		if err != nil {
			panic(backwardError{err})
		}
		return gs
	}
}

// backwardResult geri'nin döndürdüğü gradyan listesini denetler: her konumlu girdi için bir
// öğe; tensör girdilerde girdiyle aynı şekilde bir tensör ya da yok, tensör olmayan
// girdilerde yok.
func backwardResult(v interp.Value, args []interp.Value) ([]*tensor.Tensor, error) {
	fn, backward, none := codes.DisplayName(codes.T310), nameBackward.TR, codes.DisplayName(codes.A042)
	l, ok := v.(*interp.ListVal)
	if !ok {
		return nil, messages.New(messages.H6414, fn, backward, none, codes.DisplayName(codes.A021), shortForm(v))
	}
	if len(l.Elements) != len(args) {
		return nil, messages.New(messages.H6415, fn, backward, len(l.Elements), len(args), none)
	}
	out := make([]*tensor.Tensor, len(args))
	for i, e := range l.Elements {
		if _, none := e.(*interp.NullVal); none {
			continue
		}
		d, ok := e.(*TensorValue)
		if !ok {
			return nil, messages.New(messages.H6416, fn, backward, i+1, none, typeName(e))
		}
		input, ok := args[i].(*TensorValue)
		if !ok {
			return nil, messages.New(messages.H6417, fn, i+1, typeName(args[i]), backward, none)
		}
		if gs, is := d.t.Shape(), input.t.Shape(); !tensor.SameShape(gs, is) {
			return nil, messages.New(messages.H6418, fn, backward, i+1, shapeText(gs), shapeText(is))
		}
		out[i] = d.t
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// türev_denetle
// ----------------------------------------------------------------------------

// checkGrad türev_denetle(f, girdiler, fark_adımı: 0.001, tolerans: 0.01) yerleşiğidir:
// f'nin geri yayılımla bulunan gradyanlarını merkezi farkla bulunan sayısal gradyanlarla
// karşılaştırır. Denetim türev izleyen girdilerin float32 kopyalarıyla yapılır (kullanıcının
// tensörleri ve gradyanları değişmez). Skaler olmayan çıktılar sabit tohumlu rastgele bir
// yön vektörüyle iç çarpımla skalere indirgenir; kayıp ve farklar float64'te hesaplanır.
// Eleman başına koşul |analitik - sayısal| ≤ tolerans · max(1, |analitik|, |sayısal|).
func checkGrad(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T311)
	if err := bind.CheckNamed(fn+"()", named, nameStep, nameTolerance); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, messages.New(messages.H6419, fn, nameStep.TR, nameTolerance.TR, len(args))
	}
	f := args[0]
	if !isCallable(f) {
		return nil, messages.New(messages.H6420, fn, typeName(f))
	}
	list, ok := args[1].(*interp.ListVal)
	if !ok {
		return nil, messages.New(messages.H6421, fn, codes.DisplayName(codes.T100), codes.DisplayName(codes.T900), codes.DisplayName(codes.A040), typeName(args[1]))
	}
	step, err := positiveFloat(fn, named, nameStep, 1e-3)
	if err != nil {
		return nil, err
	}
	tol, err := positiveFloat(fn, named, nameTolerance, 1e-2)
	if err != nil {
		return nil, err
	}
	if !tensor.GradEnabled() {
		return nil, messages.New(messages.H6422, fn, codes.DisplayName(codes.T301))
	}

	// Türev izleyen girdilerin float32 yaprak kopyaları.
	inputs := make([]interp.Value, len(list.Elements))
	copies := make([]*tensor.Tensor, len(list.Elements))
	checked := 0
	for i, e := range list.Elements {
		inputs[i] = e
		if d, ok := e.(*TensorValue); ok && d.t.RequiresGrad() {
			c := tensor.NewOn(d.t.Device(), d.t.Data(), d.t.Shape()...).SetRequiresGrad(true)
			copies[i] = c
			inputs[i] = New(c)
			checked++
		}
	}
	if checked == 0 {
		return nil, messages.New(messages.H6423, fn, codes.DisplayName(codes.T100), codes.DisplayName(codes.T900), codes.DisplayName(codes.A040))
	}

	// Analitik gradyan: L = Σ_k <v_k, y_k> üzerinden geri yayılım.
	result, err := m.ApplyFunction(f, inputs, nil, 0)
	if err != nil {
		return nil, err
	}
	outputs, err := checkOutputs(fn, result)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(20240917))
	dirs := make([][]float64, len(outputs))
	for k, y := range outputs {
		dirs[k] = make([]float64, y.Numel())
		for j := range dirs[k] {
			if len(outputs) == 1 && y.Numel() == 1 {
				dirs[k][j] = 1
			} else {
				dirs[k][j] = rng.NormFloat64()
			}
		}
	}
	var loss *tensor.Tensor
	if _, err := safe(func() bool {
		for k, y := range outputs {
			if !y.RequiresGrad() {
				continue
			}
			v := tensor.FromFloat64(dirs[k], y.Shape()...).To(y.Device())
			p := y.Cast(tensor.Float32).Mul(v).SumAll()
			if loss == nil {
				loss = p
			} else {
				loss = loss.Add(p)
			}
		}
		return true
	}); err != nil {
		return nil, err
	}
	if loss != nil {
		if err := backprop(m, loss, nil); err != nil {
			return nil, err
		}
	}
	analytic := make([][]float64, len(copies))
	for i, c := range copies {
		if c == nil {
			continue
		}
		if g := c.Grad(); g != nil {
			analytic[i] = g.Float64s()
		} else {
			analytic[i] = make([]float64, c.Numel())
		}
	}

	// Sayısal gradyan: merkezi fark, kayıp float64'te.
	numericLoss := func() (float64, error) {
		var s interp.Value
		var err error
		tensor.NoGrad(func() { s, err = m.ApplyFunction(f, inputs, nil, 0) })
		if err != nil {
			return 0, err
		}
		ys, err := checkOutputs(fn, s)
		if err != nil {
			return 0, err
		}
		if len(ys) != len(dirs) {
			return 0, messages.New(messages.H6424, fn)
		}
		var top float64
		for k, y := range ys {
			if y.Numel() != len(dirs[k]) {
				return 0, messages.New(messages.H6425, fn)
			}
			for j, x := range y.Data() {
				top += float64(x) * dirs[k][j]
			}
		}
		return top, nil
	}
	type worst struct {
		input      int
		index      []int
		an, count  float64
		rate, diff float64
		wasFound   bool
	}
	var bad worst
	for i, c := range copies {
		if c == nil {
			continue
		}
		sh := c.Shape()
		data := c.Data()
		idx := make([]int, len(sh))
		for j, orig := range data {
			flatIndex(j, sh, idx)
			xp, xm := orig+float32(step), orig-float32(step)
			if err := setSafe(c, xp, idx); err != nil {
				return nil, err
			}
			lp, err := numericLoss()
			if err == nil {
				if err = setSafe(c, xm, idx); err == nil {
					var lm float64
					lm, err = numericLoss()
					if err == nil {
						num := (lp - lm) / (float64(xp) - float64(xm))
						an := analytic[i][j]
						diff := math.Abs(an - num)
						rate := diff / math.Max(1, math.Max(math.Abs(an), math.Abs(num)))
						if math.IsNaN(rate) {
							rate = math.Inf(1)
						}
						if !bad.wasFound || rate > bad.rate {
							bad = worst{input: i, index: append([]int(nil), idx...), an: an, count: num, rate: rate, diff: diff, wasFound: true}
						}
					}
				}
			}
			if serr := setSafe(c, orig, idx); err == nil {
				err = serr
			}
			if err != nil {
				return nil, err
			}
		}
	}
	if bad.rate > tol {
		return nil, messages.New(messages.H6432, fn, bad.input+1, shapeText(bad.index), fmt.Sprintf("%.6g", bad.an),
			fmt.Sprintf("%.6g", bad.count), fmt.Sprintf("%.3g", bad.diff), fmt.Sprintf("%g", tol))
	}
	return interp.True, nil
}

// checkOutputs f'nin dönüşünü tensör listesine çevirir (tek tensör ya da tensör listesi).
func checkOutputs(fn string, v interp.Value) ([]*tensor.Tensor, error) {
	switch x := v.(type) {
	case *TensorValue:
		return []*tensor.Tensor{x.t}, nil
	case *interp.ListVal:
		if len(x.Elements) == 0 {
			return nil, messages.New(messages.H6426, fn)
		}
		out := make([]*tensor.Tensor, len(x.Elements))
		for i, e := range x.Elements {
			d, ok := e.(*TensorValue)
			if !ok {
				return nil, messages.New(messages.H6427, fn, i+1, typeName(e))
			}
			out[i] = d.t
		}
		return out, nil
	}
	return nil, messages.New(messages.H6428, fn, shortForm(v))
}

// positiveFloat adlı pozitif sayısal argümanı okur.
func positiveFloat(fn string, named map[string]interp.Value, ad bind.Named, defaultVal float64) (float64, error) {
	v, ok := bind.GetNamed(named, ad)
	if !ok {
		return defaultVal, nil
	}
	f, ok := interp.ToFloat(v)
	if !ok || !(f > 0) || math.IsInf(f, 0) {
		return 0, messages.New(messages.H6429, fn, ad.TR, v.Inspect())
	}
	return f, nil
}

// flatIndex satır öncelikli düz indeksi çok boyutlu indekse çevirir.
func flatIndex(j int, sh []int, idx []int) {
	for a := len(sh) - 1; a >= 0; a-- {
		idx[a] = j % sh[a]
		j /= sh[a]
	}
}

func setSafe(t *tensor.Tensor, v float32, idx []int) error {
	_, err := safe(func() bool { t.Set(v, idx...); return true })
	return err
}
