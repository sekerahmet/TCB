package veriyolu

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestParcaEslemeVeReadAtAyniVeri(t *testing.T) {
	parcaSizintiDenetle(t)
	d := t.TempDir()
	u := sozdeU16(100_003, 7)
	yolU := u16Yaz(t, d, "u.npy", u)
	iv := make([]int32, 50_001)
	for i := range iv {
		iv[i] = int32(i*31 - 20_000)
	}
	yolI := i32Yaz(t, d, "i.npy", iv)

	for _, kapali := range []bool{false, true} {
		p, err := ParcaAc(yolU, ParcaSecenekleri{EslemeKapali: kapali})
		if err != nil {
			t.Fatal(err)
		}
		if kapali && p.Eslemeli() {
			t.Fatal("eşleme kapalıyken eşlemeli açıldı")
		}
		if !kapali && eslemeDestekli && !p.Eslemeli() {
			t.Fatalf("eşleme bekleniyordu: %v", p.EslemeHatasi())
		}
		if p.Uzunluk() != int64(len(u)) || p.Tur() != TurU16 {
			t.Fatalf("yanlış başlık: %+v", p.Baslik())
		}
		// Tüm dosya, ReadAt tamponundan büyük tek okuma ve küçük okumalar.
		hepsi := make([]int32, len(u))
		if err := p.Oku(hepsi, 0); err != nil {
			t.Fatal(err)
		}
		for i := range u {
			if hepsi[i] != int32(u[i]) {
				t.Fatalf("eşleme=%v: %d konumunda %d != %d", !kapali, i, hepsi[i], u[i])
			}
		}
		k := make([]uint16, 17)
		if err := p.OkuU16(k, 99_986); err != nil {
			t.Fatal(err)
		}
		for i := range k {
			if k[i] != u[99_986+i] {
				t.Fatal("OkuU16 yanlış")
			}
		}
		if err := p.Oku(make([]int32, 4), int64(len(u))-3); err == nil || !strings.Contains(err.Error(), "sınır") {
			t.Fatalf("sınır hatası bekleniyordu: %v", err)
		}
		if err := p.Oku(make([]int32, 1), -1); err == nil {
			t.Fatal("negatif konum kabul edildi")
		}
		if err := p.Kapat(); err != nil {
			t.Fatal(err)
		}

		pi, err := ParcaAc(yolI, ParcaSecenekleri{EslemeKapali: kapali})
		if err != nil {
			t.Fatal(err)
		}
		g := make([]int32, 1000)
		if err := pi.Oku(g, 49_001); err != nil {
			t.Fatal(err)
		}
		for i := range g {
			if g[i] != iv[49_001+i] {
				t.Fatalf("i4 %d: %d != %d", i, g[i], iv[49_001+i])
			}
		}
		if err := pi.OkuU16(make([]uint16, 1), 0); err == nil {
			t.Fatal("i4 parçada OkuU16 kabul edildi")
		}
		_ = pi.Kapat()
	}
}

func TestParcaKapatmaVeSizinti(t *testing.T) {
	parcaSizintiDenetle(t)
	d := t.TempDir()
	yol := u16Yaz(t, d, "k.npy", sozdeU16(5000, 1))
	for _, kapali := range []bool{false, true} {
		taban := acikParcaSayisi.Load()
		p, err := ParcaAc(yol, ParcaSecenekleri{EslemeKapali: kapali})
		if err != nil {
			t.Fatal(err)
		}
		if acikParcaSayisi.Load() != taban+1 {
			t.Fatal("açık parça sayacı artmadı")
		}
		// Eşzamanlı okumalar sırasında kapatma güvenli olmalı.
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				b := make([]int32, 64)
				for i := 0; i < 200; i++ {
					err := p.Oku(b, int64((g*97+i*13)%4900))
					if err != nil && !errors.Is(err, ErrKapali) {
						t.Error(err)
						return
					}
				}
			}(g)
		}
		if err := p.Kapat(); err != nil {
			t.Fatal(err)
		}
		wg.Wait()
		if err := p.Kapat(); err != nil {
			t.Fatal("ikinci Kapat hata verdi:", err)
		}
		if acikParcaSayisi.Load() != taban {
			t.Fatal("açık parça sayacı geri dönmedi")
		}
		if err := p.Oku(make([]int32, 1), 0); !errors.Is(err, ErrKapali) {
			t.Fatalf("kapalı parçada ErrKapali bekleniyordu: %v", err)
		}
	}
	// Tüm tutamaklar ve eşlemeler bırakıldıysa dosya silinebilmeli
	// (bazı platformlarda açık ya da eşlenmiş dosya silinemez).
	if err := os.Remove(yol); err != nil {
		t.Fatalf("kapatılan parça silinemedi (kaynak sızıntısı?): %v", err)
	}
}

func TestParcaBosDosya(t *testing.T) {
	parcaSizintiDenetle(t)
	d := t.TempDir()
	yol := u16Yaz(t, d, "bos.npy", nil)
	p, err := ParcaAc(yol, ParcaSecenekleri{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Kapat()
	if p.Uzunluk() != 0 || p.Eslemeli() {
		t.Fatal("boş parça yanlış")
	}
	if err := p.Oku(nil, 0); err != nil {
		t.Fatal(err)
	}
}
