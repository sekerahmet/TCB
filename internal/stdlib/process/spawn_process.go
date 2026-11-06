package process

import (
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/sysmem"
)

// program_başlat (TR_fn_spawn_process, 21.2): dış programı başlatır, beklemeden Süreç verir. Başlatma ve
// durdurma TR_run_process'inkidir (job, 21.5); fark çıktının yoludur.
//
// Çıktı kendi borusundan okunur. StdoutPipe, Wait erken çağrılınca veriyi yitirir; bloklayan bir Writer
// da Wait'i kendi kopyalayıcısında asar [ölçüldü: DENETİM 233]. Bu yüzden Wait boruya dokunmaz ve
// WaitDelay'in stdout için yaptığını watch yapar.

var nameBufferBytes = bind.NamedArg(codes.ArgBufferBytes)

// physicalMemory makinenin belleğidir; testler küçültür.
var physicalMemory = sysmem.Physical

// memoryLimit bellek sınırıdır; okunamıyorsa false.
func memoryLimit() (int64, bool) {
	b, ok := physicalMemory()
	if !ok || b == 0 || b > 1<<62 {
		return 0, false
	}
	return int64(b), true
}

// bufferBytes TR_buffer_bytes: argümanını okur. Verilmezse 0. Ön ayırma yapılmaz: bellek denetimi bir
// kez, burada yapılır; sonra tampon bu sınırı aşmadığı için okuyucuda bellek hatası yolu yoktur (21.2).
func bufferBytes(fn any, named map[string]interp.Value) (int, error) {
	v, ok := bind.GetNamed(named, nameBufferBytes)
	if !ok {
		return 0, nil
	}
	n, ok := v.(*interp.IntVal)
	if !ok || n.Val <= 0 {
		return 0, messages.New(messages.H7520, fn, nameBufferBytes, interp.Shown(v))
	}
	if limit, known := memoryLimit(); known && n.Val > limit {
		return 0, messages.New(messages.H7521, fn, nameBufferBytes, n.Val, limit)
	}
	return int(n.Val), nil
}

// spawnProcess program_başlat(program, argümanlar) yerleşiğidir.
func (d *State) spawnProcess(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.FnSpawnProcess)
	line := m.BuiltinLine()
	if err := bind.CheckNamed(fn, named, nameEnv, nameExitCodes, nameBufferBytes); err != nil {
		return nil, err
	}
	call, err := readProgramCall(m, codes.FnSpawnProcess, args, named)
	if err != nil {
		return nil, err
	}
	buffer, err := bufferBytes(fn, named)
	if err != nil {
		return nil, err
	}
	p := &Process{state: d, call: call, line: line, buffer: buffer, tail: &stderrTail{longest: m.LongestSecret}}
	if err := p.start(); err != nil {
		return nil, messages.New(messages.H7501, fn, call.name, err)
	}
	return p, nil
}

// start boruları açar ve programı başlatır. Üst süreç kendi kopyalarını kapatır, başlatma hatasında
// iki uç da kapanır: yoksa dosya sonu hiç gelmez.
func (p *Process) start() error {
	stdin, input, err := os.Pipe()
	if err != nil {
		return err
	}
	output, stdout, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		_ = input.Close()
		return err
	}
	// exec.Command kullanılmaz: yolu yeniden arar (21.3, program_lookup.go).
	c := &exec.Cmd{Path: p.call.path, Args: append([]string{p.call.path}, p.call.args...), Env: p.call.env}
	c.Stdin, c.Stdout, c.Stderr = stdin, stdout, p.tail
	c.WaitDelay = stopGrace // yalnız standart hata içindir: Stdin ve Stdout dosyadır
	j, err := startJob(c)
	_ = stdin.Close()
	_ = stdout.Close()
	if err != nil {
		_ = input.Close()
		_ = output.Close()
		return err
	}
	limit := outputBuffer
	if p.buffer > 0 {
		limit = p.buffer
	}
	p.job, p.input = j, &programInput{w: input}
	p.out = newProgramOutput(limit, p.buffer > 0, j.stopping.Load, func() { go j.stop() })
	p.state.track(j)
	p.state.keep(p)
	pumped := make(chan struct{})
	go func() {
		defer close(pumped)
		p.out.pump(output)
	}()
	go p.watch(output, pumped)
	return nil
}

// watch program bitince işleri kapatır. Girdi kapanır: bekleyen TR_write döner. Sonra sınır kalkar ve
// okuyucu boruda kalanı alır; torun boruyu tutuyorsa dosya sonu gelmez ve okuma stopGrace sonunda
// kapanır. Tamponda bekleyen veri kesilmez: yavaş okuyan yine hepsini alır.
func (p *Process) watch(output *os.File, pumped <-chan struct{}) {
	<-p.job.done
	p.input.shut()
	p.out.programEnded(p.job.stopping.Load())
	p.state.untrack(p.job)
	p.job.release()
	p.settle()
	timer := time.NewTimer(stopGrace)
	defer timer.Stop()
	select {
	case <-pumped:
	case <-timer.C:
	}
	p.out.finish()
	_ = output.Close()
}

// programInput programın standart girdisidir. Yazmalar sıralıdır: iki görevin baytları karışmaz.
type programInput struct {
	w      *os.File
	mu     sync.Mutex
	once   sync.Once
	closed atomic.Bool // TR programı kapattı
}

// close TR_close_stdin ve TR_wait içindir.
func (i *programInput) close() {
	i.closed.Store(true)
	i.shut()
}

// shut girdiyi kapatır; bekleyen yazma hatayla döner.
func (i *programInput) shut() { i.once.Do(func() { _ = i.w.Close() }) }

// write bütün baytları yazar.
func (i *programInput) write(b []byte) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, err := i.w.Write(b)
	return err
}

// keep süreci program sonu için kaydeder: durdurulacak ya da hatası yazılacak (21.2).
func (d *State) keep(p *Process) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.processes == nil {
		d.processes = map[*Process]struct{}{}
	}
	d.processes[p] = struct{}{}
}

// forget hatası kalmayan süreci kayıttan siler.
func (d *State) forget(p *Process) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.processes, p)
}

// endProcesses program ya da test biterken çalışır: süren programlar 21.5'teki gibi durdurulur, hepsi
// aynı anda; görülmeyen hatalar yazılmak üzere verilir (21.2).
func (d *State) endProcesses(m *interp.VM) []interp.EndError {
	d.mu.Lock()
	processes := make([]*Process, 0, len(d.processes))
	for p := range d.processes {
		processes = append(processes, p)
	}
	d.processes = nil
	d.mu.Unlock()
	var wg sync.WaitGroup
	for _, p := range processes {
		wg.Go(p.job.stop)
	}
	wg.Wait()
	var out []interp.EndError
	for _, p := range processes {
		if err := p.unseenError(m.Mask); err != nil {
			out = append(out, interp.EndError{Line: p.line, Err: err})
		}
	}
	return out
}
