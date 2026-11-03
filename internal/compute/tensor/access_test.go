package tensor

import (
	"sync"
	"testing"
	"time"
)

// heldKernel verilen depolara yazan ve onları okuyan, çalışırken release kapanana dek bekleyen bir çekirdektir.
func heldKernel(writes, reads []Storage, release chan struct{}) *Kernel {
	return newKernel(BackendFor(CPU), writes, reads, nil, func() { <-release })
}

func cpuStorages(n int) []Storage {
	out := make([]Storage, n)
	for i := range out {
		out[i] = BackendFor(CPU).Alloc(4, Float32)
	}
	return out
}

// Kural (S-kilit, tasarım (1)-(3)): bir depoya ya bir yazan ya çok okuyan; izinler ya hep ya hiç
// alınır; yerinde işlem (aynı depoyu okuyup yazan) kendisiyle çakışmaz.
func TestKernelPermitsAreAllOrNothing(t *testing.T) {
	s := cpuStorages(3)
	a, b, c := s[0], s[1], s[2]
	r1, r2 := newKernel(BackendFor(CPU), nil, []Storage{a}, nil, func() {}), newKernel(BackendFor(CPU), nil, []Storage{a}, nil, func() {})
	if !r1.TryAcquire() || !r2.TryAcquire() {
		t.Fatal("iki okuyan aynı depoyu birlikte okuyabilmeliydi")
	}
	if w := newKernel(BackendFor(CPU), []Storage{b, a}, nil, nil, func() {}); w.TryAcquire() {
		t.Fatal("okunan depoya yazma izni verildi")
	}
	if p := perms.held[b]; p != nil {
		t.Fatalf("başarısız istek b'yi tuttu (ya hep ya hiç): %+v", *p)
	}
	r1.Run()
	r2.Run()
	inPlace := newKernel(BackendFor(CPU), []Storage{c}, []Storage{c, a}, nil, func() {})
	if !inPlace.TryAcquire() {
		t.Fatal("yerinde işlem kendi izniyle çakıştı")
	}
	if len(inPlace.reads) != 1 || inPlace.reads[0] != a {
		t.Fatalf("yazılan depo okunanlardan çıkmalıydı: %v", inPlace.reads)
	}
	if newKernel(BackendFor(CPU), nil, []Storage{c}, nil, func() {}).TryAcquire() {
		t.Fatal("yazılan depoya okuma izni verildi")
	}
	inPlace.Run()
	if len(perms.held) != 0 || unlockedOps.Load() != 0 {
		t.Fatalf("bırakılan izinler kaldı: %d depo, %d işlem", len(perms.held), unlockedOps.Load())
	}
}

// Kural (S-kilit; DENETİM istek 64, T2): kilit altındaki bir erişim, çakışan kilitsiz işlem bitene
// dek kilidi tutarak bekler ve kilitlenmez. lock yorumlayıcı kilidinin yerindedir: bekleyen onu
// tutar (Exclusive'in durumu), kilitsiz işlem ona hiç dokunmadan biter.
func TestGateWaitsForTheUnlockedKernelWhileHoldingTheLock(t *testing.T) {
	s := cpuStorages(2)
	release := make(chan struct{})
	k := heldKernel([]Storage{s[0]}, []Storage{s[1]}, release)
	if !k.TryAcquire() {
		t.Fatal("izin alınamadı")
	}
	finished := make(chan struct{})
	go func() { k.Run(); close(finished) }()

	var lock sync.Mutex
	lock.Lock()
	gate(nil, []Storage{s[1]}) // okunan depoyu okumak beklemez
	gate(cpuStorages(1), nil)  // başka depoya yazmak beklemez
	lock.Unlock()
	done := make(chan struct{})
	go func() {
		lock.Lock()
		defer lock.Unlock()
		gate(nil, []Storage{s[0]})
		gate([]Storage{s[1]}, nil)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("kapı, kilitsiz işlem yazarken beklemeden geçti")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("kapı kilitsiz işlem bittikten sonra da bekledi (kilitlendi)")
	}
	<-finished
}

// Kural (S-kilit, tasarım (6)): kilitsiz işlem depolarını tutar; başka görev son başlığı bıraksa da
// depo, işlem bitene dek arka uca dönmez.
func TestKernelHoldsItsStorages(t *testing.T) {
	x := Zeros(64)
	s := x.storage
	var returned []Storage
	storageReturned = func(r Storage) { returned = append(returned, r) }
	defer func() { storageReturned = nil }()
	k := newKernel(x.be, nil, []Storage{s}, []*Tensor{x}, func() {})
	if !k.TryAcquire() {
		t.Fatal("izin alınamadı")
	}
	x.Drop()
	if containsStorage(returned, s) {
		t.Fatal("kilitsiz işlem okurken depo arka uca döndü")
	}
	k.Run()
	if !containsStorage(returned, s) {
		t.Fatal("işlem bitince sahipsiz depo arka uca dönmeliydi")
	}
}
