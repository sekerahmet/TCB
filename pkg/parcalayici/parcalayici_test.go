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

const kucukYol = "testdata/kucuk_parcalayici.json"

type altinKodlama struct {
	Metin    string `json:"metin"`
	Idler    []int  `json:"idler"`
	IdlerDuz []int  `json:"idler_duz"`
}

type altinCozme struct {
	Idler    []int  `json:"idler"`
	OzelAtla bool   `json:"ozel_atla"`
	Metin    string `json:"metin"`
}

type altinVeri struct {
	SozlukBoyutu int            `json:"sozluk_boyutu"`
	Kodlama      []altinKodlama `json:"kodlama"`
	Cozme        []altinCozme   `json:"cozme"`
}

var (
	kucukBir  sync.Once
	kucukP    *Parcalayici
	kucukHata error
	altin     altinVeri
	altinHata error
)

func kucukYukle(t testing.TB) (*Parcalayici, *altinVeri) {
	t.Helper()
	kucukBir.Do(func() {
		kucukP, kucukHata = Yukle(kucukYol)
		var veri []byte
		veri, altinHata = os.ReadFile("testdata/altin.json")
		if altinHata == nil {
			altinHata = json.Unmarshal(veri, &altin)
		}
	})
	if kucukHata != nil {
		t.Fatalf("küçük parçalayıcı yüklenemedi: %v", kucukHata)
	}
	if altinHata != nil {
		t.Fatalf("altın veri okunamadı: %v", altinHata)
	}
	return kucukP, &altin
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

func TestAltinKodlama(t *testing.T) {
	p, a := kucukYukle(t)
	if len(a.Kodlama) < 100 {
		t.Fatalf("altın veride çok az örnek: %d", len(a.Kodlama))
	}
	hata := 0
	for _, k := range a.Kodlama {
		if got := p.Kodla(k.Metin); !esitInt(got, k.Idler) {
			hata++
			if hata <= 10 {
				t.Errorf("Kodla(%q)\n alınan: %v\nbeklenen: %v", k.Metin, got, k.Idler)
			}
		}
		if got := p.KodlaDuz(k.Metin); !esitInt(got, k.IdlerDuz) {
			hata++
			if hata <= 10 {
				t.Errorf("KodlaDuz(%q)\n alınan: %v\nbeklenen: %v", k.Metin, got, k.IdlerDuz)
			}
		}
	}
	if hata > 0 {
		t.Fatalf("%d / %d kodlama uyuşmadı", hata, 2*len(a.Kodlama))
	}
}

func TestAltinCozme(t *testing.T) {
	p, a := kucukYukle(t)
	hata := 0
	for _, c := range a.Cozme {
		if got := p.Coz(c.Idler, c.OzelAtla); got != c.Metin {
			hata++
			if hata <= 10 {
				t.Errorf("Coz(%v, %v)\n alınan: %q\nbeklenen: %q", c.Idler, c.OzelAtla, got, c.Metin)
			}
		}
	}
	if hata > 0 {
		t.Fatalf("%d / %d çözme uyuşmadı", hata, len(a.Cozme))
	}
}

func TestSozlukBoyutu(t *testing.T) {
	p, a := kucukYukle(t)
	if p.SozlukBoyutu() != a.SozlukBoyutu {
		t.Fatalf("sözlük boyutu %d, beklenen %d", p.SozlukBoyutu(), a.SozlukBoyutu)
	}
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
	_, a := kucukYukle(t)
	for _, k := range a.Kodlama {
		if got := strings.Join(OnBol(k.Metin), ""); got != k.Metin {
			t.Errorf("ön bölme metni kapsamıyor: %q -> %q", k.Metin, got)
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
	durumlar := map[string]func(m map[string]any){
		"normalizer": func(m map[string]any) { m["normalizer"] = map[string]any{"type": "NFC"} },
		"desen": func(m map[string]any) {
			pt := m["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
			pt[0].(map[string]any)["pattern"] = map[string]any{"Regex": `\w+`}
		},
		"önek boşluğu": func(m map[string]any) {
			pt := m["pre_tokenizer"].(map[string]any)["pretokenizers"].([]any)
			pt[1].(map[string]any)["add_prefix_space"] = true
		},
		"model türü":  func(m map[string]any) { m["model"].(map[string]any)["type"] = "WordPiece" },
		"bayt yedeği": func(m map[string]any) { m["model"].(map[string]any)["byte_fallback"] = true },
		"dropout":     func(m map[string]any) { m["model"].(map[string]any)["dropout"] = 0.1 },
		"çözücü":      func(m map[string]any) { m["decoder"] = map[string]any{"type": "WordPiece"} },
		"son işlem":   func(m map[string]any) { m["post_processor"] = map[string]any{"type": "TemplateProcessing"} },
		"dolgu":       func(m map[string]any) { m["padding"] = map[string]any{"length": 8} },
		"birleştirme": func(m map[string]any) {
			mm := m["model"].(map[string]any)
			mm["merges"] = append(mm["merges"].([]any), []any{"yokyok", "zzz"})
		},
	}
	for ad, f := range durumlar {
		if _, err := Ayristir(degistir(f)); err == nil {
			t.Errorf("%s: hata bekleniyordu", ad)
		} else if !utf8.ValidString(err.Error()) || err.Error() == "" {
			t.Errorf("%s: geçersiz hata iletisi", ad)
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
		got := p.TopluKodla(metinler, true, isci)
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
		g32 := p.TopluKodla32(metinler[:50], true, isci)
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
	for _, k := range a.Kodlama {
		for _, s := range OnBol(k.Metin) {
			var sem []int32
			for i := 0; i < len(s); i++ {
				if id := p.baytID[s[i]]; id >= 0 {
					sem = append(sem, id)
				}
			}
			k1 := p.birlestirKisa(append([]int32(nil), sem...))
			if len(sem) == 0 {
				continue
			}
			k2 := p.birlestirUzun(append([]int32(nil), sem...))
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
	_, a := kucukYukle(t)
	for _, k := range a.Kodlama {
		if got := p.Coz(p.Kodla(k.Metin), false); got != k.Metin {
			t.Errorf("gidiş-dönüş bozuk: %q -> %q", k.Metin, got)
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
	got := p.TopluKodla(metinler, true, 0)
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
		p.Kodla32(nil, metin, true)
	}
}

func BenchmarkKodlaOnbelleksiz(b *testing.B) {
	p := olcumParcalayicisi(b)
	p.OnbellekKapasitesiAyarla(0)
	metin := olcumMetni(b)
	b.SetBytes(int64(len(metin)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Kodla32(nil, metin, true)
	}
}

func BenchmarkTopluKodlaParalel(b *testing.B) {
	p := olcumParcalayicisi(b)
	metin := olcumMetni(b)
	satirlar := strings.SplitAfter(metin, "\n")
	b.SetBytes(int64(len(metin)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.TopluKodla32(satirlar, true, 0)
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
