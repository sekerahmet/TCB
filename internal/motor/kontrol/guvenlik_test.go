package kontrol_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/kontrol"
)

// Kontrol noktası dosyaları güvenilmeyen girdidir (SECURITY.md): kurcalanmış, yarım
// kopyalanmış ya da daha yeni bir biçimle yazılmış nokta açık bir hatayla reddedilir; biçim
// yalnızca veri taşır, yol çözmez.

// noktaKur tek parametreli bir modelin 1. ve 2. adım noktalarını yazar; klasörü verir.
func noktaKur(t *testing.T) string {
	t.Helper()
	kosu := t.TempDir()
	cikti, err := calistir(t, fmt.Sprintf(`
değişken w = tensör([1.0, 2.0, 3.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w, op: AdamW([w]), d: {a: 1}})
kn.güncelle(1)
kn.kaydet()
kn.güncelle(2)
kn.kaydet()
`, trYol(kosu)))
	if err != nil {
		t.Fatalf("%v\n%s", err, cikti)
	}
	return kosu
}

func yukle(t *testing.T, kosu string) (string, error) {
	t.Helper()
	return calistir(t, fmt.Sprintf(`
değişken w = tensör([0.0, 0.0, 0.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w, op: AdamW([w]), d: {a: 1}})
yazdır("adım", kn.kaldığı_adım, w.değer)
`, trYol(kosu)))
}

func TestKurcalanmisNoktaReddedilir(t *testing.T) {
	durumlar := []struct {
		ad, dosya string
		boz       func([]byte) []byte
		beklenen  string
	}{
		{"tensör baytı", "tensorler.trt", func(b []byte) []byte { b[len(b)/2] ^= 1; return b }, "tensorler.trt: dosya bozuk: sağlama toplamı tutmuyor"},
		{"yarım tensör", "tensorler.trt", func(b []byte) []byte { return b[:len(b)-7] }, "tensorler.trt 6"},
		{"yeni sürüm", "ozet.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"sürüm": 1`, `"sürüm": 2`, 1)) }, "daha yeni bir TR sürümüyle yazılmış"},
		{"bilinmeyen alan", "ozet.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `{`, `{"yol": "../../x", `, 1)) }, "unknown field"},
		{"başka adım", "ozet.json", func(b []byte) []byte { return []byte(strings.Replace(string(b), `"adım": 2`, `"adım": 3`, 1)) }, "adım 3 diyor, klasör adı 2"},
		{"büyük özet", "ozet.json", func(b []byte) []byte { return append(b, make([]byte, 70<<10)...) }, "çok büyük"},
		{"eksik özet", "ozet.json", func([]byte) []byte { return nil }, "ozet.json çözülemedi"},
	}
	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			kosu := noktaKur(t)
			yol := filepath.Join(kosu, "kontrol_noktalari", "adim_0000000002", d.dosya)
			b, err := os.ReadFile(yol)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(yol, d.boz(b), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = yukle(t, kosu)
			if err == nil || !strings.Contains(err.Error(), d.beklenen) || !strings.Contains(err.Error(), "Bir önceki geçerli nokta: adim_0000000001 (adım 1)") {
				t.Fatalf("%q ve bir önceki nokta önerisi bekleniyordu: %v", d.beklenen, err)
			}
		})
	}
}

// son.json yalnızca araçlar içindir: bozuk ya da var olmayan bir noktayı gösterse de yükleme
// klasörlere bakar. Kenara alınmış (aynı adımlı yenisi yerine konamadan kesilmiş) bir nokta
// yerine geri konur.
func TestSonJSONVeKenardakiNokta(t *testing.T) {
	kosu := noktaKur(t)
	kn := filepath.Join(kosu, "kontrol_noktalari")
	if err := os.WriteFile(filepath.Join(kn, "son.json"), []byte(`{"klasör": "../../../etc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if cikti, err := yukle(t, kosu); err != nil || !strings.Contains(cikti, "adım 2 [1.") {
		t.Fatalf("2. adımdan sürmeliydi: %v\n%s", err, cikti)
	}
	if err := os.Rename(filepath.Join(kn, "adim_0000000002"), filepath.Join(kn, "adim_0000000002.eski-77")); err != nil {
		t.Fatal(err)
	}
	if cikti, err := yukle(t, kosu); err != nil || !strings.Contains(cikti, "Kaldığı yerden devam: adım 2") {
		t.Fatalf("kenardaki nokta geri konmalıydı: %v\n%s", err, cikti)
	}
}

// Kesme isteği kontrol noktası kurulduktan sonra gelirse ertelenir: sıradaki güncelle noktayı
// kaydeder, program nazikçe durur (ertelenen işler çalışır, Run ErrKesildi verir).
func TestErtelenenKesmeAdimSinirindaKaydeder(t *testing.T) {
	kosu := t.TempDir()
	m := yurut.New()
	w := &esliYazici{}
	m.Out, m.Err = w, w
	m.GlobalEnv.Define("kesme_iste", &yurut.BuiltinVal{Name: "kesme_iste", Fn: func(m *yurut.VM, _ []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
		return yurut.BoolToVal(m.KesmeIste()), nil
	}}, true)
	_, err := calistirVM(t, m, fmt.Sprintf(`
ertele { yazdır("ertelenen çalıştı") }
değişken w = tensör([1.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w})
döngü adım içinde 0..10 {
    eğer adım == 3 { yazdır("ertelendi:", kesme_iste()) }
    kn.güncelle(adım + 1)
    yazdır("adım bitti", adım + 1)
}
yazdır("döngü bitti")
`, trYol(kosu)))
	cikti := w.String()
	if !errors.Is(err, yurut.ErrKesildi) {
		t.Fatalf("program kesilmeliydi: %v\n%s", err, cikti)
	}
	for _, s := range []string{"ertelendi: doğru", "adım bitti 3", "Kontrol noktası kaydedildi: adım 4", "ertelenen çalıştı"} {
		if !strings.Contains(cikti, s) {
			t.Errorf("%q bekleniyordu:\n%s", s, cikti)
		}
	}
	if strings.Contains(cikti, "adım bitti 4") || strings.Contains(cikti, "döngü bitti") {
		t.Errorf("program adım sınırında durmalıydı:\n%s", cikti)
	}
	if _, err := os.Stat(filepath.Join(kosu, "kontrol_noktalari", "adim_0000000004")); err != nil {
		t.Errorf("4. adımın noktası yok: %v", err)
	}
}

// Kontrol noktası kurulduktan sonra tohumsuz rastgelelik kullanılırsa bir kez uyarılır.
func TestTohumsuzRastgelelikUyarisi(t *testing.T) {
	cikti, err := calistir(t, fmt.Sprintf(`
değişken w = tensör(rastgele_normal([2]), türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w})
rastgele_normal([2], tohum: 1)
rastgele_normal([2])
örnekle(tensör([[0.1, 0.2]]), sıcaklık: 1.0)
`, trYol(t.TempDir())))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(cikti, "tohumsuz rastgelelik kullanıldı") != 1 || !strings.Contains(cikti, "rastgele_normal ya da örnekle, tohum: verilmeden") {
		t.Fatalf("bir kez uyarı bekleniyordu:\n%s", cikti)
	}
}

// Biçim çözücüleri rastgele girdide çökmez; geçerli bir noktanın dosyaları tohumdur.
func FuzzNoktaDosyalari(f *testing.F) {
	f.Add([]byte(`{"sürüm":1,"adım":2,"dosyalar":{"durum.json":{"bayt":1,"sha256":"00"},"tensorler.trt":{"bayt":1,"sha256":"00"}}}`))
	f.Add([]byte(`{"sürüm":1,"adım":2,"zaman":"","tr":"","program":"","işçi":1,"nesneler":[{"ad":"d","tür":"değerler","değer":{"sözlük":[["a",1]]}}]}`))
	f.Add([]byte(`{"sözlük":[[1,{"ondalık":"NaN"}],["b",[null,true,"x"]]]}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = kontrol.OzetCoz(b, 2)
		_, _ = kontrol.DurumCoz(b, 2)
		if v, err := kontrol.DuzOku(b); err == nil {
			if _, err := kontrol.DuzYaz(v); err != nil {
				t.Fatalf("okunan değer yeniden yazılamadı: %v", err)
			}
		}
	})
}
