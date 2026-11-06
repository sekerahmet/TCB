package process

import (
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ProcessType program_başlat'ın verdiği değerin tip adıdır.
var ProcessType = interp.TypeName(codes.TypeProcess)

// Process başlatılmış bir dış programdır (21.2).
type Process struct {
	state  *State
	call   *runCall
	line   int // başlatıldığı satır
	buffer int // TR_buffer_bytes: değeri; 0 ise program dolunca bekler
	job    *job
	input  *programInput
	out    *programOutput
	tail   *stderrTail
	// limit süre sınırı dolan p.TR_wait'in değeridir (H2921).
	limit atomic.Pointer[interp.Value]
	// seen hatası bir çağrıya ulaştı: program biterken yazılmaz (10.6 gibi).
	seen atomic.Bool
}

func (p *Process) Type() interp.ValueType { return ProcessType }
func (p *Process) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı, her okuyucu için aynı.
func (p *Process) Inspect() string { return codes.ValueNames().Name(codes.TypeProcess) }

// finished program bitti mi?
func (p *Process) finished() bool {
	select {
	case <-p.job.done:
		return true
	default:
		return false
	}
}

var processMembers = []codes.Code{codes.MemRead, codes.MemWrite, codes.MemCloseStdin, codes.MemWait, codes.MemDone, codes.MemCancel}

// GetProperty süreç yöntemlerini verir.
func (p *Process) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	var fn interp.BuiltinFunc
	switch kd {
	case codes.MemRead:
		fn = p.read
	case codes.MemWrite:
		fn = p.write
	case codes.MemCloseStdin:
		fn = p.closeStdin
	case codes.MemWait:
		return &interp.BuiltinVal{Name: codes.Name(kd), Fn: p.wait}, nil
	case codes.MemDone:
		fn = p.done
	case codes.MemCancel:
		fn = p.cancel
	default:
		return nil, bind.MemberError(ProcessType, name, processMembers)
	}
	return &interp.BuiltinVal{Name: codes.Name(kd), NoNamedArgs: true, Fn: fn}, nil
}

// read p.TR_read(n)'dir: tam n bayt, dosya sonunda kalan. Yeni bir şey bekler (21.5): üstlenilmiş
// programda istekten sonra "durdurma istendi" hatasıyla döner, program ve baytlar kalır.
func (p *Process) read(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.MemRead)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.MemRead, 1, len(args))
	}
	v, ok := args[0].(*interp.IntVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.MemRead, 1, args[0], codes.TypeInt)
	}
	switch limit, known := memoryLimit(); {
	case v.Val <= 0:
		return nil, messages.New(messages.H7524, fn, v.Val)
	case p.buffer > 0 && v.Val > int64(p.buffer):
		return nil, messages.New(messages.H7523, fn, v.Val, p.buffer)
	case known && v.Val > limit:
		return nil, messages.New(messages.H7525, fn, v.Val, limit)
	}
	began := p.job.stopping.Load()
	r := p.out.read(int(v.Val))
	stop := func() {
		p.job.stop()
		p.out.abandon(r)
	}
	if err := m.AwaitExternalNew(fn, r.done, stop); err != nil {
		p.out.abandon(r)
		return nil, err
	}
	switch {
	case r.overflowed:
		return nil, p.overflowError()
	case r.stopped:
		return nil, p.stopError()
	case len(r.data) < r.n && !began && p.job.stopping.Load():
		// Bekleyen okumayı program durduruldu; dosya sonu öne geçmiş olabilir. Baytlar tamponda kalır:
		// durdurulmuş programdan yeni okuma kalanı verir.
		p.out.abandon(r)
		return nil, p.stopError()
	}
	return interp.NewBytes(r.data), nil
}

// overflowError taşma hatasıdır; yapışkandır, ulaştığı çağrı onu görmüş sayılır (21.2).
func (p *Process) overflowError() error {
	p.seen.Store(true)
	p.settle()
	return p.overflowMessage()
}

func (p *Process) overflowMessage() error {
	return messages.New(messages.H7522, interp.CallPrefix(codes.FnSpawnProcess), p.call.name, p.buffer, p.call.programText(), listText(p.call.args))
}

// stopError TR'nin durdurduğu programı bekleyenin hatasıdır: süre sınırında zaman aşımı, öbür
// durumlarda iptal (21.4).
func (p *Process) stopError() error {
	if limit := p.limit.Load(); limit != nil && p.job.timedOut.Load() {
		return messages.New(messages.H2921, interp.CallPrefix(codes.MemWait), interp.Shown(*limit), p.line)
	}
	return messages.New(messages.H2920, p.line)
}

// write p.TR_write(b)'dir: süren bir işi bitirir (21.5).
func (p *Process) write(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.MemWrite)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.MemWrite, 1, len(args))
	}
	b, ok := args[0].(*interp.BytesVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.MemWrite, 1, args[0], codes.TypeBytes)
	}
	if err := p.writeError(fn); err != nil {
		return nil, err
	}
	data := b.Data()
	done := make(chan struct{})
	var failed error
	go func() {
		defer close(done)
		failed = p.input.write(data)
	}()
	if err := m.AwaitExternal(done, p.job.stop); err != nil {
		return nil, err
	}
	if failed == nil {
		return interp.Null, nil
	}
	if err := p.writeError(fn); err != nil {
		return nil, err
	}
	return nil, messages.New(messages.H7526, fn, p.call.name) // boru kırıldı: program çıkıyor
}

// writeError yazılamayan programın hatasıdır: bitmiş programa yazmak hatadır (21.2).
func (p *Process) writeError(fn any) error {
	switch {
	case p.job.stopping.Load():
		return p.stopError()
	case p.finished():
		return messages.New(messages.H7526, fn, p.call.name)
	case p.input.closed.Load():
		return messages.New(messages.H7528, fn, p.call.name)
	}
	return nil
}

// closeStdin p.TR_close_stdin()'dir.
func (p *Process) closeStdin(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.MemCloseStdin, 0, len(args))
	}
	p.input.close()
	return interp.Null, nil
}

// wait p.TR_wait()'tir: girdiyi kapatır, program bitene kadar bekler, çıkış kodunu verir (21.4). Süren
// bir işi bitirir; süre sınırı dolarsa program durdurulur, durması beklenir (10.7).
func (p *Process) wait(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.MemWait)
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.MemWait, 0, len(args))
	}
	if err := bind.CheckNamed(fn, named, nameTimeout); err != nil {
		return nil, err
	}
	v, limited := bind.GetNamed(named, nameTimeout)
	var d time.Duration
	if limited {
		var err error
		if d, err = interp.Seconds(codes.MemWait, v); err != nil {
			return nil, err
		}
	}
	p.input.close()
	if limited {
		timer := time.AfterFunc(d, func() {
			p.limit.CompareAndSwap(nil, &v)
			p.job.stopFor(&p.job.timedOut)
		})
		defer timer.Stop()
	}
	if err := m.AwaitExternal(p.job.done, p.job.stop); err != nil {
		return nil, err
	}
	return p.result(m, fn)
}

// result biten programın sonucudur. Taşma programın çıkışından ayrıdır ve önce gelir (21.2).
func (p *Process) result(m *interp.VM, fn any) (interp.Value, error) {
	p.seen.Store(true)
	p.settle()
	if p.out.isOverflowed() {
		return nil, p.overflowError()
	}
	if p.job.stopping.Load() {
		return nil, p.stopError()
	}
	state := p.job.cmd.ProcessState
	if state == nil {
		return nil, messages.New(messages.H7501, fn, p.call.name, p.job.waitErr)
	}
	if err := p.call.endError(fn, state, p.tail.text(m.Mask)); err != nil {
		return nil, err
	}
	return interp.NewInt(int64(state.ExitCode())), nil
}

// done p.TR_done()'dur. Program bitmiş olsa da tamponda veri olabilir (21.2).
func (p *Process) done(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.MemDone, 0, len(args))
	}
	return interp.BoolToVal(p.finished()), nil
}

// cancel p.TR_cancel()'dır: programı 21.5'teki gibi durdurur, durmasını bekler; bitmiş programda
// etkisi yoktur. Taşma hatasını silmez.
func (p *Process) cancel(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.MemCancel, 0, len(args))
	}
	m.Blocking(p.job.stop)
	return interp.Null, nil
}

// unseenError program biterken yazılacak hatadır: görülmemiş taşma ya da hiç beklenmemiş programın
// kabul edilmeyen bitişi. TR'nin durdurduğu program hata yazmaz (21.2).
func (p *Process) unseenError(mask func(string) string) error {
	if p.seen.Load() {
		return nil
	}
	fn := interp.CallPrefix(codes.FnSpawnProcess)
	if p.out.isOverflowed() {
		return p.overflowMessage()
	}
	if p.job.stopping.Load() || p.job.cmd.ProcessState == nil {
		return nil
	}
	return p.call.endError(fn, p.job.cmd.ProcessState, p.tail.text(mask))
}

// settle biten ve yazılacak hatası kalmayan süreci kayıttan siler: uzun süren program yeniden başlattığı
// süreçleri biriktirmez.
func (p *Process) settle() {
	if !p.finished() {
		return
	}
	if p.unseenError(func(s string) string { return s }) == nil {
		p.state.forget(p)
	}
}

// Derleme anı denetimi: yorumlayıcı üyeleri tip denetimiyle bulur.
var _ interp.PropertyGetter = (*Process)(nil)
