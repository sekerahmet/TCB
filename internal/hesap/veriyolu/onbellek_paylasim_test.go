package veriyolu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Aynı dizinle açılan iki Onbellek tek yöneticiyi paylaşır: kota ortaktır,
// ikinci açılış birincinin süren kopyasına (geçici dosya) dokunmaz.
func TestOnbellekAyniDizinPaylasilir(t *testing.T) {
	kaynak, yerel := t.TempDir(), t.TempDir()
	var yollar []string
	var boy int64
	for i := 0; i < 6; i++ {
		y, b := dosyaBoyutlu(t, kaynak, fmt.Sprintf("p%d.npy", i), 2000, uint64(i))
		yollar, boy = append(yollar, y), b
	}
	a, err := OnbellekAc(OnbellekSecenekleri{Dizin: yerel, Kota: 3 * boy})
	if err != nil {
		t.Fatal(err)
	}
	// Birincinin yarım kalmış gibi görünen (süren) kopyası.
	gecici := filepath.Join(yerel, yerelAd(yollar[5])+geciciSoneki)
	if err := os.WriteFile(gecici, []byte("süren kopya"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := OnbellekAc(OnbellekSecenekleri{Dizin: filepath.Join(yerel, "."), Kota: 3 * boy})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gecici); err != nil {
		t.Fatal("ikinci açılış birincinin geçici dosyasını sildi")
	}
	if a.c != b.c {
		t.Fatal("aynı dizin için iki ayrı yönetici kuruldu")
	}
	ctx := context.Background()
	a.Hazirla(yollar[:3])
	b.Hazirla(yollar[3:5])
	if err := a.Bekle(ctx); err != nil {
		t.Fatal(err)
	}
	for _, y := range yollar[:5] {
		yol, birak, err := a.Getir(ctx, y)
		if err != nil {
			t.Fatal(err)
		}
		if !yereldeMi(a, yol) {
			t.Fatal("kopya yerelde değil")
		}
		birak()
		if k := a.Istatistik().Kullanim; k > a.Kota() {
			t.Fatalf("paylaşılan kota aşıldı: %d > %d", k, a.Kota())
		}
	}
	// Ayırmalar toplanır.
	if err := a.Ayir(2 * boy); err != nil {
		t.Fatal(err)
	}
	if err := b.Ayir(2 * boy); err == nil || !errors.Is(err, ErrOnbellekYetersiz) {
		t.Fatalf("paylaşılan ayırma kotayı aşmalıydı: %v", err)
	}
	if err := b.Ayir(boy); err != nil {
		t.Fatal(err)
	}
	// Biri kapanınca öteki çalışmayı sürdürür; sonuncusu kapanınca dizin
	// yeniden (yeni kotayla) açılabilir.
	_ = a.Kapat()
	if _, birak, err := b.Getir(ctx, yollar[0]); err != nil {
		t.Fatalf("paylaşan kapandıktan sonra: %v", err)
	} else {
		birak()
	}
	if _, _, err := a.Getir(ctx, yollar[0]); !errors.Is(err, ErrKapali) {
		t.Fatalf("kapalı Onbellek: %v", err)
	}
	_ = b.Kapat()
	c, err := OnbellekAc(OnbellekSecenekleri{Dizin: yerel, Kota: 4 * boy})
	if err != nil {
		t.Fatal(err)
	}
	if c.c == a.c || c.Kota() != 4*boy {
		t.Fatal("kapanan yönetici yeniden kullanıldı")
	}
	_ = c.Kapat()
}

// Dizin başka bir tutamakta (başka bir süreçte) kilitliyse açılış açık
// hatayla reddedilir.
func TestOnbellekSureclerArasiKilit(t *testing.T) {
	yerel := t.TempDir()
	f, err := os.OpenFile(filepath.Join(yerel, kilitDosyasi), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := dosyaKilitle(f); err != nil {
		t.Fatal(err)
	}
	if _, err := OnbellekAc(OnbellekSecenekleri{Dizin: yerel, Kota: 1 << 20}); !errors.Is(err, ErrOnbellekKilitli) {
		_ = f.Close()
		t.Fatalf("kilitli dizin reddedilmeliydi: %v", err)
	}
	_ = f.Close() // kilit bırakılır
	o, err := OnbellekAc(OnbellekSecenekleri{Dizin: yerel, Kota: 1 << 20})
	if err != nil {
		t.Fatalf("kilit bırakılınca açılmalı: %v", err)
	}
	_ = o.Kapat()
}
