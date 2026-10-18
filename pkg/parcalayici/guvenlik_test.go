package parcalayici

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Gerçek parçalayıcılarda görülen desenler.
const (
	llama3Deseni = `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`
	qwenDeseni   = `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`
)

// ayarlaGeriAl sınama süresince motor ayarlarını değiştirir.
func ayarlaGeriAl(t *testing.T, a DuzIfadeAyarlari) {
	eski := DuzIfadeAyarlariniAl()
	DuzIfadeAyarla(a)
	t.Cleanup(func() { DuzIfadeAyarla(eski) })
}

func TestRiskliDesenlerReddedilir(t *testing.T) {
	riskli := []string{`(a+)+$`, `(a*)*b`, `(\w+\s*)+$`, `(a+|b)+c`, `((ab)*)*x`, `(?:x?y*)+z`, `(a{1,})+`, `(x+x+)+y`, `(\d+\.?\d*)+$`}
	for _, d := range riskli {
		_, err := DuzIfadeDerle(d)
		if err == nil || !strings.Contains(err.Error(), "iç içe sınırsız niceleyici") || !strings.Contains(err.Error(), fmt.Sprintf("%q", d)) {
			t.Errorf("%q reddedilmeliydi, hata: %v", d, err)
		}
	}
	guvenli := []string{BolmeDeseni, BaytDuzeyiDeseni, llama3Deseni, qwenDeseni, `(ab+)+`, `(?>a+)+`, `(a++)+`,
		`\d+(?=\D)`, `(?:\s*\S+)`, `[a-z]+(?:-[a-z]+)*`}
	for _, d := range guvenli {
		ifade, err := DuzIfadeDerle(d)
		if err != nil {
			t.Errorf("%q kabul edilmeliydi: %v", d, err)
			continue
		}
		if u := ifade.Uyarilar(); len(u) > 0 {
			t.Errorf("%q için beklenmeyen uyarı: %v", d, u)
		}
	}
	// İzin verilince derlenir ama risk uyarı olarak kalır.
	ayarlaGeriAl(t, DuzIfadeAyarlari{RiskliDesenIzni: true})
	ifade, err := DuzIfadeDerle(`(a+)+$`)
	if err != nil || len(ifade.Uyarilar()) == 0 {
		t.Fatalf("izinli derleme: %v, uyarılar %v", err, ifade.Uyarilar())
	}
}

func TestAdimSiniriDevreyeGirer(t *testing.T) {
	ayarlaGeriAl(t, DuzIfadeAyarlari{RiskliDesenIzni: true})
	durumlar := []struct{ desen, girdi string }{
		{`(a+)+$`, strings.Repeat("a", 40) + "!"},
		{`(a|a)*b`, strings.Repeat("a", 40)},
		{`(\w+\s?)+$`, strings.Repeat("kelime ", 15) + "!"},
		{`(x+x+)+y`, strings.Repeat("x", 50)},
	}
	for _, d := range durumlar {
		ifade, err := DuzIfadeDerle(d.desen)
		if err != nil {
			t.Fatal(err)
		}
		bas := time.Now()
		err = ifade.TumEslesmelerHata(d.girdi, func(int, int) {})
		sure := time.Since(bas)
		var h *AdimSiniriHatasi
		if !errors.As(err, &h) {
			t.Errorf("%q: adım sınırı hatası bekleniyordu, alınan %v", d.desen, err)
			continue
		}
		if h.Desen != d.desen || !strings.Contains(err.Error(), "bayt konumundan") {
			t.Errorf("%q: hata iletisi deseni/konumu içermiyor: %v", d.desen, err)
		}
		if sure > 5*time.Second {
			t.Errorf("%q: sınır çok geç devreye girdi (%v)", d.desen, sure)
		}
		t.Logf("%q: %v sonra durdu: %v", d.desen, sure, err)
	}
	// Bul da hata döndürür.
	ifade, _ := DuzIfadeDerle(`(a|a)*b`)
	if _, _, _, err := ifade.Bul(strings.Repeat("a", 40), 0); err == nil {
		t.Error("Bul adım sınırı hatası döndürmeliydi")
	}
}

// Parçalayıcı düzeyinde: riskli desenli bir dosya uyarı verir, Guvenli...
// işlevleri hata döndürür, hata döndürmeyen işlevler panikler (paralel
// işçilerdeki panik de çağırana taşınır, süreç çökmez).
func TestParcalayiciAdimSiniri(t *testing.T) {
	veri, err := os.ReadFile(kucukYol)
	if err != nil {
		t.Fatal(err)
	}
	var ham map[string]any
	if err := json.Unmarshal(veri, &ham); err != nil {
		t.Fatal(err)
	}
	pt := ham["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
	pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `(a|a)*b|\s+|\S`}
	yeni, _ := json.Marshal(ham)
	p, err := Ayristir(yeni)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Uyarilar()) == 0 {
		t.Error("çakışan seçenek uyarısı bekleniyordu")
	}
	kotu := strings.Repeat("a", 40)
	if _, err := p.GuvenliKodla("merhaba dünya", Secenekler{}); err != nil {
		t.Fatalf("zararsız girdi hata verdi: %v", err)
	}
	if ids, err := p.GuvenliKodla(kotu, Secenekler{}); err == nil || ids != nil {
		t.Fatalf("GuvenliKodla hata döndürmeliydi: %v %v", ids, err)
	}
	_, hatalar := p.GuvenliTopluKodla([]string{"iyi", kotu, "yine iyi"}, Secenekler{}, 2)
	if hatalar == nil || hatalar[1] == nil || hatalar[0] != nil || hatalar[2] != nil {
		t.Fatalf("GuvenliTopluKodla hataları: %v", hatalar)
	}
	panikBekle := func(ad string, f func()) {
		defer func() {
			if _, ok := recover().(*AdimSiniriHatasi); !ok {
				t.Errorf("%s: *AdimSiniriHatasi paniği bekleniyordu", ad)
			}
		}()
		f()
	}
	panikBekle("Kodla", func() { p.Kodla(kotu) })
	panikBekle("TopluKodla", func() { p.TopluKodla([]string{"x", kotu, "y", "z"}, Secenekler{}, 3) })
	// Riskli (iç içe) desenli dosya varsayılan ayarla yüklenmez.
	pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `(\w+\s*)+$`}
	yeni, _ = json.Marshal(ham)
	if _, err := Ayristir(yeni); err == nil || !strings.Contains(err.Error(), "reddedildi") {
		t.Fatalf("riskli desenli dosya reddedilmeliydi: %v", err)
	}
}

// Gerçek parçalayıcı desenleri bütçeye takılmamalı ve adım sayısı girdi
// uzunluğuyla doğrusal kalmalı (zararlı biçimde seçilmiş girdiler dahil).
func TestBilinenDesenlerDogrusal(t *testing.T) {
	ornek, err := os.ReadFile("testdata/ornek_metin.txt")
	if err != nil {
		t.Fatal(err)
	}
	girdiUret := func(n int) map[string]string {
		tekrar := func(s string) string { return strings.Repeat(s, n/len(s)+1)[:n] }
		return map[string]string{
			"metin":        tekrar(string(ornek)),
			"bosluk+x":     strings.Repeat(" ", n) + "x",
			"bosluk":       strings.Repeat(" ", n),
			"satir-bosluk": tekrar(" \n\t\r "),
			"noktalama":    tekrar("!?.,;"),
			"harf":         strings.Repeat("ğ", n/2),
			"rakam":        tekrar("0123456789"),
			"harf-bosluk":  tekrar("a "),
			"kesme":        tekrar("'s'S'x'"),
			"karisik":      tekrar("a1 !\n ş’e"),
		}
	}
	enCok := 0.0
	defer func() { t.Logf("en yüksek: %.1f adım/bayt", enCok) }()
	desenler := map[string]string{"bolme": BolmeDeseni, "bayt": BaytDuzeyiDeseni, "llama3": llama3Deseni, "qwen": qwenDeseni}
	for dad, d := range desenler {
		ifade, err := DuzIfadeDerle(d)
		if err != nil {
			t.Fatal(err)
		}
		kucuk, buyuk := girdiUret(20_000), girdiUret(80_000)
		for gad := range kucuk {
			var a1, a2 int64
			if err := adimHatasiniYakala(func() {
				a1 = ifade.tumEslesmeler(kucuk[gad], func(int, int) {}, nil)
				a2 = ifade.tumEslesmeler(buyuk[gad], func(int, int) {}, nil)
			}); err != nil {
				t.Errorf("%s/%s: bütçeye takıldı: %v", dad, gad, err)
				continue
			}
			bayt := float64(len(buyuk[gad]))
			if v := float64(a2) / bayt; v > enCok {
				enCok = v
			}
			oran := float64(a2) / float64(a1) * float64(len(kucuk[gad])) / bayt
			if float64(a2)/bayt > 40 || oran > 1.5 {
				t.Errorf("%s/%s: doğrusal değil: %d bayt -> %d adım (%.1f adım/bayt, büyüme oranı %.2f)",
					dad, gad, len(buyuk[gad]), a2, float64(a2)/bayt, oran)
			}
		}
	}
}
