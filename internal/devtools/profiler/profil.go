// Package profiler TR'nin canlı profilidir: 'tr izle' ve 'tr profil' komutlarının ortak
// çekirdeği. Üç kaynaktan okur:
//
//   - Tensör sayaçları (internal/compute/tensor, sayac.go): her zaman açık, işlem türüne,
//     yöne ve cihaza göre çağrı, süre ve bayt.
//
//   - Yorumlayıcı örneklemesi (ornekleyici.go): yalnızca istenirse, hata ayıklayıcı
//     yuvasıyla; satır ve fonksiyon başına öz süre.
//
//   - Adım sınırı kancası (internal/stdlib/logging): ilerleme(...).güncelle(...) her çağrıda
//     bir adım sınırıdır; ms/adım ve adım başına işlem dağılımı buradan.
//
//   - GPU arka ucunun kendi profili (gpu.go): çekirdeklerin cihazda geçen süresi ve GPU
//     belleği.
//
// Profil saniyede bir anlık görüntü alır; isteyene (canlı tablo) verir ve istenmişse durum
// dosyasına (profil.json, arka plan işinde) atomik yazar. Program bitince son raporu kurar.
// Biçim: goruntu.go; mimari: docs/mimari.md, "Canlı profil".
//
// Katman: araç (internal/devtools); dil, motor, hesap ve araç katmanlarını içe aktarabilir.
package profiler

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Secenekler bir profilin ayarlarıdır.
type Secenekler struct {
	Program string       // raporda görünen program adı (betik yolu)
	Kaynak  string       // ana programın kaynağı (satır metinleri)
	Agac    *ast.Program // ana programın sözdizimi ağacı (hangi fonksiyonlar bu dosyada)
	// Ornekle satır/fonksiyon örneklemesini açar (yorumlayıcının hata ayıklayıcı yuvası).
	Ornekle bool
	// GPUOlcumu GPU çekirdeklerinin cihazdaki süresinin ölçümünü açar (gpu.go; tr izle ve
	// tr profil açar, arka plan işinin profili açmaz).
	GPUOlcumu bool
	// DurumDosyasi boş değilse canlı görüntü saniyede en çok bir kez bu dosyaya yazılır.
	DurumDosyasi string
	// Uyari durum dosyası yazılamazsa (bir kez) çağrılır; nil ise sessiz kalınır.
	Uyari func(string)
}

// Profil çalışan bir programın profilidir.
type Profil struct {
	sec       Secenekler
	baslangic time.Time
	orn       *ornekleyici
	yazici    *durumYazici
	anaFonk   map[string]bool
	satirlar  []string

	adimMu sync.Mutex
	adim   adimDurumu

	mu        sync.Mutex
	taban     anlik   // profil başladığındaki sayaçlar (süreç geneli sayaçlardan çıkarılır)
	gecmis    []anlik // son pencereSuresi'nin ham görüntüleri (pencere için)
	dinleyici func(*Goruntu)

	// GPU çekirdek ölçümü (gpu.go; mu ile korunur): gpuActi ölçümü bu profil açtı ve henüz
	// geri almadı, gpuOnceki açmadan önceki durum, gpuOlculdu ölçüm profil süresince
	// açıktı, gpuOzet arka ucun son raporu (KapanmadanOnce).
	gpuActi, gpuOnceki, gpuOlculdu bool
	gpuOzet                        string

	dur   chan struct{}
	bitti sync.WaitGroup
	once  sync.Once
}

// DurumDosyasiAdi arka plan işinin klasöründeki canlı profil dosyasının adıdır.
const DurumDosyasiAdi = "profil.json"

// IsDurumDosyasi program bir arka plan işi olarak çalışıyorsa (TR_İŞ) iş klasöründeki
// profil.json'un yoludur; değilse boş (durum dosyası yazılmaz).
func IsDurumDosyasi() string {
	if k := logging.IsKlasoru(); k != "" {
		return filepath.Join(k, DurumDosyasiAdi)
	}
	return ""
}

// pencereSuresi canlı tablonun "son" ölçülerinin kapsadığı süredir.
const pencereSuresi = 5 * time.Second

// adimDurumu adım sınırı kancasının biriktirdiğidir.
type adimDurumu struct {
	var_               bool
	ilk, son, toplam   int64
	ilkZaman, sonZaman time.Time
}

// Baslat m üzerinde çalışacak programın profilini başlatır; m.Run'dan önce çağrılır.
// Örnekleme istenmişse m.Debugger'a bağlanır (hata ayıklayıcıyla birlikte kullanılmaz).
func Baslat(m *interp.VM, s Secenekler) *Profil {
	p := &Profil{sec: s, baslangic: time.Now(), dur: make(chan struct{}), anaFonk: anaFonksiyonlar(s.Agac)}
	p.satirlar = strings.Split(s.Kaynak, "\n")
	if s.DurumDosyasi != "" {
		p.yazici = &durumYazici{yol: s.DurumDosyasi, uyari: s.Uyari}
	}
	if s.Ornekle {
		p.orn = yeniOrnekleyici()
		m.Debugger = p.orn
		p.orn.baslat()
	}
	if s.GPUOlcumu {
		// Ölçüm zaten açıksa (TR_GPU_PROFIL=1) dokunulmaz: arka uç raporunu kendisi yazar.
		p.gpuOnceki = gpuOlcumunuAc()
		p.gpuActi, p.gpuOlculdu = !p.gpuOnceki, true
	}
	logging.AdimKancasiAyarla(p.adimSiniri)
	p.mu.Lock()
	p.taban = p.anlikAl()
	p.gecmis = []anlik{p.taban}
	p.mu.Unlock()
	p.bitti.Add(1)
	go p.dongu()
	return p
}

// Dinle her saniye alınan canlı görüntüyü f'ye verir (f goroutine'den çağrılır; kısa olmalı).
func (p *Profil) Dinle(f func(*Goruntu)) {
	p.mu.Lock()
	p.dinleyici = f
	p.mu.Unlock()
}

// adimSiniri ilerleme(...).güncelle(...)'nin her çağrısında çalışır (yorumlayıcı kilidi altında).
func (p *Profil) adimSiniri(a logging.AdimSiniri) {
	p.adimMu.Lock()
	d := &p.adim
	if !d.var_ {
		d.var_, d.ilk, d.ilkZaman = true, a.Adim, a.Zaman
	}
	d.son, d.sonZaman, d.toplam = a.Adim, a.Zaman, a.Toplam
	p.adimMu.Unlock()
}

func (p *Profil) dongu() {
	defer p.bitti.Done()
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-p.dur:
			return
		case <-t.C:
			p.tik()
		}
	}
}

// tik saniyelik görüntüyü alır, geçmişe ekler, dinleyiciye verir ve durum dosyasına yazar.
func (p *Profil) tik() {
	a := p.anlikAl()
	p.mu.Lock()
	p.gecmis = append(p.gecmis, a)
	for len(p.gecmis) > 2 && a.zaman.Sub(p.gecmis[1].zaman) >= pencereSuresi {
		p.gecmis = p.gecmis[1:]
	}
	once := p.gecmis[0]
	f := p.dinleyici
	p.mu.Unlock()
	g := p.kur(a, &once, false)
	if f != nil {
		f(g)
	}
	if p.yazici != nil {
		p.yazici.yaz(g)
	}
}

// Bitir profili durdurur ve son raporu verir; kod programın çıkış kodudur. Birden çok kez
// çağrılabilir (sonraki çağrılar yalnızca raporu yeniden kurar). KapanmadanOnce
// çağrılmadıysa (ör. program başlamadan çıkıldı) GPU ölçümü burada eski durumuna döner.
func (p *Profil) Bitir(kod int) *Goruntu {
	p.once.Do(func() {
		close(p.dur)
		p.bitti.Wait()
		if p.orn != nil {
			p.orn.durdur()
		}
		logging.AdimKancasiAyarla(nil)
		p.mu.Lock()
		if p.gpuActi {
			p.gpuActi = false
			gpu.ProfilAc(p.gpuOnceki)
		}
		p.mu.Unlock()
	})
	a := p.anlikAl()
	p.mu.Lock()
	once := p.gecmis[0]
	p.mu.Unlock()
	g := p.kur(a, &once, true)
	g.CikisKodu = &kod
	if p.yazici != nil {
		p.yazici.yaz(g)
	}
	return g
}

// anlik bir anın ham ölçüleridir (pencere farkları bunlardan alınır).
type anlik struct {
	zaman     time.Time
	sayaclar  []tensor.SayacKaydi
	adim      adimDurumu
	konumlar  []konumOrnegi
	ornekSure time.Duration
	ornek     int64
	bellek    runtime.MemStats
	gpu       gpuAnlik
}

func (p *Profil) anlikAl() anlik {
	a := anlik{zaman: time.Now(), sayaclar: tensor.SayacGoruntusu()}
	p.adimMu.Lock()
	a.adim = p.adim
	p.adimMu.Unlock()
	if p.orn != nil {
		a.konumlar, a.ornekSure, a.ornek = p.orn.goruntu()
	}
	runtime.ReadMemStats(&a.bellek)
	a.gpu = gpuAnlikAl()
	return a
}

// anaFonksiyonlar ana programda tanımlı fonksiyon ve metot adlarıdır (satır metni ancak
// bunlarda ana dosyadan okunur; başka modülün fonksiyonunun satırı başka dosyadadır).
func anaFonksiyonlar(prog *ast.Program) map[string]bool {
	m := map[string]bool{}
	if prog == nil {
		return m
	}
	ekle := func(f *ast.FunctionLiteral) {
		if f != nil && f.Name != "" {
			m[f.Name] = true
		}
	}
	for _, st := range prog.Statements {
		switch s := st.(type) {
		case *ast.ExpressionStatement:
			if f, ok := s.Expression.(*ast.FunctionLiteral); ok {
				ekle(f)
			}
		case *ast.VarStatement:
			if f, ok := s.Value.(*ast.FunctionLiteral); ok {
				ekle(f)
			}
		case *ast.StructStatement:
			for ad := range s.Methods { // metotların çağrı yığınındaki adı
				m[ad] = true
			}
		}
	}
	return m
}
