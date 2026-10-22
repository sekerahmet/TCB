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
// (tensörlerin şekli, türü, cihazı, tam sayı işareti, aynı tensörün tekrarı; sayı argümanlarının
// değeri; hesap_türü()) ilk çağrı f'i olduğu gibi çalıştırır (ısınma: çekirdekler derlenir,
// kitaplık planları ve tembel kurulan durum, ör. KV önbelleği, hazırlanır), ikinci çağrı tensör
// argümanlarını kaydın kendi girdi tensörlerine kopyalayıp f'i kayıt altında çalıştırır, sonraki
// çağrılar girdileri kopyalayıp kaydı oynatır. f'in yerinde değiştirdiği girdiler her oynatmadan
// sonra çağıranın tensörüne geri yazılır. Çıktı her çağrıda kopyalanarak verilir (sonraki oynatma
// üzerine yazmaz). f doğrudan çağrılır: kayıt arka uçta yoksa (CPU), tensörler farklı
// cihazlardaysa, ayrı argümanlar aynı depolamanın farklı görünümleriyse (kopyalar bu bağı
// koparırdı) ve cihazda zaten bir kayıt sürüyorsa (iç içe tekrar_oynat: iç fonksiyonun işi
// dıştaki kayda girer, JAX'ta iç içe jit gibi). Kayıt sırasında yorumlayıcı kilidi başka göreve
// geçmez (VM.Exclusive): başka bir eşzamanlı görevin işi kayda girmez.

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

// replayEntry bir girdi bileşiminin kaydı. inputs her tensör argümanının kayıttaki karşılığıdır
// (aynı tensör iki kez verildiyse aynı kopya; alias[i] >= 0 o argümanın ilk geçtiği yer);
// mutated f'in yerinde değiştirdiği girdilerdir. Girdi ve çıktı başlıklarının her konumda bir
// sarıcısı kaydındır: kayıt bırakılınca her konum için bir kez bırakılır.
type replayEntry struct {
	warm    bool
	dev     tensor.Device
	rec     tensor.Recording
	inputs  []*tensor.Tensor
	alias   []int
	mutated []int
	outs    []*tensor.Tensor
	ints    []bool
	single  bool
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

// drainReleases kuyruktaki kayıtları bırakır (yorumlayıcının gorutininde). Cihazında kayıt
// süren kayıt kuyrukta kalır: bırakmak eşzamanlama ister.
func drainReleases() {
	releaseQueue.mu.Lock()
	entries := releaseQueue.entries
	releaseQueue.entries = nil
	releaseQueue.mu.Unlock()
	var keep []*replayEntry
	for _, e := range entries {
		if tensor.Capturing(e.dev) {
			keep = append(keep, e)
			continue
		}
		e.release()
		releasedRecordings.Add(1)
	}
	if len(keep) > 0 {
		releaseQueue.mu.Lock()
		releaseQueue.entries = append(releaseQueue.entries, keep...)
		releaseQueue.mu.Unlock()
	}
}

// release kaydı ve kaydın girdi ve çıktı başlıklarını bırakır.
func (e *replayEntry) release() {
	if e.rec != nil {
		e.rec.Release()
		e.rec = nil
	}
	for _, t := range e.inputs {
		t.Unwrap()
	}
	for _, t := range e.outs {
		t.Unwrap()
	}
	e.inputs, e.alias, e.outs = nil, nil, nil
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
	if tensor.GradEnabled() {
		return nil, messages.New(messages.H6440, fn)
	}
	var inputs []*tensor.Tensor
	var alias []int
	shared := false
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
		same := -1
		for j, t := range inputs {
			if t == d.t {
				same = j
				break
			}
			shared = shared || t.SharesStorage(d.t)
		}
		inputs, alias = append(inputs, d.t), append(alias, same)
		fmt.Fprintf(&key, "%v:%s:%v:%v:%d;", d.t.Shape(), d.t.DType(), d.t.Device(), d.hasInts, same)
	}
	rec := recorderOf(inputs)
	if rec == nil || shared {
		return m.ApplyFunction(r.f, args, nil, 0)
	}
	drainReleases()
	e := r.cache[key.String()]
	if e == nil {
		e = &replayEntry{dev: inputs[0].Device()}
		r.cache[key.String()] = e
	}
	if !e.warm {
		// Isınma bitmeden işaretlenmez: başka bir görev aynı bileşimi bu arada çağırırsa o da
		// ısınır (tembel kurulum yarım kalmış bir kayda girmez).
		res, err := m.ApplyFunction(r.f, args, nil, 0)
		if err == nil {
			e.warm = true
		}
		return res, err
	}
	if e.rec == nil {
		var err error
		m.Exclusive(fn, func() { err = e.capture(m, rec, r.f, args, alias) })
		if err != nil {
			delete(r.cache, key.String())
			return nil, err
		}
	} else {
		for i, t := range inputs {
			if alias[i] < 0 {
				e.inputs[i].CopyFrom(t)
			}
		}
	}
	e.rec.Replay()
	for _, i := range e.mutated {
		inputs[i].CopyFrom(e.inputs[i])
	}
	return e.result(), nil
}

// recorderOf tensör argümanlarının ortak cihazının kaydedicisini verir; argüman yoksa, cihazlar
// farklıysa, cihaz kaydı desteklemiyorsa ya da cihazda zaten bir kayıt sürüyorsa nil (f doğrudan
// çağrılır).
func recorderOf(inputs []*tensor.Tensor) tensor.Recorder {
	if len(inputs) == 0 {
		return nil
	}
	dev := inputs[0].Device()
	for _, t := range inputs[1:] {
		if t.Device() != dev {
			return nil
		}
	}
	rec, _ := tensor.BackendFor(dev).(tensor.Recorder)
	if rec == nil || !rec.CaptureSupported() || tensor.Capturing(dev) {
		return nil
	}
	return rec
}

// capture kaydın girdi tensörlerini kurar (argümanların kopyaları; aynı tensörün tekrarı aynı
// kopya) ve f'i kayıt altında çalıştırır. f'in hatası kaydı bırakıp olduğu gibi döner.
func (e *replayEntry) capture(m *interp.VM, rec tensor.Recorder, f interp.Value, args []interp.Value, alias []int) (err error) {
	fn := codes.DisplayName(codes.T313)
	vals := make([]interp.Value, len(args))
	var versions []uint64
	for i, a := range args {
		d, ok := a.(*TensorValue)
		if !ok {
			vals[i] = a // sayı: kayda sabit olarak girer
			continue
		}
		k := len(e.inputs)
		in := d.t
		if j := alias[k]; j >= 0 {
			in = e.inputs[j]
		} else {
			in = d.t.Clone()
		}
		// Değerin sarıcısı kaydındır: f'e geçen değer yorumlayıcıya kaçar, bırakılmaz.
		v := New(in)
		v.hasInts = d.hasInts
		vals[i] = v
		e.inputs, e.alias = append(e.inputs, in), append(e.alias, alias[k])
		versions = append(versions, in.Version())
	}
	e.mutated = nil
	defer func() {
		if p := recover(); p != nil {
			rec.AbortCapture()
			e.release()
			if interp.IsInterrupt(p) {
				panic(p)
			}
			err = panicError(p)
		}
	}()
	start := tensor.Created()
	rec.BeginCapture()
	res, err := m.ApplyFunction(f, vals, nil, 0)
	var outs []*tensor.Tensor
	var ints []bool
	single := false
	if err == nil {
		outs, ints, single, err = takeOutputs(fn, res, start)
	}
	if err != nil {
		rec.AbortCapture()
		e.release()
		return err
	}
	e.outs, e.ints, e.single = outs, ints, single
	for i, in := range e.inputs {
		if e.alias[i] < 0 && in.Version() != versions[i] {
			e.mutated = append(e.mutated, i)
		}
	}
	e.rec = rec.EndCapture()
	return nil
}

// takeOutputs f'in sonucunu çıktılara ayırır ve her konum için bir sarıcıyı kayda alır: f'in
// içinde oluşan başlığın kaçan değerinin sarıcısı zaten çağıranındır (ilk geçtiği konumda);
// önceden var olan başlık (girdi, dışarıdaki bir tensör) ya da tekrarı için yeni sarıcı alınır.
func takeOutputs(fn string, res interp.Value, start uint64) ([]*tensor.Tensor, []bool, bool, error) {
	outs, ints, single, err := replayOutputs(fn, res)
	if err != nil {
		return nil, nil, false, err
	}
	vals := []interp.Value{res}
	if l, ok := res.(*interp.ListVal); ok {
		vals = l.Elements
	}
	seen := map[*TensorValue]bool{}
	for _, el := range vals {
		v := el.(*TensorValue)
		if v.t.Serial() > start && !seen[v] {
			seen[v] = true
			continue
		}
		v.t.Wrap()
	}
	return outs, ints, single, nil
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
// Hedef bitişik olmalıdır; kaynağın eksen sayısı ve öbür eksenleri hedefinkiyle aynıdır
// (yayınlama yok). Bu denetimler kayıt içinde ve dışında aynıdır. Kayıt dışında konum konakta
// okunur ve denetlenir; kayıt içinde tensörle verilen konum cihazdan okunur (her oynatmada),
// sınır dışıysa kırpılır (lax.dynamic_update_slice gibi).
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
		return nil, messages.New(messages.H6444, fn, typeName(args[0]))
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
	if !dst.t.IsContiguous() {
		return nil, messages.New(messages.H6443, fn)
	}
	axis, err := writeSliceAxis(fn, named, dst.t, src)
	if err != nil {
		return nil, err
	}
	sh := dst.t.Shape()
	n := src.Shape()[axis]
	sv, ok := bind.GetNamed(named, nameStart)
	if !ok {
		sv = interp.NewInt(0)
	}
	st, isTensor := sv.(*TensorValue)
	if isTensor {
		if st.t.Dim() != 0 {
			return nil, messages.New(messages.H6441, fn, sv.Inspect())
		}
		if err := checkDevice(fn, dst.t, st.t); err != nil {
			return nil, err
		}
	}
	_, err = safe(func() *tensor.Tensor {
		if isTensor && tensor.Capturing(dst.t.Device()) {
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
		if start < 0 || start+n > sh[axis] {
			panic(messages.New(messages.H6442, fn, start, n, sh[axis]))
		}
		tensor.NoGrad(func() { dst.t.Slice(axis, start, start+n).CopyFrom(src) })
		return nil
	})
	if err != nil {
		return nil, err
	}
	return interp.Null, nil
}

// writeSliceAxis yerine_yaz'ın eksenini çözer (varsayılan son eksen) ve kaynağın o eksendeki
// dilime yazılabildiğini denetler (H3111).
func writeSliceAxis(fn string, named map[string]interp.Value, dst, src *tensor.Tensor) (int, error) {
	axis := -1
	if av, ok := bind.GetNamed(named, nameAxis); ok {
		a, integer := intNumber(av)
		if !integer {
			return 0, messages.New(messages.H6016, fn, av.Inspect(), shapeText(dst.Shape()), dst.Dim(), -dst.Dim(), dst.Dim()-1)
		}
		axis = a
	}
	sh := dst.Shape()
	axis, err := parseAxis(fn, axis, sh)
	if err != nil {
		return 0, err
	}
	if ssh := src.Shape(); !sliceFits(ssh, sh, axis) {
		return 0, messages.New(messages.H3111, fn, ssh, sh, axis)
	}
	return axis, nil
}

// sliceFits kaynak şeklinin hedefin eksenindeki bir dilime yazılabildiği: eksen sayısı ve
// öbür eksenler aynı, eksende kaynak hedeften uzun değil.
func sliceFits(src, dst []int, axis int) bool {
	if len(src) != len(dst) {
		return false
	}
	for i, n := range dst {
		if i == axis && src[i] > n || i != axis && src[i] != n {
			return false
		}
	}
	return true
}
