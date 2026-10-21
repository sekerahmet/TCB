package gpu

import (
	"time"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Benzetim sürücüsü: cihaz belleği Go dilimleri, çekirdekler PTX benzetimi
// ============================================================================
//
// Eşzamansız işler (çekirdek başlatma, eşzamansız kopya, doldurma) gerçek sürücüdeki
// gibi sıraya eklenir ve ancak eşzamanlamada (indir, esitle, eşzamanlı yükleme)
// yürütülür. Böylece konak tarafında "kopya bitmeden kaynağın değiştirilmesi" gibi
// sıralama hataları benzetimde de yanlış sonuç verir ve sınamada yakalanır. Çekirdek
// parametreleri, gerçek sürücüdeki gibi başlatma anında kopyalanır.

type simDriver struct {
	mem       simMemory
	fns       []*simFunc
	order     []func() error
	failure   result
	detail2   error
	memory    uint64 // bildirilen toplam bellek
	allocated uint64
	dim       map[uint64]uint64
	// events profil olaylarının geçildiği an (sıfır: henüz geçilmedi).
	events []time.Time
	// cc ve smSayisi bildirilen yetenek (döşeme seçimi sınamaları değiştirebilir).
	cc, smCount int
	// sharedMem blok başına açılabilecek paylaşımlı bellek; izin işlev başına açılan
	// devingen paylaşımlı bellek (gerçek sürücüdeki gibi 48 KB üstü izinsiz başlatılamaz).
	sharedMem int
	allow     map[uintptr]int
	// Kayıt: capturing doğruyken sıraya konan işler çalıştırılmaz, captured'a eklenir;
	// kayıt sırasında eşzamanlama gerektiren işlem kaydı bozar (broken; gerçek sürücüde
	// CUDA_ERROR_STREAM_CAPTURE_IMPLICIT). graphs çizge numarasından iş listesine.
	capturing, broken bool
	captured          []func() error
	graphs            map[uintptr][]func() error
}

// newSim toplam belleği verilen bir benzetim sürücüsü kurar.
func newSim(memory uint64) *simDriver {
	return &simDriver{memory: memory, dim: map[uint64]uint64{}, cc: 52, smCount: 4,
		sharedMem: 99 << 10, allow: map[uintptr]int{}}
}

func (b *simDriver) capability() (int, int) { return b.cc, b.smCount }

func (b *simDriver) createEvent() (uintptr, result) {
	b.events = append(b.events, time.Time{})
	return uintptr(len(b.events)), succeeded
}

func (b *simDriver) recordEvent(ev uintptr) result {
	if ev == 0 || int(ev) > len(b.events) {
		return 400
	}
	b.events[ev-1] = time.Time{}
	return b.add(func() error { b.events[ev-1] = time.Now(); return nil })
}

func (b *simDriver) eventReady(ev uintptr) bool {
	return ev != 0 && int(ev) <= len(b.events) && !b.events[ev-1].IsZero()
}

func (b *simDriver) eventTime(begin, last uintptr) (float32, result) {
	if !b.eventReady(begin) || !b.eventReady(last) {
		return 0, 600
	}
	return float32(b.events[last-1].Sub(b.events[begin-1]).Seconds() * 1e3), succeeded
}

func (b *simDriver) flushTools() {}

func (b *simDriver) streamHandle() uintptr { return 0 }

func (b *simDriver) captureSupported() bool { return true }

func (b *simDriver) beginCapture() result {
	if b.capturing {
		return errCaptureInvalidated
	}
	if s := b.flush(); s != succeeded {
		return s
	}
	b.capturing, b.broken, b.captured = true, false, nil
	return succeeded
}

func (b *simDriver) endCapture() (uintptr, result) {
	if !b.capturing {
		return 0, errCaptureInvalidated
	}
	b.capturing = false
	if b.broken {
		return 0, errCaptureInvalidated
	}
	if b.graphs == nil {
		b.graphs = map[uintptr][]func() error{}
	}
	id := uintptr(len(b.graphs) + 1)
	b.graphs[id] = b.captured
	b.captured = nil
	return id, succeeded
}

func (b *simDriver) instantiate(graph uintptr) (uintptr, result) {
	if _, ok := b.graphs[graph]; !ok {
		return 0, 400
	}
	return graph, succeeded
}

func (b *simDriver) launchGraph(exec uintptr) result {
	order, ok := b.graphs[exec]
	if !ok {
		return 400
	}
	for _, f := range order {
		if s := b.add(f); s != succeeded {
			return s
		}
	}
	return succeeded
}

func (b *simDriver) destroyGraph(graph, exec uintptr) { delete(b.graphs, graph) }

func (b *simDriver) bind() result        { return b.failure }
func (b *simDriver) cardName() string    { return messages.Text(messages.H4064) }
func (b *simDriver) totalMemory() uint64 { return b.memory }

func (b *simDriver) freeMemory() (uint64, uint64, result) {
	return b.memory - b.allocated, b.memory, succeeded
}

func (b *simDriver) allocate(byteN uint64) (uint64, result) {
	if b.allocated+byteN > b.memory {
		return 0, errNoMemory
	}
	addr := b.mem.allocate(byteN)
	b.dim[addr] = byteN
	b.allocated += byteN
	return addr, succeeded
}

func (b *simDriver) release(addr uint64) result {
	if s := b.flush(); s != succeeded {
		return s
	}
	if !b.mem.release(addr) {
		return errInvalidValue
	}
	b.allocated -= b.dim[addr]
	delete(b.dim, addr)
	return succeeded
}

func (b *simDriver) hostAlloc(byteN int) (unsafe.Pointer, result) {
	d := make([]byte, byteN)
	return unsafe.Pointer(&d[0]), succeeded
}

// add bir işi sıraya ekler; yapışkan hata varsa eklenmez.
func (b *simDriver) add(f func() error) result {
	if b.failure != succeeded {
		return b.failure
	}
	if b.capturing {
		b.captured = append(b.captured, f)
		return succeeded
	}
	b.order = append(b.order, f)
	return succeeded
}

// flush sıradaki işleri yürütür; ilk hata yapışkandır (gerçek sürücüdeki gibi bağlam
// bundan sonra kullanılamaz).
func (b *simDriver) flush() result {
	if b.failure != succeeded {
		return b.failure
	}
	if b.capturing {
		b.broken = true
		return errCaptureImplicit
	}
	order := b.order
	b.order = nil
	for _, f := range order {
		if err := f(); err != nil {
			b.failure, b.detail2 = errInvalidAddress, err
			return b.failure
		}
	}
	return succeeded
}

func (b *simDriver) load(target uint64, source unsafe.Pointer, byteN uint64, concurrent2 bool) result {
	dup := func() error {
		d, err := b.mem.slice(target, int(byteN))
		if err != nil {
			return messages.New(messages.H4121, err)
		}
		copy(d, unsafe.Slice((*byte)(source), byteN))
		return nil
	}
	if byteN == 0 {
		return succeeded
	}
	if concurrent2 {
		return b.add(dup)
	}
	if s := b.flush(); s != succeeded {
		return s
	}
	if err := dup(); err != nil {
		b.failure, b.detail2 = errInvalidAddress, err
	}
	return b.failure
}

func (b *simDriver) download(target unsafe.Pointer, source uint64, byteN uint64) result {
	if s := b.flush(); s != succeeded || byteN == 0 {
		return s
	}
	d, err := b.mem.slice(source, int(byteN))
	if err != nil {
		b.failure, b.detail2 = errInvalidAddress, messages.New(messages.H4122, err)
		return b.failure
	}
	copy(unsafe.Slice((*byte)(target), byteN), d)
	return succeeded
}

func (b *simDriver) copyFn(target, source, byteN uint64) result {
	return b.add(func() error {
		d, err := b.mem.slice(target, int(byteN))
		if err != nil {
			return messages.New(messages.H4123, err)
		}
		k, err := b.mem.slice(source, int(byteN))
		if err != nil {
			return messages.New(messages.H4124, err)
		}
		copy(d, k)
		return nil
	})
}

func (b *simDriver) fill32(target uint64, value uint32, n uint64) result {
	return b.add(func() error {
		d, err := b.mem.slice(target, int(4*n))
		if err != nil {
			return messages.New(messages.H4125, err)
		}
		for i := uint64(0); i < n; i++ {
			d[4*i], d[4*i+1], d[4*i+2], d[4*i+3] = byte(value), byte(value>>8), byte(value>>16), byte(value>>24)
		}
		return nil
	})
}

func (b *simDriver) fill16(target uint64, value uint16, n uint64) result {
	return b.add(func() error {
		d, err := b.mem.slice(target, int(2*n))
		if err != nil {
			return messages.New(messages.H4125, err)
		}
		for i := uint64(0); i < n; i++ {
			d[2*i], d[2*i+1] = byte(value), byte(value>>8)
		}
		return nil
	})
}

func (b *simDriver) module(ptx string) (uintptr, string, result) {
	is, err := simParse(ptx)
	if err != nil {
		return 0, err.Error(), errInvalidPTX
	}
	b.fns = append(b.fns, is)
	return uintptr(len(b.fns)), "", succeeded
}

func (b *simDriver) fn(module uintptr, ad string) (uintptr, result) {
	if module == 0 || int(module) > len(b.fns) || b.fns[module-1].ad != ad {
		return 0, 500
	}
	return module, succeeded
}

func (b *simDriver) launch(fn uintptr, grid, block [3]uint32, sharedMem uint32, param [][]byte) result {
	if fn == 0 || int(fn) > len(b.fns) {
		return 400
	}
	is := b.fns[fn-1]
	if (sharedMem > 0) != is.dynamic || is.sharedBytes+int(sharedMem) > max(b.allow[fn]+is.sharedBytes, staticSharedMemLimit) {
		b.detail2 = messages.New(messages.H4126, is.ad, sharedMem, b.allow[fn], is.dynamic)
		return errInvalidValue
	}
	dup := make([][]byte, len(param))
	for i, p := range param {
		dup[i] = append([]byte(nil), p...)
	}
	if grid[0] == 0 || grid[1] == 0 || grid[2] == 0 {
		return errInvalidValue
	}
	return b.add(func() error { return simLaunch(is, &b.mem, grid, block, int(sharedMem), dup) })
}

func (b *simDriver) equalize() result { return b.flush() }

func (b *simDriver) sharedMemOptIn(fn uintptr, byteN int) result {
	if fn == 0 || int(fn) > len(b.fns) || byteN > b.sharedMem {
		return errInvalidValue
	}
	b.allow[fn] = byteN
	return succeeded
}

func (b *simDriver) sharedMemLimit() int { return b.sharedMem }

func (b *simDriver) errName(s result) string { return nameFromTable(s) }

func (b *simDriver) detail() error { return b.detail2 }
