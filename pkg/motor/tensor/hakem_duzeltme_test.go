package tensor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sert hakem denetiminin motor tarafı bulgularının regresyon testleri.

// Bulgu 7: indeksle atama tek kopya ve tek sürüm artışıdır (eleman eleman yazılmaz);
// bitişik olmayan hedefte de.
func TestIndekseAtamaTekKopya(t *testing.T) {
	v := deger(t, `değişken cikti = sıfırlar([300, 200])
cikti[0..300] = birler([300, 200])
cikti.devrik()[5] = 2
cikti[1][2] = 3`)
	d := v.(*Deger)
	if s := d.t.Version(); s != 3 {
		t.Fatalf("3 atama için sürüm %d; her atama bir kez artırmalı", s)
	}
	veri := d.t.Data()
	if veri[0] != 1 || veri[5] != 2 || veri[200+5] != 2 || veri[299*200+5] != 2 || veri[200+2] != 3 || veri[6] != 1 {
		t.Fatalf("atama sonucu yanlış: %v", veri[:8])
	}
}

// Bulgu 9: kayıt atomiktir: içerik önce aynı klasörde geçici dosyaya yazılır, sonra yerine
// konur. Yazma yarıda hata verirse eski dosya bozulmaz ve geçici dosya kalmaz.
func TestAtomikYaz(t *testing.T) {
	klasor := t.TempDir()
	yol := filepath.Join(klasor, "model.trt")
	if err := os.WriteFile(yol, []byte("eski"), 0o644); err != nil {
		t.Fatal(err)
	}
	hata := errors.New("disk doldu")
	err := atomikYaz(yol, func(w io.Writer) error {
		w.Write([]byte("yarım"))
		return hata
	})
	if !errors.Is(err, hata) {
		t.Fatalf("hata: %v", err)
	}
	if b, _ := os.ReadFile(yol); string(b) != "eski" {
		t.Fatalf("yarım yazma eski dosyayı bozdu: %q", b)
	}
	if g, _ := os.ReadDir(klasor); len(g) != 1 {
		t.Fatalf("geçici dosya kaldı: %v", g)
	}
	if err := atomikYaz(yol, func(w io.Writer) error { _, e := w.Write([]byte("yeni")); return e }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(yol); string(b) != "yeni" {
		t.Fatalf("içerik: %q", b)
	}
	if g, _ := os.ReadDir(klasor); len(g) != 1 {
		t.Fatalf("geçici dosya kaldı: %v", g)
	}
}

// Bulgu 20: yayın uyarısı haritası sınırlıdır: değişken uzunluklu şekillerle sınırsız
// büyümez; sınıra varılınca bir kez bildirilir ve sonraki yeni uyarılar susturulur.
func TestYayinUyarisiSinirli(t *testing.T) {
	var buf bytes.Buffer
	uyariMu.Lock()
	eski := uyariCikisi
	uyariCikisi = &buf
	verilenUyari = map[string]bool{}
	uyariMu.Unlock()
	defer func() {
		uyariMu.Lock()
		uyariCikisi = eski
		uyariMu.Unlock()
	}()
	mustCalistir(t, fmt.Sprintf(`döngü n içinde 1..%d {
    değişken c = sıfırlar([n, 1]) + sıfırlar([n + 1])
}`, enFazlaYayinUyarisi+50))
	uyariMu.Lock()
	boy := len(verilenUyari)
	uyariMu.Unlock()
	if boy > enFazlaYayinUyarisi {
		t.Fatalf("uyarı haritası %d anahtar (sınır %d)", boy, enFazlaYayinUyarisi)
	}
	if n := strings.Count(buf.String(), "\n"); n != enFazlaYayinUyarisi+1 {
		t.Fatalf("%d satır uyarı; %d uyarı ve bir susturma bildirimi bekleniyordu", n, enFazlaYayinUyarisi)
	}
	if !strings.Contains(buf.String(), "sonraki yayın uyarıları gösterilmeyecek") {
		t.Fatalf("susturma bildirimi yok:\n%s", buf.String()[len(buf.String())-300:])
	}
}
