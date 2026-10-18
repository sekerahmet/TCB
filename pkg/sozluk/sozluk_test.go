package sozluk

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"tr_dili/pkg/turkce"
)

// Gömülü tr.json geçerli olmalı ve hatasız yüklenmeli (yinelenen kod, ad alanı içinde
// Türkçe ya da ASCII biçimiyle çakışan ad, geçerli adla çakışan eski ad olmamalı).
func TestTRYuklenir(t *testing.T) {
	if !json.Valid(trJSON) {
		t.Fatal("tr.json geçerli JSON değil")
	}
	if _, err := Yukle("tr", trJSON); err != nil {
		t.Fatal(err)
	}
}

// Dosya insanlar tarafından düzenlenir: kodlar artan sırada ve her girdi kendi satırında
// olmalı.
func TestTRSiraliVeSatirSatir(t *testing.T) {
	dec := json.NewDecoder(bytes.NewReader(trJSON))
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	var onceki Kod
	for dec.More() {
		tk, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		k, err := KodCoz(tk.(string))
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
	for i, satir := range strings.Split(strings.TrimSpace(string(trJSON)), "\n") {
		s := strings.TrimSpace(satir)
		if s == "{" || s == "}" {
			continue
		}
		if !strings.HasPrefix(satir, "  \"") || strings.Count(s, "\"ad\"") != 1 {
			t.Errorf("satır %d: her girdi iki boşluk girintili tek satır olmalı: %s", i+1, satir)
		}
	}
}

// tumAdlar kodun adı ve eş yazımlarıdır.
func tumAdlar(k Kod) []string {
	g, _ := Etkin().Girdisi(k)
	return append([]string{g.Ad}, g.Es...)
}

// Farklı iki ad (eş yazımlar dahil), ad alanları farklı olsa bile aynı ASCII biçime
// düşmemeli: AsilAd ve ASCII yazımdan Türkçe yazıma ipucu belirsiz olurdu.
func TestASCIIBicimlerCakismaz(t *testing.T) {
	type sahibi struct {
		kod Kod
		ad  string
	}
	sahip := map[string]sahibi{}
	for _, k := range Kodlar() {
		for _, ad := range tumAdlar(k) {
			a := turkce.ASCII(ad)
			if o, ok := sahip[a]; ok && o.ad != ad {
				t.Errorf("%s %q ile %s %q aynı ASCII biçime (%q) düşüyor", o.kod, o.ad, k, ad, a)
				continue
			}
			sahip[a] = sahibi{k, ad}
		}
	}
	// Ad alanı içinde ASCII biçim ile eski ad çakışması Yukle'de yakalanır (bkz.
	// TestCakismalarYakalanir). Farklı ad alanları arasındaki çakışma bir hata değildir
	// (ör. G201'in eski adı "tur" genel adlarda, Y402 tür'ün ASCII biçimi "tur" üyelerde);
	// burada yalnızca bilgi olarak listelenir.
	for _, k := range Kodlar() {
		g, _ := Etkin().Girdisi(k)
		for _, e := range g.Eski {
			yazimlar := []string{e}
			if a := turkce.ASCII(e); a != e {
				yazimlar = append(yazimlar, a)
			}
			for _, y := range yazimlar {
				if o, ok := sahip[turkce.ASCII(y)]; ok && (y == o.ad || y == turkce.ASCII(o.ad)) && o.kod.Alan() != k.Alan() {
					t.Logf("bilgi: %s'in eski adı %q, başka ad alanındaki %s %q ile aynı yazılıyor", k, y, o.kod, o.ad)
				}
			}
		}
	}
}

// Bir ad alanı içinde her ad benzersizdir; anahtar kelimeler yerleşiklerle çakışmaz.
func TestAdAlanlariBenzersiz(t *testing.T) {
	for al := AlanGenel; al < alanSayisi; al++ {
		gorulen := map[string]Kod{}
		for _, k := range Kodlar() {
			if k.Alan() != al {
				continue
			}
			for _, y := range Yazimlar(k) {
				if o, ok := gorulen[y]; ok {
					t.Errorf("%s: %q hem %s hem %s", al, y, o, k)
				}
				gorulen[y] = k
				if b, ok := Bul(al, y); !ok || b != k {
					t.Errorf("Bul(%s, %q) = %s, %s bekleniyordu", al, y, b, k)
				}
			}
		}
	}
}

func TestAsilAd(t *testing.T) {
	for _, c := range []struct{ ad, want string }{
		{"yazdir", "yazdır"}, {"yazdır", "yazdır"}, {"sonuc", "sonuç"}, {"degisken", "değişken"},
		{"sekil", "şekil"}, {"ogrenme_orani", "öğrenme_oranı"}, {"tanimsiz_ad", "tanimsiz_ad"},
	} {
		if got := AsilAd(c.ad); got != c.want {
			t.Errorf("AsilAd(%q) = %q, %q bekleniyordu", c.ad, got, c.want)
		}
	}
	if a, ok := Etkin().AsilAdAlanda(AlanUye, "tur"); !ok || a != "tür" {
		t.Errorf("üyelerde tur → tür bekleniyordu, %q %v", a, ok)
	}
	if _, ok := Etkin().AsilAdAlanda(AlanGenel, "tur"); ok {
		t.Error("genel adlarda 'tur' bir ad değil (G201'in eski adı)")
	}
}

func TestKodBicimi(t *testing.T) {
	for _, c := range []struct {
		k Kod
		s string
	}{{A + 1, "A001"}, {G + 100, "G100"}, {Y + 400, "Y400"}, {W + 936, "W936"}} {
		if c.k.String() != c.s {
			t.Errorf("%d: %s, %s bekleniyordu", c.k, c.k, c.s)
		}
		if k, err := KodCoz(c.s); err != nil || k != c.k {
			t.Errorf("KodCoz(%q) = %v, %v", c.s, k, err)
		}
	}
	for _, s := range []string{"", "A01", "A0001", "Z001", "A000", "a001", "A0x1"} {
		if _, err := KodCoz(s); err == nil {
			t.Errorf("KodCoz(%q) hata vermeliydi", s)
		}
	}
	if (A+13).Alan() != AlanGenel || (A+200).Alan() != AlanTip || (Y+1).Alan() != AlanUye ||
		(T+900).Alan() != AlanParametre || (G+100).Alan() != AlanGenel || (H+1).Alan() != AlanYok {
		t.Error("ad alanları yanlış")
	}
}

// Eski adlar ve ASCII biçimleri: işlev → islev de eski sayılır ve ascii olarak işaretlenir.
func TestEskiBul(t *testing.T) {
	for _, c := range []struct {
		ad    string
		kod   Kod
		ascii bool
	}{{"işlev", A + 20, false}, {"islev", A + 20, true}, {"as", A + 100, false}, {"en_buyuk", G + 402, true}} {
		k, a, ok := EskiBul(AlanGenel, c.ad, nil)
		if !ok || k != c.kod || a != c.ascii {
			t.Errorf("EskiBul(%q) = %s %v %v; %s %v bekleniyordu", c.ad, k, a, ok, c.kod, c.ascii)
		}
	}
	if _, _, ok := EskiBul(AlanGenel, "işlev", func(k Kod) bool { return k.Yerlesik() }); ok {
		t.Error("süzgeç yerleşik olmayan kodu dışlamalıydı")
	}
	if _, _, ok := EskiBul(AlanUye, "boy", nil); !ok {
		t.Error("üyelerde boy → uzunluk bekleniyordu")
	}
}

// Yukle sözlük hatalarını hangi kodların çakıştığını söyleyerek yakalar.
func TestCakismalarYakalanir(t *testing.T) {
	for _, c := range []struct{ json, want string }{
		{`{"G100": {"ad": "a"}, "G100": {"ad": "b"}}`, "G100 iki kez"},
		{`{"G100": {"ad": "yazdır"}, "G101": {"ad": "yazdır"}}`, "G100 \"yazdır\" ile G101"},
		{`{"G100": {"ad": "yazdır"}, "G101": {"ad": "yazdir"}}`, "G100 \"yazdır\" ile G101 \"yazdir\""},
		{`{"A001": {"ad": "eğer"}, "G100": {"ad": "eger"}}`, "A001 \"eğer\" ile G100"},
		{`{"G100": {"ad": "yazdır"}, "G101": {"ad": "girdi", "eski": ["yazdir"]}}`, "G101'in eski adı \"yazdir\", G100"},
		{`{"G100": {"ad": "a", "eski": ["işlev"]}, "G101": {"ad": "b", "eski": ["islev"]}}`, "\"islev\" hem G100 hem G101"},
		{`{"G100": {"ad": "a", "eski": ["a"]}}`, "adın kendisi"},
		{`{"G100": {"ad": "a b"}}`, "boşluk"},
		{`{"G100": {"ad": "yazdır", "es": ["yaz"]}, "G101": {"ad": "yaz"}}`, "G100 \"yaz\" ile G101 \"yaz\""},
		{`{"G100": {"ad": "yazdır", "es": ["yaz"]}, "G101": {"ad": "girdi", "eski": ["yaz"]}}`, "G101'in eski adı \"yaz\", G100 \"yaz\""},
		{`{"X100": {"ad": "a"}}`, "aile harfi"},
		{`{"H100": {"ad": "a"}}`, "henüz kullanılmıyor"},
		{`{"G100": {"ad": "a", "bilinmeyen": 1}}`, "bilinmeyen"},
	} {
		_, err := Yukle("deneme", []byte(c.json))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.json, c.want, err)
		}
	}
	// Farklı ad alanlarında aynı ad serbesttir.
	if _, err := Yukle("deneme", []byte(`{"G303": {"ad": "sırala"}, "Y202": {"ad": "sırala"}, "W932": {"ad": "sırala"}}`)); err != nil {
		t.Errorf("farklı ad alanlarında aynı ad kabul edilmeliydi: %v", err)
	}
}

// "ascii": false olan adın yalnızca Türkçe biçimi kabul edilir.
func TestASCIIKapali(t *testing.T) {
	if y := Yazimlar(W + 930); len(y) != 2 || y[0] != "koşul" || y[1] != "nerede" {
		t.Errorf("W930 yalnızca 'koşul' ve eş yazımı 'nerede' olarak kabul edilmeli: %v", y)
	}
	if y := Yazimlar(G + 100); ASCIIKabul && (len(y) != 2 || y[1] != "yazdir") {
		t.Errorf("G100 yazımları: %v", y)
	}
	if y := Yazimlar(G + 400); len(y) != 1 {
		t.Errorf("ASCII ad tek yazımlı olmalı: %v", y)
	}
}

// Eş yazımlar asıl adla aynı koda bağlıdır ve kabul edilir; EsYazimi onları ayırt eder.
func TestEsYazimlar(t *testing.T) {
	for _, c := range []struct {
		kod   Kod
		yazim string
	}{{Y + 702, "sorgu"}, {Y + 702, "seç"}, {Y + 702, "sec"}, {Y + 403, "türev"}, {Y + 403, "turev"},
		{W + 200, "veritabanı"}, {A + 200, "tamsayı"}, {A + 212, "matris"}} {
		if k, ok := Bul(c.kod.Alan(), c.yazim); !ok || k != c.kod {
			t.Errorf("Bul(%q) = %s, %s bekleniyordu", c.yazim, k, c.kod)
		}
		if !EsYazimi(c.kod, c.yazim) {
			t.Errorf("%q, %s'in eş yazımı olmalı", c.yazim, c.kod)
		}
	}
	if EsYazimi(Y+702, "sorgula") || EsYazimi(Y+403, "gradyan") {
		t.Error("asıl ad eş yazım sayılmamalı")
	}
	if AsilAd("sec") != "seç" {
		t.Errorf("AsilAd(sec) = %q", AsilAd("sec"))
	}
}
