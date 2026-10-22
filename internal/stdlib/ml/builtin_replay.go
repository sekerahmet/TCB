package ml

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bu dosya tekrar_oynat(f)'i (dil tanımı 15.48) ve yerine_yaz'ı TR'ye bağlar.
//
// tekrar_oynat(f) f'in cihaz işini bir kez kaydedip sonraki çağrılarda oynatır (GPU'da CUDA
// Graph; PyTorch torch.cuda.graph ve make_graphed_callables gibi). Girdi bileşimi başına
// (tensörlerin şekli, türü, cihazı, tam sayı işareti; sayı argümanlarının değeri; hesap_türü())
// ilk çağrı f'i olduğu gibi çalıştırır (ısınma: çekirdekler derlenir, kitaplık planları ve
// tembel kurulan durum, ör. KV önbelleği, hazırlanır), ikinci çağrı tensör argümanlarını
// kaydın kendi girdi tensörlerine kopyalayıp f'i kayıt altında çalıştırır, sonraki çağrılar
// girdileri kopyalayıp kaydı oynatır. Çıktı her çağrıda kopyalanarak verilir (sonraki oynatma
// üzerine yazmaz). Kayıt arka uçta yoksa (CPU) f doğrudan çağrılır.

// registerReplay tekrar_oynat ve yerine_yaz yerleşiklerini kaydeder.
func registerReplay(v *interp.VM) {
	wrap(v, codes.T313, newBuiltin(replayBuiltin))
	wrap(v, codes.T122, newBuiltin(writeSliceBuiltin))
}

// replayFn tekrar_oynat'ın döndürdüğü fonksiyonun durumu.
type replayFn struct {
	f     interp.Value
	cache map[string]*replayEntry
}

// replayEntry bir girdi bileşiminin kaydı: girdi ve çıktı tensörleri kaydın kendisinindir
// (başlıkları sarılı tutulur, düşmez).
type replayEntry struct {
	warm   bool
	rec    tensor.Recording
	inputs []*tensor.Tensor
	outs   []*tensor.Tensor
	ints   []bool
	single bool
}

// releaseQueue çöp toplayıcının ulaşılamaz bulduğu tekrar_oynat fonksiyonlarının kayıtları.
// Temizlik ayrı bir gorutinde çalışır; kaydı bırakmak ise arka ucun kilidini alır ve cihazı
// eşzamanlar (kayıt sürerken yapılamaz), girdi ve çıktı başlıklarını düşürmek de yorumlayıcının
// işidir. Bu yüzden kayıtlar kuyruğa girer ve sonraki tekrar_oynat çağrısı, kayda başlamadan
// önce onları bırakır (üret her çağrıda yeni bir tekrar_oynat kurar: kuyruk olmasa her çağrının
// kaydı ve kayda özel belleği süreç boyunca kalırdı).
var releaseQueue struct {
	mu      sync.Mutex
	entries []*replayEntry
}

// releasedRecordings bırakılan kayıt sayısı (sınama).
var releasedRecordings atomic.Int64

// queueRelease ulaşılamaz bir fonksiyonun kayıtlarını kuyruğa koyar (temizlik gorutini).
func queueRelease(cache map[string]*replayEntry) {
	releaseQueue.mu.Lock()
	defer releaseQueue.mu.Unlock()
	for _, e := range cache {
		if e.rec != nil {
			releaseQueue.entries = append(releaseQueue.entries, e)
		}
	}
}

// drainReleases kuyruktaki kayıtları bırakır: çizge ve kayda özel bellek, girdi ve çıktı
// başlıkları (yorumlayıcının gorutininde, kayıt dışında).
func drainReleases() {
	releaseQueue.mu.Lock()
	entries := releaseQueue.entries
	releaseQueue.entries = nil
	releaseQueue.mu.Unlock()
	for _, e := range entries {
		e.rec.Release()
		for _, t := range append(e.inputs, e.outs...) {
			t.Unwrap()
		}
		releasedRecordings.Add(1)
	}
}

func replayBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T313)
	if len(named) > 0 {
		return nil, interp.NoNamedArgsError(fn, named)
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.T313, "1", len(args))
	}
	if !isCallable(args[0]) {
		return nil, messages.New(messages.H6437, fn, typeName(args[0]))
	}
	r := &replayFn{f: args[0], cache: map[string]*replayEntry{}}
	runtime.AddCleanup(r, queueRelease, r.cache)
	return &interp.BuiltinVal{Name: codes.Name(codes.T313), Fn: guarded(r.call), NoNamedArgs: true}, nil
}

// call oynatılan fonksiyonu çalıştırır (bkz. dosya başı).
func (r *replayFn) call(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T313)
	drainReleases()
	if tensor.GradEnabled() {
		return nil, messages.New(messages.H6440, fn)
	}
	var inputs []*tensor.Tensor
	var key strings.Builder
	fmt.Fprintf(&key, "%s|", mixedDType(m))
	for i, a := range args {
		if x, ok := interp.ToFloat(a); ok {
			fmt.Fprintf(&key, "#%T%v;", a, x)
			continue
		}
		d, ok := a.(*TensorValue)
		if !ok {
			return nil, messages.New(messages.H6438, fn, i+1, typeName(a))
		}
		inputs = append(inputs, d.t)
		fmt.Fprintf(&key, "%v:%s:%v:%v;", d.t.Shape(), d.t.DType(), d.t.Device(), d.hasInts)
	}
	var rec tensor.Recorder
	if len(inputs) > 0 {
		rec, _ = tensor.BackendFor(inputs[0].Device()).(tensor.Recorder)
	}
	if rec == nil || !rec.CaptureSupported() {
		return m.ApplyFunction(r.f, args, nil, 0)
	}
	e := r.cache[key.String()]
	if e == nil {
		e = &replayEntry{}
		r.cache[key.String()] = e
	}
	if !e.warm {
		e.warm = true
		return m.ApplyFunction(r.f, args, nil, 0)
	}
	if e.rec == nil {
		if err := e.capture(m, rec, r.f, args); err != nil {
			delete(r.cache, key.String())
			return nil, err
		}
	} else {
		for i, t := range inputs {
			e.inputs[i].CopyFrom(t)
		}
	}
	e.rec.Replay()
	return e.result(), nil
}

// capture kaydın girdi tensörlerini kurar (argümanların kopyaları) ve f'i kayıt altında
// çalıştırır. f'in hatası kaydı bırakıp olduğu gibi döner.
func (e *replayEntry) capture(m *interp.VM, rec tensor.Recorder, f interp.Value, args []interp.Value) error {
	fn := codes.DisplayName(codes.T313)
	vals := make([]interp.Value, len(args))
	for i, a := range args {
		d, ok := a.(*TensorValue)
		if !ok {
			vals[i] = a // sayı: kayda sabit olarak girer
			continue
		}
		in := d.t.Clone()
		in.Wrap() // kaydın girdisi: yorumlayıcı argümanı bıraksa da düşmez
		e.inputs = append(e.inputs, in)
		v := New(in)
		v.hasInts = d.hasInts
		vals[i] = v
	}
	var res interp.Value
	err := func() (err error) {
		rec.BeginCapture()
		defer func() {
			if p := recover(); p != nil {
				rec.AbortCapture()
				if interp.IsInterrupt(p) {
					panic(p)
				}
				err = panicError(p)
			}
		}()
		if res, err = m.ApplyFunction(f, vals, nil, 0); err != nil {
			rec.AbortCapture()
			return err
		}
		outs, ints, single, err := replayOutputs(fn, res)
		if err != nil {
			rec.AbortCapture()
			return err
		}
		for _, o := range outs {
			o.Wrap() // kaydın çıktısı: her oynatma buraya yazar
		}
		e.outs, e.ints, e.single = outs, ints, single
		e.rec = rec.EndCapture()
		return nil
	}()
	if err != nil {
		for _, in := range e.inputs {
			in.Unwrap()
		}
		e.inputs = nil
	}
	return err
}

// replayOutputs f'in sonucunu (tensör ya da tensör listesi) çıktılara ayırır.
func replayOutputs(fn string, res interp.Value) (outs []*tensor.Tensor, ints []bool, single bool, err error) {
	switch x := res.(type) {
	case *TensorValue:
		return []*tensor.Tensor{x.t}, []bool{x.hasInts}, true, nil
	case *interp.ListVal:
		for _, el := range x.Elements {
			d, ok := el.(*TensorValue)
			if !ok {
				return nil, nil, false, messages.New(messages.H6439, fn, typeName(res))
			}
			outs, ints = append(outs, d.t), append(ints, d.hasInts)
		}
		if len(outs) > 0 {
			return outs, ints, false, nil
		}
	}
	return nil, nil, false, messages.New(messages.H6439, fn, typeName(res))
}

// result kaydın çıktılarının kopyalarını TR değeri olarak verir.
func (e *replayEntry) result() interp.Value {
	el := make([]interp.Value, len(e.outs))
	for i, o := range e.outs {
		v := New(o.Clone())
		v.hasInts = e.ints[i]
		el[i] = v
	}
	if e.single {
		return el[0]
	}
	return &interp.ListVal{Elements: el}
}

var nameStart = bind.NamedArg(codes.T908)

// writeSliceBuiltin yerine_yaz(hedef, kaynak, eksen:, başlangıç:): kaynağı hedefin eksenindeki
// dilime başlangıçtan itibaren yerinde yazar. Başlangıç sayı ya da 0 boyutlu tam sayı tensörüdür.
// Kayıt dışında konum konakta okunur ve denetlenir; kayıt içinde tensörle verilen konum
// cihazdan okunur (her oynatmada), sınır dışıysa kırpılır (lax.dynamic_update_slice gibi).
func writeSliceBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T122)
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.T122, "2", len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameAxis, nameStart); err != nil {
		return nil, err
	}
	dst, ok := args[0].(*TensorValue)
	if !ok {
		return nil, messages.New(messages.H6324, fn, "hedef", typeName(args[0]))
	}
	src, err := toTensorOn(fn, args[1], dst.t)
	if err != nil {
		return nil, err
	}
	if err := checkDevice(fn, dst.t, src); err != nil {
		return nil, err
	}
	if dst.t.RequiresGrad() && tensor.GradEnabled() {
		return nil, messages.New(messages.H6716)
	}
	axis := -1
	if av, ok := bind.GetNamed(named, nameAxis); ok {
		a, integer := intNumber(av)
		if !integer {
			return nil, messages.New(messages.H6016, fn, av.Inspect(), shapeText(dst.t.Shape()), dst.t.Dim(), -dst.t.Dim(), dst.t.Dim()-1)
		}
		axis = a
	}
	sh := dst.t.Shape()
	if axis, err = parseAxis(fn, axis, sh); err != nil {
		return nil, err
	}
	sv, ok := bind.GetNamed(named, nameStart)
	if !ok {
		sv = interp.NewInt(0)
	}
	st, isTensor := sv.(*TensorValue)
	if isTensor && st.t.Dim() != 0 {
		return nil, messages.New(messages.H6441, fn, sv.Inspect())
	}
	n := src.Shape()[min(axis, max(src.Dim()-1, 0))]
	if src.Dim() != dst.t.Dim() {
		n = -1
	}
	_, err = safe(func() *tensor.Tensor {
		if isTensor && tensor.Capturing(dst.t.Device()) {
			if err := checkDevice(fn, dst.t, st.t); err != nil {
				panic(err)
			}
			tensor.UpdateSliceAt(dst.t, src, st.t, axis)
			return nil
		}
		start, integer := 0, true
		if isTensor {
			x := st.t.Item()
			start, integer = int(x), float32(int(x)) == x
		} else {
			start, integer = intNumber(sv)
		}
		if !integer {
			panic(messages.New(messages.H6441, fn, sv.Inspect()))
		}
		if n < 0 || start < 0 || start+n > sh[axis] {
			panic(messages.New(messages.H6442, fn, start, max(n, 0), sh[axis]))
		}
		tensor.NoGrad(func() { dst.t.Slice(axis, start, start+n).CopyFrom(src) })
		return nil
	})
	if err != nil {
		return nil, err
	}
	return interp.Null, nil
}
