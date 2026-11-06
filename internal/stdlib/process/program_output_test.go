package process

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// OutputBuffer iç tamponun boyudur (dış test paketi için).
const OutputBuffer = outputBuffer

// SetPhysicalMemory makinenin belleğini küçültür; dönen işlev geri alır.
func SetPhysicalMemory(n uint64) (restore func()) {
	saved := physicalMemory
	physicalMemory = func() (uint64, bool) { return n, true }
	return func() { physicalMemory = saved }
}

func notStopping() bool { return false }

func ready(t *testing.T, r *readRequest) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Fatal("okuma bitmedi")
	}
}

// Okuma sırası korunur, n bayt bölünmez, dosya sonunda kalan sıranın başına gider (21.2).
func TestOutputServesReadsInOrder(t *testing.T) {
	o := newProgramOutput(1<<10, false, notStopping, func() {})
	a, b, c := o.read(4), o.read(4), o.read(4)
	o.add([]byte("abc"))
	o.add([]byte("defgh"))
	ready(t, a)
	ready(t, b)
	o.add([]byte("ij"))
	o.finish()
	ready(t, c)
	if string(a.data) != "abcd" || string(b.data) != "efgh" || string(c.data) != "ij" {
		t.Errorf("%q %q %q", a.data, b.data, c.data)
	}
	if r := o.read(1); len(r.data) != 0 {
		t.Errorf("dosya sonundan sonra okuma boş olmalı: %q", r.data)
	}
}

// Tampon baştan alındıkça kayar: içerik bozulmaz.
func TestOutputKeepsTheContentWhileItShifts(t *testing.T) {
	o := newProgramOutput(1<<20, false, notStopping, func() {})
	var want, got bytes.Buffer
	for i := range 3000 {
		piece := bytes.Repeat([]byte{byte(i)}, 7+i%5)
		want.Write(piece)
		o.add(piece)
		if o.unread() >= 9 {
			r := o.read(9)
			ready(t, r)
			got.Write(r.data)
		}
	}
	o.finish()
	r := o.read(1 << 20)
	ready(t, r)
	got.Write(r.data)
	if !bytes.Equal(want.Bytes(), got.Bytes()) {
		t.Errorf("%d bayt yazıldı, %d okundu ya da içerik bozuk", want.Len(), got.Len())
	}
}

// Taşma sınırın üstündeki ilk baytta kurulur; tam sınıra kadar kurulmaz (21.2). Biriken baytlar
// atılır, bekleyen de sonraki de hatayı alır.
func TestOutputOverflowsPastTheLimit(t *testing.T) {
	overflowed := 0
	o := newProgramOutput(10, true, notStopping, func() { overflowed++ })
	o.add(make([]byte, 10))
	if o.isOverflowed() {
		t.Fatal("tam sınır taşma değil")
	}
	o.add([]byte{1})
	if !o.isOverflowed() || overflowed != 1 || o.unread() != 0 {
		t.Errorf("taşma %v, çağrı %d, kalan %d", o.isOverflowed(), overflowed, o.unread())
	}
	if r := o.read(1); !r.overflowed {
		t.Error("sonraki okuma taşmayı almalı")
	}
	o.add([]byte{2})
	if overflowed != 1 {
		t.Error("taşma bir kez kurulur")
	}
}

// Bekleyen tampon boyu okuma yeni baytı taşma göstermez: okuyucu onu kilit altında doldurur (21.2).
func TestPendingReadOfTheBufferSizeIsNoOverflow(t *testing.T) {
	o := newProgramOutput(10, true, notStopping, func() {})
	r := o.read(10)
	o.add(make([]byte, 10))
	o.add(make([]byte, 10))
	ready(t, r)
	if o.isOverflowed() || len(r.data) != 10 {
		t.Errorf("taşma %v, %d bayt", o.isOverflowed(), len(r.data))
	}
}

// Program bitince ya da durdurulurken sınırın üstü taşma değildir (21.2).
func TestNoOverflowOnceTheProgramEnded(t *testing.T) {
	o := newProgramOutput(10, true, notStopping, func() {})
	o.programEnded(false)
	o.add(make([]byte, 100))
	if o.isOverflowed() {
		t.Error("bitmiş program taşmaz")
	}
	o = newProgramOutput(10, true, func() bool { return true }, func() {})
	o.add(make([]byte, 100))
	if o.isOverflowed() {
		t.Error("durdurulan program taşmaz")
	}
}

// Geri alınan okumanın baytları tamponun başına döner (1.6).
func TestAbandonedReadGivesTheBytesBack(t *testing.T) {
	o := newProgramOutput(1<<10, false, notStopping, func() {})
	o.add([]byte("abcdef"))
	r := o.read(3)
	ready(t, r)
	o.abandon(r)
	again := o.read(6)
	ready(t, again)
	if string(again.data) != "abcdef" {
		t.Errorf("%q", again.data)
	}
	waiting := o.read(5)
	o.abandon(waiting)
	if !waiting.stopped || len(o.queue) != 0 {
		t.Errorf("bekleyen okuma sıradan çıkmalıydı: %v, %d", waiting.stopped, len(o.queue))
	}
}

// Dolu tamponda okuyucu durur, okuma yer açınca sürer; sınırdan büyük okuma sınırı büyütür: TR_read
// tam n bayt verir (21.2).
func TestPumpStopsWhenFullAndGoesOn(t *testing.T) {
	r, w := io.Pipe()
	defer func() { _ = w.Close() }()
	o := newProgramOutput(8, false, notStopping, func() {})
	go o.pump(r)
	wrote := make(chan struct{})
	go func() {
		_, _ = w.Write(make([]byte, 8))
		_, _ = w.Write(make([]byte, 8))
		close(wrote)
	}()
	select {
	case <-wrote:
		t.Fatal("dolu tamponda yazan beklemeliydi")
	case <-time.After(200 * time.Millisecond):
	}
	big := o.read(16)
	ready(t, big)
	<-wrote
	if len(big.data) != 16 {
		t.Errorf("%d bayt", len(big.data))
	}
}
