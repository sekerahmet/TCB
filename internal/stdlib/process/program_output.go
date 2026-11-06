package process

import (
	"io"
	"sync"
)

// Çıktı tamponunun boyları. outputBuffer kurala yazılmaz (21.2): yalnız "dolunca program bekler"
// kuraldır. Emsali Node'un maxBuffer'ı ve Linux'un pipe-max-size'ıdır (1 MiB); her ikisi de en büyük
// boru boyundan küçük değildir, program bitince boruda kalanı alacak pay da budur.
const (
	outputBuffer = 1 << 20
	outputChunk  = 64 << 10
)

// programOutput programın standart çıktısını kilit dışında sürekli okur (21.2). İki kipte de okuyucu
// aynıdır, kip yalnız dolunca ne olacağını seçer: drop yanlışsa okuyucu durur, boru dolar, program
// bekler; doğruysa (TR_buffer_bytes:) yeni bayt gelince taşma kurulur.
//
// Okuma istekleri sıradadır ve okuyucu onları kilit altında doldurur: bekleyen TR_read yorumlayıcı
// kilidini beklerken gelen bayt tamponu dolu göstermez, n bayt iki göreve bölünmez.
type programOutput struct {
	limit    int         // tampon sınırı
	drop     bool        // sınır aşılırsa taşma kurulur (TR_buffer_bytes:)
	stopping func() bool // TR programı durduruyor: taşma sayılmaz
	overflow func()      // taşma kurulunca bir kez çağrılır; beklemez

	mu         sync.Mutex
	room       *sync.Cond
	data       []byte // data[off:] okunmamış baytlardır
	off        int
	queue      []*readRequest
	ended      bool // program bitti: sınır kalkar (boruda kalan alınır)
	closed     bool // dosya sonu: yeni bayt gelmez
	overflowed bool
}

// readRequest bekleyen bir okumadır. done kapanınca alanlar okunur.
type readRequest struct {
	n          int
	done       chan struct{}
	data       []byte
	stopped    bool // TR programı durdurdu, okuma bitmedi
	overflowed bool
}

func newProgramOutput(limit int, drop bool, stopping func() bool, overflow func()) *programOutput {
	o := &programOutput{limit: limit, drop: drop, stopping: stopping, overflow: overflow}
	o.room = sync.NewCond(&o.mu)
	return o
}

// pump dosya sonuna ya da finish'e kadar okur.
func (o *programOutput) pump(r io.Reader) {
	chunk := make([]byte, outputChunk)
	for o.waitRoom() {
		n, err := r.Read(chunk)
		o.add(chunk[:n])
		if err != nil {
			o.finish()
			return
		}
	}
}

// waitRoom tamponda yer açılana kadar bekler; okuma bittiyse false verir.
func (o *programOutput) waitRoom() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for !o.closed && !o.hasRoom() {
		o.room.Wait()
	}
	return !o.closed
}

// hasRoom okuyucunun boruya gidip gidemeyeceğini söyler. Sıranın başındaki okuma sınırdan büyükse
// sınır onun boyudur: TR_read tam n bayt verir (21.2).
func (o *programOutput) hasRoom() bool {
	if o.overflowed || o.dropping() {
		return true
	}
	limit := o.limit
	if len(o.queue) > 0 {
		limit = max(limit, o.queue[0].n)
	}
	if o.ended || o.drop {
		limit += outputBuffer
	}
	return o.unread() < limit
}

// dropping yeni baytın taşma kurabildiği durumdur: program biterken ya da durdurulurken taşma olmaz.
func (o *programOutput) dropping() bool { return o.drop && !o.ended && !o.stopping() }

func (o *programOutput) unread() int { return len(o.data) - o.off }

// add okunan baytları tampona ekler.
func (o *programOutput) add(b []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.overflowed || len(b) == 0 {
		return
	}
	o.data = append(o.data, b...)
	o.serve()
	if o.dropping() && o.unread() > o.limit {
		o.setOverflow()
	}
}

// setOverflow taşmayı kurar: biriken baytlar atılır, bekleyenler hatayı alır (21.2).
func (o *programOutput) setOverflow() {
	o.overflowed = true
	o.data, o.off = nil, 0
	for _, r := range o.queue {
		r.overflowed = true
		close(r.done)
	}
	o.queue = nil
	o.overflow()
}

// serve sıradaki okumaları sırayla doldurur: dosya sonunda kalan, sonraki okumalara boş gider.
func (o *programOutput) serve() {
	for len(o.queue) > 0 {
		r := o.queue[0]
		if o.unread() < r.n && !o.closed {
			return
		}
		r.data = o.take(min(r.n, o.unread()))
		o.queue = o.queue[1:]
		close(r.done)
		o.room.Broadcast()
	}
}

// take tamponun başından n bayt alır.
func (o *programOutput) take(n int) []byte {
	out := make([]byte, n)
	copy(out, o.data[o.off:])
	o.off += n
	switch {
	case o.off == len(o.data):
		o.data, o.off = o.data[:0], 0
	case o.off > len(o.data)/2: // kaydırma tamponun boyuyla orantılı aralıkla olur
		o.data, o.off = o.data[:copy(o.data, o.data[o.off:])], 0
	}
	return out
}

// read n baytlık okumayı sıraya koyar.
func (o *programOutput) read(n int) *readRequest {
	r := &readRequest{n: n, done: make(chan struct{})}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.overflowed {
		r.overflowed = true
		close(r.done)
		return r
	}
	o.queue = append(o.queue, r)
	o.serve()
	o.room.Broadcast() // okuma sınırdan büyükse okuyucu yeniden boruya gider
	return r
}

// abandon biten bekleyişin okumasını geri alır; doldurulmuşsa baytlar tamponun başına döner: istek
// üstlenilmiş programda okumayı bırakır, baytlar kaybolmaz (1.6).
func (o *programOutput) abandon(r *readRequest) {
	o.mu.Lock()
	defer o.mu.Unlock()
	select {
	case <-r.done:
		if len(r.data) > 0 && !o.overflowed {
			o.data, o.off = append(r.data, o.data[o.off:]...), 0
			r.data = nil
		}
	default:
		for i, q := range o.queue {
			if q == r {
				o.queue = append(o.queue[:i:i], o.queue[i+1:]...)
				break
			}
		}
		r.stopped = true
		close(r.done)
	}
}

// programEnded program bitince çağrılır: sınır kalkar, okuyucu boruda kalanı alır ve bu taşma
// sayılmaz. TR programı durdurduysa bekleyen okumalar hemen döner (21.5).
func (o *programOutput) programEnded(stopped bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ended = true
	if stopped {
		for _, r := range o.queue {
			r.stopped = true
			close(r.done)
		}
		o.queue = nil
	}
	o.room.Broadcast()
}

// finish okumayı bitirir: dosya sonu gelmiş sayılır, bekleyenler kalanı alır.
func (o *programOutput) finish() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	o.serve()
	o.room.Broadcast()
}

// isOverflowed taşma kuruldu mu?
func (o *programOutput) isOverflowed() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.overflowed
}
