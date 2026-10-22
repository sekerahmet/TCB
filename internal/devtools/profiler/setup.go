package profiler

import (
	"sort"
	"strings"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Anlık görüntünün (Goruntu) ham ölçülerden kurulması: süreç başından beri biriken
// değerler ve son pencerenin (en çok windowTime) farkları.

// Listelerin uzunlukları: dosyaya yazılan görüntü ekrandakinden fazlasını taşır.
const (
	maxOps       = 30
	maxImplicit  = 30
	maxLines     = 30
	maxFunctions = 20
)

// timeFormat görüntüdeki zaman damgalarıdır (günlükle aynı biçim).
const timeFormat = "2006-01-02T15:04:05.000Z07:00"

// setup a anından (once pencerenin başı) bir görüntü kurar; son ise program bitmiştir.
func (p *Profile) setup(a snap, before *snap, last bool) *Snapshot {
	elapsed := a.time2.Sub(p.start)
	g := &Snapshot{
		Format: FormatName, Version: FormatVersion, Kind: "canlı", Program: p.pick.Program,
		Start: p.start.Format(timeFormat), Time: a.time2.Format(timeFormat),
		Seconds: sn(elapsed), Done: last,
	}
	if last {
		g.Kind = "rapor"
	}
	// Tensör sayaçları ve çöp toplayıcı süreç geneldir: profil başladığından beri olanlar.
	g.Step = stepInfo(a.step)
	// Sayaçlar profil başından beridir: bütün adımları (ilk adım sınırından öncekini de) içerir.
	g.Tensor = p.tensorInfo(a.counters, p.base.counters, elapsed, a.step.all)
	g.Tensor.Implicit = implicitInfo(a.implicit, p.base.implicit)
	g.GPU = p.gpuInfo(a.gpu)
	g.Memory = p.memoryInfo(a)
	g.GC = GCInfo{Count: a.memory.NumGC - p.base.memory.NumGC,
		PauseSec: float64(a.memory.PauseTotalNs-p.base.memory.PauseTotalNs) / 1e9}
	if p.smp != nil {
		g.Sampling = p.samplingInfo(a.positions, nil, a.sampleTime, a.sample)
		if last {
			g.gpuPeak, g.gpuPeakTotal = p.gpuPeakLines()
		}
	}
	if !last && before != nil && a.time2.Sub(before.time2) > 0 {
		g.Window = p.window(a, before)
	}
	return g
}

// stepInfo adım sınırlarından ms/adım hesaplar; adım yoksa nil.
func stepInfo(d stepState) *StepInfo {
	if !d.var_ {
		return nil
	}
	b := &StepInfo{Count: d.last, Total: d.total, All: d.all, Source: "ilerleme_cubugu"}
	if d.fromOptimizer {
		b.Source = "iyilestirici"
	}
	// Süre ilk adım sınırından sonuncusuna kadardır: aradaki adımlara bölünür.
	if n := d.all - d.allAtFirst; n > 0 {
		s := d.lastTime.Sub(d.firstTime)
		b.MsPerStep = float64(s.Nanoseconds()) / 1e6 / float64(n)
		b.StepsPerSec = ratio(float64(n), s.Seconds())
		perStep := func(x time.Duration) float64 { return float64(x.Nanoseconds()) / 1e6 / float64(n) }
		b.ForwardMs, b.BackwardMs, b.AfterMs = perStep(d.forward), perStep(d.backward), perStep(d.after)
	}
	return b
}

// window son pencerenin farklarıdır.
func (p *Profile) window(a snap, before *snap) *WindowInfo {
	dur := a.time2.Sub(before.time2)
	w := &WindowInfo{Seconds: sn(dur)}
	if a.step.var_ {
		// Pencerede tamamlanan adımlar (sayaç farkları bunlara bölünür); süre pencere içindeki
		// ilk adım sınırından (önceki görüntüde sınır yoksa programın ilk sınırından) sonuncusuna.
		w.Step = a.step.all - before.step.all
		startAll, startTime := before.step.all, before.step.lastTime
		if !before.step.var_ {
			startAll, startTime = a.step.allAtFirst, a.step.firstTime
		}
		if n := a.step.all - startAll; n > 0 {
			s := a.step.lastTime.Sub(startTime)
			w.MsPerStep = float64(s.Nanoseconds()) / 1e6 / float64(n)
			w.StepsPerSec = ratio(float64(n), s.Seconds())
		}
	}
	t := p.tensorInfo(a.counters, before.counters, dur, w.Step)
	w.TensorShare, w.Ops = t.Share, t.Ops
	if p.smp != nil {
		o := p.samplingInfo(a.positions, before.positions, a.sampleTime-before.sampleTime, a.sample-before.sample)
		w.Lines = o.Lines
	}
	w.CPUTensorAlloc = cpuBytes(a.counters, before.counters)
	gpuWindow(w, a.gpu, before.gpu, dur)
	w.GCCount = a.memory.NumGC - before.memory.NumGC
	w.PauseSec = float64(a.memory.PauseTotalNs-before.memory.PauseTotalNs) / 1e9
	return w
}

// counterKey bir sayaç kaydının kimliğidir.
type counterKey struct {
	c tensor.DeviceKind
	g bool
	i tensor.OpKind
}

// tensorInfo sayaçlardan (eksi varsa onların) işlem listesini kurar; dur payların
// paydasıdır, steps sayaçların kapsadığı adımlardır (0'dan büyükse adım başına süre eklenir).
func (p *Profile) tensorInfo(counters, minus []tensor.CounterRecord, dur time.Duration, steps int64) TensorInfo {
	prev := map[counterKey]tensor.CounterRecord{}
	for _, k := range minus {
		prev[counterKey{k.Device, k.Backward, k.OpKind}] = k
	}
	var t TensorInfo
	for _, k := range counters {
		o := prev[counterKey{k.Device, k.Backward, k.OpKind}]
		k.Calls, k.Measured, k.Duration, k.Bytes = k.Calls-o.Calls, k.Measured-o.Measured, k.Duration-o.Duration, k.Bytes-o.Bytes
		if k.Calls == 0 && k.Bytes == 0 {
			continue
		}
		b := OpInfo{
			Code: k.OpKind.Code(), Name: fullName(k.OpKind, k.Backward, k.Calls), Direction: "ileri", Device: "cpu",
			Calls: k.Calls, Measured: k.Measured, Seconds: sn(k.Duration), Bytes: k.Bytes,
		}
		if k.Backward {
			b.Direction = "geri"
		}
		if k.Device != tensor.KindCPU {
			b.Device = "gpu"
		}
		if steps > 0 {
			ms := b.Seconds * 1000 / float64(steps)
			b.MsPerStep = &ms
		}
		t.Seconds += b.Seconds
		t.Calls += b.Calls
		t.Bytes += b.Bytes
		t.Ops = append(t.Ops, b)
	}
	for i := range t.Ops {
		t.Ops[i].Share = ratio(t.Ops[i].Seconds, t.Seconds)
	}
	sort.SliceStable(t.Ops, func(i, j int) bool { return t.Ops[i].Seconds > t.Ops[j].Seconds })
	if len(t.Ops) > maxOps {
		t.Ops = t.Ops[:maxOps]
	}
	t.Share = ratio(t.Seconds, sn(dur))
	return t
}

// implicitKey bir örtük iş kaydının kimliğidir.
type implicitKey struct {
	c tensor.DeviceKind
	g bool
	p tensor.OpKind
	k tensor.ImplicitKind
}

// implicitInfo örtük iş kayıtlarından (eksi varsa farkından) listeyi kurar; işlemin kendisi
// olan kopyalar (.kopya()'nın kopyası, .türe'nin dönüşümü) işlem tablosunda zaten vardır,
// alınmaz.
func implicitInfo(records, minus []tensor.ImplicitRecord) []ImplicitInfo {
	prev := map[implicitKey]tensor.ImplicitRecord{}
	for _, r := range minus {
		prev[implicitKey{r.Device, r.Backward, r.Parent, r.Kind}] = r
	}
	var out []ImplicitInfo
	for _, r := range records {
		if explicitCopy(r.Parent, r.Kind) {
			continue
		}
		o := prev[implicitKey{r.Device, r.Backward, r.Parent, r.Kind}]
		r.Calls, r.Elems, r.Measured, r.Duration = r.Calls-o.Calls, r.Elems-o.Elems, r.Measured-o.Measured, r.Duration-o.Duration
		if r.Calls == 0 {
			continue
		}
		b := ImplicitInfo{
			Code: r.Kind.Code(), Parent: r.Parent.Code(), Direction: "ileri", Device: "cpu",
			Name: implicitName(r.Kind), ParentName: fullName(r.Parent, r.Backward, 1),
			Calls: r.Calls, Elements: r.Elems, Measured: r.Measured, Seconds: sn(r.Duration),
		}
		if r.Backward {
			b.Direction = "geri"
		}
		if r.Device != tensor.KindCPU {
			b.Device = "gpu"
		}
		out = append(out, b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Elements > out[j].Elements })
	if len(out) > maxImplicit {
		out = out[:maxImplicit]
	}
	return out
}

// explicitCopy örtük işin, üst işlemin kendi işi olduğudur.
func explicitCopy(parent tensor.OpKind, k tensor.ImplicitKind) bool {
	switch parent {
	case tensor.OpKindCopy:
		return k == tensor.ImplicitCopy || k == tensor.ImplicitStrided
	case tensor.OpKindCast:
		return k == tensor.ImplicitCast
	}
	return false
}

// samplingInfo konum örneklerinden (eksi varsa farkından) satır ve fonksiyon listesini
// kurar.
func (p *Profile) samplingInfo(positions, minus []posSample, total time.Duration, sample int64) *SamplingInfo {
	type key struct {
		f   string
		src *interp.Source
		s   int
	}
	prev := map[key]posSample{}
	for _, k := range minus {
		prev[key{k.function, k.src, k.line}] = k
	}
	o := &SamplingInfo{IntervalMs: float64(sampleInterval) / float64(time.Millisecond), Samples: sample, Seconds: sn(total)}
	fn2 := map[string]time.Duration{}
	var inTensor time.Duration
	for _, k := range positions {
		e := prev[key{k.function, k.src, k.line}]
		k.dur, k.tensor, k.back = k.dur-e.dur, k.tensor-e.tensor, k.back-e.back
		if k.dur <= 0 {
			continue
		}
		inTensor += k.tensor
		fn2[k.function] += k.dur
		o.Lines = append(o.Lines, p.lineInfo(k, total))
	}
	o.TensorShare = ratio(sn(inTensor), sn(total))
	sort.SliceStable(o.Lines, func(i, j int) bool { return o.Lines[i].Seconds > o.Lines[j].Seconds })
	if len(o.Lines) > maxLines {
		o.Lines = o.Lines[:maxLines]
	}
	for ad, s := range fn2 {
		o.Functions = append(o.Functions, FunctionInfo{Name: ad, Seconds: sn(s), Share: ratio(sn(s), sn(total))})
	}
	sort.Slice(o.Functions, func(i, j int) bool {
		if o.Functions[i].Seconds != o.Functions[j].Seconds {
			return o.Functions[i].Seconds > o.Functions[j].Seconds
		}
		return o.Functions[i].Name < o.Functions[j].Name
	})
	if len(o.Functions) > maxFunctions {
		o.Functions = o.Functions[:maxFunctions]
	}
	return o
}

// lineInfo bir konumun satırıdır: dosyası ('aktar' ile yüklenen modülde modülün) ve metniyle.
func (p *Profile) lineInfo(k posSample, total time.Duration) LineInfo {
	s := LineInfo{
		Line: k.line, Function: k.function, Seconds: sn(k.dur),
		Share: ratio(sn(k.dur), sn(total)), TensorShare: ratio(sn(k.tensor), sn(k.dur)),
		BackwardShare: ratio(sn(k.back), sn(k.dur)),
	}
	lines := p.lines
	s.File = p.pick.Program
	if k.src != nil {
		s.File, lines = k.src.Name, p.moduleLines(k.src)
	}
	if k.line >= 1 && k.line <= len(lines) {
		s.Text = strings.TrimSpace(strings.TrimSuffix(lines[k.line-1], "\r"))
	}
	return s
}

// moduleLines bir modülün satırlarıdır (ilk istekte bölünüp saklanır).
func (p *Profile) moduleLines(src *interp.Source) []string {
	p.isLines.Lock()
	defer p.isLines.Unlock()
	lines, ok := p.modLines[src]
	if !ok {
		lines = strings.Split(src.Code, "\n")
		if p.modLines == nil {
			p.modLines = map[*interp.Source][]string{}
		}
		p.modLines[src] = lines
	}
	return lines
}

// memoryInfo konak belleğidir; CPU tensör ayırması profil başından beri.
func (p *Profile) memoryInfo(a snap) MemoryInfo {
	return MemoryInfo{GoHeap: a.memory.HeapAlloc, GoSys: a.memory.Sys,
		CPUTensorAlloc: cpuBytes(a.counters, p.base.counters)}
}

// cpuBytes CPU tensör işlemlerinin (eksi'den bu yana) ayırdığı bayttır.
func cpuBytes(counters, minus []tensor.CounterRecord) int64 {
	var n int64
	for _, k := range counters {
		if k.Device == tensor.KindCPU {
			n += k.Bytes
		}
	}
	for _, k := range minus {
		if k.Device == tensor.KindCPU {
			n -= k.Bytes
		}
	}
	return n
}
