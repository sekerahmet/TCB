package codes

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Gömülü tr.json geçerli olmalı ve hatasız yüklenmeli (yinelenen kod, ad alanı içinde
// Türkçe ya da ASCII biçimiyle çakışan ad, geçerli adla çakışan eski ad olmamalı).
func TestTRYuklenir(t *testing.T) {
	if !json.Valid(trJSON) {
		t.Fatal("tr.json geçerli JSON değil")
	}
	if _, err := Load("tr", trJSON); err != nil {
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
	var onceki Code
	for dec.More() {
		tk, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		k, err := Parse(tk.(string))
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

// Farklı iki ad, ad alanları farklı olsa bile aynı ASCII biçime
// düşmemeli: ASCII yazımdan Türkçe yazıma ipucu (Oneri) belirsiz olurdu.
func TestASCIIBicimlerCakismaz(t *testing.T) {
	type sahibi struct {
		kod Code
		ad  string
	}
	sahip := map[string]sahibi{}
	for _, k := range Codes() {
		ad := Name(k)
		a := turkish.ASCII(ad)
		if o, ok := sahip[a]; ok && o.ad != ad {
			t.Errorf("%s %q ile %s %q aynı ASCII biçime (%q) düşüyor", o.kod, o.ad, k, ad, a)
			continue
		}
		sahip[a] = sahibi{k, ad}
	}
	// Ad alanı içinde ASCII biçim ile eski ad çakışması Yukle'de yakalanır (bkz.
	// TestCakismalarYakalanir). Farklı ad alanları arasındaki çakışma bir hata değildir
	// (ör. G201'in eski adı "tur" genel adlarda, Y402 tür'ün ASCII biçimi "tur" üyelerde);
	// burada yalnızca bilgi olarak listelenir.
	for _, k := range Codes() {
		g, _ := Active().Entry(k)
		for _, e := range g.Old {
			yazimlar := []string{e}
			if a := turkish.ASCII(e); a != e {
				yazimlar = append(yazimlar, a)
			}
			for _, y := range yazimlar {
				if o, ok := sahip[turkish.ASCII(y)]; ok && (y == o.ad || y == turkish.ASCII(o.ad)) && o.kod.Namespace() != k.Namespace() {
					t.Logf("bilgi: %s'in eski adı %q, başka ad alanındaki %s %q ile aynı yazılıyor", k, y, o.kod, o.ad)
				}
			}
		}
	}
}

// Bir ad alanı içinde her ad benzersizdir; anahtar kelimeler yerleşiklerle çakışmaz.
func TestAdAlanlariBenzersiz(t *testing.T) {
	for al := NamespaceGlobal; al < alanSayisi; al++ {
		gorulen := map[string]Code{}
		for _, k := range Codes() {
			if k.Namespace() != al {
				continue
			}
			for _, y := range Spellings(k) {
				if o, ok := gorulen[y]; ok {
					t.Errorf("%s: %q hem %s hem %s", al, y, o, k)
				}
				gorulen[y] = k
				if b, ok := Find(al, y); !ok || b != k {
					t.Errorf("Bul(%s, %q) = %s, %s bekleniyordu", al, y, b, k)
				}
			}
		}
	}
}

func TestAsilAdAlanda(t *testing.T) {
	for _, c := range []struct {
		al        Namespace
		ad, want  string
		bulunmali bool
	}{
		{NamespaceGlobal, "yazdir", "yazdır", true}, {NamespaceGlobal, "yazdır", "yazdır", true},
		{NamespaceGlobal, "sonuc", "sonuç", true}, {NamespaceGlobal, "degisken", "değişken", true},
		{NamespaceMember, "sekil", "şekil", true}, {NamespaceParam, "ogrenme_orani", "öğrenme_oranı", true},
		{NamespaceMember, "tur", "tür", true}, {NamespaceGlobal, "tur", "", false}, {NamespaceGlobal, "tanimsiz_ad", "", false},
	} {
		if got, ok := CanonicalNameIn(c.al, c.ad); ok != c.bulunmali || got != c.want {
			t.Errorf("AsilAdAlanda(%s, %q) = %q %v; %q %v bekleniyordu", c.al, c.ad, got, ok, c.want, c.bulunmali)
		}
	}
}

// ASCII biçimler kabul edilmez (her işin tek yazımı); Oneri onlar için ve eski adlar için
// her zaman Türkçe adı önerir, ASCII biçimi asla önermez.
func TestASCIIKabulEdilmezOneriTurkce(t *testing.T) {
	if AcceptASCII {
		t.Fatal("ASCII biçimler kapalı olmalı")
	}
	for _, y := range []string{"yazdir", "degisken", "sonuc", "dogru"} {
		if k, ok := Find(NamespaceGlobal, y); ok {
			t.Errorf("%q kabul edilmemeli (%s)", y, k)
		}
	}
	for _, c := range []struct {
		al   Namespace
		ad   string
		want string
	}{
		{NamespaceGlobal, "yazdir", "yazdır"}, {NamespaceGlobal, "degisken", "değişken"},
		{NamespaceGlobal, "islev", "fonksiyon"}, {NamespaceGlobal, "dondur", "sonuç"}, {NamespaceGlobal, "işlev", "fonksiyon"},
		{NamespaceGlobal, "tur", "tip"}, {NamespaceMember, "tur", "tür"}, {NamespaceMember, "sekil", "şekil"},
		{NamespaceGlobal, "en_buyuk", "maks"}, {NamespaceParam, "turev_izle", "türev_izle"},
	} {
		k, ok := Suggest(c.al, c.ad, nil)
		if !ok || Name(k) != c.want {
			t.Errorf("Oneri(%s, %q) = %s %v; %q bekleniyordu", c.al, c.ad, k, ok, c.want)
		}
	}
	if _, ok := Suggest(NamespaceGlobal, "yazdır", nil); ok {
		t.Error("geçerli ad için öneri olmamalı")
	}
	if _, ok := Suggest(NamespaceGlobal, "yazdir", func(k Code) bool { return k.Letter() == 'A' }); ok {
		t.Error("süzgeç yerleşik kodu dışlamalıydı")
	}
}

func TestKodBicimi(t *testing.T) {
	for _, c := range []struct {
		k Code
		s string
	}{{A + 1, "A001"}, {G + 100, "G100"}, {Y + 400, "Y400"}, {W + 936, "W936"}} {
		if c.k.String() != c.s {
			t.Errorf("%d: %s, %s bekleniyordu", c.k, c.k, c.s)
		}
		if k, err := Parse(c.s); err != nil || k != c.k {
			t.Errorf("Coz(%q) = %v, %v", c.s, k, err)
		}
	}
	for _, s := range []string{"", "A01", "A0001", "Z001", "A000", "a001", "A0x1"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Coz(%q) hata vermeliydi", s)
		}
	}
	if (A+13).Namespace() != NamespaceGlobal || (A+200).Namespace() != NamespaceType || (Y+1).Namespace() != NamespaceMember ||
		(T+900).Namespace() != NamespaceParam || (G+100).Namespace() != NamespaceGlobal || (H+1).Namespace() != NamespaceNone {
		t.Error("ad alanları yanlış")
	}
	// Ayrılmış ad biçimi (2.3a): bütün büyük harfler, aile harfi olmasa da
	for s, want := range map[string]bool{"G100": true, "K001": true, "Z999": true, "A000": true,
		"g100": false, "G10": false, "G1000": false, "G10x": false, "Ç100": false, "": false} {
		if IsCodeForm(s) != want {
			t.Errorf("KodBicimi(%q) = %v, %v bekleniyordu", s, !want, want)
		}
	}
}

// Eski adlar ve ASCII biçimleri: işlev → islev de eski sayılır ve ascii olarak işaretlenir.
func TestEskiBul(t *testing.T) {
	for _, c := range []struct {
		ad    string
		kod   Code
		ascii bool
	}{{"işlev", A + 20, false}, {"islev", A + 20, true}, {"as", A + 100, false}, {"en_buyuk", G + 402, true}} {
		k, a, ok := FindOld(NamespaceGlobal, c.ad, nil)
		if !ok || k != c.kod || a != c.ascii {
			t.Errorf("EskiBul(%q) = %s %v %v; %s %v bekleniyordu", c.ad, k, a, ok, c.kod, c.ascii)
		}
	}
	if _, _, ok := FindOld(NamespaceGlobal, "işlev", func(k Code) bool { return k.IsBuiltin() }); ok {
		t.Error("süzgeç yerleşik olmayan kodu dışlamalıydı")
	}
	if _, _, ok := FindOld(NamespaceMember, "boy", nil); !ok {
		t.Error("üyelerde boy → uzunluk bekleniyordu")
	}
}

// Yukle kod listesi hatalarını hangi kodların çakıştığını söyleyerek yakalar.
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
		{`{"G100": {"ad": "yazdır", "es": ["yaz"]}}`, "bilinmeyen alan \"es\""},
		{`{"X100": {"ad": "a"}}`, "aile harfi"},
		{`{"H100": {"ad": "a"}}`, "henüz kullanılmıyor"},
		{`{"G100": {"ad": "a", "bilinmeyen": 1}}`, "bilinmeyen"},
	} {
		_, err := Load("deneme", []byte(c.json))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.json, c.want, err)
		}
	}
	// Farklı ad alanlarında aynı ad serbesttir.
	if _, err := Load("deneme", []byte(`{"G303": {"ad": "sırala"}, "Y202": {"ad": "sırala"}, "W932": {"ad": "sırala"}}`)); err != nil {
		t.Errorf("farklı ad alanlarında aynı ad kabul edilmeliydi: %v", err)
	}
}

// 'kapat' ölü Y008'in eski adıydı; Y008'in adı kodun kendisi yapıldığından (17, ölü kodlar)
// ad serbesttir ve Y012'ye verildi. Ad üyelerde yalnızca Y012'dir, hiçbir ölü kodun adı ya da
// eski adı değildir; eski ad kalsaydı yükleme çakışmayı reddederdi.
func TestKapatY012ileCakismaz(t *testing.T) {
	if k, ok := Find(NamespaceMember, "kapat"); !ok || k != Y+12 {
		t.Fatalf("üyelerde 'kapat' Y012 olmalı: %s %v", k, ok)
	}
	if k, ok := FindDead(NamespaceMember, "kapat"); ok {
		t.Errorf("'kapat' ölü bir kodun adı olmamalı: %s", k)
	}
	if k, _, ok := FindOld(NamespaceMember, "kapat", nil); ok {
		t.Errorf("'kapat' bir kodun eski adı olmamalı: %s", k)
	}
	if Name(Y+8) != "Y008" {
		t.Errorf("Y008'in adı kodun kendisi olmalı: %q", Name(Y+8))
	}
	_, err := Load("deneme", []byte(`{"Y008": {"ad": "Y008", "eski": ["kapat"]}, "Y012": {"ad": "kapat"}}`))
	if err == nil || !strings.Contains(err.Error(), "Y008'in adı \"kapat\", Y012") {
		t.Errorf("ölü kodun eski adı canlı adla çakışınca reddedilmeli: %v", err)
	}
}

// Her adın tek yazımı vardır: yalnızca Türkçe biçimi kabul edilir.
func TestTekYazim(t *testing.T) {
	for _, k := range Codes() {
		if y := Spellings(k); len(y) != 1 || y[0] != Name(k) {
			t.Errorf("%s tek yazımlı olmalı (%q): %v", k, Name(k), y)
		}
	}
	if y := Spellings(G + 100); len(y) != 1 || y[0] != "yazdır" {
		t.Errorf("G100 yalnızca Türkçe biçimiyle kabul edilmeli: %v", y)
	}
	if y := Spellings(G + 400); len(y) != 1 {
		t.Errorf("ASCII ad tek yazımlı olmalı: %v", y)
	}
}

// Kaldırılan eş yazımlar (eskiden kabul edilen ikinci adlar) artık eski adlardır: kabul
// edilmezler, Oneri asıl adı önerir.
func TestEsYazimlarKaldirildi(t *testing.T) {
	for _, c := range []struct {
		kod   Code
		yazim string
	}{{Y + 403, "türev"}, {Y + 4, "kayıt_ekle"}, {T + 910, "öğrenme_hızı"}, {A + 200, "tamsayı"}, {A + 200, "int"}, {A + 201, "float"}, {A + 202, "string"},
		{A + 203, "bool"}, {A + 204, "null"}, {A + 205, "dizi"}, {A + 206, "map"}, {A + 212, "matris"}} {
		if k, ok := Find(c.kod.Namespace(), c.yazim); ok {
			t.Errorf("%q kabul edilmemeli (%s)", c.yazim, k)
		}
		if k, ok := Suggest(c.kod.Namespace(), c.yazim, nil); !ok || k != c.kod {
			t.Errorf("Oneri(%q) = %s %v, %s bekleniyordu", c.yazim, k, ok, c.kod)
		}
	}
	// Kaldırılan kodların adları ve eski adları öneri değil "kaldırıldı" bilgisi verir.
	for _, c := range []struct {
		al    Namespace
		yazim string
	}{{NamespaceMember, "sorgula"}, {NamespaceMember, "sorgu"}, {NamespaceGlobal, "veritabanı"}, {NamespaceGlobal, "web_sunucu"}, {NamespaceParam, "params"}} {
		if k, ok := FindDead(c.al, c.yazim); !ok || !Removed(k) {
			t.Errorf("OluBul(%s, %q) = %s %v; kaldırılmış bir kod bekleniyordu", c.al, c.yazim, k, ok)
		}
	}
}

// Her yazım dili hatasız yüklenir ve Türkçenin her canlı kodunu adlandırır (yeni bir kodun adı
// her dil dosyasına eklenmelidir); ölü kodlar yalnızca Türkçede adlıdır (başka dillerde eski
// kod hiç olmadı, 17.6).
func TestYazimDilleriTamDir(t *testing.T) {
	if strings.Join(Languages(), ",") != "en,tr" {
		t.Fatalf("diller: %v", Languages())
	}
	for _, kodu := range Languages() {
		d, err := LoadLanguage(kodu)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range Active().Codes() {
			if _, ok := d.Entry(k); !ok {
				t.Errorf("%s dilinde %s (%s) adsız", kodu, k, Active().Name(k))
			}
		}
		if kodu == "tr" {
			continue
		}
		for _, k := range Active().DeadCodes() {
			if g, ok := d.Entry(k); ok {
				t.Errorf("%s dilinde ölü kod %s adlı (%s)", kodu, k, g.Name)
			}
		}
	}
	if _, err := LoadLanguage("xx"); err == nil || !strings.Contains(err.Error(), "var olanlar: en, tr") {
		t.Fatalf("bilinmeyen dil: %v", err)
	}
}
