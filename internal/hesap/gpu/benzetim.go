package gpu

import (
	"fmt"
	"unsafe"
)

// ============================================================================
// Benzetim sürücüsü: cihaz belleği Go dilimleri, çekirdekler PTX benzetimi
// ============================================================================
//
// Eşzamansız işler (çekirdek başlatma, eşzamansız kopya, doldurma) gerçek sürücüdeki
// gibi sıraya eklenir ve ancak eşzamanlamada (indir, esitle, eşzamanlı yükleme)
// yürütülür. Böylece konak tarafında "kopya bitmeden kaynağın değiştirilmesi" gibi
// sıralama hataları benzetimde de yanlış sonuç verir ve sınamada yakalanır. Çekirdek
// parametreleri, gerçek sürücüdeki gibi başlatma anında kopyalanır.

type benzetimSurucu struct {
	bel      bzBellek
	islevler []*bzIslev
	sira     []func() error
	hata     sonuc
	detay    string
	bellek   uint64 // bildirilen toplam bellek
	ayrilan  uint64
	boyut    map[uint64]uint64
}

// yeniBenzetim toplam belleği verilen bir benzetim sürücüsü kurar.
func yeniBenzetim(bellek uint64) *benzetimSurucu {
	return &benzetimSurucu{bellek: bellek, boyut: map[uint64]uint64{}}
}

func (b *benzetimSurucu) bagla() sonuc         { return b.hata }
func (b *benzetimSurucu) kartAdi() string      { return "benzetim (PTX yorumlayıcısı)" }
func (b *benzetimSurucu) toplamBellek() uint64 { return b.bellek }

func (b *benzetimSurucu) bosBellek() (uint64, uint64, sonuc) {
	return b.bellek - b.ayrilan, b.bellek, basarili
}

func (b *benzetimSurucu) ayir(bayt uint64) (uint64, sonuc) {
	if b.ayrilan+bayt > b.bellek {
		return 0, hataBellekYok
	}
	adr := b.bel.ayir(bayt)
	b.boyut[adr] = bayt
	b.ayrilan += bayt
	return adr, basarili
}

func (b *benzetimSurucu) birak(adr uint64) sonuc {
	if s := b.bosalt(); s != basarili {
		return s
	}
	if !b.bel.birak(adr) {
		return hataGecersizDeger
	}
	b.ayrilan -= b.boyut[adr]
	delete(b.boyut, adr)
	return basarili
}

func (b *benzetimSurucu) konakAyir(bayt int) (unsafe.Pointer, sonuc) {
	d := make([]byte, bayt)
	return unsafe.Pointer(&d[0]), basarili
}

// ekle bir işi sıraya ekler; yapışkan hata varsa eklenmez.
func (b *benzetimSurucu) ekle(f func() error) sonuc {
	if b.hata != basarili {
		return b.hata
	}
	b.sira = append(b.sira, f)
	return basarili
}

// bosalt sıradaki işleri yürütür; ilk hata yapışkandır (gerçek sürücüdeki gibi bağlam
// bundan sonra kullanılamaz).
func (b *benzetimSurucu) bosalt() sonuc {
	if b.hata != basarili {
		return b.hata
	}
	sira := b.sira
	b.sira = nil
	for _, f := range sira {
		if err := f(); err != nil {
			b.hata, b.detay = hataGecersizAdres, err.Error()
			return b.hata
		}
	}
	return basarili
}

func (b *benzetimSurucu) yukle(hedef uint64, kaynak unsafe.Pointer, bayt uint64, esz bool) sonuc {
	kopya := func() error {
		d, err := b.bel.dilim(hedef, int(bayt))
		if err != nil {
			return fmt.Errorf("yükleme: %v", err)
		}
		copy(d, unsafe.Slice((*byte)(kaynak), bayt))
		return nil
	}
	if bayt == 0 {
		return basarili
	}
	if esz {
		return b.ekle(kopya)
	}
	if s := b.bosalt(); s != basarili {
		return s
	}
	if err := kopya(); err != nil {
		b.hata, b.detay = hataGecersizAdres, err.Error()
	}
	return b.hata
}

func (b *benzetimSurucu) indir(hedef unsafe.Pointer, kaynak uint64, bayt uint64) sonuc {
	if s := b.bosalt(); s != basarili || bayt == 0 {
		return s
	}
	d, err := b.bel.dilim(kaynak, int(bayt))
	if err != nil {
		b.hata, b.detay = hataGecersizAdres, "indirme: "+err.Error()
		return b.hata
	}
	copy(unsafe.Slice((*byte)(hedef), bayt), d)
	return basarili
}

func (b *benzetimSurucu) kopyala(hedef, kaynak, bayt uint64) sonuc {
	return b.ekle(func() error {
		d, err := b.bel.dilim(hedef, int(bayt))
		if err != nil {
			return fmt.Errorf("cihaz içi kopya hedefi: %v", err)
		}
		k, err := b.bel.dilim(kaynak, int(bayt))
		if err != nil {
			return fmt.Errorf("cihaz içi kopya kaynağı: %v", err)
		}
		copy(d, k)
		return nil
	})
}

func (b *benzetimSurucu) doldur32(hedef uint64, deger uint32, n uint64) sonuc {
	return b.ekle(func() error {
		d, err := b.bel.dilim(hedef, int(4*n))
		if err != nil {
			return fmt.Errorf("doldurma: %v", err)
		}
		for i := uint64(0); i < n; i++ {
			d[4*i], d[4*i+1], d[4*i+2], d[4*i+3] = byte(deger), byte(deger>>8), byte(deger>>16), byte(deger>>24)
		}
		return nil
	})
}

func (b *benzetimSurucu) doldur16(hedef uint64, deger uint16, n uint64) sonuc {
	return b.ekle(func() error {
		d, err := b.bel.dilim(hedef, int(2*n))
		if err != nil {
			return fmt.Errorf("doldurma: %v", err)
		}
		for i := uint64(0); i < n; i++ {
			d[2*i], d[2*i+1] = byte(deger), byte(deger>>8)
		}
		return nil
	})
}

func (b *benzetimSurucu) modul(ptx string) (uintptr, string, sonuc) {
	is, err := bzAyristir(ptx)
	if err != nil {
		return 0, err.Error(), hataGecersizPTX
	}
	b.islevler = append(b.islevler, is)
	return uintptr(len(b.islevler)), "", basarili
}

func (b *benzetimSurucu) islev(modul uintptr, ad string) (uintptr, sonuc) {
	if modul == 0 || int(modul) > len(b.islevler) || b.islevler[modul-1].ad != ad {
		return 0, 500
	}
	return modul, basarili
}

func (b *benzetimSurucu) baslat(fn uintptr, izgara, blok [3]uint32, param [][]byte) sonuc {
	if fn == 0 || int(fn) > len(b.islevler) {
		return 400
	}
	is := b.islevler[fn-1]
	kopya := make([][]byte, len(param))
	for i, p := range param {
		kopya[i] = append([]byte(nil), p...)
	}
	if izgara[0] == 0 || izgara[1] == 0 || izgara[2] == 0 {
		return hataGecersizDeger
	}
	return b.ekle(func() error { return bzBaslat(is, &b.bel, izgara, blok, kopya) })
}

func (b *benzetimSurucu) esitle() sonuc { return b.bosalt() }

func (b *benzetimSurucu) hataAdi(s sonuc) string { return tablodanAd(s) }

func (b *benzetimSurucu) ayrinti() string { return b.detay }
