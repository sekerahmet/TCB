package tensor

import (
	"bytes"
	"math/rand"
	"testing"
	"unsafe"
)

func addrOf(w []byte) uintptr { return uintptr(unsafe.Pointer(&w[0])) }

// Ardıl ilk turda öğrenilir, ikinci turda her çarpımın sonunda duyurulur; sıradaki çarpım ipucunu keser.
func TestPrefetchLearnsTheNextProduct(t *testing.T) {
	a, b, c := make([]byte, 4096), make([]byte, 8192), make([]byte, prefetchCap+64)
	seq := [][]byte{a, b, c}
	for _, w := range seq {
		prefetchAfter(prefetchBefore(w))
	}
	for i, w := range seq {
		self := prefetchBefore(w)
		if ahead.cur.Load() != nil {
			t.Fatalf("%d. çarpım başladı, ipucu kesilmedi", i)
		}
		prefetchAfter(self)
		next := seq[(i+1)%3]
		h := ahead.cur.Load()
		if h == nil || h.addr != addrOf(next) || h.size != min(len(next), prefetchCap) {
			t.Fatalf("%d. çarpımın ardılı %+v; beklenen %x, %d bayt", i, h, addrOf(next), len(next))
		}
	}
}

// Bilinmeyen ağırlıktan sonra ipucu duyurulmaz.
func TestPrefetchIsSilentForAnUnknownWeight(t *testing.T) {
	self := prefetchBefore(make([]byte, 64))
	prefetchAfter(self)
	if h := ahead.cur.Load(); h != nil {
		t.Fatalf("ardılı bilinmeyen ağırlıkta ipucu %+v", h)
	}
}

// Dönen işçilerin payları ipucunu boşluksuz ve çakışmasız böler.
func TestPrefetchCursorsCoverTheHint(t *testing.T) {
	buf := make([]byte, 3*prefetchChunk+100)
	ahead.seq.Add(1)
	ahead.cur.Store(&prefetchHint{addr: addrOf(buf), size: len(buf)})
	defer ahead.cur.Store(nil)
	from := 0
	for no := range returningWorker {
		var c prefetchCursor
		if !c.step(no) {
			t.Fatalf("işçi %d iş bulamadı", no)
		}
		if start := c.pos - min(prefetchChunk, c.end-from); start != from {
			t.Fatalf("işçi %d %d'den başladı; %d olmalı", no, start, from)
		}
		for c.step(no) {
		}
		if c.pos != c.end {
			t.Fatalf("işçi %d payını bitirmedi: %+v", no, c)
		}
		from = c.end
	}
	if from != len(buf) {
		t.Fatalf("kapsanan %d bayt; %d olmalı", from, len(buf))
	}
}

// Önden getirme belleği okumaz ve değiştirmez, geçersiz adreste düşmez: ağırlık serbest kalmış olabilir.
func TestPrefetchBytesTouchesNothing(t *testing.T) {
	buf := bytes.Repeat([]byte{7}, 1<<16)
	want := bytes.Clone(buf)
	prefetchBytes(addrOf(buf), len(buf))
	prefetchBytes(0, 1<<16)
	prefetchBytes(1<<46, 1<<16)
	prefetchBytes(addrOf(buf), 0)
	if !bytes.Equal(buf, want) {
		t.Fatal("önden getirme belleği değiştirdi")
	}
}

// Sıradaki ağırlığın önden getirildiği çarpımın sonucu öncekiyle bit bit aynıdır.
func TestQuantMatMulSameBitsWithPrefetch(t *testing.T) {
	r := rand.New(rand.NewSource(9))
	x := New(randomData(r, 64), 1, 64)
	w1 := FromQuantized(Q8, smallScaleQuant(Q8, 1, 96), 48, 64)
	w2 := FromQuantized(Q8, smallScaleQuant(Q8, 2, 96), 48, 64)
	want := [2][]float32{MatMul(x, w1.T()).Data(), MatMul(x, w2.T()).Data()}
	for range 20 {
		for i, w := range []*Tensor{w1, w2} {
			sameFloatBits(t, "önden getirmeli çarpım", MatMul(x, w.T()).Data(), want[i])
		}
	}
}
