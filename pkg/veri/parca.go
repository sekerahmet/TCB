package veri

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

var (
	// ErrKapali, kapatılmış bir kaynağın kullanılmaya çalışıldığını bildirir.
	ErrKapali = errors.New("veri: kaynak kapatılmış")

	errEslemeYok   = errors.New("veri: bu platformda bellek eşleme yok")
	errEslemeBos   = errors.New("veri: boş veri eşlenemez")
	errEslemeBuyuk = errors.New("veri: dosya adres alanına sığmıyor")
)

// acikParcaSayisi, açık (kapatılmamış) Parca sayısıdır; sızıntı testleri için.
var acikParcaSayisi atomic.Int64

// ParcaSecenekleri, bir parçanın nasıl açılacağını belirler.
type ParcaSecenekleri struct {
	// EslemeKapali true ise bellek eşleme hiç denenmez, her okuma ReadAt
	// ile yapılır (ör. ağ sürücülerinde eşleme sorun çıkarıyorsa).
	EslemeKapali bool
}

// Parca, tek boyutlu tek bir .npy dosyasıdır. Veri belleğe kopyalanmaz:
// mümkünse dosya belleğe eşlenir, değilse her okuma ReadAt ile yapılır.
//
// Oku yöntemleri birden çok gorutinden aynı anda çağrılabilir. Kapat,
// süren okumaların bitmesini bekler.
type Parca struct {
	yol    string
	baslik Baslik

	mu     sync.RWMutex
	kapali bool
	f      *os.File     // ReadAt yolunda açık; eşlemeli yolda nil
	harita []byte       // tüm dosyanın eşlemesi (eşlemeli yolda)
	veri   []byte       // harita içindeki veri bölümü
	kapatE func() error // eşlemeyi serbest bırakır
	birak  func()       // önbellek iğnesini bırakır (varsa)
	eslHat error        // eşleme denendiyse ve başarısız olduysa nedeni
	tampon *sync.Pool   // ReadAt yolu için bayt tamponları
}

// ParcaAc, yol'daki .npy dosyasını açar ve başlığını doğrular.
func ParcaAc(yol string, s ParcaSecenekleri) (*Parca, error) {
	f, err := os.Open(yol)
	if err != nil {
		return nil, fmt.Errorf("veri: parça açılamadı: %w", err)
	}
	p, err := parcaKur(yol, f, s)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	acikParcaSayisi.Add(1)
	return p, nil
}

func parcaKur(yol string, f *os.File, s ParcaSecenekleri) (*Parca, error) {
	bs, err := BaslikOku(f)
	if err != nil {
		var bh *BicimHatasi
		if errors.As(err, &bh) {
			bh.Yol = yol
		}
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("veri: %s: %w", yol, err)
	}
	gereken := bs.VeriOfseti + bs.VeriBoyutu()
	if st.Size() < gereken {
		return nil, &BicimHatasi{Yol: yol, Neden: fmt.Sprintf(
			"dosya kesik: başlık %d token (%d bayt) bildiriyor, dosya %d bayt", bs.Uzunluk, gereken, st.Size())}
	}
	p := &Parca{yol: yol, baslik: bs, f: f}
	if !s.EslemeKapali && eslemeDestekli && bs.Uzunluk > 0 {
		harita, kapat, err := dosyaEsle(f, gereken)
		if err == nil {
			p.harita, p.kapatE = harita, kapat
			p.veri = harita[bs.VeriOfseti:gereken]
			// Eşleme dosya tutamağından bağımsızdır; tutamağı hemen bırak.
			_ = f.Close()
			p.f = nil
		} else {
			p.eslHat = err
		}
	}
	if p.f != nil {
		p.tampon = &sync.Pool{New: func() any { b := make([]byte, 0, 64<<10); return &b }}
	}
	return p, nil
}

// Yol, parçanın dosya yoludur.
func (p *Parca) Yol() string { return p.yol }

// Baslik, çözümlenmiş .npy başlığıdır.
func (p *Parca) Baslik() Baslik { return p.baslik }

// Uzunluk, parçadaki token sayısıdır.
func (p *Parca) Uzunluk() int64 { return p.baslik.Uzunluk }

// Tur, eleman türüdür.
func (p *Parca) Tur() Tur { return p.baslik.Tur }

// Eslemeli, parçanın belleğe eşlenerek okunup okunmadığıdır.
func (p *Parca) Eslemeli() bool { return p.veri != nil }

// EslemeHatasi, eşleme denenip başarısız olduysa nedenidir (yoksa nil).
func (p *Parca) EslemeHatasi() error { return p.eslHat }

func (p *Parca) aralikDenetle(n int, bas int64) error {
	if bas < 0 || int64(n) > p.baslik.Uzunluk-bas {
		return fmt.Errorf("veri: %s: [%d, %d) aralığı parça sınırını (%d) aşıyor",
			p.yol, bas, bas+int64(n), p.baslik.Uzunluk)
	}
	return nil
}

// Oku, bas konumundan başlayarak len(dst) tokenı dst'ye int32 olarak okur.
// Aralık parça dışına taşarsa hata döner. '<u4' değerleri 2^31'i aşarsa
// int32'ye taşar; sözlük boyutları bunun çok altındadır.
func (p *Parca) Oku(dst []int32, bas int64) error {
	if err := p.aralikDenetle(len(dst), bas); err != nil {
		return err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.kapali {
		return ErrKapali
	}
	if len(dst) == 0 {
		return nil
	}
	es := p.baslik.Tur.Boyut()
	if p.veri != nil {
		cozI32(dst, p.veri[bas*int64(es):(bas+int64(len(dst)))*int64(es)], p.baslik.Tur)
		return nil
	}
	return p.readAtOku(len(dst), bas, func(i int, b []byte, k int) { cozI32(dst[i:i+k], b, p.baslik.Tur) })
}

// OkuU16, bas konumundan başlayarak len(dst) tokenı dst'ye uint16 olarak
// okur. Yalnızca '<u2' parçalarda kayıpsızdır; diğer türlerde değerler
// 16 bite kırpılmaz, hata döner.
func (p *Parca) OkuU16(dst []uint16, bas int64) error {
	if p.baslik.Tur != TurU16 {
		return fmt.Errorf("veri: %s: OkuU16 yalnızca '<u2' parçalarda kullanılabilir (tür %s)", p.yol, p.baslik.Tur)
	}
	if err := p.aralikDenetle(len(dst), bas); err != nil {
		return err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.kapali {
		return ErrKapali
	}
	if len(dst) == 0 {
		return nil
	}
	if p.veri != nil {
		cozU16(dst, p.veri[bas*2:(bas+int64(len(dst)))*2])
		return nil
	}
	return p.readAtOku(len(dst), bas, func(i int, b []byte, k int) { cozU16(dst[i:i+k], b) })
}

// readAtOku, n elemanı parçalar hâlinde ReadAt ile okur ve her parçayı cevir'e
// verir. Kilit çağıran tarafından tutulur.
func (p *Parca) readAtOku(n int, bas int64, cevir func(i int, b []byte, k int)) error {
	es := p.baslik.Tur.Boyut()
	tp := p.tampon.Get().(*[]byte)
	defer p.tampon.Put(tp)
	parcaEleman := cap(*tp) / es
	for i := 0; i < n; i += parcaEleman {
		k := min(parcaEleman, n-i)
		b := (*tp)[:k*es]
		ofs := p.baslik.VeriOfseti + (bas+int64(i))*int64(es)
		m, err := p.f.ReadAt(b, ofs)
		if m < len(b) {
			if err == nil || errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return fmt.Errorf("veri: %s: okuma hatası: %w", p.yol, err)
		}
		cevir(i, b, k)
	}
	return nil
}

// Kapat, eşlemeyi ve dosyayı serbest bırakır. Birden çok kez çağrılabilir.
func (p *Parca) Kapat() error {
	p.mu.Lock()
	if p.kapali {
		p.mu.Unlock()
		return nil
	}
	p.kapali = true
	var hatalar []error
	if p.kapatE != nil {
		hatalar = append(hatalar, p.kapatE())
		p.kapatE = nil
	}
	p.harita, p.veri = nil, nil
	if p.f != nil {
		hatalar = append(hatalar, p.f.Close())
		p.f = nil
	}
	birak := p.birak
	p.birak = nil
	p.mu.Unlock()
	// Önbellek iğnesi, eşleme kaldırıldıktan sonra bırakılmalı: bazı
	// platformlarda eşlenmiş dosya silinemez.
	if birak != nil {
		birak()
	}
	acikParcaSayisi.Add(-1)
	return errors.Join(hatalar...)
}

// ---------------------------------------------------------------------------
// Küçük uçlu çözme döngüleri
// ---------------------------------------------------------------------------

func cozI32(dst []int32, b []byte, t Tur) {
	switch t {
	case TurU16:
		b = b[:2*len(dst)]
		for i := range dst {
			dst[i] = int32(binary.LittleEndian.Uint16(b[2*i:]))
		}
	case TurU32, TurI32:
		b = b[:4*len(dst)]
		for i := range dst {
			dst[i] = int32(binary.LittleEndian.Uint32(b[4*i:]))
		}
	}
}

func cozU16(dst []uint16, b []byte) {
	b = b[:2*len(dst)]
	for i := range dst {
		dst[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
}
