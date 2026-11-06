package tensor

import (
	le "encoding/binary"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// ============================================================================
// Öbek dışı blok: dosyadan gelen büyük ağırlıklar (15.53)
// ============================================================================
//
// Ağırlık Go öbeğinde durunca çöp toplayıcının hedefi onunla büyür: 200 yüzdesinde canlının 3 katı,
// 4B modelinde 8 463 MB. Hedefi süreç genelinde kısan ayar (GOMEMLIMIT, SetGCPercent) aynı programdaki
// eğitimi de yavaşlatır; blok süreç ayarı istemez. Bellek anonimdir, dosya eşlemesi değil: dosya
// kısaltılınca eşlemeye erişim süreci düşürür.
//
// Ömür: çöp toplayıcı bu belleğe bakan dilimi görmez, yalnız bloğun kendisini görür. Depo bloğu
// tutar (cpuStorage.block); blok erişilmez olunca bellek geri verilir, adres verilmez: aralık erişime
// kapalı kalır, başka yüklemeye gitmez. Atlanan bir canlı tutma böylece başka modelin baytını sessizce
// okumaz, süreci düşürür (DENETİM 216: aynı adres yeniden verilince eski dilim yeni bloğu okudu).
// Bedel adres uzayıdır: 128 TiB'de 4 GiB'lik model 32 768 yükleme. Bir çekirdek dilimi alıp depoyu
// bıraktıysa derleyici depoyu ölü sayabilir: bu yüzden her kapı depolarını çağrı bitene dek canlı
// tutar (gate_calls.go) ve kapı dışından depoya bakan tensör yöntemleri tensörü canlı tutar.
// TestHostBlockReadersKeepTheirTensors ikisini yapısal olarak sınar. Deponun dilimi başka depoya
// verilmez: blok bir depoya aittir, yalnız görünümler paylaşır.

// hostBlockMin bundan küçük tensör öbekte kalır: norm vektörleri kilobayt mertebesindedir, her biri
// için sayfa ve sistem çağrısı harcanmaz.
const hostBlockMin = 1 << 20

// hostTail bloğun arkasında eşlenen yedek baytlardır; heap'te bir okuma dilimin ardına taşarsa
// komşu nesneye düşer, burada eşlenmemiş sayfaya düşüp süreci kapatırdı.
const hostTail = 4096

// hostPressureFloor çöp toplamayı zorlamadan ayrılabilecek en az bayt.
const hostPressureFloor = 256 << 20

// hostLittleEndian dosyaların küçük uçlu baytları belleğe çevrilmeden okunabilir mi.
var hostLittleEndian = le.NativeEndian.Uint16([]byte{1, 0}) == 1

// hostMemory süreçteki blokların toplamı ve son toplamadan beri ayrılan bayt; çöp toplayıcı
// blokları görmez, baskıyı burada sayar.
var hostMemory struct {
	live, sinceGC atomic.Int64
}

// HostBlock Go öbeğinin dışında sıfırlı, yazılabilir bellektir.
type HostBlock struct {
	mem []byte
}

// NewHostBlock n baytlık blok verir. Tensör küçükse, platformda yol yoksa ya da işletim sistemi
// vermezse nil döner: çağıran öbeği kullanır.
func NewHostBlock(n int) *HostBlock {
	if n < hostBlockMin || !hostLittleEndian {
		return nil
	}
	relieveHostPressure(n)
	mem, free, ok := hostMap(n)
	if !ok {
		return nil
	}
	b := &HostBlock{mem: mem}
	hostMemory.live.Add(int64(n))
	runtime.AddCleanup(b, func(size int) {
		free()
		hostMemory.live.Add(-int64(size))
	}, n)
	return b
}

// relieveHostPressure blokları süreç toplamı kadar ayırdıktan sonra çöp toplamayı çağırır: öbek
// küçükken toplama kendiliğinden gelmez, ölü bloklar geri dönmezdi. Eşik canlıyla büyüdüğü için
// model yüklerken toplam log(n) kez çalışır; her çalışması küçük öbeği tarar.
func relieveHostPressure(n int) {
	if hostMemory.sinceGC.Add(int64(n)) >= max(hostMemory.live.Load(), hostPressureFloor) {
		hostMemory.sinceGC.Store(int64(n))
		runtime.GC()
	}
}

// Bytes bloğun baytlarıdır; dilim bloğu canlı tutmaz.
func (b *HostBlock) Bytes() []byte { return b.mem }

// Float32s bloğun float32 görünümüdür; dilim bloğu canlı tutmaz.
func (b *HostBlock) Float32s() []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(unsafe.SliceData(b.mem))), len(b.mem)/4)
}

// Uint16s bloğun 16 bitlik görünümüdür; dilim bloğu canlı tutmaz.
func (b *HostBlock) Uint16s() []uint16 {
	return unsafe.Slice((*uint16)(unsafe.Pointer(unsafe.SliceData(b.mem))), len(b.mem)/2)
}

// storage bloğu n öğelik dt deposu yapar.
func (b *HostBlock) storage(dt DType, n int) *cpuStorage {
	switch {
	case dt == Float32:
		return &cpuStorage{d: b.Float32s()[:n], block: b}
	case dt.IsTyped() || dt.IsQuantized():
		return &cpuStorage{dtyp: dt, raw: b.mem, n: n, block: b}
	}
	return &cpuStorage{dtyp: dt, h: b.Uint16s()[:n], block: b}
}

// FromHostBlock bloğu tensörün deposu yapar, kopyalamaz; varsayılan cihaz kartsa baytlar karta
// gider. Blok dosyanın bitlerini tutar: float32, 16 bitlik ya da nicemlenmiş.
func FromHostBlock(b *HostBlock, dt DType, shape ...int) *Tensor {
	defer runtime.KeepAlive(b)
	be := defaultBackend()
	if dt.IsQuantized() {
		if _, cpu := be.(cpuBackend); !cpu {
			return FromQuantized(dt, b.mem, shape...)
		}
		checkQuantShape(dt, shape)
	}
	n := allocSizeOn(be, isCreate, shape, dt)
	if _, cpu := be.(cpuBackend); cpu {
		return fromStorage(be, b.storage(dt, n), shape)
	}
	if dt == Float32 {
		return New(b.Float32s()[:n], shape...)
	}
	return FromBits16On(DefaultDevice(), dt, b.Uint16s()[:n], shape...)
}

// hostTarget t'den türeyen n öğelik dt sonucu için blok deposu verir: blokta duran ağırlığın çevrilmiş
// kopyası da öbeğe düşmez, yoksa çevirme hedefi yeniden ağırlıkla büyürdü. t blokta değilse nil.
func (t *Tensor) hostTarget(dt DType, n int) *cpuStorage {
	s, ok := t.storage.(*cpuStorage)
	if !ok || s.block == nil {
		return nil
	}
	size := n * dt.Bits() / 8
	if elems, bytes := dt.QuantBlock(); elems > 0 {
		size = n / elems * bytes
	}
	b := NewHostBlock(size)
	if b == nil {
		return nil
	}
	return b.storage(dt, n)
}
