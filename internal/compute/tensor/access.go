package tensor

import (
	"sync"
	"sync/atomic"
)

// ============================================================================
// Depo izinleri (dil tanımı 10.2; S-kilit)
// ============================================================================
//
// Yorumlayıcı kilidi bırakılmışken çalışan bir çekirdek (kilitsiz işlem) okuduğu depolara okuma,
// yazdıklarına yazma izni tutar. Bir depoya aynı anda ya bir yazan ya çok okuyan erişir (Rust'ın
// ödünç kuralı; Swift'in erişim tekliği gibi çalışma anında denetlenir). İzin depoya verilir,
// tensöre değil: görünümler aynı depoyu paylaşır. Kurallar:
//
//   - İzinler yalnız yorumlayıcı kilidi tutulurken alınır (Kernel.TryAcquire); kilitsiz işlem
//     sayacı ve tablo yalnız o zaman artar. Kilit altındaki her arka uç çağrısı kapıdan geçer
//     (gate_calls.go): sayaç sıfırsa tabloya bakmaz. Sayaç kilit tutulurken artamadığından
//     kapının okuması ile çağrı arasında yeni bir izin doğamaz.
//   - İzinler ya hep ya hiç alınır; kısmi tutup bekleme yoktur, çevrim olmaz.
//   - İzinler kilitsiz bırakılır (Kernel.Run'ın sonu); izin tutulurken yorumlayıcı kilidi
//     beklenmez.
//   - Kapı, çakışan bir kilitsiz işlem bitene dek kilidi tutarak bekler. Kilitsiz işlem bitmek
//     için kilide ihtiyaç duymadığından bekleme kilitlenmez; Exclusive içinde de böyledir.
//     Kilit, bileşik bir yerleşiğin (geri yayılım, iyileştirici adımı) ortasında bırakılmaz:
//     öbür görevler yarım güncellenmiş tensör görmez.
//   - Kilitsiz işlem girdi ve çıktı tensörlerinin depolarını tutar (Hold): başka görev son
//     başlığı bıraksa da depo arka uca dönmez (lifetime.go).

// permit bir deponun tutulan izinleri.
type permit struct {
	readers int
	writer  bool
}

var (
	perms = struct {
		mu   sync.Mutex
		cond *sync.Cond
		held map[Storage]*permit
	}{held: map[Storage]*permit{}}
	// unlockedOps izin tutan kilitsiz işlem sayısıdır; yalnız kilit altında artar.
	unlockedOps atomic.Int32
	// sharingOps işlemcileri paylaşan (ShareProcessors) kilitsiz işlem sayısıdır (parallel.go).
	sharingOps atomic.Int32
)

func init() { perms.cond = sync.NewCond(&perms.mu) }

// Kernel kilit altında hazırlanmış bir çekirdektir: okuduğu ve yazdığı depolar ile yalnız
// hazırlıkta kopyalanmış görünümlerle çalışan gövdesi. Gövde tensör nesnesine, hesap çizgesine,
// türev bağlamına ve paylaşılan duruma dokunmaz; bu yüzden kilitsiz çalışabilir.
type Kernel struct {
	be            Backend
	reads, writes []Storage
	// holds TR'ye görünen tensörlerin depolarıdır (girdiler ve çıktı); çekirdek çalışırken
	// tutulur. Hazırlığın özel kopyaları (bitişik kopya) tutulmaz: onları hazırlık ya da
	// gövde kendisi iade eder.
	holds []Storage
	run   func()
	held  bool
	// share: programda başka görev varken çalışan çekirdek işlemcilerin yarısını bırakır.
	share bool
	// begin ve end kilit altında, çekirdeğin bir kez çalışmasının başında (izinler alınınca) ve
	// sonunda (kilit geri alınınca) çağrılır: bileşik yerleşiğin sayaçları ve sürümleri. Hazırlık
	// çakışmada yinelenebildiğinden böyle durum hazırlıkta değişmez. nil olabilirler.
	begin, end func()
}

// newKernel çekirdeği kurar; holds çalışırken tutulacak tensörlerdir. Yazılan depo okunanlardan
// çıkarılır (yerinde işlem: a += a).
func newKernel(be Backend, writes, reads []Storage, holds []*Tensor, run func()) *Kernel {
	k := &Kernel{be: be, writes: writes, run: run}
	for _, t := range holds {
		if !containsStorage(k.holds, t.storage) {
			k.holds = append(k.holds, t.storage)
		}
	}
	for _, r := range reads {
		if r != nil && !containsStorage(writes, r) && !containsStorage(k.reads, r) {
			k.reads = append(k.reads, r)
		}
	}
	return k
}

func containsStorage(ss []Storage, s Storage) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// TryAcquire izinlerin hepsini birden alır; biri bile çakışıyorsa hiçbirini almaz ve yanlış
// döner. Yalnız yorumlayıcı kilidi tutulurken çağrılır.
func (k *Kernel) TryAcquire() bool {
	if !k.acquire() {
		return false
	}
	if k.begin != nil {
		k.begin()
	}
	return true
}

func (k *Kernel) acquire() bool {
	perms.mu.Lock()
	defer perms.mu.Unlock()
	if conflicts(k.writes, k.reads) {
		return false
	}
	for _, s := range k.writes {
		permitOf(s).writer = true
	}
	for _, s := range k.reads {
		permitOf(s).readers++
	}
	for _, s := range k.holds {
		s.Hold()
	}
	unlockedOps.Add(1)
	if k.share {
		sharingOps.Add(1)
	}
	k.held = true
	return true
}

// ShareProcessors çekirdeği, çalışırken işlemcilerin yarısını başka görevlere bırakacak biçimde
// işaretler (TryAcquire'dan önce). Programda başka görev yokken çağrılmaz: tek görevde çekirdek
// bütün işlemcileri kullanır.
func (k *Kernel) ShareProcessors() { k.share = true }

// WaitFree çekirdeğin izinleriyle çakışan kilitsiz işlemler bitene dek bekler. Yorumlayıcı
// kilidi bırakılmışken çağrılır; dönünce kilit geri alınır ve çekirdek yeniden hazırlanır
// (girdiler bu arada değişmiş olabilir).
func (k *Kernel) WaitFree() { waitFree(k.writes, k.reads) }

// Run izinleri alınmış çekirdeği çalıştırır ve izinleri bırakır; kilitsiz çağrılabilir.
func (k *Kernel) Run() {
	defer k.release()
	k.run()
}

// RunLocked çekirdeği kilit altında, kapıdan geçerek çalıştırır (izin almadan).
func (k *Kernel) RunLocked() {
	if k.begin != nil {
		k.begin()
	}
	gate(k.writes, k.reads)
	k.run()
	k.End()
}

// End çekirdeğin bitişini kilit altında tamamlar (kilitsiz Run'dan sonra, kilit geri alınınca).
func (k *Kernel) End() {
	if k.end != nil {
		k.end()
	}
}

// release izinleri bırakır, bekleyenleri uyandırır ve depoların tutuşunu düşer.
func (k *Kernel) release() {
	if !k.held {
		return
	}
	k.held = false
	perms.mu.Lock()
	for _, s := range k.writes {
		p := perms.held[s]
		p.writer = false
		dropPermit(s, p)
	}
	for _, s := range k.reads {
		p := perms.held[s]
		p.readers--
		dropPermit(s, p)
	}
	unlockedOps.Add(-1)
	if k.share {
		sharingOps.Add(-1)
	}
	perms.cond.Broadcast()
	perms.mu.Unlock()
	for _, s := range k.holds {
		unholdStorage(k.be, s)
	}
}

func permitOf(s Storage) *permit {
	p := perms.held[s]
	if p == nil {
		p = &permit{}
		perms.held[s] = p
	}
	return p
}

func dropPermit(s Storage, p *permit) {
	if p.readers == 0 && !p.writer {
		delete(perms.held, s)
	}
}

// conflicts yazılacak ya da okunacak bir depoda çakışan izin olup olmadığıdır (perms.mu tutulur).
func conflicts(writes, reads []Storage) bool {
	for _, s := range writes {
		if p := perms.held[s]; p != nil && (p.writer || p.readers > 0) {
			return true
		}
	}
	for _, s := range reads {
		if p := perms.held[s]; p != nil && p.writer {
			return true
		}
	}
	return false
}

// waitFree çakışma kalmayana dek bekler; beklediyse doğru döner.
func waitFree(writes, reads []Storage) bool {
	perms.mu.Lock()
	waited := false
	for conflicts(writes, reads) {
		waited = true
		perms.cond.Wait()
	}
	perms.mu.Unlock()
	return waited
}

// GateWaited, kilit altındaki bir arka uç çağrısı çakışan kilitsiz işlemi beklediğinde çağrılır
// (yalnız ölçüm ve sınama içindir: çakışma beklemesi gözcünün ≤ 10 ms sözünün dışındadır).
// Program çalışmadan kurulur.
var GateWaited func()

// gate kilit altındaki bir arka uç çağrısının kapısıdır: çakışan kilitsiz işlem yoksa hemen
// döner (çoğu zaman tek atomik okuma), varsa o bitene dek bekler.
func gate(writes, reads []Storage) {
	if unlockedOps.Load() == 0 {
		return
	}
	if waitFree(writes, reads) && GateWaited != nil {
		GateWaited()
	}
}
