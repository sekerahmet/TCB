package gpu

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

// ============================================================================
// Gerçek NVIDIA sürücüsü (cgo'suz)
// ============================================================================
//
// Sürücü kitaplığı çalışma anında yüklenir (Linux: libcuda.so.1, purego ile; Windows:
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

// kutuphane sürücü kitaplığından işlev adresi bulan ve çağıran platform katmanı.
type kutuphane interface {
	bul(ad string) (uintptr, error)
	cagir(fn uintptr, args ...uintptr) uintptr
}

// cudaIslevAdlari yüklenen sürücü işlevleri (sıra cudaIslevler alanlarıyla aynı).
var cudaIslevAdlari = [...]string{
	"cuInit", "cuDeviceGetCount", "cuDeviceGet", "cuDeviceGetName", "cuDeviceTotalMem_v2",
	"cuDeviceGetAttribute", "cuDevicePrimaryCtxRetain", "cuCtxSetCurrent", "cuCtxSynchronize",
	"cuMemAlloc_v2", "cuMemFree_v2", "cuMemAllocHost_v2", "cuMemcpyHtoD_v2", "cuMemcpyHtoDAsync_v2",
	"cuMemcpyDtoH_v2", "cuMemcpyDtoDAsync_v2", "cuMemsetD32Async", "cuMemsetD16Async",
	"cuMemGetInfo_v2", "cuModuleLoadDataEx", "cuModuleGetFunction", "cuLaunchKernel",
	"cuGetErrorName", "cuDriverGetVersion",
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
	fSayisi
)

// cudaKitaplik süreç genelinde bir kez yüklenen sürücü kitaplığı.
type cudaKitaplik struct {
	k      kutuphane
	fn     [fSayisi]uintptr
	sayi   int
	surum  int32
	hata   error
	cikti  *cudaCikti
	cagriM sync.Mutex // kitaplık düzeyindeki çağrılar (kurulum)
	adM    sync.Mutex // hata adı çıkış alanı
	ad     *struct{ g unsafe.Pointer }
}

// cudaCikti sürücünün yazdığı çıkış değerleri (yığında, taşınmaz).
type cudaCikti struct {
	u64, u64b uint64
	i32       int32
	tutamac   uintptr
	gosterge  unsafe.Pointer
	ad        [256]byte
}

var (
	cudaBirKez sync.Once
	cudaKit    *cudaKitaplik
)

// cudaYukle sürücü kitaplığını yükler ve cuInit çağırır (bir kez).
func cudaYukle() *cudaKitaplik {
	cudaBirKez.Do(func() {
		c := &cudaKitaplik{cikti: &cudaCikti{}, ad: &struct{ g unsafe.Pointer }{}}
		cudaKit = c
		k, err := kutuphaneAc()
		if err != nil {
			c.hata = err
			return
		}
		c.k = k
		for i, ad := range cudaIslevAdlari {
			if c.fn[i], err = k.bul(ad); err != nil {
				c.hata = fmt.Errorf("sürücüde %s işlevi yok (sürücü çok eski olabilir): %v", ad, err)
				return
			}
		}
		if s := c.cagir(fInit, 0); s != basarili {
			c.hata = fmt.Errorf("cuInit başarısız: %s", c.hataAdi(s))
			return
		}
		c.cagir(fDriverGetVersion, uintptr(unsafe.Pointer(&c.cikti.i32)))
		c.surum = c.cikti.i32
		if s := c.cagir(fDeviceGetCount, uintptr(unsafe.Pointer(&c.cikti.i32))); s != basarili {
			c.hata = fmt.Errorf("cuDeviceGetCount başarısız: %s", c.hataAdi(s))
			return
		}
		c.sayi = int(c.cikti.i32)
	})
	return cudaKit
}

func (c *cudaKitaplik) cagir(f int, args ...uintptr) sonuc {
	r := c.k.cagir(c.fn[f], args...)
	runtime.KeepAlive(c.cikti)
	return sonuc(int32(uint32(r)))
}

// hataAdi sürücüden kodun adını ister (cuGetErrorName); veremezse tablodan.
func (c *cudaKitaplik) hataAdi(s sonuc) string {
	if c.k == nil || c.fn[fGetErrorName] == 0 {
		return tablodanAd(s)
	}
	c.adM.Lock()
	defer c.adM.Unlock()
	c.ad.g = nil
	if r := c.k.cagir(c.fn[fGetErrorName], uintptr(s), uintptr(unsafe.Pointer(&c.ad.g))); r != 0 || c.ad.g == nil {
		return tablodanAd(s)
	}
	return cMetin(c.ad.g, 128)
}

// cMetin sıfırla biten C metnini okur (en çok n bayt).
func cMetin(p unsafe.Pointer, n int) string {
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

// cudaSurucu bir cihazın sürücü bağlantısı.
type cudaSurucu struct {
	c        *cudaKitaplik
	aygit    int32
	baglam   uintptr
	ad       string
	bellek   uint64
	yetenek  [2]int32
	smSayisi int32
	cikti    *cudaCikti
	paramBuf []byte
	paramPtr []uintptr
	jit      *jitAlani
	sonDetay string
}

type jitAlani struct {
	secenek [4]int32
	deger   [4]uintptr
	bilgi   [8192]byte
	hata    [8192]byte
}

// cudaAc verilen sıradaki cihazın birincil bağlamını açar.
func cudaAc(sira int) (*cudaSurucu, error) {
	c := cudaYukle()
	if c.hata != nil {
		return nil, c.hata
	}
	c.cagriM.Lock()
	defer c.cagriM.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s := &cudaSurucu{c: c, cikti: &cudaCikti{}, paramBuf: make([]byte, 0, 8192), jit: &jitAlani{}}
	o := s.cikti
	if r := c.cagir(fDeviceGet, uintptr(unsafe.Pointer(&o.i32)), uintptr(sira)); r != basarili {
		return nil, fmt.Errorf("cuDeviceGet(%d): %s", sira, c.hataAdi(r))
	}
	s.aygit = o.i32
	c.cagir(fDeviceGetName, uintptr(unsafe.Pointer(&o.ad[0])), uintptr(len(o.ad)), uintptr(s.aygit))
	s.ad = cMetin(unsafe.Pointer(&o.ad[0]), len(o.ad))
	if r := c.cagir(fDeviceTotalMem, uintptr(unsafe.Pointer(&o.u64)), uintptr(s.aygit)); r != basarili {
		return nil, fmt.Errorf("cuDeviceTotalMem: %s", c.hataAdi(r))
	}
	s.bellek = o.u64
	for i, oz := range []int{75, 76, 16} { // yetenek büyük/küçük, SM sayısı
		c.cagir(fDeviceGetAttribute, uintptr(unsafe.Pointer(&o.i32)), uintptr(oz), uintptr(s.aygit))
		if i < 2 {
			s.yetenek[i] = o.i32
		} else {
			s.smSayisi = o.i32
		}
	}
	if r := c.cagir(fPrimaryCtxRetain, uintptr(unsafe.Pointer(&o.tutamac)), uintptr(s.aygit)); r != basarili {
		return nil, fmt.Errorf("cuDevicePrimaryCtxRetain: %s", c.hataAdi(r))
	}
	s.baglam = o.tutamac
	if r := s.bagla(); r != basarili {
		return nil, fmt.Errorf("cuCtxSetCurrent: %s", c.hataAdi(r))
	}
	return s, nil
}

func (s *cudaSurucu) cagir(f int, args ...uintptr) sonuc {
	r := s.c.cagir(f, args...)
	runtime.KeepAlive(s)
	return r
}

func (s *cudaSurucu) bagla() sonuc { return s.cagir(fCtxSetCurrent, s.baglam) }

func (s *cudaSurucu) kartAdi() string {
	return fmt.Sprintf("%s (sm_%d%d, %d SM, sürücü CUDA %d.%d)", s.ad, s.yetenek[0], s.yetenek[1], s.smSayisi,
		s.c.surum/1000, s.c.surum%1000/10)
}

func (s *cudaSurucu) toplamBellek() uint64 { return s.bellek }

func (s *cudaSurucu) bosBellek() (uint64, uint64, sonuc) {
	r := s.cagir(fMemGetInfo, uintptr(unsafe.Pointer(&s.cikti.u64)), uintptr(unsafe.Pointer(&s.cikti.u64b)))
	return s.cikti.u64, s.cikti.u64b, r
}

func (s *cudaSurucu) ayir(bayt uint64) (uint64, sonuc) {
	r := s.cagir(fMemAlloc, uintptr(unsafe.Pointer(&s.cikti.u64)), uintptr(bayt))
	return s.cikti.u64, r
}

func (s *cudaSurucu) birak(adr uint64) sonuc { return s.cagir(fMemFree, uintptr(adr)) }

func (s *cudaSurucu) konakAyir(bayt int) (unsafe.Pointer, sonuc) {
	r := s.cagir(fMemAllocHost, uintptr(unsafe.Pointer(&s.cikti.gosterge)), uintptr(bayt))
	return s.cikti.gosterge, r
}

func (s *cudaSurucu) yukle(hedef uint64, kaynak unsafe.Pointer, bayt uint64, esz bool) sonuc {
	if esz {
		return s.cagir(fMemcpyHtoDAsync, uintptr(hedef), uintptr(kaynak), uintptr(bayt), 0)
	}
	return s.cagir(fMemcpyHtoD, uintptr(hedef), uintptr(kaynak), uintptr(bayt))
}

func (s *cudaSurucu) indir(hedef unsafe.Pointer, kaynak uint64, bayt uint64) sonuc {
	return s.cagir(fMemcpyDtoH, uintptr(hedef), uintptr(kaynak), uintptr(bayt))
}

func (s *cudaSurucu) kopyala(hedef, kaynak, bayt uint64) sonuc {
	return s.cagir(fMemcpyDtoDAsync, uintptr(hedef), uintptr(kaynak), uintptr(bayt), 0)
}

func (s *cudaSurucu) doldur32(hedef uint64, deger uint32, n uint64) sonuc {
	return s.cagir(fMemsetD32Async, uintptr(hedef), uintptr(deger), uintptr(n), 0)
}

func (s *cudaSurucu) doldur16(hedef uint64, deger uint16, n uint64) sonuc {
	return s.cagir(fMemsetD16Async, uintptr(hedef), uintptr(deger), uintptr(n), 0)
}

// modul PTX'i cuModuleLoadDataEx ile derler; derleyicinin bilgi ve hata günlüklerini
// toplar.
func (s *cudaSurucu) modul(ptx string) (uintptr, string, sonuc) {
	j := s.jit
	j.secenek = [4]int32{3, 4, 5, 6} // bilgi günlüğü, boyu, hata günlüğü, boyu
	j.deger = [4]uintptr{uintptr(unsafe.Pointer(&j.bilgi[0])), uintptr(len(j.bilgi)),
		uintptr(unsafe.Pointer(&j.hata[0])), uintptr(len(j.hata))}
	metin := append([]byte(ptx), 0)
	r := s.cagir(fModuleLoadDataEx, uintptr(unsafe.Pointer(&s.cikti.tutamac)), uintptr(unsafe.Pointer(&metin[0])),
		4, uintptr(unsafe.Pointer(&j.secenek[0])), uintptr(unsafe.Pointer(&j.deger[0])))
	runtime.KeepAlive(metin)
	gunluk := strings.TrimSpace(cMetin(unsafe.Pointer(&j.hata[0]), len(j.hata)))
	if b := strings.TrimSpace(cMetin(unsafe.Pointer(&j.bilgi[0]), len(j.bilgi))); b != "" {
		gunluk = strings.TrimSpace(gunluk + "\n" + b)
	}
	return s.cikti.tutamac, gunluk, r
}

func (s *cudaSurucu) islev(modul uintptr, ad string) (uintptr, sonuc) {
	b := append([]byte(ad), 0)
	r := s.cagir(fModuleGetFunction, uintptr(unsafe.Pointer(&s.cikti.tutamac)), modul, uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
	return s.cikti.tutamac, r
}

// baslat parametreleri kalıcı tampona kopyalar (her biri 8 bayta hizalı) ve
// cuLaunchKernel'e her parametrenin adresini verir; sürücü değerleri başlatmada kopyalar.
func (s *cudaSurucu) baslat(fn uintptr, izgara, blok [3]uint32, param [][]byte) sonuc {
	toplam := 0
	for _, p := range param {
		toplam += (len(p) + 7) &^ 7
	}
	if cap(s.paramBuf) < toplam {
		s.paramBuf = make([]byte, 0, 2*toplam)
	}
	buf := s.paramBuf[:toplam]
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
	r := s.cagir(fLaunchKernel, fn, uintptr(izgara[0]), uintptr(izgara[1]), uintptr(izgara[2]),
		uintptr(blok[0]), uintptr(blok[1]), uintptr(blok[2]), 0, 0, uintptr(unsafe.Pointer(&s.paramPtr[0])), 0)
	runtime.KeepAlive(buf)
	return r
}

func (s *cudaSurucu) esitle() sonuc { return s.cagir(fCtxSynchronize) }

func (s *cudaSurucu) hataAdi(r sonuc) string { return s.c.hataAdi(r) }

func (s *cudaSurucu) ayrinti() string { return s.sonDetay }
