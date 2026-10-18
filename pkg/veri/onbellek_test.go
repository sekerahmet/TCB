package veri

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// dosyaBoyutlu, verilen boyutta (bayt) sahte .npy dosyası yazar.
func dosyaBoyutlu(t *testing.T, dizin, ad string, token int, tohum uint64) (string, int64) {
	t.Helper()
	yol := u16Yaz(t, dizin, ad, sozdeU16(token, tohum))
	st, err := os.Stat(yol)
	if err != nil {
		t.Fatal(err)
	}
	return yol, st.Size()
}

func yereldeMi(o *Onbellek, yol string) bool {
	return strings.HasPrefix(yol, o.Dizin())
}

func TestOnbellekKotaVeLRU(t *testing.T) {
	taban := runtime.NumGoroutine()
	kaynak, yerel := t.TempDir(), t.TempDir()
	a, boy := dosyaBoyutlu(t, kaynak, "a.npy", 1000, 1)
	b, _ := dosyaBoyutlu(t, kaynak, "b.npy", 1000, 2)
	c, _ := dosyaBoyutlu(t, kaynak, "c.npy", 1000, 3)
	o, err := OnbellekAc(OnbellekSecenekleri{Dizin: yerel, Kota: boy*2 + boy/2})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	al := func(k string) (string, func()) {
		t.Helper()
		y, birak, err := o.Getir(ctx, k)
		if err != nil {
			t.Fatal(err)
		}
		return y, birak
	}
	ya, ba := al(a)
	if !yereldeMi(o, ya) {
		t.Fatalf("a yerelde değil: %s", ya)
	}
	ba()
	_, bb := al(b)
	bb()
	// a'yı yeniden kullan: artık b daha eski.
	_, ba = al(a)
	ba()
	_, bc := al(c) // kota 2.5 dosya: b silinmeli
	bc()
	ist := o.Istatistik()
	if ist.Silinen != 1 || ist.DosyaSayisi != 2 || ist.Kullanim > boy*2+boy/2 {
		t.Fatalf("kota uygulanmadı: %+v", ist)
	}
	if _, err := os.Stat(filepath.Join(yerel, yerelAd(b))); !os.IsNotExist(err) {
		t.Fatal("en eski kullanılan (b) silinmedi")
	}
	if _, err := os.Stat(filepath.Join(yerel, yerelAd(a))); err != nil {
		t.Fatal("yakın zamanda kullanılan (a) silindi")
	}
	if ist.Isabet != 1 || ist.Iska != 3 {
		t.Fatalf("isabet/ıska yanlış: %+v", ist)
	}

	// Açık (iğneli) dosyalar silinmez: a ve c açıkken b için yer yok →
	// açık hata (kaynağa sessizce geri dönülmez).
	_, ba = al(a)
	_, bc = al(c)
	if y, _, err := o.Getir(ctx, b); !errors.Is(err, ErrOnbellekYetersiz) || y != "" ||
		!strings.Contains(err.Error(), "kota") {
		t.Fatalf("yer yokken ErrOnbellekYetersiz bekleniyordu: %q %v", y, err)
	}
	ba()
	bc()
	if o.Istatistik().Atlanan != 1 {
		t.Fatal("atlanan sayılmadı")
	}
	// Artık yer var: b alınabilmeli.
	_, bb = al(b)
	bb()
	// Kotadan büyük dosya hiç kopyalanmaz ve hata verir.
	buyuk, _ := dosyaBoyutlu(t, kaynak, "buyuk.npy", 4000, 4)
	if _, _, err := o.Getir(ctx, buyuk); !errors.Is(err, ErrOnbellekYetersiz) {
		t.Fatalf("kotadan büyük dosyada hata bekleniyordu: %v", err)
	}
	_ = o.Kapat()
	if _, _, err := o.Getir(ctx, a); err != ErrKapali {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
	gorutinBekle(t, taban)
}

func TestOnbellekOnYuklemeBayatlikBenimseme(t *testing.T) {
	taban := runtime.NumGoroutine()
	kaynak, yerel := t.TempDir(), t.TempDir()
	var yollar []string
	var boy int64
	for i := 0; i < 4; i++ {
		y, b := dosyaBoyutlu(t, kaynak, parcaAdi(i), 2000, uint64(i))
		yollar, boy = append(yollar, y), b
	}
	o, err := OnbellekAc(OnbellekSecenekleri{Dizin: yerel, Kota: 3 * boy})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	o.Hazirla(yollar) // 4 istendi, kotaya 3 sığar
	if err := o.Bekle(ctx); err != nil {
		t.Fatal(err)
	}
	ist := o.Istatistik()
	if ist.DosyaSayisi != 3 || ist.KopyalananBayt != 3*boy || ist.Silinen != 0 {
		t.Fatalf("önden yükleme kotayı aştı ya da istenenleri sildi: %+v", ist)
	}
	for i := 0; i < 3; i++ {
		y, birak, err := o.Getir(ctx, yollar[i])
		if err != nil || !yereldeMi(o, y) {
			t.Fatalf("önden yüklenen dosya yerelde değil: %v", err)
		}
		birak()
	}
	if o.Istatistik().Isabet != 3 || o.Istatistik().Iska != 0 {
		t.Fatalf("önden yükleme isabet vermedi: %+v", o.Istatistik())
	}
	// İçerik aynı olmalı.
	y0, birak, _ := o.Getir(ctx, yollar[0])
	k0, _ := os.ReadFile(yollar[0])
	l0, _ := os.ReadFile(y0)
	birak()
	if string(k0) != string(l0) {
		t.Fatal("yerel kopya kaynakla aynı değil")
	}

	// Kaynak değişince kopya bayat sayılır ve yeniden kopyalanır.
	u16Yaz(t, kaynak, parcaAdi(0), sozdeU16(2500, 99))
	gelecek := time.Now().Add(time.Hour)
	_ = os.Chtimes(yollar[0], gelecek, gelecek)
	y0, birak, err = o.Getir(ctx, yollar[0])
	if err != nil {
		t.Fatal(err)
	}
	l0, _ = os.ReadFile(y0)
	k0, _ = os.ReadFile(yollar[0])
	birak()
	if string(k0) != string(l0) {
		t.Fatal("bayat kopya yenilenmedi")
	}
	_ = o.Kapat()
	gorutinBekle(t, taban)

	// Yeniden açılışta mevcut kopyalar benimsenir; yarım kalmış geçici
	// dosyalar silinir; yabancı dosyalara dokunulmaz.
	gecici := filepath.Join(yerel, yerelAd(yollar[3])+geciciSoneki)
	_ = os.WriteFile(gecici, []byte("yarım"), 0o644)
	yabanci := filepath.Join(yerel, "benim_notum.txt")
	_ = os.WriteFile(yabanci, []byte("dokunma"), 0o644)
	o2, err := OnbellekAc(OnbellekSecenekleri{Dizin: yerel, Kota: 4 * boy})
	if err != nil {
		t.Fatal(err)
	}
	defer o2.Kapat()
	if _, err := os.Stat(gecici); !os.IsNotExist(err) {
		t.Fatal("geçici dosya silinmedi")
	}
	if _, err := os.Stat(yabanci); err != nil {
		t.Fatal("yabancı dosya silindi")
	}
	// yollar[1] bayat kopya yenilenirken kota için silindi (en eski); [2] durmalı.
	y1, birak, err := o2.Getir(ctx, yollar[2])
	if err != nil || !yereldeMi(o2, y1) {
		t.Fatal(err)
	}
	birak()
	if ist := o2.Istatistik(); ist.Isabet != 1 || ist.Iska != 0 {
		t.Fatalf("mevcut kopya benimsenmedi: %+v", ist)
	}
}

func TestOnbellekliOrnekleyiciAyniVeri(t *testing.T) {
	parcaSizintiDenetle(t)
	taban := runtime.NumGoroutine()
	d := kodluKume(t, 3000, 2000, 2500, 1800, 2200)
	k, _ := KumeAc(d, KumeSecenekleri{})
	sec := OrnekleyiciSecenekleri{B: 8, T: 64, Tohum: 5, GrupBoyu: 2, IleriBak: 1}
	o, _ := k.Ornekleyici(sec)
	beklenen := toplulariAl(t, o, 60)
	_ = o.Kapat()

	st, _ := os.Stat(k.Parcalar()[0].Yol)
	// (GrupBoyu + IleriBak) × en büyük parça gerekli; 5 parça sığmaz,
	// eski gruplar silinmeli.
	ob, err := OnbellekAc(OnbellekSecenekleri{Dizin: t.TempDir(), Kota: 3 * st.Size()})
	if err != nil {
		t.Fatal(err)
	}
	kc, _ := KumeAc(d, KumeSecenekleri{Onbellek: ob})
	oc, _ := kc.Ornekleyici(sec)
	yk := OnYukle(context.Background(), oc, 3)
	var ti TopluIs
	var alinan [][]int32
	for i := 0; i < 60; i++ {
		if err := yk.Al(&ti); err != nil {
			t.Fatal(err)
		}
		alinan = append(alinan, append([]int32(nil), ti.Veri...))
	}
	_ = yk.Kapat()
	_ = oc.Kapat()
	_ = ob.Kapat()
	if !reflect.DeepEqual(alinan, beklenen) {
		t.Fatal("önbellekli örnekleyici farklı veri verdi")
	}
	ist := ob.Istatistik()
	if ist.Kullanim > 3*st.Size() || ist.KopyalananBayt == 0 || ist.Atlanan != 0 || ist.Silinen == 0 {
		t.Fatalf("önbellek kullanılmadı ya da kota aşıldı: %+v", ist)
	}
	gorutinBekle(t, taban)
}

func TestOnbellekKotaDenetimiVeSessizDonusYok(t *testing.T) {
	parcaSizintiDenetle(t)
	d := kodluKume(t, 3000, 2000, 2500)
	st, _ := os.Stat(filepath.Join(d, parcaAdi(0))) // en büyük parça
	ob, err := OnbellekAc(OnbellekSecenekleri{Dizin: t.TempDir(), Kota: 2 * st.Size()})
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Kapat()
	k, _ := KumeAc(d, KumeSecenekleri{Onbellek: ob})
	// Varsayılanlarla (grup 4 → 3 parça, ileri bakış 3) tüm parçalar
	// gerekir → kurulmaz; grup 2 + ileri bakış 0 da 2'yi aşmaz ama grup 2 +
	// ileri bakış 1 aşar.
	for _, s := range []OrnekleyiciSecenekleri{{B: 2, T: 8}, {B: 2, T: 8, GrupBoyu: 2, IleriBak: 1}} {
		if _, err := k.Ornekleyici(s); !errors.Is(err, ErrOnbellekYetersiz) {
			t.Fatalf("yetersiz kota için hata bekleniyordu: %+v %v", s, err)
		}
	}
	if o, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: 2, T: 8, GrupBoyu: 2, IleriBak: -1}); err != nil {
		t.Fatal(err)
	} else {
		_ = o.Kapat()
	}
	// Grup 1 + ileri bakış 1 ile 2 parça yeter.
	o, err := k.Ornekleyici(OrnekleyiciSecenekleri{B: 2, T: 8, GrupBoyu: 1, IleriBak: 1})
	if err != nil {
		t.Fatal(err)
	}
	var ti TopluIs
	if err := o.Sonraki(&ti); err != nil {
		t.Fatal(err)
	}
	_ = o.Kapat()
	// Kotayı iğneli dosyalarla doldurunca parça açma kaynağa dönmez, hata verir.
	p0, err := k.ParcaAc(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	p1, err := k.ParcaAc(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !yereldeMi(ob, p0.Yol()) || !yereldeMi(ob, p1.Yol()) {
		t.Fatal("parçalar yerel kopyadan açılmadı")
	}
	if p, err := k.ParcaAc(context.Background(), 2); !errors.Is(err, ErrOnbellekYetersiz) {
		if p != nil {
			_ = p.Kapat()
		}
		t.Fatalf("yer yokken parça açma hata vermeli: %v", err)
	}
	_ = p0.Kapat()
	_ = p1.Kapat()
	// Sıralı okuyucu için tek parça yeter; daha küçük kota reddedilir.
	ob2, _ := OnbellekAc(OnbellekSecenekleri{Dizin: t.TempDir(), Kota: st.Size() - 1})
	defer ob2.Kapat()
	k2, _ := KumeAc(d, KumeSecenekleri{Onbellek: ob2})
	if _, err := k2.Sirali(SiraliSecenekleri{B: 1, T: 8}); !errors.Is(err, ErrOnbellekYetersiz) {
		t.Fatalf("sıralı okuyucu yetersiz kotayı kabul etti: %v", err)
	}
}

func TestOnbellekKopyaDogrulama(t *testing.T) {
	kaynak, yerel := t.TempDir(), t.TempDir()
	a, boy := dosyaBoyutlu(t, kaynak, "a.npy", 1000, 1)
	st, _ := os.Stat(a)
	hedef := filepath.Join(yerel, yerelAd(a))
	// Beklenenden kısa kopya (kaynak kopya sırasında kısalmış gibi) reddedilir
	// ve geride ne hedef ne geçici dosya kalır.
	err := kopyala(context.Background(), a, hedef, boy+10, st.ModTime())
	if err == nil || !strings.Contains(err.Error(), "eksik") {
		t.Fatalf("kısa kopya hatası bekleniyordu: %v", err)
	}
	if _, err := os.Stat(hedef); !os.IsNotExist(err) {
		t.Fatal("kısa kopya hedef adına taşındı")
	}
	if _, err := os.Stat(hedef + geciciSoneki); !os.IsNotExist(err) {
		t.Fatal("geçici dosya kaldı")
	}
	if err := kopyala(context.Background(), a, hedef, boy, st.ModTime()); err != nil {
		t.Fatal(err)
	}

	// Açılışta bozuk başlıklı ya da kesik kopyalar benimsenmez ve silinir.
	b, _ := dosyaBoyutlu(t, kaynak, "b.npy", 1000, 2)
	c, _ := dosyaBoyutlu(t, kaynak, "c.npy", 1000, 3)
	bozuk := filepath.Join(yerel, yerelAd(b))
	_ = os.WriteFile(bozuk, []byte("bu bir npy değil, bozuk kopya"), 0o644)
	kesik := filepath.Join(yerel, yerelAd(c))
	ham, _ := os.ReadFile(c)
	_ = os.WriteFile(kesik, ham[:len(ham)/2], 0o644)
	o, err := OnbellekAc(OnbellekSecenekleri{Dizin: yerel, Kota: 10 * boy})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Kapat()
	if ist := o.Istatistik(); ist.DosyaSayisi != 1 {
		t.Fatalf("yalnızca geçerli kopya benimsenmeli: %+v", ist)
	}
	for _, y := range []string{bozuk, kesik} {
		if _, err := os.Stat(y); !os.IsNotExist(err) {
			t.Fatalf("bozuk kopya silinmedi: %s", y)
		}
	}
	// Geçerli kopya isabet verir; bozuklar yeniden kopyalanır.
	for _, k := range []string{a, b, c} {
		y, birak, err := o.Getir(context.Background(), k)
		if err != nil || !yereldeMi(o, y) {
			t.Fatal(err)
		}
		birak()
	}
	if ist := o.Istatistik(); ist.Isabet != 1 || ist.Iska != 2 {
		t.Fatalf("isabet/ıska yanlış: %+v", ist)
	}
}
