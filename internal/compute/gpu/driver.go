package gpu

import (
	"unsafe"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Sürücü arayüzü
// ============================================================================
//
// Arka uç sürücüye bu dar arayüzden erişir. İki uygulaması vardır: gerçek NVIDIA sürücüsü
// (cuda.go; Linux'ta libcuda.so, Windows'ta nvcuda.dll, cgo'suz) ve PTX benzetimi
// (benzetim.go; donanımsız sınama). Bütün işler tek akışta (sürücünün oluşturduğu akış;
// kayıt için gerekir, varsayılan akış kaydedilemez) sırayla yürür; "eşzamansız" yöntemler sıraya ekler ve hemen döner, konağa okuma (indir) ve
// esitle sıradaki her işin bitmesini bekler.

// result sürücü dönüş kodudur (CUresult).
type result int32

const succeeded result = 0

// driver arka ucun kullandığı sürücü işlemleri.
type driver interface {
	// bind sürücü bağlamını çağıran işletim sistemi iş parçacığına bağlar (her girişte).
	bind() result
	cardName() string
	totalMemory() uint64
	freeMemory() (empty, total uint64, s result)
	allocate(byteN uint64) (uint64, result)
	release(addr uint64) result
	// hostAlloc sayfası kilitli (eşzamansız kopyaya uygun) konak belleği ayırır.
	hostAlloc(byteN int) (unsafe.Pointer, result)
	// load konaktan cihaza kopyalar; esz doğruysa eşzamansızdır (kaynak sayfası kilitli
	// bellekte olmalı ve kopya bitene kadar değişmemeli).
	load(target uint64, source unsafe.Pointer, byteN uint64, concurrent2 bool) result
	// download cihazdan konağa eşzamanlı kopyalar (önce sıradaki işler biter).
	download(target unsafe.Pointer, source uint64, byteN uint64) result
	copyFn(target, source, byteN uint64) result
	fill32(target uint64, value uint32, n uint64) result
	fill16(target uint64, value uint16, n uint64) result
	// module PTX metnini derler; hata durumunda derleyici günlüğünü de verir.
	module(ptx string) (uintptr, string, result)
	fn(module uintptr, ad string) (uintptr, result)
	// launch çekirdeği sıraya koyar; paylasim devingen paylaşımlı bellek (bayt, .extern
	// .shared; 0: yok).
	launch(fn uintptr, grid, block [3]uint32, sharedMem uint32, param [][]byte) result
	// sharedMemOptIn işlevin devingen paylaşımlı belleğini bayt'a kadar açar (48 KB üstü için
	// gerekir); paylasimSiniri blok başına açılabilecek en çok paylaşımlı bellek (bayt).
	sharedMemOptIn(fn uintptr, byteN int) result
	sharedMemLimit() int
	equalize() result
	// streamHandle işlerin sıraya konduğu akış (kitaplık çağrıları da ona konur; 0: varsayılan).
	streamHandle() uintptr
	// Kayıt (graph.go): captureSupported kayıt işlevlerinin varlığı; beginCapture akışı
	// kayda alır; endCapture kaydı bitirip çizgeyi verir (kayıt bozulduysa 0); instantiate
	// çalıştırılabilir çizgeyi kurar; launchGraph onu akışa koyar; destroyGraph ikisini de
	// (0 olmayanı) yok eder.
	captureSupported() bool
	beginCapture() result
	endCapture() (uintptr, result)
	instantiate(graph uintptr) (uintptr, result)
	launchGraph(exec uintptr) result
	destroyGraph(graph, exec uintptr)
	// errName sonuc kodunun adı (CUDA_ERROR_...).
	errName(s result) string
	// detail son hatanın sürücüye özgü açıklaması (benzetimde çekirdek ve komut).
	detail() error
	// capability hesaplama yeteneği (ör. 120 = sm_120) ve SM sayısı.
	capability() (cc, sm int)
	// Olaylar (profil): olayOlustur zamanlamalı bir olay verir; olayKaydet olayı sıraya
	// koyar; olayHazir olay geçildiyse doğru döner (beklemez); olaySure iki geçilmiş
	// olay arasındaki süre (ms).
	createEvent() (uintptr, result)
	recordEvent(ev uintptr) result
	eventReady(ev uintptr) bool
	eventTime(begin, last uintptr) (float32, result)
	// flushTools bağlı profil araçlarına (dış profilleyiciler) toplanan veriyi
	// yazdırır (program sonunda; araç yoksa etkisiz).
	flushTools()
}

// errNames sürücü dönüş kodlarının adlarıdır (sürücü adı veremezse ve benzetimde).
var errNames = map[result]string{
	0: "CUDA_SUCCESS", 1: "CUDA_ERROR_INVALID_VALUE", 2: "CUDA_ERROR_OUT_OF_MEMORY",
	3: "CUDA_ERROR_NOT_INITIALIZED", 4: "CUDA_ERROR_DEINITIALIZED", 34: "CUDA_ERROR_STUB_LIBRARY",
	46: "CUDA_ERROR_DEVICE_UNAVAILABLE", 100: "CUDA_ERROR_NO_DEVICE", 101: "CUDA_ERROR_INVALID_DEVICE",
	200: "CUDA_ERROR_INVALID_IMAGE", 201: "CUDA_ERROR_INVALID_CONTEXT", 209: "CUDA_ERROR_NO_BINARY_FOR_GPU",
	218: "CUDA_ERROR_INVALID_PTX", 221: "CUDA_ERROR_JIT_COMPILER_NOT_FOUND",
	222: "CUDA_ERROR_UNSUPPORTED_PTX_VERSION", 223: "CUDA_ERROR_JIT_COMPILATION_DISABLED",
	300: "CUDA_ERROR_INVALID_SOURCE", 301: "CUDA_ERROR_FILE_NOT_FOUND", 400: "CUDA_ERROR_INVALID_HANDLE",
	401: "CUDA_ERROR_ILLEGAL_STATE", 500: "CUDA_ERROR_NOT_FOUND", 600: "CUDA_ERROR_NOT_READY",
	700: "CUDA_ERROR_ILLEGAL_ADDRESS", 701: "CUDA_ERROR_LAUNCH_OUT_OF_RESOURCES",
	702: "CUDA_ERROR_LAUNCH_TIMEOUT", 715: "CUDA_ERROR_ILLEGAL_INSTRUCTION",
	716: "CUDA_ERROR_MISALIGNED_ADDRESS", 717: "CUDA_ERROR_INVALID_ADDRESS_SPACE", 718: "CUDA_ERROR_INVALID_PC",
	719: "CUDA_ERROR_LAUNCH_FAILED", 800: "CUDA_ERROR_NOT_PERMITTED", 801: "CUDA_ERROR_NOT_SUPPORTED",
	803: "CUDA_ERROR_SYSTEM_DRIVER_MISMATCH", 804: "CUDA_ERROR_COMPAT_NOT_SUPPORTED_ON_DEVICE",
	900: "CUDA_ERROR_STREAM_CAPTURE_UNSUPPORTED", 901: "CUDA_ERROR_STREAM_CAPTURE_INVALIDATED",
	906: "CUDA_ERROR_STREAM_CAPTURE_IMPLICIT", 910: "CUDA_ERROR_GRAPH_EXEC_UPDATE_FAILURE",
	999: "CUDA_ERROR_UNKNOWN",
}

// nameFromTable kodun tablodaki adı; yoksa sayısıyla.
func nameFromTable(s result) string {
	if ad, ok := errNames[s]; ok {
		return ad
	}
	return messages.Text(messages.H4019, int32(s))
}

// Sık kullanılan kodlar.
const (
	errInvalidValue   result = 1
	errNoMemory       result = 2
	errInvalidPTX     result = 218
	errInvalidAddress result = 700
	errLaunch         result = 719
	// errCaptureInvalidated kayıt bozuldu; errCaptureImplicit kayıt sürerken eşzamanlama.
	errCaptureInvalidated result = 901
	errCaptureImplicit    result = 906
)

// staticSharedMemLimit blok başına statik (.shared dizileriyle) paylaşımlı belleğin sınırıdır;
// üstü devingen paylaşımlı bellekle (.extern .shared) ve işlev başına izinle açılır.
const staticSharedMemLimit = 48 << 10
