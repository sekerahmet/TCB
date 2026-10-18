package gpu

import (
	"fmt"
	"unsafe"
)

// ============================================================================
// Sürücü arayüzü
// ============================================================================
//
// Arka uç sürücüye bu dar arayüzden erişir. İki uygulaması vardır: gerçek NVIDIA sürücüsü
// (cuda.go; Linux'ta libcuda.so, Windows'ta nvcuda.dll, cgo'suz) ve PTX benzetimi
// (benzetim.go; donanımsız sınama). Bütün işler tek akışta (varsayılan akış) sırayla
// yürür; "eşzamansız" yöntemler sıraya ekler ve hemen döner, konağa okuma (indir) ve
// esitle sıradaki her işin bitmesini bekler.

// sonuc sürücü dönüş kodudur (CUresult).
type sonuc int32

const basarili sonuc = 0

// surucu arka ucun kullandığı sürücü işlemleri.
type surucu interface {
	// bagla sürücü bağlamını çağıran işletim sistemi iş parçacığına bağlar (her girişte).
	bagla() sonuc
	kartAdi() string
	toplamBellek() uint64
	bosBellek() (bos, toplam uint64, s sonuc)
	ayir(bayt uint64) (uint64, sonuc)
	birak(adr uint64) sonuc
	// konakAyir sayfası kilitli (eşzamansız kopyaya uygun) konak belleği ayırır.
	konakAyir(bayt int) (unsafe.Pointer, sonuc)
	// yukle konaktan cihaza kopyalar; esz doğruysa eşzamansızdır (kaynak sayfası kilitli
	// bellekte olmalı ve kopya bitene kadar değişmemeli).
	yukle(hedef uint64, kaynak unsafe.Pointer, bayt uint64, esz bool) sonuc
	// indir cihazdan konağa eşzamanlı kopyalar (önce sıradaki işler biter).
	indir(hedef unsafe.Pointer, kaynak uint64, bayt uint64) sonuc
	kopyala(hedef, kaynak, bayt uint64) sonuc
	doldur32(hedef uint64, deger uint32, n uint64) sonuc
	doldur16(hedef uint64, deger uint16, n uint64) sonuc
	// modul PTX metnini derler; hata durumunda derleyici günlüğünü de verir.
	modul(ptx string) (uintptr, string, sonuc)
	islev(modul uintptr, ad string) (uintptr, sonuc)
	baslat(fn uintptr, izgara, blok [3]uint32, param [][]byte) sonuc
	esitle() sonuc
	// hataAdi sonuc kodunun adı (CUDA_ERROR_...).
	hataAdi(s sonuc) string
	// ayrinti son hatanın sürücüye özgü açıklaması (benzetimde çekirdek ve komut).
	ayrinti() string
}

// hataAdlari sürücü dönüş kodlarının adlarıdır (sürücü adı veremezse ve benzetimde).
var hataAdlari = map[sonuc]string{
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
	999: "CUDA_ERROR_UNKNOWN",
}

// tablodanAd kodun tablodaki adı; yoksa sayısıyla.
func tablodanAd(s sonuc) string {
	if ad, ok := hataAdlari[s]; ok {
		return ad
	}
	return fmt.Sprintf("CUDA hata kodu %d", int32(s))
}

// Sık kullanılan kodlar.
const (
	hataGecersizDeger sonuc = 1
	hataBellekYok     sonuc = 2
	hataGecersizPTX   sonuc = 218
	hataGecersizAdres sonuc = 700
	hataBaslatma      sonuc = 719
)
