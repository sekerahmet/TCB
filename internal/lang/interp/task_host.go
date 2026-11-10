package interp

import (
	"slices"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- DIŞARIDAN AÇILAN GÖREVLER (10.1, 28.4) ---
//
// Bir motor (HTTP sunucusu) TR görevini kilidi tutmayan bir goroutine'den açar. TaskHost bu görevlerin
// kaynağıdır: NewTaskHost anında şablon görünüm alınır, kancalar o an çalışır, her görev şablondan
// çatallanır (10.4); süren görevler yaşayan görevlere sayılır, program sonunda AddMainEnd'in işi
// Close ve Wait ile onları bitirir; iptal ağacında ebeveynleri host'tur (10.9); hata Run'dan döner,
// yazılmaz, çıkış kodu değişmez (10.6); Go paniği iç hata iletisine çevrilir.

// ErrHostClosed Run'ın kapanmış host'taki hatasıdır: görev açılmadı.
var ErrHostClosed error = messages.New(messages.H7601)

// TaskHost dışarıdan açılan görevlerin kaynağıdır.
type TaskHost struct {
	template *VM
	owner    *TaskVal
	mu       sync.Mutex
	closed   bool
	running  sync.WaitGroup
}

// NewTaskHost şablonu alır; çağıran kilidi tutar. line görevlerin satırıdır (iptal iletisi).
func (v *VM) NewTaskHost(line int) *TaskHost {
	return &TaskHost{template: v.NewTaskVM(), owner: newTask(line, false)}
}

// forkView v'den çatallanan görünümdür, kancalar çalışmadan. Görev başlatıldığı yerin dosyasında ve
// aktarım zincirinde başlar (modül yüklenirken başlatılan görev o modülü aktarırsa bu döngüdür).
func (v *VM) forkView() *VM {
	return &VM{vmShared: v.vmShared, CallStack: v.StackSnapshot(), asyncTask: true,
		source: v.source, loading: slices.Clip(v.loading)}
}

// Run fn'i yeni bir görevde, kilidi tutarak çalıştırır ve bitmesini bekler; kilidi tutmayan bir
// goroutine'den çağrılır. Görev iptal edildiyse ya da program durdurulduysa hata ErrInterrupted'dır
// (cevap gönderilmez); host kapandıysa ErrHostClosed.
func (h *TaskHost) Run(fn func(task *VM) (Value, error)) (Value, error) { return h.RunUntil(nil, fn) }

// RunUntil Run gibidir; stop kapanırsa görev iptal edilir (10.9): istemcisi kopan akışın bloğu
// (28.7). stop nil ise Run'dır. İptali kilidi alan bir izleyici goroutine yapar; görev bitince çıkar.
func (h *TaskHost) RunUntil(stop <-chan struct{}, fn func(task *VM) (Value, error)) (Value, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil, ErrHostClosed
	}
	h.running.Add(1)
	h.mu.Unlock()
	defer h.running.Done()
	p := h.template
	p.liveTasks.Add(1)
	defer p.liveTasks.Add(-1)
	task := p.forkView()
	task.acquireGIL()
	defer task.releaseGIL()
	for _, f := range p.forkHooks {
		f(p, task)
	}
	t := newTask(h.owner.line, true)
	task.task = t
	h.owner.addChild(task, t)
	if stop != nil {
		ended := make(chan struct{})
		defer close(ended)
		go cancelOnStop(p, t, stop, ended)
	}
	task.runHosted(t, fn)
	if t.cancelled.Load() || task.interrupted.Load() {
		return nil, ErrInterrupted
	}
	if t.err != nil {
		if err := taskError(t.err); err != nil {
			return nil, err
		}
		return nil, ErrInterrupted // TR_exit: görevin hatası değil, cevap da yok
	}
	return t.result, nil
}

// cancelOnStop stop kapanınca, görev bitmediyse, kilidi alıp görevi iptal eder (10.9). Görev o arada
// bittiyse iptal etkisizdir (cancel bitmiş göreve bakmaz).
func cancelOnStop(p *VM, t *TaskVal, stop, ended <-chan struct{}) {
	select {
	case <-stop:
	case <-ended:
		return
	}
	y := p.forkView()
	y.acquireGIL()
	defer y.releaseGIL()
	t.cancel(y)
}

// runHosted görevin gövdesidir (taskStart'ın goroutine'i gibi): alt görevleri beklenir, panik iç
// hataya döner, görev bitişi kaydedilir.
func (task *VM) runHosted(t *TaskVal, fn func(task *VM) (Value, error)) {
	var result Value
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = panicError(r)
		}
		t.finish(task, result, err)
	}()
	defer func() {
		if r := recover(); r != nil {
			err = panicError(r)
		}
		err = task.takeUnwindErrors(err)
		t.endChildren(task, err != nil)
	}()
	task.checkInterrupt() // başlamadan iptal edilen görev hiç çalışmaz
	result, err = fn(task)
}

// Close yeni görev açılmasını durdurur ve süren görevleri iptal eder (10.9); y kilidi tutar. Bu
// çağrı kapattıysa true verir. Görevler bitince sahibin finish'i kilitsiz bir goroutine'den çağrılır:
// finish yorumlayıcının kilidini istemez, yalnız t.mu'yu ve atomikleri (cancelled, pending, interrupted) kullanır.
func (h *TaskHost) Close(y *VM) bool {
	h.mu.Lock()
	was := h.closed
	h.closed = true
	h.mu.Unlock()
	if was {
		return false
	}
	h.owner.cancel(y)
	go func() {
		h.running.Wait()
		h.owner.finish(y, nil, nil) // iptalin açtığı güvenli nokta yolu kapanır
	}()
	return true
}

// Wait, Close'dan sonra, görevler bitene kadar ya da en çok limit kadar bekler; kilit bırakılır.
// Görevler bittiyse true verir.
func (h *TaskHost) Wait(y *VM, limit time.Duration) bool {
	done := make(chan struct{})
	go func() {
		h.running.Wait()
		close(done)
	}()
	ok := false
	y.Blocking(func() {
		select {
		case <-done:
			ok = true
		case <-time.After(limit):
		}
	})
	return ok
}

// AddMainEnd, program bittiğinde (hatayla ya da durdurmayla da), görevler beklenmeden önce
// çalışacak bir iş ekler: kilit tutulur, iş bir temizliktir (istek onu kesmez). Açık kalan sunucu
// burada kapanır (28.8). Yalnız motor kaydı sırasında çağrılır; etkileşimli kabukta çalışmaz.
func (v *VM) AddMainEnd(f func(y *VM)) { v.mainEnds = append(v.mainEnds, f) }

// runMainEnds Run'da, görevleri beklemeden önce çağrılır.
func (v *VM) runMainEnds() {
	if v.interactive {
		return
	}
	v.cleanupDepth++
	defer func() { v.cleanupDepth-- }()
	for _, f := range v.mainEnds {
		f(v)
	}
}
