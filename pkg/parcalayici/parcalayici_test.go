package parcalayici

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// ============================================================================
// Altın veriler (testdata/uret.py ile üretilmiş)
// ============================================================================

const kucukYol = "testdata/p_bayt_bpe.json"

// sentetikParcalayicilar testdata'daki parçalayıcıların adlarıdır.
var sentetikParcalayicilar = []string{
	"bayt_bpe", "metaspace_bpe", "llama_tarzi", "kelime_bpe", "onekli_bpe",
	"wordpiece", "unigram", "gpt2_tarzi", "llama3_tarzi",
}

// altinDosya altin_<ad>.json biçimidir. Kodlama: [girdi sırası, kimlikler,
// özel tanımadan kimlikler, özel eklemeden kimlikler]. Çift: [a, b, kimlikler,
// tür kimlikleri, özel maske, özelsiz kimlikler, özelsiz tür kimlikleri].
// Çözme: [kimlikler, metin, özel atlanmış metin].
type altinDosya struct {
	SozlukBoyutu int                 `json:"sozluk_boyutu"`
	Kodlama      [][]json.RawMessage `json:"kodlama"`
	Cift         [][]json.RawMessage `json:"cift"`
	Cozme        [][]json.RawMessage `json:"cozme"`
}

type altinKodlama struct {
	Metin    string
	Idler    []int
	IdlerDuz []int
}

type altinVeri struct {
	SozlukBoyutu int
	Kodlama      []altinKodlama
}

var (
	girdiBir   sync.Once
	girdiler   []string
	girdiHata  error
	kucukBir   sync.Once
	kucukP     *Parcalayici
	kucukHata  error
	kucukAltin altinVeri
)

func girdileriYukle(t testing.TB) []string {
	t.Helper()
	girdiBir.Do(func() {
		var d struct {
			Girdiler []string `json:"girdiler"`
		}
		veri, err := os.ReadFile("testdata/girdiler.json")
		if err == nil {
			err = json.Unmarshal(veri, &d)
		}
		girdiler, girdiHata = d.Girdiler, err
	})
	if girdiHata != nil {
		t.Fatalf("girdiler okunamadı: %v", girdiHata)
	}
	return girdiler
}

func altinYukle(t testing.TB, ad string) (*Parcalayici, *altinDosya) {
	t.Helper()
	p, err := Yukle("testdata/p_" + ad + ".json")
	if err != nil {
		t.Fatalf("%s yüklenemedi: %v", ad, err)
	}
	veri, err := os.ReadFile("testdata/altin_" + ad + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var a altinDosya
	if err := json.Unmarshal(veri, &a); err != nil {
		t.Fatalf("altin_%s.json çözümlenemedi: %v", ad, err)
	}
	return p, &a
}

func coz[T any](t testing.TB, r json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r, &v); err != nil {
		t.Fatalf("altın veri çözümlenemedi: %v", err)
	}
	return v
}

// kucukYukle bayt düzeyi BPE parçalayıcısını ve altın kodlamalarını verir.
func kucukYukle(t testing.TB) (*Parcalayici, *altinVeri) {
	t.Helper()
	g := girdileriYukle(t)
	kucukBir.Do(func() {
		var a *altinDosya
		kucukP, a = altinYukle(t, "bayt_bpe")
		kucukAltin.SozlukBoyutu = a.SozlukBoyutu
		for _, k := range a.Kodlama {
			kucukAltin.Kodlama = append(kucukAltin.Kodlama, altinKodlama{
				Metin: g[coz[int](t, k[0])], Idler: coz[[]int](t, k[1]), IdlerDuz: coz[[]int](t, k[2]),
			})
		}
	})
	if kucukHata != nil {
		t.Fatal(kucukHata)
	}
	return kucukP, &kucukAltin
}

func esitInt(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func esit32(a []int32, b []int) bool { return esitInt(intCevir(nil, a), b) }

func esitBool(a []bool, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != (b[i] != 0) {
			return false
		}
	}
	return true
}

// TestSentetikParcalayicilar her sentetik parçalayıcıyı altın çıktılarla
// karşılaştırır: tek metin kodlama (üç seçenekle), çift kodlama (tür
// kimlikleri ve özel maske dahil) ve çözme (özel atlamalı ve atlamasız).
func TestSentetikParcalayicilar(t *testing.T) {
	g := girdileriYukle(t)
	for _, ad := range sentetikParcalayicilar {
		t.Run(ad, func(t *testing.T) {
			p, a := altinYukle(t, ad)
			if p.SozlukBoyutu() != a.SozlukBoyutu {
				t.Errorf("sözlük boyutu %d, beklenen %d", p.SozlukBoyutu(), a.SozlukBoyutu)
			}
			hata, toplam := 0, 0
			rapor := func(f string, x ...any) {
				hata++
				if hata <= 8 {
					t.Errorf(f, x...)
				}
			}
			for _, k := range a.Kodlama {
				m := g[coz[int](t, k[0])]
				secenekler := []Secenekler{{}, {OzelTaninmasin: true}, {OzelEklenmesin: true}}
				for i, s := range secenekler {
					toplam++
					beklenen := coz[[]int](t, k[1+i])
					if got := p.KodlaSecenekli(m, s); !esitInt(got, beklenen) {
						rapor("Kodla(%q, %+v)\n alınan: %v\nbeklenen: %v", m, s, got, beklenen)
					}
				}
			}
			for _, c := range a.Cift {
				toplam += 2
				x, y := g[coz[int](t, c[0])], g[coz[int](t, c[1])]
				k := p.KodlaCift(x, &y, Secenekler{})
				if !esit32(k.Idler, coz[[]int](t, c[2])) || !esit32(k.TurIdleri, coz[[]int](t, c[3])) || !esitBool(k.OzelMaske, coz[[]int](t, c[4])) {
					rapor("KodlaCift(%q, %q)\n alınan: %v %v %v\nbeklenen: %s %s %s", x, y, k.Idler, k.TurIdleri, k.OzelMaske, c[2], c[3], c[4])
				}
				k2 := p.KodlaCift(x, &y, Secenekler{OzelEklenmesin: true})
				if !esit32(k2.Idler, coz[[]int](t, c[5])) || !esit32(k2.TurIdleri, coz[[]int](t, c[6])) {
					rapor("KodlaCift özelsiz(%q, %q)\n alınan: %v %v\nbeklenen: %s %s", x, y, k2.Idler, k2.TurIdleri, c[5], c[6])
				}
			}
			for _, c := range a.Cozme {
				toplam += 2
				ids := coz[[]int](t, c[0])
				if got, b := p.Coz(ids, false), coz[string](t, c[1]); got != b {
					rapor("Coz(%v, false)\n alınan: %q\nbeklenen: %q", ids, got, b)
				}
				if got, b := p.Coz(ids, true), coz[string](t, c[2]); got != b {
					rapor("Coz(%v, true)\n alınan: %q\nbeklenen: %q", ids, got, b)
				}
			}
			if hata > 0 {
				t.Fatalf("%d / %d karşılaştırma uyuşmadı", hata, toplam)
			}
			t.Logf("%d karşılaştırma eşleşti", toplam)
		})
	}
}

// TestBilesenler tek tek bileşenleri (normalleştirici, ön bölücü, çözücü)
// altın çıktılarla karşılaştırır.
func TestBilesenler(t *testing.T) {
	g := girdileriYukle(t)[1:140]
	veri, err := os.ReadFile("testdata/bilesenler.json")
	if err != nil {
		t.Fatal(err)
	}
	var kayitlar [][]json.RawMessage
	if err := json.Unmarshal(veri, &kayitlar); err != nil {
		t.Fatal(err)
	}
	bilesenKarsilastir(t, g, kayitlar)
}

// TestBilesenlerGenis, TR_PARCALAYICI_BILESEN_ALTIN ile verilen geniş altın
// veriyle (ör. bütün Unicode kod noktaları) bileşenleri karşılaştırır.
// Biçim: {"girdiler": [...], "kayitlar": [bilesenler.json kayıtları]}.
func TestBilesenlerGenis(t *testing.T) {
	yol := os.Getenv("TR_PARCALAYICI_BILESEN_ALTIN")
	if yol == "" {
		t.Skip("TR_PARCALAYICI_BILESEN_ALTIN ayarlı değil")
	}
	veri, err := os.ReadFile(yol)
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Girdiler []string            `json:"girdiler"`
		Kayitlar [][]json.RawMessage `json:"kayitlar"`
	}
	if err := json.Unmarshal(veri, &d); err != nil {
		t.Fatal(err)
	}
	bilesenKarsilastir(t, d.Girdiler, d.Kayitlar)
}

func bilesenKarsilastir(t *testing.T, g []string, kayitlar [][]json.RawMessage) {
	sayac := map[string]int{}
	hata := 0
	rapor := func(f string, x ...any) {
		hata++
		if hata <= 15 {
			t.Errorf(f, x...)
		}
	}
	for _, k := range kayitlar {
		tur := coz[string](t, k[0])
		tanim := k[1]
		switch tur {
		case "normalizer":
			n, err := NormallestiriciKur(tanim)
			if err != nil {
				rapor("%s: %v", tanim, err)
				continue
			}
			for i, b := range coz[[]string](t, k[2]) {
				sayac[tur]++
				if got := n.Normallestir(g[i]); got != b {
					rapor("%s\n girdi: %q\n alınan: %q\nbeklenen: %q", tanim, g[i], got, b)
				}
			}
		case "pre_tokenizer":
			o, err := OnBolucuKur(tanim)
			if err != nil {
				rapor("%s: %v", tanim, err)
				continue
			}
			for i, b := range coz[[][]string](t, k[2]) {
				sayac[tur]++
				var got []string
				for _, x := range o.OnBol(nil, []Parca{{Metin: g[i], Ilk: true}}) {
					got = append(got, x.Icerik())
				}
				if len(got) != len(b) || (len(got) > 0 && !reflect.DeepEqual(got, b)) {
					rapor("%s\n girdi: %q\n alınan: %q\nbeklenen: %q", tanim, g[i], got, b)
				}
			}
		case "decoder":
			c, err := CozucuKur(tanim)
			if err != nil {
				rapor("%s: %v", tanim, err)
				continue
			}
			listeler := coz[[][]string](t, k[2])
			for i, b := range coz[[]string](t, k[3]) {
				sayac[tur]++
				if got := strings.Join(c.CozZincir(append([]string(nil), listeler[i]...)), ""); got != b {
					rapor("%s\n girdi: %q\n alınan: %q\nbeklenen: %q", tanim, listeler[i], got, b)
				}
			}
		}
	}
	if hata > 0 {
		t.Fatalf("%d karşılaştırma uyuşmadı (%v)", hata, sayac)
	}
	t.Logf("eşleşen karşılaştırmalar: %v", sayac)
}

// Kodlama sonra çözme, bayt düzeyi parçalayıcıda metnin kendisini vermeli.
func TestGidisDonus(t *testing.T) {
	p, a := kucukYukle(t)
	// Bu ek tokenlar tasarım gereği gidiş-dönüşü bozar: boşluk yutan
	// (lstrip/rstrip) ya da çözücüde bayt eşlemesine takılan içerikler.
	bozanlar := []string{"<|maske|>", "[KELİME]", "çaycı", "Özel"}
	for _, k := range a.Kodlama {
		atla := false
		for _, b := range bozanlar {
			atla = atla || strings.Contains(k.Metin, b)
		}
		if atla {
			continue
		}
		if got := p.Coz(p.Kodla(k.Metin), false); got != k.Metin {
			t.Errorf("gidiş-dönüş bozuk: %q -> %q", k.Metin, got)
		}
	}
}

// Ön bölme parçalarının birleşimi girdinin kendisi olmalı.
func TestOnBolmeKapsar(t *testing.T) {
	for _, m := range girdileriYukle(t) {
		if got := strings.Join(OnBol(m), ""); got != m {
			t.Errorf("ön bölme metni kapsamıyor: %q -> %q", m, got)
		}
	}
}

func TestOnBolmeOrnekleri(t *testing.T) {
	durumlar := []struct {
		metin string
		parca []string
	}{
		{"Ankara'ya gitti", []string{"Ankara", "'ya", " gitti"}},
		{"İzmir’e 3'te", []string{"İzmir", "’e", " 3", "'te"}},
		{"2024", []string{"2", "0", "2", "4"}},
		{"a   b", []string{"a", "  ", " b"}},
		{"a \n b", []string{"a", " \n", " b"}},
		{"son   ", []string{"son", "   "}},
		{"a\nb", []string{"a", "\n", "b"}},
		{"hey!!! ...", []string{"hey", "!!!", " ..."}},
		{" 'ya", []string{" '", "ya"}},
		{"", nil},
	}
	for _, d := range durumlar {
		if got := OnBol(d.metin); !reflect.DeepEqual(got, d.parca) {
			t.Errorf("OnBol(%q) = %q, beklenen %q", d.metin, got, d.parca)
		}
	}
}

// Elle yazılmış hızlı eşleştirici ile genel düzenli ifade motoru aynı
// parçaları vermeli.
func TestHizliDesenMotorlaAyni(t *testing.T) {
	ifade, err := DuzIfadeDerle(BolmeDeseni)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range girdileriYukle(t) {
		var got []string
		ifade.TumEslesmeler(m, func(b, e int) { got = append(got, m[b:e]) })
		if want := OnBol(m); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: motor %q, hızlı yol %q", m, got, want)
		}
	}
}

// Birleştirmeler eski "a b" metin biçiminde de okunabilmeli.
func TestMetinBicimliBirlesmeler(t *testing.T) {
	_, a := kucukYukle(t)
	veri, err := os.ReadFile(kucukYol)
	if err != nil {
		t.Fatal(err)
	}
	var ham map[string]any
	if err := json.Unmarshal(veri, &ham); err != nil {
		t.Fatal(err)
	}
	model := ham["model"].(map[string]any)
	var metinler []string
	for _, m := range model["merges"].([]any) {
		c := m.([]any)
		metinler = append(metinler, c[0].(string)+" "+c[1].(string))
	}
	model["merges"] = metinler
	yeni, _ := json.Marshal(ham)
	p, err := Ayristir(yeni)
	if err != nil {
		t.Fatalf("metin biçimli birleştirmeler okunamadı: %v", err)
	}
	for _, k := range a.Kodlama {
		if got := p.Kodla(k.Metin); !esitInt(got, k.Idler) {
			t.Fatalf("metin biçimli birleştirmelerle Kodla(%q) = %v, beklenen %v", k.Metin, got, k.Idler)
		}
	}
}

func TestDesteklenmeyenYapilandirma(t *testing.T) {
	veri, err := os.ReadFile(kucukYol)
	if err != nil {
		t.Fatal(err)
	}
	degistir := func(f func(m map[string]any)) []byte {
		var ham map[string]any
		if err := json.Unmarshal(veri, &ham); err != nil {
			t.Fatal(err)
		}
		f(ham)
		b, _ := json.Marshal(ham)
		return b
	}
	// Her durum: değişiklik ve hata iletisinde geçmesi gereken bileşen/tür adı.
	durumlar := map[string]struct {
		f      func(m map[string]any)
		iceren string
	}{
		"normalizer": {func(m map[string]any) { m["normalizer"] = map[string]any{"type": "Benzersiz"} }, "normalleştirici (normalizer) türü \"Benzersiz\""},
		"ön bölücü": {func(m map[string]any) {
			pt := m["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
			pt[0] = map[string]any{"type": "UnicodeScripts"}
		}, "ön bölücü (pre_tokenizer) türü \"UnicodeScripts\""},
		"desen": {func(m map[string]any) {
			pt := m["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
			pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `(a)\1`}
		}, "geri başvuru"},
		"model türü": {func(m map[string]any) { m["model"].(map[string]any)["type"] = "Benzersiz" }, "model türü \"Benzersiz\""},
		"dropout":    {func(m map[string]any) { m["model"].(map[string]any)["dropout"] = 0.1 }, "dropout"},
		"çözücü":     {func(m map[string]any) { m["decoder"] = map[string]any{"type": "CTC"} }, "çözücü (decoder) türü \"CTC\""},
		"son işlem":  {func(m map[string]any) { m["post_processor"] = map[string]any{"type": "Yok"} }, "son işleyici (post_processor) türü \"Yok\""},
		"dolgu":      {func(m map[string]any) { m["padding"] = map[string]any{"length": 8} }, "padding"},
		"birleştirme": {func(m map[string]any) {
			mm := m["model"].(map[string]any)
			mm["merges"] = append(mm["merges"].([]any), []any{"yokyok", "zzz"})
		}, "sözlükte olmayan"},
	}
	for ad, d := range durumlar {
		_, err := Ayristir(degistir(d.f))
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", ad)
		} else if !strings.Contains(err.Error(), d.iceren) || !utf8.ValidString(err.Error()) {
			t.Errorf("%s: hata iletisi %q, %q içermeli", ad, err.Error(), d.iceren)
		}
	}
	if _, err := Ayristir(degistir(func(map[string]any) {})); err != nil {
		t.Fatalf("değiştirilmemiş dosya hata verdi: %v", err)
	}
}

func TestKayipliUTF8(t *testing.T) {
	durumlar := []struct {
		girdi []byte
		cikti string
	}{
		{[]byte("abc"), "abc"},
		{[]byte{0xE2, 0x82, 'a'}, "�a"},
		{[]byte{0xE2, 0x82}, "�"},
		{[]byte{0x80, 0x80}, "��"},
		{[]byte{0xC4}, "�"},
		{[]byte{0xC4, 0x9F}, "ğ"},
		{[]byte{0xED, 0xA0, 0x80}, "���"},
		{[]byte{0xF0, 0x9F, 0x98}, "�"},
		{[]byte{0xF0, 0x9F, 0x98, 0x80}, "😀"},
		{[]byte{0xF4, 0x90, 0x80, 0x80}, "����"},
		{[]byte{0xFF, 'x'}, "�x"},
	}
	for _, d := range durumlar {
		if got := kayipliUTF8(d.girdi); got != d.cikti {
			t.Errorf("kayipliUTF8(% x) = %q, beklenen %q", d.girdi, got, d.cikti)
		}
	}
}

func TestGecersizKimlikler(t *testing.T) {
	p, _ := kucukYukle(t)
	if got := p.Coz([]int{-1, -100, 1 << 40}, false); got != "" {
		t.Fatalf("geçersiz kimlikler atlanmalıydı, alınan %q", got)
	}
}

func TestTopluVeParalel(t *testing.T) {
	p, a := kucukYukle(t)
	var metinler []string
	var beklenen [][]int
	for tekrar := 0; tekrar < 5; tekrar++ {
		for _, k := range a.Kodlama {
			metinler = append(metinler, k.Metin)
			beklenen = append(beklenen, k.Idler)
		}
	}
	for _, isci := range []int{1, 3, 0} {
		got := p.TopluKodla(metinler, Secenekler{}, isci)
		for i := range got {
			if !esitInt(got[i], beklenen[i]) {
				t.Fatalf("isci=%d: TopluKodla[%d] uyuşmadı", isci, i)
			}
		}
		cozulen := p.TopluCoz(got, false, isci)
		for i := range cozulen {
			if cozulen[i] != p.Coz(got[i], false) {
				t.Fatalf("isci=%d: TopluCoz[%d] uyuşmadı", isci, i)
			}
		}
		g32 := p.TopluKodla32(metinler[:50], Secenekler{}, isci)
		for i := range g32 {
			if !esitInt(p.KodlaEkle(nil, metinler[i], true), beklenen[i]) || p.Coz32(g32[i], false) != cozulen[i] {
				t.Fatalf("isci=%d: TopluKodla32[%d] uyuşmadı", isci, i)
			}
		}
	}
	eot, ok := p.TokenKimligi("<|endoftext|>")
	if !ok {
		t.Fatal("<|endoftext|> bulunamadı")
	}
	birlesik := p.BelgeleriBirlestir([]string{"bir", "iki"}, eot, 0)
	beklenenB := append(append(append(p.Kodla("bir"), eot), p.Kodla("iki")...), eot)
	got := make([]int, len(birlesik))
	for i, v := range birlesik {
		got[i] = int(v)
	}
	if !esitInt(got, beklenenB) {
		t.Fatalf("BelgeleriBirlestir = %v, beklenen %v", got, beklenenB)
	}
}

// Önbellek kapalıyken de sonuçlar aynı olmalı; uzun parçalar yığın yolunu sınar.
func TestOnbelleksizVeUzun(t *testing.T) {
	p, err := Yukle(kucukYol)
	if err != nil {
		t.Fatal(err)
	}
	_, a := kucukYukle(t)
	p.OnbellekKapasitesiAyarla(0)
	for _, k := range a.Kodlama {
		if got := p.Kodla(k.Metin); !esitInt(got, k.Idler) {
			t.Fatalf("önbelleksiz Kodla(%q) uyuşmadı", k.Metin)
		}
	}
	// Kısa ve uzun birleştirme yolları aynı sonucu vermeli.
	bm := p.model.(*bpeModel)
	for _, k := range a.Kodlama {
		for _, s := range OnBol(k.Metin) {
			var sem []int32
			for i := 0; i < len(s); i++ {
				if id := bm.baytKarID[s[i]]; id >= 0 {
					sem = append(sem, id)
				}
			}
			k1 := bm.birlestirKisa(append([]int32(nil), sem...))
			if len(sem) == 0 {
				continue
			}
			k2 := bm.birlestirUzun(append([]int32(nil), sem...))
			if !reflect.DeepEqual(k1, k2) {
				t.Fatalf("kısa/uzun birleştirme farklı: %q: %v / %v", s, k1, k2)
			}
		}
	}
}

// ============================================================================
// Gerçek parçalayıcıya karşı sınama (isteğe bağlı)
// ============================================================================
//
// TR_PARCALAYICI_YOLU ayarlıysa çalışır. TR_PARCALAYICI_ALTIN ayrıca ayarlıysa
// (her satırı {"metin":..., "idler":[...]} olan JSONL dosyası), kodlama ve
// çözme sonuçları bu altın çıktılarla birebir karşılaştırılır.

func TestGercekParcalayici(t *testing.T) {
	yol := os.Getenv("TR_PARCALAYICI_YOLU")
	if yol == "" {
		t.Skip("TR_PARCALAYICI_YOLU ayarlı değil")
	}
	p, err := Yukle(yol)
	if err != nil {
		t.Fatalf("gerçek parçalayıcı yüklenemedi: %v", err)
	}
	t.Logf("sözlük: %d, birleştirme: %d", p.SozlukBoyutu(), p.BirlesmeSayisi())
	// Kayıpsız (normalleştiricisiz, bayt düzeyi) parçalayıcıda gidiş-dönüş.
	if p.baytCozucu && p.normallestirici == nil && p.sonIsleyici == nil {
		for _, m := range girdileriYukle(t) {
			if got := p.Coz(p.Kodla(m), false); got != m {
				t.Errorf("gidiş-dönüş bozuk: %q -> %q", m, got)
			}
		}
	}
	altinYol := os.Getenv("TR_PARCALAYICI_ALTIN")
	if altinYol == "" {
		return
	}
	veri, err := os.ReadFile(altinYol)
	if err != nil {
		t.Fatal(err)
	}
	// Satır biçimi: {"metin": metin ya da null, "idler": [...], "cozme": metin}
	// metin null değilse Kodla(metin) == idler; her satırda Coz(idler) == cozme.
	type kayit struct {
		Metin *string `json:"metin"`
		Idler []int   `json:"idler"`
		Cozme string  `json:"cozme"`
		Atla  bool    `json:"ozel_atla"`
	}
	var kayitlar []kayit
	var metinler []string
	var metinKayit []int
	for _, satir := range strings.Split(string(veri), "\n") {
		if strings.TrimSpace(satir) == "" {
			continue
		}
		var k kayit
		if err := json.Unmarshal([]byte(satir), &k); err != nil {
			t.Fatal(err)
		}
		if k.Metin != nil {
			metinler = append(metinler, *k.Metin)
			metinKayit = append(metinKayit, len(kayitlar))
		}
		kayitlar = append(kayitlar, k)
	}
	got := p.TopluKodla(metinler, Secenekler{}, 0)
	eslesen, hatali := 0, 0
	toplamBayt := 0
	for i := range metinler {
		toplamBayt += len(metinler[i])
		b := kayitlar[metinKayit[i]].Idler
		if esitInt(got[i], b) {
			eslesen++
		} else if hatali++; hatali <= 10 {
			t.Errorf("uyuşmazlık: %q\n alınan: %v\nbeklenen: %v", metinler[i], got[i], b)
		}
	}
	cozEslesen := 0
	for _, k := range kayitlar {
		if p.Coz(k.Idler, k.Atla) == k.Cozme {
			cozEslesen++
		} else if hatali++; hatali <= 10 {
			t.Errorf("çözme uyuşmazlığı: %v\n alınan: %q\nbeklenen: %q", k.Idler, p.Coz(k.Idler, k.Atla), k.Cozme)
		}
	}
	t.Logf("kodlama: %d / %d eşleşti (%d bayt); çözme: %d / %d eşleşti",
		eslesen, len(metinler), toplamBayt, cozEslesen, len(kayitlar))
	if eslesen != len(metinler) || cozEslesen != len(kayitlar) {
		t.Fatalf("%d kodlama, %d çözme uyuşmadı", len(metinler)-eslesen, len(kayitlar)-cozEslesen)
	}
}

// ============================================================================
// Hız ölçümleri
// ============================================================================

// olcumMetni ölçüm metnini verir: TR_PARCALAYICI_OLCUM_METNI ayarlıysa o
// dosyanın içeriği, değilse örnek metnin ~1 MB'a tekrarlanmış hâli.
func olcumMetni(tb testing.TB) string {
	if yol := os.Getenv("TR_PARCALAYICI_OLCUM_METNI"); yol != "" {
		veri, err := os.ReadFile(yol)
		if err != nil {
			tb.Fatal(err)
		}
		return string(veri)
	}
	veri, err := os.ReadFile("testdata/ornek_metin.txt")
	if err != nil {
		tb.Fatal(err)
	}
	// ~1 MB'lık metin; her tekrarda bir sayı katarak önbelleğin tamamen
	// isabet etmesini biraz bozarız.
	var sb strings.Builder
	for i := 0; sb.Len() < 1<<20; i++ {
		sb.Write(veri)
		fmt.Fprintf(&sb, " %d\n", i)
	}
	return sb.String()
}

func olcumParcalayicisi(b *testing.B) *Parcalayici {
	if yol := os.Getenv("TR_PARCALAYICI_YOLU"); yol != "" {
		p, err := Yukle(yol)
		if err != nil {
			b.Fatal(err)
		}
		return p
	}
	p, err := Yukle(filepath.FromSlash(kucukYol))
	if err != nil {
		b.Fatal(err)
	}
	return p
}

func BenchmarkKodla(b *testing.B) {
	p := olcumParcalayicisi(b)
	metin := olcumMetni(b)
	b.SetBytes(int64(len(metin)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Kodla32(nil, metin, Secenekler{})
	}
}

func BenchmarkKodlaOnbelleksiz(b *testing.B) {
	p := olcumParcalayicisi(b)
	p.OnbellekKapasitesiAyarla(0)
	metin := olcumMetni(b)
	b.SetBytes(int64(len(metin)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Kodla32(nil, metin, Secenekler{})
	}
}

func BenchmarkTopluKodlaParalel(b *testing.B) {
	p := olcumParcalayicisi(b)
	metin := olcumMetni(b)
	satirlar := strings.SplitAfter(metin, "\n")
	b.SetBytes(int64(len(metin)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.TopluKodla32(satirlar, Secenekler{}, 0)
	}
}

func BenchmarkCoz(b *testing.B) {
	p := olcumParcalayicisi(b)
	metin := olcumMetni(b)
	ids := p.Kodla(metin)
	b.SetBytes(int64(len(metin)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Coz(ids, false)
	}
}
