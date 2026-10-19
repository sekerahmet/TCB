// Package gpu, TR tensör çekirdeğinin NVIDIA GPU arka ucudur: tensor.Backend arayüzünü
// sürücü API'si üzerinden uygular. Çekirdekler Go içinden PTX olarak üretilir
// (ptx*.go, cekirdek_*.go) ve sürücünün içindeki derleyiciyle ilk kullanımda karta
// derlenir; derlenen modüller süreç boyunca önbellekte tutulur. cgo ve CUDA araç takımı
// gerekmez: sürücü kitaplığı çalışma anında yüklenir (cuda*.go). Sürücü bulunamazsa
// paket hiçbir şey kaydetmez ve tensörler CPU'da çalışır.
//
// Donanımsız sınama için sürücünün yerine geçen bir PTX benzetimi vardır (benzetim*.go):
// arka ucun bütün çekirdekleri bu makinede de yürütülüp CPU arka ucuyla karşılaştırılır.
//
// Katman: hesap çekirdeği (internal/hesap); yalnızca internal/hesap paketlerini içe aktarır.
package gpu

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	"tr_dili/internal/hesap/tensor"
)

// arkaUc bir GPU'nun tensor.Backend uygulaması.
type arkaUc struct {
	dev tensor.Device
	sr  surucu
	// mu bütün sürücü çağrılarını sıraya koyar (tek akış). Dışa açık yöntemler girişte
	// alır (gir); iç yardımcılar kilidin tutulduğunu varsayar.
	mu          sync.Mutex
	cekirdekler map[string]*cekirdek
	havuz       havuz
	hazirlik    hazirlik
	prm         paramYazici
	sonCekirdek string
	// eszamanli her başlatmadan sonra eşitler (teşhis: hatalı çekirdeği tam yerinde
	// yakalar; TR_GPU_ESZAMANLI=1).
	eszamanli bool
	carpim    carpimAyari
	// carpimSabit doluysa matris çarpımı döşemesi otomatik seçilmez (sınama).
	carpimSabit *carpimAyari
	// prof çekirdek ölçümleri (bkz. profil.go).
	prof profil
	// hacim son çöp toplamadan bu yana verilen depolama baytı (havuzdan gelenler dahil).
	hacim uint64
	// lt matris çarpımı kitaplığı (yüklenemediyse nil: kendi çekirdeğimiz).
	lt      *ltCarpim
	ltNeden string // lt nil ise nedeni (teşhis)
	// ltNotlar kurulumda atlanan kitaplık adayları; ltSonHata kitaplığın son reddettiği çağrı.
	ltNotlar  []string
	ltSonHata string
	// dikkatYolu TR_GPU_DIKKAT: "birlesik" (varsayılan) ya da "carpim" (ölçüm ve karşılaştırma).
	dikkatYolu string
	// copEsigi son çöp toplamadan bu yana bu kadar depolama verilince çöp toplama başlatılır
	// (bkz. copEsigiSec; TR_GPU_COP_ESIGI ile MB olarak değiştirilir).
	copEsigi uint64
}

func init() {
	tensor.RegisterLazyBackend(tensor.KindGPU, tensor.LazyBackend{Count: cihazSayisi, New: kur})
}

// kurulumHatasi son başarısız kurulumun nedeni (teşhis için; farklı gorutinlerden
// yazılıp okunabildiği için kilitli).
var kurulumHatasi struct {
	mu  sync.Mutex
	err error
}

func kurulumHatasiYaz(err error) {
	kurulumHatasi.mu.Lock()
	kurulumHatasi.err = err
	kurulumHatasi.mu.Unlock()
}

func kurulumHatasiOku() error {
	kurulumHatasi.mu.Lock()
	defer kurulumHatasi.mu.Unlock()
	return kurulumHatasi.err
}

// cihazSayisi sürücünün gördüğü GPU sayısı (sürücü yoksa 0; geliştirme için
// TR_GPU_BENZETIM=1 ile benzetim tek GPU sayılır).
func cihazSayisi() int {
	c := cudaYukle()
	if c.hata != nil {
		kurulumHatasiYaz(c.hata)
		if benzetimIstendi() {
			return 1
		}
		return 0
	}
	return c.sayi
}

// benzetimIstendi gerçek GPU yokken PTX benzetiminin "gpu" cihazı olarak kurulmasının
// istenip istenmediği (geliştirme ve sınama aracı: GPU kodunu donanımsız, dil düzeyinde
// uçtan uca çalıştırır; çok yavaştır).
func benzetimIstendi() bool { return os.Getenv("TR_GPU_BENZETIM") == "1" }

func kur(sira int) tensor.Backend {
	if cudaYukle().hata != nil && benzetimIstendi() && sira == 0 {
		return kaydet(yeniArkaUc(tensor.GPU(0), yeniBenzetim(1<<30)))
	}
	s, err := cudaAc(sira)
	if err != nil {
		kurulumHatasiYaz(err)
		return nil
	}
	g := kaydet(yeniArkaUc(tensor.GPU(sira), s))
	g.ltKur()
	return g
}

// Durum sürücünün yüklenme durumunu açıklar (teşhis: GPU neden görünmüyor).
func Durum() string {
	n := cihazSayisi()
	if n == 0 {
		if err := kurulumHatasiOku(); err != nil {
			return "GPU yok: " + err.Error()
		}
		return "GPU yok: sürücü cihaz bulamadı"
	}
	if !tensor.HasDevice(tensor.GPU(0)) {
		return fmt.Sprintf("%d GPU görünüyor ama arka uç kurulamadı: %v", n, kurulumHatasiOku())
	}
	be := tensor.BackendFor(tensor.GPU(0)).(*arkaUc)
	return fmt.Sprintf("%d GPU; gpu: %s, %s bellek; matris çarpımı: %s", n, be.sr.kartAdi(), baytYazi(be.sr.toplamBellek()), be.carpimYolu())
}

func yeniArkaUc(dev tensor.Device, sr surucu) *arkaUc {
	g := &arkaUc{dev: dev, sr: sr, cekirdekler: map[string]*cekirdek{}}
	g.havuz.bos = map[uint64][]uint64{}
	g.eszamanli = os.Getenv("TR_GPU_ESZAMANLI") == "1"
	g.dikkatYolu = os.Getenv("TR_GPU_DIKKAT")
	g.carpim = varsayilanCarpim
	g.copEsigi = copEsigiSec(sr.toplamBellek(), os.Getenv("TR_GPU_COP_ESIGI"))
	return g
}

// copEsigiSec çöp toplama eşiği: kart belleğinin 1/64'ü, 256 MB ile 2 GB arasında (95 GB'ta
// ~1,5 GB, 24 GB'ta 384 MB). Ara tensörlerin cihaz belleği ancak Go çöp toplayıcısı
// çalışınca havuza döner; eşik tepe belleği bir adımın canlı tensörlerine yakın tutar.
// Ölçüm (RTX PRO 6000, gpt30m): eşiksiz 38,9 GB, 2 GB'ta 23,5 GB, 512 MB'ta 21,1 GB tepe;
// hız farkı %0,6. ortam (TR_GPU_COP_ESIGI, MB) verilirse o kullanılır; "0" eşiği kapatır.
func copEsigiSec(toplam uint64, ortam string) uint64 {
	if mb, err := strconv.Atoi(ortam); err == nil && mb >= 0 {
		return uint64(mb) << 20
	}
	return min(max(toplam/64, 256<<20), 2<<30)
}

// BenzetimiKaydet, gerçek bir GPU yoksa PTX benzetimini "gpu" cihazı olarak kaydeder
// (yalnızca sınamalar için: GPU kodunun dil düzeyinde uçtan uca sınanması). Benzetim
// yavaştır; küçük şekiller kullanın. Kaydedildiyse doğru döner.
func BenzetimiKaydet() bool {
	if tensor.HasDevice(tensor.GPU(0)) {
		return false
	}
	tensor.RegisterBackend(kaydet(yeniArkaUc(tensor.GPU(0), yeniBenzetim(1<<30))))
	return true
}

func (g *arkaUc) Device() tensor.Device { return g.dev }

// MemoryLimit (tensor.MemoryLimiter) tek ayırmanın sınırı: cihazın toplam belleği.
func (g *arkaUc) MemoryLimit() int64 { return int64(g.sr.toplamBellek()) }

// MemoryPlace hata mesajlarında anılan yer.
func (g *arkaUc) MemoryPlace() string { return "bu GPU'da" }

// gir kilidi alır, gorutini iş parçacığına kilitler ve bağlamı bağlar.
func (g *arkaUc) gir() {
	g.mu.Lock()
	if gpuKapandi.Load() {
		g.mu.Unlock()
		panic("tensor: GPU: program sonunda dış profilleyici için GPU kapatıldı; GPU artık kullanılamaz")
	}
	g.prm.acik = false // panikle yarıda kalmış bir önceki zincir
	runtime.LockOSThread()
	if s := g.sr.bagla(); s != basarili {
		m := g.hataMetni("cuCtxSetCurrent", s)
		g.cik()
		panic(m)
	}
	g.konakBasla()
}

func (g *arkaUc) cik() {
	g.konakBitir()
	runtime.UnlockOSThread()
	g.mu.Unlock()
}

// hataMetni sürücü hatasının Türkçe açıklaması: kodun adı, son çekirdek, ayrıntı.
func (g *arkaUc) hataMetni(islem string, s sonuc) string {
	m := fmt.Sprintf("tensor: GPU: %s başarısız: %s (%d)", islem, g.sr.hataAdi(s), int32(s))
	if g.sonCekirdek != "" {
		m += "; son başlatılan çekirdek: " + g.sonCekirdek
		if !g.eszamanli {
			m += " (hata daha önceki bir çekirdekten de olabilir; yerini bulmak için TR_GPU_ESZAMANLI=1 ile çalıştırın)"
		}
	}
	if d := g.sr.ayrinti(); d != "" {
		m += "\n" + d
	}
	return m
}

// dene sürücü dönüşünü denetler; hata Türkçe paniğe çevrilir (dil 'dene/yakala' ile
// yakalayabilir).
func (g *arkaUc) dene(islem string, s sonuc) {
	if s != basarili {
		panic(g.hataMetni(islem, s))
	}
}

// esitle sıradaki bütün işlerin bitmesini bekler.
func (g *arkaUc) esitle() {
	g.esitBasla()
	g.dene("cuCtxSynchronize", g.sr.esitle())
	g.hazirlik.ofset = 0
	g.esitBitir()
}

// enCok tek bir çekirdeğin eleman sayısı sınırı (32 bitlik indeksler).
const enCok = 1<<31 - 1

// boyDenetle n elemanın tek çekirdekte işlenebildiğini denetler.
func boyDenetle(n int) {
	if n > enCok {
		panic(fmt.Sprintf("tensor: GPU: tek işlemde %d eleman var; bu sürümde GPU işlemleri en çok 2^31 eleman işler", n))
	}
}

// yok GPU'da henüz olmayan bir işlem için açık hata verir (sessizce CPU'ya düşülmez).
func yok(islem string) {
	panic("tensor: bu işlem GPU'da henüz yok: " + islem)
}

func baytYazi(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	}
	return fmt.Sprintf("%d bayt", b)
}
