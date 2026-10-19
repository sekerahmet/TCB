package tokenize

import (
	"encoding/json"
	"errors"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// kucukYigin sınama süresince goroutine yığınını küçük bir sınıra indirir:
// girdi uzunluğuyla büyüyen bir özyineleme, süreci (kurtarılamaz) yığın
// taşmasıyla hemen çökertir.
func kucukYigin(t *testing.T) {
	eski := debug.SetMaxStack(16 << 20)
	t.Cleanup(func() { debug.SetMaxStack(eski) })
}

// Grup yinelemesi (tek karakterlik olmayan alt desen) girdi uzunluğundan
// bağımsız yığın kullanmalı; eskiden karakter başına ~1,3 KB yığın harcanıyor,
// birkaç MB'lık girdi süreci "stack overflow" ile çökertiyordu.
func TestGrupYinelemesiYiginTasirmaz(t *testing.T) {
	kucukYigin(t)
	s := strings.Repeat("ab", 1<<20) // 2 MB
	durumlar := []struct {
		desen    string
		bas, son int
	}{
		{`(?:ab)+`, 0, len(s)},
		{`(?:ab)+?$`, 0, len(s)},
		{`(?:a(?:b))*`, 0, len(s)},
		{`(?:ab|a)+b`, 0, len(s)},
		{`(?:ab){2,}`, 0, len(s)},
		{`(?:(?:ab)+)?x|(?:ab)+`, 0, len(s)},
	}
	for _, d := range durumlar {
		ifade, err := DuzIfadeDerle(d.desen)
		if err != nil {
			t.Fatalf("%q: %v", d.desen, err)
		}
		b, e, ok, err := ifade.Bul(s, 0)
		if err != nil || !ok || b != d.bas || e != d.son {
			t.Errorf("%q: (%d, %d, %v, %v), beklenen (%d, %d)", d.desen, b, e, ok, err, d.bas, d.son)
		}
	}
}

// Geri izleme noktalarının belleği sınırlıdır; aşılırsa yakalanabilir hata.
func TestGeriIzlemeBellekSiniri(t *testing.T) {
	ayarlaGeriAl(t, DuzIfadeAyarlari{GeriIzlemeSiniri: 1000})
	ifade, err := DuzIfadeDerle(`(?:ab)+`)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = ifade.Bul(strings.Repeat("ab", 10_000), 0)
	var h *AdimSiniriHatasi
	if !errors.As(err, &h) || !h.Bellek || !strings.Contains(err.Error(), "geri izleme belleği") {
		t.Fatalf("geri izleme belleği hatası bekleniyordu: %v", err)
	}
	// Sınırın altında kalan girdi sorunsuz eşleşir.
	if _, e, ok, err := ifade.Bul(strings.Repeat("ab", 100), 0); err != nil || !ok || e != 200 {
		t.Fatalf("kısa girdi: %d %v %v", e, ok, err)
	}
}

// Çok derin iç içe desenler derlemede açık hatayla reddedilir (eskiden
// ayrıştırıcının özyinelemesi yığını taşırıyordu).
func TestDerinDesenReddedilir(t *testing.T) {
	kucukYigin(t)
	n := 3_000_000
	kotu := map[string]string{
		"grup":       strings.Repeat("(?:", n) + "a" + strings.Repeat(")", n),
		"bakma":      strings.Repeat("(?=", n) + "a" + strings.Repeat(")", n),
		"sınıf":      strings.Repeat("[", n) + "a" + strings.Repeat("]", n),
		"niceleyici": "a" + strings.Repeat("{1}", n),
	}
	for ad, d := range kotu {
		_, err := DuzIfadeDerle(d)
		if err == nil || !strings.Contains(err.Error(), "sınırı") {
			t.Errorf("%s: derinlik hatası bekleniyordu: %v", ad, err)
		}
		if err != nil && len(err.Error()) > 1000 {
			t.Errorf("%s: hata iletisi deseni kısaltmalı (%d bayt)", ad, len(err.Error()))
		}
	}
	// Sınırın altındaki derinlik derlenir ve doğru eşleşir.
	derin := strings.Repeat("(?:", dfEnCokDerinlik-1) + "a" + strings.Repeat(")", dfEnCokDerinlik-1)
	ifade, err := DuzIfadeDerle(derin)
	if err != nil {
		t.Fatal(err)
	}
	if b, e, ok, err := ifade.Bul("xa", 0); err != nil || !ok || b != 1 || e != 2 {
		t.Fatalf("derin desen: %d %d %v %v", b, e, ok, err)
	}
}

// Parçalayıcı düzeyinde: Split deseninde grup yinelemesi olan bir dosya,
// uzun girdide süreci çökertmeden kodlar.
func TestParcalayiciGrupYinelemesi(t *testing.T) {
	kucukYigin(t)
	p := desenliParcalayici(t, `(?:ab)+|\s+|.`)
	metin := strings.Repeat("ab", 1<<20)
	ids, err := p.GuvenliKodla(metin, Secenekler{})
	if err != nil || len(ids) == 0 {
		t.Fatalf("GuvenliKodla: %d kimlik, %v", len(ids), err)
	}
}

// desenliParcalayici küçük sınama parçalayıcısının ön bölücüsünü verilen
// Split deseniyle (önce boşluklarla bölüp) değiştirir.
func desenliParcalayici(t *testing.T, desen string) *Parcalayici {
	t.Helper()
	veri, err := os.ReadFile(kucukYol)
	if err != nil {
		t.Fatal(err)
	}
	var ham map[string]any
	if err := json.Unmarshal(veri, &ham); err != nil {
		t.Fatal(err)
	}
	ham["pre_tokenizer"] = map[string]any{"type": "Sequence", "pretokenizers": []any{
		map[string]any{"type": "WhitespaceSplit"},
		map[string]any{"type": "Split", "pattern": map[string]any{"Regex": desen}, "behavior": "Isolated", "invert": false},
	}}
	yeni, _ := json.Marshal(ham)
	p, err := Ayristir(yeni)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Adım bütçesi aramaların toplamına uygulanır: her araması kendi sınırında
// kalan ama toplamda karesel büyüyen bir desen açık hatayla durur (eskiden
// 40 bin karakterde ~11 sn hatasız sürüyordu, 1 MB'ta saatler).
func TestKareselBuyumeDurdurulur(t *testing.T) {
	ifade, err := DuzIfadeDerle(`.(?=.*)`)
	if err != nil {
		t.Fatal(err)
	}
	// Kısa girdide sorun yok.
	say := 0
	if err := ifade.TumEslesmelerHata(strings.Repeat("x", 100), func(int, int) { say++ }); err != nil || say != 100 {
		t.Fatalf("kısa girdi: %d eşleşme, %v", say, err)
	}
	bas := time.Now()
	err = ifade.TumEslesmelerHata(strings.Repeat("x", 40_000), func(int, int) {})
	var h *AdimSiniriHatasi
	if !errors.As(err, &h) {
		t.Fatalf("adım sınırı hatası bekleniyordu: %v", err)
	}
	if sure := time.Since(bas); sure > 3*time.Second {
		t.Errorf("sınır geç devreye girdi: %v", sure)
	}
}

// Bir kodlama çağrısının bütün parçaları tek bütçeyi paylaşır: ön bölücünün
// ürettiği her kısa parçada sınırın altında kalan üstel arama, toplamda
// durdurulur.
func TestKodlamaButcesiPaylasilir(t *testing.T) {
	ayarlaGeriAl(t, DuzIfadeAyarlari{RiskliDesenIzni: true})
	p := desenliParcalayici(t, `(a|a)*b|\S`)
	parca := strings.Repeat("a", 14)
	// Tek parça kendi başına bütçeyi aşmaz.
	if _, err := p.GuvenliKodla(parca, Secenekler{}); err != nil {
		t.Fatalf("tek parça: %v", err)
	}
	metin := strings.Repeat(parca+" ", 300)
	bas := time.Now()
	_, err := p.GuvenliKodla(metin, Secenekler{})
	var h *AdimSiniriHatasi
	if !errors.As(err, &h) {
		t.Fatalf("paylaşılan bütçe aşılmalıydı: %v", err)
	}
	if sure := time.Since(bas); sure > 3*time.Second {
		t.Errorf("sınır geç devreye girdi: %v", sure)
	}
}

// Rastgele desen ve girdilerde motor asla çökmemeli: ya derleme hatası, ya
// sonuç, ya da *AdimSiniriHatasi.
func FuzzDuzIfadeCokmez(f *testing.F) {
	for _, d := range []string{`(?:ab)+`, `(a|a)*b`, `\s+(?!\S)`, `(?<=a{1,3})b`, `(?>a+)b`, `[[:alpha:]]+`, `(?i:ÇAY)`, `\p{Han}`} {
		f.Add(d, "ab abab aaab ÇaY 中文")
	}
	f.Fuzz(func(t *testing.T, desen, girdi string) {
		if len(desen) > 200 || len(girdi) > 2000 {
			return
		}
		ifade, err := DuzIfadeDerle(desen)
		if err != nil {
			return
		}
		onceki := -1
		err = ifade.TumEslesmelerHata(girdi, func(b, e int) {
			if b < onceki || e < b || e > len(girdi) {
				t.Fatalf("%q: geçersiz eşleşme (%d, %d)", desen, b, e)
			}
			onceki = e
		})
		var h *AdimSiniriHatasi
		if err != nil && !errors.As(err, &h) {
			t.Fatalf("%q: beklenmeyen hata %v", desen, err)
		}
	})
}

// Düzenli ifade motorunun ham hızı (bayt düzeyi ve Llama 3 tarzı desenler).
func BenchmarkDuzIfade(b *testing.B) {
	metin := olcumMetni(b)
	for _, d := range []struct{ ad, desen string }{
		{"bayt", BaytDuzeyiDeseni}, {"llama3", llama3Deseni}, {"grup", `(?:\p{L}\p{L})+|\s+|.`},
	} {
		ifade, err := DuzIfadeDerle(d.desen)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(d.ad, func(b *testing.B) {
			b.SetBytes(int64(len(metin)))
			for i := 0; i < b.N; i++ {
				ifade.TumEslesmeler(metin, func(int, int) {})
			}
		})
	}
}
