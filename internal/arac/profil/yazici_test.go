package profil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Yerine koymanın geçici hatası (Windows'ta dosyayı o an okuyan varken) kısa yeniden
// denemeyle aşılır: uyarı yok, doğrudan yazmaya geçilmez.
func TestYerineKoymaGeciciHatasiYenidenDenenir(t *testing.T) {
	eski := yerineKoy
	kalan := len(yerineKoymaBeklemeleri) // ilk deneme ve son bekleme dışındakiler başarısız
	yerineKoy = func(a, b string) error {
		if kalan > 0 {
			kalan--
			return os.ErrPermission
		}
		return eski(a, b)
	}
	defer func() { yerineKoy = eski }()
	klasor := t.TempDir()
	y := &durumYazici{yol: filepath.Join(klasor, DurumDosyasiAdi), uyari: func(m string) { t.Errorf("beklenmeyen uyarı: %s", m) }}
	if err := y.yaz(&Goruntu{Bicim: BicimAdi, SureSn: 7}); err != nil {
		t.Fatal(err)
	}
	if y.hatali != 0 || y.dogrudan {
		t.Errorf("geçici hata kalıcı sayıldı: %d hata, doğrudan %v", y.hatali, y.dogrudan)
	}
	if girdiler, _ := os.ReadDir(klasor); len(girdiler) != 1 {
		t.Errorf("geçici dosya kaldı: %v", girdiler)
	}
}

// Atomik yazma: yazıcı dosyayı art arda yenilerken okuyan hiçbir zaman yarım ya da bozuk
// bir dosya görmez (her okuma geçerli JSON'dur ve bir görüntünün tamamıdır).
func TestDurumDosyasiOkuyanYarimDosyaGormez(t *testing.T) {
	yol := filepath.Join(t.TempDir(), DurumDosyasiAdi)
	y := &durumYazici{yol: yol, uyari: func(m string) { t.Errorf("beklenmeyen uyarı: %s", m) }}
	// Büyük bir görüntü: tek yazmada bitmesin.
	g := &Goruntu{Bicim: BicimAdi, Surum: BicimSurumu}
	for i := 0; i < 400; i++ {
		g.Tensor.Islemler = append(g.Tensor.Islemler, IslemBilgisi{Kod: "matris_carpimi", Ad: strings.Repeat("ç", 40), Cagri: int64(i)})
	}
	if err := y.yaz(g); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	dur := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-dur:
				return
			default:
			}
			time.Sleep(5 * time.Millisecond) // gerçek yazıcı saniyede bir yazar
			g.SureSn = float64(i)
			if err := y.yaz(g); err != nil {
				t.Errorf("yazma: %v", err)
				return
			}
		}
	}()
	okunan := 0
	for okunan < 100 {
		veri, err := os.ReadFile(yol)
		if err != nil {
			time.Sleep(time.Millisecond)
			continue // Windows'ta yerine koyma anında dosya kısa süre açılamayabilir
		}
		var d Goruntu
		if err := json.Unmarshal(veri, &d); err != nil {
			t.Fatalf("okuyan yarım ya da bozuk dosya gördü (%d bayt): %v", len(veri), err)
		}
		if len(d.Tensor.Islemler) != 400 {
			t.Fatalf("görüntü eksik: %d işlem", len(d.Tensor.Islemler))
		}
		okunan++
		time.Sleep(2 * time.Millisecond) // okuyan araçlar saniyede bir okur; sıkı döngü Windows'ta yerine koymayı sürekli engellerdi
	}
	close(dur)
	wg.Wait()
	if y.dogrudan {
		t.Error("yerine koyma bu sürücüde çalışmalıydı")
	}
}
