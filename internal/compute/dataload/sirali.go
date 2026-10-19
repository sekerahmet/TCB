package dataload

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// ---------------------------------------------------------------------------
// Sıralı okuma (doğrulama)
// ---------------------------------------------------------------------------

// SiraliSecenekleri, sıralı okumanın ayarlarıdır.
type SiraliSecenekleri struct {
	B, T int
	// Adim, ardışık pencere başlangıçları arasındaki token sayısıdır.
	// Varsayılan T: her token en çok bir kez hedef olur; parçanın ilk tokenı
	// ve son pencereye sığmayan kuyruğu (en çok T-1 token) hedef olmaz.
	Adim int
	// Baslangic verilirse okuma bu konumdan sürer (bir toplu işin Durum'u: Sira parça
	// sırası, Pencere o parçadaki sonraki pencerenin token konumu). Sira parça sayısına
	// eşitse okuyucu baştan bitmiştir.
	Baslangic *Durum
}

// SiraliOkuyucu, parçaları ad sırasıyla, karıştırmadan ve pencereleri
// sırayla okur. Pencereler parça sınırını aşmaz. Son toplu iş B'den az
// satırlı olabilir (ti.B buna göre ayarlanır); ardından io.EOF döner.
// Yöntemler kilitle korunur; Kapat süren bir Doldur'un bitmesini bekler.
type SiraliOkuyucu struct {
	mu     sync.Mutex
	k      *Kume
	s      SiraliSecenekleri
	satir  int64
	sira   int
	bas    int64
	parca  *Parca
	ilkBas int64 // Baslangic'ın pencere konumu: ilk açılan parçada bas bununla başlar
	kapali bool
}

// Sirali, kümeden sıralı okuyan bir okuyucu kurar.
func (k *Kume) Sirali(s SiraliSecenekleri) (*SiraliOkuyucu, error) {
	if s.B <= 0 || s.T <= 0 {
		return nil, fmt.Errorf("veri: B ve T pozitif olmalı (B=%d, T=%d)", s.B, s.T)
	}
	if s.Adim == 0 {
		s.Adim = s.T
	}
	if s.Adim < 0 {
		return nil, fmt.Errorf("veri: adım negatif olamaz")
	}
	if err := k.onbellekKotaDenetle(1, int64(s.T)+1); err != nil {
		return nil, err
	}
	so := &SiraliOkuyucu{k: k, s: s, satir: int64(s.T) + 1}
	if b := s.Baslangic; b != nil {
		if b.Sira < 0 || b.Sira > len(k.parcalar) || b.Pencere < 0 || (b.Sira == len(k.parcalar) && b.Pencere != 0) {
			return nil, fmt.Errorf("veri: geçersiz başlangıç durumu %+v (parça sayısı %d)", *b, len(k.parcalar))
		}
		so.sira, so.ilkBas = b.Sira, b.Pencere
	}
	return so, nil
}

// PencereSayisi, okuyucunun baştan sona üreteceği pencere sayısıdır.
func (so *SiraliOkuyucu) PencereSayisi() int64 {
	var t int64
	for _, p := range so.k.parcalar {
		if p.Token >= so.satir {
			t += (p.Token-so.satir)/int64(so.s.Adim) + 1
		}
	}
	return t
}

// Doldur, ti'yi sonraki en çok B pencereyle doldurur; bitince io.EOF döner.
func (so *SiraliOkuyucu) Doldur(ctx context.Context, ti *TopluIs) error {
	so.mu.Lock()
	defer so.mu.Unlock()
	if so.kapali {
		return ErrKapali
	}
	ti.hazirla(so.s.B, so.s.T)
	r := 0
	for r < so.s.B {
		if so.parca == nil {
			if so.sira >= len(so.k.parcalar) {
				break
			}
			if so.k.parcalar[so.sira].Token < so.satir {
				so.sira++
				so.ilkBas = 0
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			p, err := so.k.ParcaAc(ctx, so.sira)
			if err != nil {
				return err
			}
			so.parca, so.bas = p, so.ilkBas
			so.ilkBas = 0
		}
		if so.bas+so.satir > so.parca.Uzunluk() {
			err := so.parca.Kapat()
			so.parca = nil
			so.sira++
			so.bas = 0
			if err != nil {
				return err
			}
			continue
		}
		if err := so.parca.Oku(ti.Satir(r), so.bas); err != nil {
			return err
		}
		so.bas += int64(so.s.Adim)
		r++
	}
	if r == 0 {
		return io.EOF
	}
	ti.B = r
	ti.Veri = ti.Veri[:r*int(so.satir)]
	ti.Durum = Durum{Sira: so.sira, Pencere: so.bas}
	return nil
}

// Basa, okuyucuyu başa sarar.
func (so *SiraliOkuyucu) Basa() error {
	so.mu.Lock()
	defer so.mu.Unlock()
	return so.basa()
}

func (so *SiraliOkuyucu) basa() error {
	var err error
	if so.parca != nil {
		err = so.parca.Kapat()
		so.parca = nil
	}
	so.sira, so.bas, so.ilkBas = 0, 0, 0
	return err
}

// Kapat, açık parçayı kapatır.
func (so *SiraliOkuyucu) Kapat() error {
	so.mu.Lock()
	defer so.mu.Unlock()
	so.kapali = true
	return so.basa()
}
