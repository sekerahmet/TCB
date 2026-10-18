package parcalayici

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ============================================================================
// Bileşen arayüzleri ve kayıt tablosu
// ============================================================================
//
// Bir parçalayıcı birbirinden bağımsız bileşenlerin zinciridir:
//
//	metin -> [ek token ayırma] -> Normallestirici -> OnBolucu -> Model -> SonIsleyici -> kimlikler
//	kimlikler -> (özel token atlama) -> Cozucu -> metin
//
// Her bileşen türü tokenizer.json'daki "type" alanıyla seçilir. Türler bir
// kayıt tablosunda tutulur; yeni bir tür ...Kaydet işlevleriyle eklenebilir.
// Tabloda olmayan bir türle karşılaşılırsa hangi bileşenin hangi türünün
// desteklenmediğini söyleyen bir hata döner.

// Normallestirici ön bölmeden önce metni dönüştürür (ör. NFC, küçük harf).
type Normallestirici interface {
	Normallestir(metin string) string
}

// Parca ön bölme sırasında bir metin parçasıdır.
type Parca struct {
	// Metin parçanın içeriği. baytDuzeyi doğruysa ham baytlardır ve bayt
	// düzeyi karakterlere çevrilmesi (verim için) modele bırakılmıştır.
	Metin string
	// Ilk parça özgün metnin en başından (0. konumdan) başlıyorsa doğrudur.
	Ilk bool

	baytDuzeyi bool
}

// Icerik parçanın asıl metnini verir (gerekirse bayt düzeyi gösterime çevirir).
func (p Parca) Icerik() string {
	if p.baytDuzeyi {
		return baytlariKarakterle([]byte(p.Metin))
	}
	return p.Metin
}

// OnBolucu parçaları daha küçük parçalara ayırır ya da dönüştürür. Sonuçlar
// dst'nin sonuna eklenir. Boş parçalar üretilmemelidir.
type OnBolucu interface {
	OnBol(dst, parcalar []Parca) []Parca
}

// Model bir ön bölme parçasını token kimliklerine çevirir.
type Model interface {
	// Kodla parçanın kimliklerini dst'ye ekler.
	Kodla(dst []int32, p Parca) []int32
	// Sozluk token -> kimlik eşlemesini verir (salt okunur).
	Sozluk() map[string]int32
}

// Kodlama bir kodlama sonucudur.
type Kodlama struct {
	Idler     []int32 // token kimlikleri
	TurIdleri []int32 // dizi türü kimlikleri (çift girdide 0/1)
	OzelMaske []bool  // son işleyicinin eklediği özel tokenlar
}

// SonIsleyici modelin çıktısına özel tokenlar (ör. BOS/EOS) ekler.
type SonIsleyici interface {
	// Isle tek (b == nil) ya da çift girdinin kodlamasını birleştirir.
	Isle(a, b *Kodlama, ozelEkle bool) *Kodlama
}

// Cozucu tokenların metin gösterimlerini son metne çevirir. Zincirdeki her
// adım token listesini dönüştürür; sonuç parçaları bitiştirilerek birleştirilir.
type Cozucu interface {
	CozZincir(tokenlar []string) []string
}

// Kurucu, bileşenin JSON tanımından bileşeni kuran işlevdir.
type (
	NormallestiriciKurucu func(ham json.RawMessage) (Normallestirici, error)
	OnBolucuKurucu        func(ham json.RawMessage) (OnBolucu, error)
	ModelKurucu           func(ham json.RawMessage) (Model, error)
	SonIsleyiciKurucu     func(ham json.RawMessage, sozluk func(string) (int32, bool)) (SonIsleyici, error)
	CozucuKurucu          func(ham json.RawMessage) (Cozucu, error)
)

var (
	kayitKilit         sync.RWMutex
	normallestiriciler = map[string]NormallestiriciKurucu{}
	onBoluculer        = map[string]OnBolucuKurucu{}
	modeller           = map[string]ModelKurucu{}
	sonIsleyiciler     = map[string]SonIsleyiciKurucu{}
	cozuculer          = map[string]CozucuKurucu{}
)

// NormallestiriciKaydet bir normalleştirici türünü kayıt tablosuna ekler.
func NormallestiriciKaydet(tur string, k NormallestiriciKurucu) {
	kayitKilit.Lock()
	normallestiriciler[tur] = k
	kayitKilit.Unlock()
}

// OnBolucuKaydet bir ön bölücü türünü kayıt tablosuna ekler.
func OnBolucuKaydet(tur string, k OnBolucuKurucu) {
	kayitKilit.Lock()
	onBoluculer[tur] = k
	kayitKilit.Unlock()
}

// ModelKaydet bir model türünü kayıt tablosuna ekler.
func ModelKaydet(tur string, k ModelKurucu) {
	kayitKilit.Lock()
	modeller[tur] = k
	kayitKilit.Unlock()
}

// SonIsleyiciKaydet bir son işleyici türünü kayıt tablosuna ekler.
func SonIsleyiciKaydet(tur string, k SonIsleyiciKurucu) {
	kayitKilit.Lock()
	sonIsleyiciler[tur] = k
	kayitKilit.Unlock()
}

// CozucuKaydet bir çözücü türünü kayıt tablosuna ekler.
func CozucuKaydet(tur string, k CozucuKurucu) {
	kayitKilit.Lock()
	cozuculer[tur] = k
	kayitKilit.Unlock()
}

func turler[T any](m map[string]T) string {
	var ad []string
	for k := range m {
		ad = append(ad, k)
	}
	sort.Strings(ad)
	return strings.Join(ad, ", ")
}

// turOku bileşen tanımının "type" alanını okur.
func turOku(ham json.RawMessage, bilesen string) (string, error) {
	var t struct {
		Type *string `json:"type"`
	}
	if err := json.Unmarshal(ham, &t); err != nil {
		return "", fmt.Errorf("%s tanımı çözümlenemedi: %w", bilesen, err)
	}
	if t.Type == nil {
		return "", fmt.Errorf("%s tanımında \"type\" alanı yok", bilesen)
	}
	return *t.Type, nil
}

func desteklenmiyor(bilesen, tur, desteklenen string) error {
	return fmt.Errorf("desteklenmeyen %s türü %q (desteklenenler: %s)", bilesen, tur, desteklenen)
}

// NormallestiriciKur JSON tanımından normalleştirici kurar (null için nil).
func NormallestiriciKur(ham json.RawMessage) (Normallestirici, error) {
	if bosMu(ham) {
		return nil, nil
	}
	tur, err := turOku(ham, "normalleştirici")
	if err != nil {
		return nil, err
	}
	kayitKilit.RLock()
	k, ok := normallestiriciler[tur]
	liste := turler(normallestiriciler)
	kayitKilit.RUnlock()
	if !ok {
		return nil, desteklenmiyor("normalleştirici (normalizer)", tur, liste)
	}
	n, err := k(ham)
	if err != nil {
		return nil, fmt.Errorf("normalleştirici %s: %w", tur, err)
	}
	return n, nil
}

// OnBolucuKur JSON tanımından ön bölücü kurar (null için nil).
func OnBolucuKur(ham json.RawMessage) (OnBolucu, error) {
	if bosMu(ham) {
		return nil, nil
	}
	tur, err := turOku(ham, "ön bölücü")
	if err != nil {
		return nil, err
	}
	kayitKilit.RLock()
	k, ok := onBoluculer[tur]
	liste := turler(onBoluculer)
	kayitKilit.RUnlock()
	if !ok {
		return nil, desteklenmiyor("ön bölücü (pre_tokenizer)", tur, liste)
	}
	o, err := k(ham)
	if err != nil {
		return nil, fmt.Errorf("ön bölücü %s: %w", tur, err)
	}
	return o, nil
}

// ModelKur JSON tanımından model kurar. "type" alanı yoksa tür alanlardan
// çıkarılır (eski dosya biçimleri).
func ModelKur(ham json.RawMessage) (Model, error) {
	if bosMu(ham) {
		return nil, fmt.Errorf("parçalayıcı dosyasında model yok")
	}
	var alanlar map[string]json.RawMessage
	if err := json.Unmarshal(ham, &alanlar); err != nil {
		return nil, fmt.Errorf("model tanımı çözümlenemedi: %w", err)
	}
	var tur string
	if t, ok := alanlar["type"]; ok {
		if err := json.Unmarshal(t, &tur); err != nil {
			return nil, fmt.Errorf("model türü okunamadı: %w", err)
		}
	} else {
		_, merges := alanlar["merges"]
		_, maks := alanlar["max_input_chars_per_word"]
		v := strings.TrimSpace(string(alanlar["vocab"]))
		switch {
		case merges:
			tur = "BPE"
		case maks:
			tur = "WordPiece"
		case strings.HasPrefix(v, "["):
			tur = "Unigram"
		default:
			tur = "WordLevel"
		}
	}
	kayitKilit.RLock()
	k, ok := modeller[tur]
	liste := turler(modeller)
	kayitKilit.RUnlock()
	if !ok {
		return nil, desteklenmiyor("model", tur, liste)
	}
	m, err := k(ham)
	if err != nil {
		return nil, fmt.Errorf("model %s: %w", tur, err)
	}
	return m, nil
}

// SonIsleyiciKur JSON tanımından son işleyici kurar (null için nil).
func SonIsleyiciKur(ham json.RawMessage, sozluk func(string) (int32, bool)) (SonIsleyici, error) {
	if bosMu(ham) {
		return nil, nil
	}
	tur, err := turOku(ham, "son işleyici")
	if err != nil {
		return nil, err
	}
	kayitKilit.RLock()
	k, ok := sonIsleyiciler[tur]
	liste := turler(sonIsleyiciler)
	kayitKilit.RUnlock()
	if !ok {
		return nil, desteklenmiyor("son işleyici (post_processor)", tur, liste)
	}
	s, err := k(ham, sozluk)
	if err != nil {
		return nil, fmt.Errorf("son işleyici %s: %w", tur, err)
	}
	return s, nil
}

// CozucuKur JSON tanımından çözücü kurar (null için nil).
func CozucuKur(ham json.RawMessage) (Cozucu, error) {
	if bosMu(ham) {
		return nil, nil
	}
	tur, err := turOku(ham, "çözücü")
	if err != nil {
		return nil, err
	}
	kayitKilit.RLock()
	k, ok := cozuculer[tur]
	liste := turler(cozuculer)
	kayitKilit.RUnlock()
	if !ok {
		return nil, desteklenmiyor("çözücü (decoder)", tur, liste)
	}
	c, err := k(ham)
	if err != nil {
		return nil, fmt.Errorf("çözücü %s: %w", tur, err)
	}
	return c, nil
}

// katiCoz JSON'u yapıya çözer; bilinmeyen alanlara izin verir.
func katiCoz(ham json.RawMessage, hedef any) error {
	if err := json.Unmarshal(ham, hedef); err != nil {
		return fmt.Errorf("tanım çözümlenemedi: %w", err)
	}
	return nil
}

// desenBicimi Split/Replace desenidir: {"String": ...} ya da {"Regex": ...}.
type desenBicimi struct {
	String *string `json:"String"`
	Regex  *string `json:"Regex"`
}
