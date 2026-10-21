package logging

import (
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Yenileme sıklıkları: terminalde çubuk en çok saniyede 10 kez çizilir; terminal değilse
// (dosya, boru, not defteri hücresi) en çok 10 saniyede bir düz satır yazılır.
const (
	terminalInterval  = 100 * time.Millisecond
	plainLineInterval = 10 * time.Second
	// rateWindow adım hızının üstel ortalamasının zaman sabitidir (kalan süre bundan).
	rateWindow = 30.0 // saniye
	// rateSampleInterval hız ölçümleri arasındaki en kısa süredir (çok hızlı adımlarda
	// gürültüyü azaltır).
	rateSampleInterval = 0.5 // saniye
)

// barOrder çubukların açılış sırasıdır (program bitince sonra açılan önce biter).
var barOrder atomic.Int64

// metric çubukta gösterilen ve günlüğe yazılan bir ölçüdür.
type metric struct {
	name  string
	value interp.Value
}

// Progress TR'de açılmış bir ilerleme çubuğudur (tip İlerleme).
type Progress struct {
	m        *interp.VM
	d        *state
	w        io.Writer
	terminal bool
	width    int
	log      *Log
	jobPath  string // arka plan işinde ilerleme.json (yoksa boş)
	order    int64

	mu      sync.Mutex
	total   int64 // 0: toplamsız sayaç
	step    int64
	start   time.Time
	metrics []metric
	stopped bool

	rate     float64 // adım/sn (üstel ortalama; 0: henüz ölçülmedi)
	rateStep int64
	rateTime time.Time
	advanced bool // en az bir güncelleme yapıldı (ilk güncellemedeki sıçrama sürdürmedir)

	lastDraw  time.Time
	lastPlain time.Time
	lastJob   time.Time
	line      string      // terminalde çizilen satır (çıktı kilidi altında okunur)
	dirty     atomic.Bool // satır son güncellemeden eski (çıktı kilidi dışından da okunur)
	prev      *Progress
}

func (c *Progress) Type() interp.ValueType { return ProgressType }
func (c *Progress) IsTruthy() bool         { return true }
func (c *Progress) Inspect() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return messages.Text(messages.H7312, ProgressType, c.stepText())
	}
	return fmt.Sprintf("%s(%s)", ProgressType, c.stepText())
}

func (c *Progress) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case codes.Y010:
		return &interp.BuiltinVal{Name: codes.Name(codes.Y010), Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			return bind.Safe(interp.CallName(codes.Y010), func() (interp.Value, error) { return c.update(m, args, named) })
		}}, nil
	case codes.Y011:
		return &interp.BuiltinVal{Name: codes.Name(codes.Y011), NoNamedArgs: true, Fn: func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, interp.ArgCountError(codes.Y011, "0", len(args))
			}
			return interp.BoolToVal(c.finish(clock())), nil
		}}, nil
	}
	return nil, bind.MemberError(string(ProgressType), prop, codes.Names(codes.Y010, codes.Y011))
}

// openProgress ilerleme(toplam, günlük: g) yerleşiğidir.
func (d *state) openProgress(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G811)
	if err := bind.CheckNamed(fn, named, nameLog); err != nil {
		return nil, err
	}
	if len(args) > 1 {
		return nil, interp.ArgCountError(codes.G811, interp.Counts(0, 1), len(args), "", messages.Text(messages.H2322))
	}
	var total int64
	if len(args) == 1 && args[0] != interp.Null {
		n, ok := bind.Int(args[0])
		if !ok {
			return nil, interp.ArgTypeError(codes.G811, 1, args[0], codes.A200, codes.A204)
		}
		if n <= 0 {
			return nil, messages.New(messages.H7313, fn, n, codes.DisplayName(codes.A042))
		}
		total = n
	}
	c := &Progress{m: m, d: d, total: total, start: clock(), order: barOrder.Add(1)}
	if v, ok := bind.GetNamed(named, nameLog); ok && v != interp.Null {
		g, ok := v.(*Log)
		if !ok {
			return nil, messages.New(messages.H7314, fn, nameLog.TR, LogType, codes.DisplayName(codes.G810), interp.TypeAndSummary(v))
		}
		c.log = g
		g.setTotal(total)
	}
	c.setOutput()
	c.lastPlain, c.rateTime = c.start, c.start
	isOpen.Lock()
	openBars[c] = struct{}{}
	isOpen.Unlock()
	if c.terminal {
		c.prev = d.active
		d.active = c
		m.SetLiveLine(c)
	}
	return c, nil
}

// setOutput çubuğun yazacağı yeri ve kipini seçer: standart hata (VM.Err) terminalse canlı
// satır, değilse düz satırlar. Arka plan işinde (TR_IS_KLASORU) her zaman düz satır ve ilerleme.json.
func (c *Progress) setOutput() {
	c.w = c.m.Err
	if c.w == nil {
		c.w = os.Stderr
	}
	c.jobPath = jobStatusPath()
	if c.jobPath == "" {
		c.terminal, c.width = terminalChoice(c.w)
	}
}

// terminalChoice çıkışın terminal olup olmadığını ve genişliğini söyler (testler değiştirir).
var terminalChoice = func(w io.Writer) (bool, int) {
	if f, ok := w.(*os.File); ok && isTerminal(f) {
		return true, terminalWidth(f)
	}
	return false, 0
}

// update İlerleme.güncelle([ölçüler], adım: n) metodudur.
func (c *Progress) update(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.Y010)
	if err := bind.CheckNamed(fn, named, nameStep); err != nil {
		return nil, err
	}
	if len(args) > 1 {
		return nil, interp.ArgCountError(codes.Y010, interp.Counts(0, 1), len(args), "", messages.Text(messages.H2323))
	}
	var metrics []metric
	if len(args) == 1 && args[0] != interp.Null {
		s, ok := args[0].(*interp.DictVal)
		if !ok {
			return nil, interp.ArgTypeError(codes.Y010, 1, args[0], codes.A206)
		}
		record := interp.NewDictVal()
		if err := addMetrics(fn, record, s, keyKind, keyTime, keyStep, keyRate); err != nil {
			return nil, err
		}
		for _, k := range record.Order {
			metrics = append(metrics, metric{name: k, value: record.Pairs[k]})
		}
	}
	newStep := int64(-1)
	if v, ok := bind.GetNamed(named, nameStep); ok {
		n, ok := bind.Int(v)
		if !ok || n < 0 {
			return nil, messages.New(messages.H7315, fn, nameStep.TR, codes.DisplayName(codes.A200), interp.TypeAndSummary(v))
		}
		newStep = n
	}
	now := clock()
	done, err := c.advance(fn, now, newStep, metrics)
	if err != nil {
		return nil, err
	}
	if k := stepHook.Load(); k != nil {
		(*k)(StepBoundary{VM: m, Step: c.step, Total: c.total, Time: now})
	}
	if done {
		c.finish(now)
	}
	return interp.True, nil
}

// advance sayacı ilerletir, hızı ölçer, günlüğe yazar ve çubuğu (sıklık sınırıyla) çizer.
// Toplama ulaşıldıysa bitti doğrudur.
func (c *Progress) advance(fn string, now time.Time, newStep int64, metrics []metric) (bool, error) {
	c.mu.Lock()
	if c.stopped {
		text := c.stepText()
		c.mu.Unlock()
		return false, messages.New(messages.H7316, fn, text, codes.DisplayName(codes.G811))
	}
	measure := true
	switch {
	case newStep < 0:
		c.step++
	case c.total > 0 && newStep > c.total:
		c.mu.Unlock()
		return false, messages.New(messages.H7317, fn, nameStep.TR, newStep, c.total)
	case newStep < c.step || (!c.advanced && newStep != c.step+1):
		// Sıçrama (ilk güncellemede kaldığı yerden sürdürme ya da geri gitme): hız sıçramadan
		// sonra yeniden ölçülür. İleri atlamalar (her 10 adımda bir güncelleme) sıçrama değildir.
		c.step, c.rate, c.rateStep, c.rateTime = newStep, 0, newStep, now
	case newStep == c.step:
		measure = false // aynı adımın ikinci güncellemesi (ör. doğrulama kaybı): hız değişmez
	default:
		c.step = newStep
	}
	c.advanced = true
	if metrics != nil {
		c.metrics = metrics
	}
	if measure {
		c.measureRate(now)
	}
	done := c.total > 0 && c.step >= c.total
	record := c.logRecord(now)
	c.mu.Unlock()
	if record != nil {
		if err := c.log.write(record); err != nil {
			return false, err
		}
	}
	if !done {
		c.show(now)
	}
	return done, nil
}

// measureRate adım hızının üstel ortalamasını günceller; ölçümler en az rateSampleInterval
// arayla alınır (ilk ölçümden önce hız bilinmez: 0). c.mu tutulur.
func (c *Progress) measureRate(now time.Time) {
	dt := now.Sub(c.rateTime).Seconds()
	if dt < rateSampleInterval {
		return
	}
	instant := float64(c.step-c.rateStep) / dt
	if c.rate == 0 {
		c.rate = instant
	} else {
		a := 1 - math.Exp(-dt/rateWindow)
		c.rate += a * (instant - c.rate)
	}
	c.rateStep, c.rateTime = c.step, now
}

// logRecord bağlı günlüğe yazılacak ölçü kaydıdır (günlük yoksa nil). c.mu tutulur.
func (c *Progress) logRecord(now time.Time) *interp.DictVal {
	if c.log == nil {
		return nil
	}
	k := interp.NewDictVal()
	k.Set(keyTime, timeValue(now))
	k.Set(keyStep, interp.NewInt(c.step))
	for _, o := range c.metrics {
		k.Set(o.name, o.value)
	}
	if h := c.rate; h > 0 {
		k.Set(keyRate, &interp.FloatVal{Val: round(h)})
	}
	return k
}

// finish çubuğu bitirir: son satırı yazar, günlüğe bitiş kaydını ekler. İlk çağrıda doğru.
func (c *Progress) finish(now time.Time) bool {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return false
	}
	c.stopped = true
	last := c.lastLine(now)
	var record *interp.DictVal
	if c.log != nil {
		record = interp.NewDictVal()
		record.Set(keyKind, &interp.StringVal{Val: kindEnd})
		record.Set(keyTime, timeValue(now))
		record.Set(keyStep, interp.NewInt(c.step))
		record.Set(keyDuration, &interp.FloatVal{Val: round(now.Sub(c.start).Seconds())})
	}
	c.mu.Unlock()
	isOpen.Lock()
	delete(openBars, c)
	isOpen.Unlock()
	c.writeResult(last)
	if record != nil {
		_ = c.log.write(record) // program biterken de çağrılır; yazılamazsa sessiz kalır
	}
	c.writeJobStatus(now, true)
	return true
}

// round bir ölçümü günlük için 3 ondalık basamağa yuvarlar.
func round(f float64) float64 { return math.Round(f*1000) / 1000 }

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Progress)(nil)
	_ interp.LiveLine       = (*Progress)(nil)
)
