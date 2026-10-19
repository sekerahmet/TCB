package codes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Taşınan kodlar canlı kodlardan ayrıdır: adları kabul edilmez, yeni koda yol gösterir;
// Go'da sabitleri yoktur, numaraları bir daha kullanılmaz.
func TestTasinanKodlar(t *testing.T) {
	beklenen := map[string]string{"G200": "Y001", "G300": "Y200", "G301": "Y201", "G302": "Y207", "G303": "Y202",
		"G304": "Y004", "T120": "Y411", "Y205": "G400", "Y206": "G410"}
	if len(MovedCodes()) != len(beklenen) {
		t.Errorf("taşınan kodlar %v, %d bekleniyordu", MovedCodes(), len(beklenen))
	}
	for e, y := range beklenen {
		ek, _ := Parse(e)
		yk, _ := Parse(y)
		if got, ok := MovedTo(ek); !ok || got != yk {
			t.Errorf("Tasindi(%s) = %s %v; %s bekleniyordu", e, got, ok, y)
		}
		if slices.Contains(Codes(), ek) {
			t.Errorf("%s taşındı; canlı kodlar arasında olmamalı", e)
		}
		ad := Name(ek)
		if k, ok := Find(ek.Namespace(), ad); ok {
			t.Errorf("taşınan %s'in adı %q kabul edilmemeli (%s)", e, ad, k)
		}
		if k, ok := FindDead(ek.Namespace(), ad); !ok || k != ek {
			t.Errorf("OluBul(%s, %q) = %s %v", ek.Namespace(), ad, k, ok)
		}
	}
	if k, ok := FindDead(NamespaceGlobal, "sirala"); !ok || k.String() != "G303" {
		t.Errorf("ASCII biçim de taşınan koda gitmeli: %s %v", k, ok)
	}
	if k, ok := FindDead(NamespaceGlobal, "boy"); !ok || k.String() != "G200" {
		t.Errorf("taşınan kodun eski adı da taşınan koda gitmeli: %s %v", k, ok)
	}
	// Taşınan bir ad aynı ad alanında canlı bir adla çakışırsa dil dosyası reddedilir.
	_, err := Load("dene", []byte(`{"G100": {"ad": "yazdır"}, "G200": {"ad": "yazdır"}, "Y001": {"ad": "uzunluk"}}`))
	if err == nil || !strings.Contains(err.Error(), "taşınmış G200'in adı \"yazdır\"") {
		t.Errorf("çakışma yakalanmalı: %v", err)
	}
}

// Ölü (kaldırılmış ya da taşınmış) kodlar yalnızca kaynak dil dosyasında (TR) zorunludur;
// başka bir dil dosyası onları yazmayabilir. Canlı bir kodun eksikliği her dilde hatadır.
func TestOluKodlarTRDisindaIstegeBagli(t *testing.T) {
	var ham map[string]json.RawMessage
	if err := json.Unmarshal(TRData(), &ham); err != nil {
		t.Fatal(err)
	}
	for _, k := range DeadCodes() {
		delete(ham, k.String())
	}
	kur := func(kodu string, m map[string]json.RawMessage) []error {
		veri, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Load(kodu, veri)
		if err != nil {
			t.Fatalf("%s yüklenmedi: %v", kodu, err)
		}
		return CheckDefinitions(d)
	}
	if hatalar := kur("en", ham); len(hatalar) != 0 {
		t.Errorf("ölü kodları olmayan başka bir dil dosyası geçmeli: %v", hatalar)
	}
	if hatalar := kur(SourceLanguage, ham); len(hatalar) == 0 {
		t.Error("kaynak dil dosyasında ölü kodlar zorunlu olmalı")
	}
	// Ölü bir kodun adı yeniden kullanılırsa ölü kodun adı kodun kendisi yazılır: kod biçimli
	// adlar ayrılmış olduğundan çakışmaz, ipucu düşer.
	if _, err := Load("dene", []byte(`{"G100": {"ad": "boy"}, "G200": {"ad": "G200"}, "Y001": {"ad": "uzunluk"}}`)); err != nil {
		t.Errorf("adı yeniden kullanılan ölü kod yüklenmeli: %v", err)
	}
	if _, err := Load("dene", []byte(`{"G100": {"ad": "G100"}}`)); err == nil || !strings.Contains(err.Error(), "kod biçiminde") {
		t.Errorf("canlı bir kodun adı kod biçiminde olamaz: %v", err)
	}
	delete(ham, "G100")
	if hatalar := kur("en", ham); len(hatalar) == 0 || !strings.Contains(fmt.Sprint(hatalar), "G100 definitions.json'da var, dil dosyasında yok") {
		t.Errorf("canlı kodun eksikliği her dilde hata olmalı: %v", hatalar)
	}
}

// definitions.json dil dosyasıyla tutarlıdır: her kodun tanımı var, türler ailelere uygun,
// imzalardaki tip ve adlı argüman kodları geçerli, taşınan kodlar canlı bir koda gider.
func TestTanimlarTutarli(t *testing.T) {
	for _, err := range CheckDefinitions(Active()) {
		t.Error(err)
	}
}

// Her kodun (taşınmışlar dahil) Türkçe bir açıklaması, her canlı kodun bir örneği vardır.
// Açıklama bir-iki cümledir.
func TestHerKodunAciklamasiVar(t *testing.T) {
	d := Active()
	for _, k := range append(d.Codes(), d.MovedCodes()...) {
		g, _ := d.Entry(k)
		if strings.TrimSpace(g.Description) == "" {
			t.Errorf("%s (%s): açıklama yok", k, g.Name)
		}
		if n := strings.Count(g.Description, ". ") - strings.Count(g.Description, "... ") + 1; n > 3 {
			t.Errorf("%s (%s): açıklama %d cümle; en çok 2-3 cümle olmalı", k, g.Name, n)
		}
		if g.MovedTo == None && strings.TrimSpace(g.Example) == "" {
			t.Errorf("%s (%s): örnek yok", k, g.Name)
		}
		if strings.Contains(g.Example, "\n") {
			t.Errorf("%s (%s): örnek tek satır olmalı", k, g.Name)
		}
	}
}

// Dosya insanlar tarafından düzenlenir: kodlar artan sırada, her girdi iki boşluk girintili
// tek bir satırdır.
func TestTanimSiraliVeSatirSatir(t *testing.T) {
	veri := DefinitionData()
	if !json.Valid(veri) {
		t.Fatal("definitions.json geçerli JSON değil")
	}
	dec := json.NewDecoder(bytes.NewReader(veri))
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
	ti, err := ParseType("A200|A201?", true)
	if err != nil || !ti.Optional || len(ti.Types) != 2 || ti.Types[1] != A201 {
		t.Fatalf("TipCoz: %+v %v", ti, err)
	}
	if _, err := ParseType("A200?", false); err == nil {
		t.Error("dönüş tipinde sonek hata olmalı")
	}
	_, _, err = ParseDefinitions([]byte(`{"G100": {"tür": "fonksiyon", "bilinmeyen": 1}}`))
	if err == nil || !strings.Contains(err.Error(), "bilinmeyen") {
		t.Errorf("bilinmeyen alan hata vermeli: %v", err)
	}
	_, _, err = ParseDefinitions([]byte(`{"G100": {"tür": "fonksiyon", "adlı": [["T902"]]}}`))
	if err == nil {
		t.Error("eksik adlı argüman hata vermeli")
	}
}
