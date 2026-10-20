package gpu

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Gerçek NVIDIA sürücüsü (cgo'suz)
// ============================================================================
//
// Sürücü kitaplığı çalışma anında yüklenir (Linux: libcuda.so.1, cgo'suz dinamik yükleyiciyle; Windows:
// nvcuda.dll, syscall ile) ve sürücü API'si doğrudan çağrılır; CUDA araç takımı ya da
// derleyici gerekmez. PTX'i sürücünün içindeki derleyici ilk yüklemede karta derler.
//
// Bağlam ve iş parçacığı: CUDA bağlamı işletim sistemi iş parçacığına bağlıdır, Go
// gorutinleri ise iş parçacığı değiştirebilir. Arka uç her girişte gorutini iş
// parçacığına kilitler (runtime.LockOSThread) ve cihazın birincil bağlamını
// cuCtxSetCurrent ile o iş parçacığına bağlar (bagla); çıkışta kilidi bırakır. Bütün
// sürücü çağrıları arka ucun kilidi altında yapılır (tek akış, sıra korunur).
//
// Bellek güvenliği: sürücünün yazdığı çıkış değerleri ve çekirdek parametre tamponu
// sürücü nesnesinin kalıcı alanlarıdır; çağrı süresince canlı tutulurlar
// (runtime.KeepAlive). Çöp toplayıcı Go nesnelerini taşımaz; sistem çağrısındaki bir
// gorutinin yığıtı da taşınmaz.

// library sürücü kitaplığından işlev adresi bulan ve çağıran platform katmanı.
type library interface {
	find(ad string) (uintptr, error)
	call(fn uintptr, args ...uintptr) uintptr
}

// cudaFuncNames yüklenen sürücü işlevleri (sıra cudaIslevler alanlarıyla aynı).
var cudaFuncNames = [...]string{
	"cuInit", "cuDeviceGetCount", "cuDeviceGet", "cuDeviceGetName", "cuDeviceTotalMem_v2",
	"cuDeviceGetAttribute", "cuDevicePrimaryCtxRetain", "cuCtxSetCurrent", "cuCtxSynchronize",
	"cuMemAlloc_v2", "cuMemFree_v2", "cuMemAllocHost_v2", "cuMemcpyHtoD_v2", "cuMemcpyHtoDAsync_v2",
	"cuMemcpyDtoH_v2", "cuMemcpyDtoDAsync_v2", "cuMemsetD32Async", "cuMemsetD16Async",
	"cuMemGetInfo_v2", "cuModuleLoadDataEx", "cuModuleGetFunction", "cuLaunchKernel",
	"cuGetErrorName", "cuDriverGetVersion", "cuDevicePrimaryCtxRelease_v2",
}

const (
	fInit = iota
	fDeviceGetCount
	fDeviceGet
	fDeviceGetName
	fDeviceTotalMem
	fDeviceGetAttribute
	fPrimaryCtxRetain
	fCtxSetCurrent
	fCtxSynchronize
	fMemAlloc
	fMemFree
	fMemAllocHost
	fMemcpyHtoD
	fMemcpyHtoDAsync
	fMemcpyDtoH
	fMemcpyDtoDAsync
	fMemsetD32Async
	fMemsetD16Async
	fMemGetInfo
	fModuleLoadDataEx
	fModuleGetFunction
	fLaunchKernel
	fGetErrorName
	fDriverGetVersion
	fPrimaryCtxRelease
	fCount
)

// cudaOptionalNames bulunamazsa yalnızca ilgili özelliği (profil, araç boşaltma)
// kapatan sürücü işlevleri.
var cudaOptionalNames = [...]string{
	"cuEventCreate", "cuEventRecord", "cuEventQuery", "cuEventElapsedTime", "cuProfilerStop",
	"cuFuncSetAttribute",
}

const (
	iEventCreate = iota
	iEventRecord
	iEventQuery
	iEventElapsed
	iProfilerStop
	iFuncSetAttribute
	iCount
)

// cudaLibrary süreç genelinde bir kez yüklenen sürücü kitaplığı.
type cudaLibrary struct {
	k       library
	fn      [fCount]uintptr
	ifn     [iCount]uintptr
	num     int
	version int32
	failure error
	output  *cudaOutput
	callM   sync.Mutex // kitaplık düzeyindeki çağrılar (kurulum)
	nameM   sync.Mutex // hata adı çıkış alanı
	ad      *struct{ g unsafe.Pointer }
}

// cudaOutput sürücünün yazdığı çıkış değerleri (yığında, taşınmaz).
type cudaOutput struct {
	u64, u64b uint64
	i32       int32
	f32       float32
	handle    uintptr
	indicator unsafe.Pointer
	ad        [256]byte
}

var (
	cudaOnce   sync.Once
	cudaKitLib *cudaLibrary
)

// loadCUDA sürücü kitaplığını yükler ve cuInit çağırır (bir kez).
func loadCUDA() *cudaLibrary {
	cudaOnce.Do(func() {
		c := &cudaLibrary{output: &cudaOutput{}, ad: &struct{ g unsafe.Pointer }{}}
		cudaKitLib = c
		k, err := openLib()
		if err != nil {
			c.failure = err
			return
		}
		c.k = k
		for i, ad := range cudaFuncNames {
			if c.fn[i], err = k.find(ad); err != nil {
				c.failure = messages.New(messages.H4004, ad, err)
				return
			}
		}
		for i, ad := range cudaOptionalNames {
			c.ifn[i], _ = k.find(ad) // yoksa 0: özellik kapalı
		}
		if s := c.call(fInit, 0); s != succeeded {
			c.failure = messages.New(messages.H4005, "cuInit", c.errName(s))
			return
		}
		c.call(fDriverGetVersion, uintptr(unsafe.Pointer(&c.output.i32)))
		c.version = c.output.i32
		if s := c.call(fDeviceGetCount, uintptr(unsafe.Pointer(&c.output.i32))); s != succeeded {
			c.failure = messages.New(messages.H4005, "cuDeviceGetCount", c.errName(s))
			return
		}
		c.num = int(c.output.i32)
	})
	return cudaKitLib
}

func (c *cudaLibrary) call(f int, args ...uintptr) result {
	r := c.k.call(c.fn[f], args...)
	runtime.KeepAlive(c.output)
	return result(int32(uint32(r)))
}

// errName sürücüden kodun adını ister (cuGetErrorName); veremezse tablodan.
func (c *cudaLibrary) errName(s result) string {
	if c.k == nil || c.fn[fGetErrorName] == 0 {
		return nameFromTable(s)
	}
	c.nameM.Lock()
	defer c.nameM.Unlock()
	c.ad.g = nil
	if r := c.k.call(c.fn[fGetErrorName], uintptr(s), uintptr(unsafe.Pointer(&c.ad.g))); r != 0 || c.ad.g == nil {
		return nameFromTable(s)
	}
	return cText(c.ad.g, 128)
}

// cText sıfırla biten C metnini okur (en çok n bayt).
func cText(p unsafe.Pointer, n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		b := *(*byte)(unsafe.Add(p, i))
		if b == 0 {
			break
		}
		sb.WriteByte(b)
	}
	return sb.String()
}

// cudaDriver bir cihazın sürücü bağlantısı.
type cudaDriver struct {
	c           *cudaLibrary
	device      int32
	context     uintptr
	ad          string
	memory      uint64
	capability_ [2]int32
	smCount     int32
	clockKHz    int32 // SM saati (kHz; okunamazsa 0)
	// sharedMem blok başına açılabilecek en çok paylaşımlı bellek (bayt; okunamazsa 48 KB).
	sharedMem int32
	output    *cudaOutput
	paramBuf  []byte
	paramPtr  []uintptr
	jit       *jitSpace
	// text sürücüye verilen C metinlerinin (PTX, çekirdek adı) kalıcı tamponu: nesnenin
	// yığın (heap) alanıdır, gorutin yığıtında değildir (yığıt büyürken taşınabilirdi).
	text []byte
}

// cTextOf s'yi sıfırla biten C metni olarak kalıcı tampona yazar ve adresini verir; adres
// bir sonraki cMetni çağrısına kadar geçerlidir.
func (s *cudaDriver) cTextOf(m string) unsafe.Pointer {
	s.text = append(append(s.text[:0], m...), 0)
	return unsafe.Pointer(&s.text[0])
}

type jitSpace struct {
	option  [4]int32
	value   [4]uintptr
	info    [8192]byte
	failure [8192]byte
}

// openCUDA verilen sıradaki cihazın birincil bağlamını açar.
func openCUDA(order int) (*cudaDriver, error) {
	c := loadCUDA()
	if c.failure != nil {
		return nil, c.failure
	}
	c.callM.Lock()
	defer c.callM.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s := &cudaDriver{c: c, output: &cudaOutput{}, paramBuf: make([]byte, 0, 8192), jit: &jitSpace{}}
	o := s.output
	if r := c.call(fDeviceGet, uintptr(unsafe.Pointer(&o.i32)), uintptr(order)); r != succeeded {
		return nil, messages.New(messages.H2000, fmt.Sprintf("cuDeviceGet(%d)", order), c.errName(r))
	}
	s.device = o.i32
	if r := c.call(fDeviceGetName, uintptr(unsafe.Pointer(&o.ad[0])), uintptr(len(o.ad)), uintptr(s.device)); r != succeeded {
		return nil, messages.New(messages.H2000, "cuDeviceGetName", c.errName(r))
	}
	s.ad = cText(unsafe.Pointer(&o.ad[0]), len(o.ad))
	if r := c.call(fDeviceTotalMem, uintptr(unsafe.Pointer(&o.u64)), uintptr(s.device)); r != succeeded {
		return nil, messages.New(messages.H2000, "cuDeviceTotalMem", c.errName(r))
	}
	s.memory = o.u64
	for i, oz := range []int{75, 76, 16} { // yetenek büyük/küçük, SM sayısı
		if r := c.call(fDeviceGetAttribute, uintptr(unsafe.Pointer(&o.i32)), uintptr(oz), uintptr(s.device)); r != succeeded {
			return nil, messages.New(messages.H2000, fmt.Sprintf("cuDeviceGetAttribute(%d)", oz), c.errName(r))
		}
		if i < 2 {
			s.capability_[i] = o.i32
		} else {
			s.smCount = o.i32
		}
	}
	if c.call(fDeviceGetAttribute, uintptr(unsafe.Pointer(&o.i32)), 13, uintptr(s.device)) == succeeded {
		s.clockKHz = o.i32 // CU_DEVICE_ATTRIBUTE_CLOCK_RATE (yalnızca kıyas raporu için)
	}
	s.sharedMem = staticSharedMemLimit
	if c.ifn[iFuncSetAttribute] != 0 && c.call(fDeviceGetAttribute, uintptr(unsafe.Pointer(&o.i32)), 97, uintptr(s.device)) == succeeded && o.i32 > s.sharedMem {
		s.sharedMem = o.i32 // CU_DEVICE_ATTRIBUTE_MAX_SHARED_MEMORY_PER_BLOCK_OPTIN
	}
	if r := c.call(fPrimaryCtxRetain, uintptr(unsafe.Pointer(&o.handle)), uintptr(s.device)); r != succeeded {
		return nil, messages.New(messages.H2000, "cuDevicePrimaryCtxRetain", c.errName(r))
	}
	s.context = o.handle
	if r := s.bind(); r != succeeded {
		c.call(fPrimaryCtxRelease, uintptr(s.device)) // tutulan bağlam bırakılır
		return nil, messages.New(messages.H2000, "cuCtxSetCurrent", c.errName(r))
	}
	return s, nil
}

func (s *cudaDriver) call(f int, args ...uintptr) result {
	r := s.c.call(f, args...)
	runtime.KeepAlive(s)
	return r
}

func (s *cudaDriver) bind() result { return s.call(fCtxSetCurrent, s.context) }

func (s *cudaDriver) cardName() string {
	return messages.Text(messages.H4025, s.ad, s.capability_[0], s.capability_[1], s.smCount,
		s.c.version/1000, s.c.version%1000/10)
}

func (s *cudaDriver) totalMemory() uint64 { return s.memory }

func (s *cudaDriver) freeMemory() (uint64, uint64, result) {
	r := s.call(fMemGetInfo, uintptr(unsafe.Pointer(&s.output.u64)), uintptr(unsafe.Pointer(&s.output.u64b)))
	return s.output.u64, s.output.u64b, r
}

func (s *cudaDriver) allocate(byteN uint64) (uint64, result) {
	r := s.call(fMemAlloc, uintptr(unsafe.Pointer(&s.output.u64)), uintptr(byteN))
	return s.output.u64, r
}

func (s *cudaDriver) release(addr uint64) result { return s.call(fMemFree, uintptr(addr)) }

func (s *cudaDriver) hostAlloc(byteN int) (unsafe.Pointer, result) {
	r := s.call(fMemAllocHost, uintptr(unsafe.Pointer(&s.output.indicator)), uintptr(byteN))
	return s.output.indicator, r
}

// load konaktan cihaza kopyalar. Eşzamansız kopyanın kaynağı sayfası kilitli (Go dışı)
// bellek olmalıdır (hazırlık halkası); eşzamanlı kopyanın kaynağı Go belleği olabilir ve
// kopya bitene kadar (çağrı dönene kadar) canlı tutulur.
func (s *cudaDriver) load(target uint64, source unsafe.Pointer, byteN uint64, concurrent2 bool) result {
	var r result
	if concurrent2 {
		r = s.call(fMemcpyHtoDAsync, uintptr(target), uintptr(source), uintptr(byteN), 0)
	} else {
		r = s.call(fMemcpyHtoD, uintptr(target), uintptr(source), uintptr(byteN))
	}
	runtime.KeepAlive(source)
	return r
}

// download cihazdan konağa eşzamanlı kopyalar; hedef çağrı süresince canlı tutulur.
func (s *cudaDriver) download(target unsafe.Pointer, source uint64, byteN uint64) result {
	r := s.call(fMemcpyDtoH, uintptr(target), uintptr(source), uintptr(byteN))
	runtime.KeepAlive(target)
	return r
}

func (s *cudaDriver) copyFn(target, source, byteN uint64) result {
	return s.call(fMemcpyDtoDAsync, uintptr(target), uintptr(source), uintptr(byteN), 0)
}

func (s *cudaDriver) fill32(target uint64, value uint32, n uint64) result {
	return s.call(fMemsetD32Async, uintptr(target), uintptr(value), uintptr(n), 0)
}

func (s *cudaDriver) fill16(target uint64, value uint16, n uint64) result {
	return s.call(fMemsetD16Async, uintptr(target), uintptr(value), uintptr(n), 0)
}

// module PTX'i cuModuleLoadDataEx ile derler; derleyicinin bilgi ve hata günlüklerini
// toplar.
func (s *cudaDriver) module(ptx string) (uintptr, string, result) {
	j := s.jit
	j.option = [4]int32{3, 4, 5, 6} // bilgi günlüğü, boyu, hata günlüğü, boyu
	j.value = [4]uintptr{uintptr(unsafe.Pointer(&j.info[0])), uintptr(len(j.info)),
		uintptr(unsafe.Pointer(&j.failure[0])), uintptr(len(j.failure))}
	// Önceki derlemenin günlüğü görünmesin (sürücü boş günlükte tampona yazmayabilir).
	j.info[0], j.failure[0] = 0, 0
	text := s.cTextOf(ptx)
	r := s.call(fModuleLoadDataEx, uintptr(unsafe.Pointer(&s.output.handle)), uintptr(text),
		4, uintptr(unsafe.Pointer(&j.option[0])), uintptr(unsafe.Pointer(&j.value[0])))
	log := strings.TrimSpace(cText(unsafe.Pointer(&j.failure[0]), len(j.failure)))
	if b := strings.TrimSpace(cText(unsafe.Pointer(&j.info[0]), len(j.info))); b != "" {
		log = strings.TrimSpace(log + "\n" + b)
	}
	return s.output.handle, log, r
}

func (s *cudaDriver) fn(module uintptr, ad string) (uintptr, result) {
	r := s.call(fModuleGetFunction, uintptr(unsafe.Pointer(&s.output.handle)), module, uintptr(s.cTextOf(ad)))
	return s.output.handle, r
}

// launch parametreleri kalıcı tampona kopyalar (her biri 8 bayta hizalı) ve
// cuLaunchKernel'e her parametrenin adresini verir; sürücü değerleri başlatmada kopyalar.
func (s *cudaDriver) launch(fn uintptr, grid, block [3]uint32, sharedMem uint32, param [][]byte) result {
	total := 0
	for _, p := range param {
		total += (len(p) + 7) &^ 7
	}
	if cap(s.paramBuf) < total {
		s.paramBuf = make([]byte, 0, 2*total)
	}
	buf := s.paramBuf[:total]
	s.paramPtr = s.paramPtr[:0]
	o := 0
	for _, p := range param {
		copy(buf[o:], p)
		s.paramPtr = append(s.paramPtr, uintptr(unsafe.Pointer(&buf[o])))
		o += (len(p) + 7) &^ 7
	}
	if len(s.paramPtr) == 0 {
		s.paramPtr = append(s.paramPtr, 0)
	}
	r := s.call(fLaunchKernel, fn, uintptr(grid[0]), uintptr(grid[1]), uintptr(grid[2]),
		uintptr(block[0]), uintptr(block[1]), uintptr(block[2]), uintptr(sharedMem), 0, uintptr(unsafe.Pointer(&s.paramPtr[0])), 0)
	runtime.KeepAlive(buf)
	return r
}

func (s *cudaDriver) equalize() result { return s.call(fCtxSynchronize) }

// sharedMemOptIn CU_FUNC_ATTRIBUTE_MAX_DYNAMIC_SHARED_SIZE_BYTES (8).
func (s *cudaDriver) sharedMemOptIn(fn uintptr, byteN int) result {
	return s.invoke(iFuncSetAttribute, fn, 8, uintptr(byteN))
}

func (s *cudaDriver) sharedMemLimit() int { return int(s.sharedMem) }

func (s *cudaDriver) errName(r result) string { return s.c.errName(r) }

func (s *cudaDriver) detail() error { return nil }

// peakGFLOPS kartın kuramsal float32 tepe hızı (GFLOPS; SM başına float32 birimi yetenek
// düzeyinden tahmin edilir: 6.0, 7.x, 8.0 için 64, diğerleri 128). Yalnızca kıyas raporu.
func (s *cudaDriver) peakGFLOPS() float64 {
	cc, sm := s.capability()
	unit := 128
	if cc == 60 || (cc >= 70 && cc < 80) || cc == 80 {
		unit = 64
	}
	return float64(sm) * float64(unit) * 2 * float64(s.clockKHz) / 1e6
}

func (s *cudaDriver) capability() (int, int) {
	return int(s.capability_[0]*10 + s.capability_[1]), int(s.smCount)
}

// invoke isteğe bağlı bir sürücü işlevini çağırır (yoksa CUDA_ERROR_NOT_SUPPORTED).
func (s *cudaDriver) invoke(f int, args ...uintptr) result {
	if s.c.ifn[f] == 0 {
		return 801
	}
	r := s.c.k.call(s.c.ifn[f], args...)
	runtime.KeepAlive(s)
	runtime.KeepAlive(s.output)
	return result(int32(uint32(r)))
}

func (s *cudaDriver) createEvent() (uintptr, result) {
	r := s.invoke(iEventCreate, uintptr(unsafe.Pointer(&s.output.handle)), 0)
	return s.output.handle, r
}

func (s *cudaDriver) recordEvent(ev uintptr) result { return s.invoke(iEventRecord, ev, 0) }

func (s *cudaDriver) eventReady(ev uintptr) bool { return s.invoke(iEventQuery, ev) == succeeded }

func (s *cudaDriver) eventTime(begin, last uintptr) (float32, result) {
	r := s.invoke(iEventElapsed, uintptr(unsafe.Pointer(&s.output.f32)), begin, last)
	return s.output.f32, r
}

// flushTools dış bir profilleyici bağlıysa (aracBagli) sıradaki işleri bitirir ve profil
// toplamayı durdurur (cuProfilerStop: araçlar bu noktada topladıklarını boşaltabilir). C çıkış
// işleyicileri ayrıca, bütün GPU'lar kapandıktan sonra bir kez çalıştırılır (bkz. Kapat).
func (s *cudaDriver) flushTools() {
	if !toolAttached() {
		return
	}
	s.equalize()
	s.invoke(iProfilerStop)
}

// toolAttached dış bir profilleyicinin (nsys, ncu) bağlı olduğu: sürücü aracı
// CUDA_INJECTION64_PATH ile yükler.
func toolAttached() bool { return os.Getenv("CUDA_INJECTION64_PATH") != "" }
