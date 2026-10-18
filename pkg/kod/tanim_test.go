package kod

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// Taşınan kodlar canlı kodlardan ayrıdır: adları kabul edilmez, yeni koda yol gösterir;
// Go'da sabitleri yoktur, numaraları bir daha kullanılmaz.
func TestTasinanKodlar(t *testing.T) {
	beklenen := map[string]string{"G200": "Y001", "G300": "Y200", "G301": "Y201", "G302": "Y207", "G303": "Y202",
		"G304": "Y004", "T120": "Y411", "Y205": "G400", "Y206": "G410"}
	if len(Tasinanlar()) != len(beklenen) {
		t.Errorf("taşınan kodlar %v, %d bekleniyordu", Tasinanlar(), len(beklenen))
	}
	for e, y := range beklenen {
		ek, _ := Coz(e)
		yk, _ := Coz(y)
		if got, ok := Tasindi(ek); !ok || got != yk {
			t.Errorf("Tasindi(%s) = %s %v; %s bekleniyordu", e, got, ok, y)
		}
		if slices.Contains(Kodlar(), ek) {
			t.Errorf("%s taşındı; canlı kodlar arasında olmamalı", e)
		}
		ad := Ad(ek)
		if k, ok := Bul(ek.Alan(), ad); ok {
			t.Errorf("taşınan %s'in adı %q kabul edilmemeli (%s)", e, ad, k)
		}
		if k, ok := TasinanBul(ek.Alan(), ad); !ok || k != ek {
			t.Errorf("TasinanBul(%s, %q) = %s %v", ek.Alan(), ad, k, ok)
		}
	}
	if k, ok := TasinanBul(AlanGenel, "sirala"); !ok || k.String() != "G303" {
		t.Errorf("ASCII biçim de taşınan koda gitmeli: %s %v", k, ok)
	}
	if k, ok := TasinanBul(AlanGenel, "boy"); !ok || k.String() != "G200" {
		t.Errorf("taşınan kodun eski adı da taşınan koda gitmeli: %s %v", k, ok)
	}
	// Taşınan bir ad aynı ad alanında canlı bir adla çakışırsa dil dosyası reddedilir.
	_, err := Yukle("dene", []byte(`{"G100": {"ad": "yazdır"}, "G200": {"ad": "yazdır"}, "Y001": {"ad": "uzunluk"}}`))
	if err == nil || !strings.Contains(err.Error(), "taşınmış G200'in adı \"yazdır\"") {
		t.Errorf("çakışma yakalanmalı: %v", err)
	}
}

// tanim.json dil dosyasıyla tutarlıdır: her kodun tanımı var, türler ailelere uygun,
// imzalardaki tip ve adlı argüman kodları geçerli, taşınan kodlar canlı bir koda gider.
func TestTanimlarTutarli(t *testing.T) {
	for _, err := range TanimDenetle(Etkin()) {
		t.Error(err)
	}
}

// Her kodun (taşınmışlar dahil) Türkçe bir açıklaması, her canlı kodun bir örneği vardır.
// Açıklama bir-iki cümledir.
func TestHerKodunAciklamasiVar(t *testing.T) {
	d := Etkin()
	for _, k := range append(d.Kodlar(), d.Tasinanlar()...) {
		g, _ := d.Girdisi(k)
		if strings.TrimSpace(g.Aciklama) == "" {
			t.Errorf("%s (%s): açıklama yok", k, g.Ad)
		}
		if n := strings.Count(g.Aciklama, ". ") - strings.Count(g.Aciklama, "... ") + 1; n > 3 {
			t.Errorf("%s (%s): açıklama %d cümle; en çok 2-3 cümle olmalı", k, g.Ad, n)
		}
		if g.Tasindi == Yok && strings.TrimSpace(g.Ornek) == "" {
			t.Errorf("%s (%s): örnek yok", k, g.Ad)
		}
		if strings.Contains(g.Ornek, "\n") {
			t.Errorf("%s (%s): örnek tek satır olmalı", k, g.Ad)
		}
	}
}

// Dosya insanlar tarafından düzenlenir: kodlar artan sırada, her girdi iki boşluk girintili
// tek bir satırdır.
func TestTanimSiraliVeSatirSatir(t *testing.T) {
	veri := TanimVerisi()
	if !json.Valid(veri) {
		t.Fatal("tanim.json geçerli JSON değil")
	}
	dec := json.NewDecoder(bytes.NewReader(veri))
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	var onceki Kod
	for dec.More() {
		tk, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		k, err := Coz(tk.(string))
		if err != nil {
			t.Fatal(err)
		}
		if k <= onceki {
			t.Errorf("%s, %s'den sonra geliyor; kodlar artan sırada olmalı", k, onceki)
		}
		onceki = k
		var g json.RawMessage
		if err := dec.Decode(&g); err != nil {
			t.Fatal(err)
		}
	}
	for i, satir := range strings.Split(strings.TrimSpace(string(veri)), "\n") {
		s := strings.TrimSpace(satir)
		if s == "{" || s == "}" {
			continue
		}
		if !strings.HasPrefix(satir, "  \"") || strings.Count(s, "\"tür\"") != 1 {
			t.Errorf("satır %d: her girdi iki boşluk girintili tek satır olmalı: %s", i+1, satir)
		}
	}
}

// Tip ifadeleri çözülür; geçersiz bir tanım satırı hatayla bildirilir.
func TestTanimCoz(t *testing.T) {
	ti, err := TipCoz("A200|A201?", true)
	if err != nil || !ti.Istege || len(ti.Tipler) != 2 || ti.Tipler[1] != A201 {
		t.Fatalf("TipCoz: %+v %v", ti, err)
	}
	if _, err := TipCoz("A200?", false); err == nil {
		t.Error("dönüş tipinde sonek hata olmalı")
	}
	_, _, err = TanimCoz([]byte(`{"G100": {"tür": "fonksiyon", "bilinmeyen": 1}}`))
	if err == nil || !strings.Contains(err.Error(), "bilinmeyen") {
		t.Errorf("bilinmeyen alan hata vermeli: %v", err)
	}
	_, _, err = TanimCoz([]byte(`{"G100": {"tür": "fonksiyon", "adlı": [["T902"]]}}`))
	if err == nil {
		t.Error("eksik adlı argüman hata vermeli")
	}
}
