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

// buildComponent kayıt defterinden (registry) bileşen kurar: null için sıfır değer (nil), "type"
// alanı yoksa, kayıtlı değilse ya da kurucu hata verirse ilgili ileti. nameCode bileşenin adı,
// kindCode desteklenen türleri sayan ileti için adıdır; failCode kurucu hatasını sarar.
func buildComponent[F, T any](raw json.RawMessage, registry map[string]F, nameCode, kindCode, failCode messages.Code, build func(F) (T, error)) (T, error) {
	var zero T
	if isEmpty(raw) {
		return zero, nil
	}
	kind, err := readKind(raw, messages.Text(nameCode))
	if err != nil {
		return zero, err
	}
	registryLock.RLock()
	k, ok := registry[kind]
	list := kinds(registry)
	registryLock.RUnlock()
	if !ok {
		return zero, unsupported(messages.Text(kindCode), kind, list)
	}
	v, err := build(k)
	if err != nil {
		return zero, messages.New(failCode, kind, err)
	}
	return v, nil
}

// buildSequence "Sequence" bileşenlerinin öğelerini build ile kurar (null öğeler atlanır); hata
// H5213 ile öğenin sırasını bildirir. Her bileşen kendi alanını strictDecode ile çözer.
func buildSequence[T comparable](items []json.RawMessage, build func(json.RawMessage) (T, error)) ([]T, error) {
	var out []T
	var zero T
	for i, h := range items {
		v, err := build(h)
		if err != nil {
			return nil, messages.New(messages.H5213, i, err)
		}
		if v != zero {
			out = append(out, v)
		}
	}
	return out, nil
}

// NewNormalizer JSON tanımından normalleştirici kurar (null için nil).
func NewNormalizer(raw json.RawMessage) (Normalizer, error) {
	return buildComponent(raw, normalizers, messages.H5280, messages.H5281, messages.H5203,
		func(k NormalizerFactory) (Normalizer, error) { return k(raw) })
}

// NewPreTokenizer JSON tanımından ön bölücü kurar (null için nil).
func NewPreTokenizer(raw json.RawMessage) (PreTokenizer, error) {
	return buildComponent(raw, preTokenizers, messages.H5282, messages.H5283, messages.H5204,
		func(k PreTokenizerFactory) (PreTokenizer, error) { return k(raw) })
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
	return buildComponent(raw, postProcessors, messages.H5284, messages.H5285, messages.H5209,
		func(k PostProcessorFactory) (PostProcessor, error) { return k(raw, vocab) })
}

// NewDecoder JSON tanımından çözücü kurar (null için nil).
func NewDecoder(raw json.RawMessage) (Decoder, error) {
	return buildComponent(raw, decoders, messages.H5286, messages.H5287, messages.H5210,
		func(k DecoderFactory) (Decoder, error) { return k(raw) })
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
