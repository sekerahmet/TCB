package tokenize

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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

// Normalizer ön bölmeden önce metni dönüştürür (ör. NFC, küçük harf).
type Normalizer interface {
	Normalize(text string) string
}

// Piece ön bölme sırasında bir metin parçasıdır.
type Piece struct {
	// Text parçanın içeriği. baytDuzeyi doğruysa ham baytlardır ve bayt
	// düzeyi karakterlere çevrilmesi (verim için) modele bırakılmıştır.
	Text string
	// Start parça özgün metnin en başından (0. konumdan) başlıyorsa doğrudur.
	Start bool

	byteLevel bool
	// budget, parçanın ait olduğu kodlama çağrısının düzenli ifade adım
	// bütçesidir (nil: her arama kendi bütçesini açar).
	budget *reBudget
}

// Content parçanın asıl metnini verir (gerekirse bayt düzeyi gösterime çevirir).
func (p Piece) Content() string {
	if p.byteLevel {
		return bytesToChars([]byte(p.Text))
	}
	return p.Text
}

// PreTokenizer parçaları daha küçük parçalara ayırır ya da dönüştürür. Sonuçlar
// dst'nin sonuna eklenir. Boş parçalar üretilmemelidir.
type PreTokenizer interface {
	PreTokenize(dst, pieces []Piece) []Piece
}

// Model bir ön bölme parçasını token kimliklerine çevirir.
type Model interface {
	// Encode parçanın kimliklerini dst'ye ekler.
	Encode(dst []int32, p Piece) []int32
	// Vocab token -> kimlik eşlemesini verir (salt okunur).
	Vocab() map[string]int32
}

// Encoding bir kodlama sonucudur.
type Encoding struct {
	IDs         []int32 // token kimlikleri
	TypeIDs     []int32 // dizi türü kimlikleri (çift girdide 0/1)
	SpecialMask []bool  // son işleyicinin eklediği özel tokenlar
}

// PostProcessor modelin çıktısına özel tokenlar (ör. BOS/EOS) ekler.
type PostProcessor interface {
	// Process tek (b == nil) ya da çift girdinin kodlamasını birleştirir.
	Process(a, b *Encoding, addSpecial bool) *Encoding
}

// Decoder tokenların metin gösterimlerini son metne çevirir. Zincirdeki her
// adım token listesini dönüştürür; sonuç parçaları bitiştirilerek birleştirilir.
type Decoder interface {
	DecodeChain(tokenList []string) []string
}

// Kurucu, bileşenin JSON tanımından bileşeni kuran işlevdir.
type (
	NormalizerFactory    func(raw json.RawMessage) (Normalizer, error)
	PreTokenizerFactory  func(raw json.RawMessage) (PreTokenizer, error)
	ModelFactory         func(raw json.RawMessage) (Model, error)
	PostProcessorFactory func(raw json.RawMessage, vocab func(string) (int32, bool)) (PostProcessor, error)
	DecoderFactory       func(raw json.RawMessage) (Decoder, error)
)

var (
	registryLock   sync.RWMutex
	normalizers    = map[string]NormalizerFactory{}
	preTokenizers  = map[string]PreTokenizerFactory{}
	models         = map[string]ModelFactory{}
	postProcessors = map[string]PostProcessorFactory{}
	decoders       = map[string]DecoderFactory{}
)

// RegisterNormalizer bir normalleştirici türünü kayıt tablosuna ekler.
func RegisterNormalizer(kind string, k NormalizerFactory) {
	registryLock.Lock()
	normalizers[kind] = k
	registryLock.Unlock()
}

// RegisterPreTokenizer bir ön bölücü türünü kayıt tablosuna ekler.
func RegisterPreTokenizer(kind string, k PreTokenizerFactory) {
	registryLock.Lock()
	preTokenizers[kind] = k
	registryLock.Unlock()
}

// RegisterModel bir model türünü kayıt tablosuna ekler.
func RegisterModel(kind string, k ModelFactory) {
	registryLock.Lock()
	models[kind] = k
	registryLock.Unlock()
}

// RegisterPostProcessor bir son işleyici türünü kayıt tablosuna ekler.
func RegisterPostProcessor(kind string, k PostProcessorFactory) {
	registryLock.Lock()
	postProcessors[kind] = k
	registryLock.Unlock()
}

// RegisterDecoder bir çözücü türünü kayıt tablosuna ekler.
func RegisterDecoder(kind string, k DecoderFactory) {
	registryLock.Lock()
	decoders[kind] = k
	registryLock.Unlock()
}

func kinds[T any](m map[string]T) string {
	var ad []string
	for k := range m {
		ad = append(ad, k)
	}
	sort.Strings(ad)
	return strings.Join(ad, ", ")
}

// readKind bileşen tanımının "type" alanını okur.
func readKind(raw json.RawMessage, component string) (string, error) {
	var t struct {
		Type *string `json:"type"`
	}
	if err := exactUnmarshal(raw, &t); err != nil {
		return "", messages.New(messages.H5200, component, err)
	}
	if t.Type == nil {
		return "", messages.New(messages.H5201, component)
	}
	return *t.Type, nil
}

func unsupported(component, kind, supported string) error {
	return messages.New(messages.H5202, component, kind, supported)
}

// NewNormalizer JSON tanımından normalleştirici kurar (null için nil).
func NewNormalizer(raw json.RawMessage) (Normalizer, error) {
	if isEmpty(raw) {
		return nil, nil
	}
	kind, err := readKind(raw, messages.Text(messages.H5280))
	if err != nil {
		return nil, err
	}
	registryLock.RLock()
	k, ok := normalizers[kind]
	list := kinds(normalizers)
	registryLock.RUnlock()
	if !ok {
		return nil, unsupported(messages.Text(messages.H5281), kind, list)
	}
	n, err := k(raw)
	if err != nil {
		return nil, messages.New(messages.H5203, kind, err)
	}
	return n, nil
}

// NewPreTokenizer JSON tanımından ön bölücü kurar (null için nil).
func NewPreTokenizer(raw json.RawMessage) (PreTokenizer, error) {
	if isEmpty(raw) {
		return nil, nil
	}
	kind, err := readKind(raw, messages.Text(messages.H5282))
	if err != nil {
		return nil, err
	}
	registryLock.RLock()
	k, ok := preTokenizers[kind]
	list := kinds(preTokenizers)
	registryLock.RUnlock()
	if !ok {
		return nil, unsupported(messages.Text(messages.H5283), kind, list)
	}
	o, err := k(raw)
	if err != nil {
		return nil, messages.New(messages.H5204, kind, err)
	}
	return o, nil
}

// NewModel JSON tanımından model kurar. "type" alanı yoksa tür alanlardan
// çıkarılır (eski dosya biçimleri).
func NewModel(raw json.RawMessage) (Model, error) {
	if isEmpty(raw) {
		return nil, messages.New(messages.H5205)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, messages.New(messages.H5206, err)
	}
	var kind string
	if t, ok := fields["type"]; ok {
		if err := json.Unmarshal(t, &kind); err != nil {
			return nil, messages.New(messages.H5207, err)
		}
	} else {
		_, merges := fields["merges"]
		_, maxv := fields["max_input_chars_per_word"]
		v := strings.TrimSpace(string(fields["vocab"]))
		switch {
		case merges:
			kind = "BPE"
		case maxv:
			kind = "WordPiece"
		case strings.HasPrefix(v, "["):
			kind = "Unigram"
		default:
			kind = "WordLevel"
		}
	}
	registryLock.RLock()
	k, ok := models[kind]
	list := kinds(models)
	registryLock.RUnlock()
	if !ok {
		return nil, unsupported("model", kind, list)
	}
	m, err := k(raw)
	if err != nil {
		return nil, messages.New(messages.H5208, kind, err)
	}
	return m, nil
}

// NewPostProcessor JSON tanımından son işleyici kurar (null için nil).
func NewPostProcessor(raw json.RawMessage, vocab func(string) (int32, bool)) (PostProcessor, error) {
	if isEmpty(raw) {
		return nil, nil
	}
	kind, err := readKind(raw, messages.Text(messages.H5284))
	if err != nil {
		return nil, err
	}
	registryLock.RLock()
	k, ok := postProcessors[kind]
	list := kinds(postProcessors)
	registryLock.RUnlock()
	if !ok {
		return nil, unsupported(messages.Text(messages.H5285), kind, list)
	}
	s, err := k(raw, vocab)
	if err != nil {
		return nil, messages.New(messages.H5209, kind, err)
	}
	return s, nil
}

// NewDecoder JSON tanımından çözücü kurar (null için nil).
func NewDecoder(raw json.RawMessage) (Decoder, error) {
	if isEmpty(raw) {
		return nil, nil
	}
	kind, err := readKind(raw, messages.Text(messages.H5286))
	if err != nil {
		return nil, err
	}
	registryLock.RLock()
	k, ok := decoders[kind]
	list := kinds(decoders)
	registryLock.RUnlock()
	if !ok {
		return nil, unsupported(messages.Text(messages.H5287), kind, list)
	}
	c, err := k(raw)
	if err != nil {
		return nil, messages.New(messages.H5210, kind, err)
	}
	return c, nil
}

// strictDecode JSON'u yapıya çözer; bilinmeyen alanlara izin verir, yanlış harfli ya da
// yinelenen anahtarı reddeder (bkz. exactUnmarshal).
func strictDecode(raw json.RawMessage, target any) error {
	if err := exactUnmarshal(raw, target); err != nil {
		return messages.New(messages.H5211, err)
	}
	return nil
}

// patternForm Split/Replace desenidir: {"String": ...} ya da {"Regex": ...}.
type patternForm struct {
	String *string `json:"String"`
	Regex  *string `json:"Regex"`
}
