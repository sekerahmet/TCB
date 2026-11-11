package tokenize

import (
	"bufio"
	"encoding/json"
	"os"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// GGUF'taki parçalayıcı (dil tanımı 16.1). GGUF depolarında tokenizer.json yoktur; ön ayırıcının
// kalıbı da dosyada yazmaz, yalnız tokenizer.ggml.pre etiketi vardır. Etiketten bellekte
// tokenizer.json'un parçası kurulur ve Parse'tan geçirilir: ignore_merges, NFC ve özel parçalar
// aynı yolda çözülür, ikinci kurucu yoktur. Sabitler modellerin kendi tokenizer.json'undandır:
// etiketi taşıyan modelin dosyası ne diyorsa o; aynı model iki dosyadan aynı kimlikleri verir.

// Üstveri anahtarları (llama.cpp llama-arch.cpp, LLM_KV_TOKENIZER_*).
const (
	ggufKeyModel  = "tokenizer.ggml.model"
	ggufKeyPre    = "tokenizer.ggml.pre"
	ggufKeyTokens = "tokenizer.ggml.tokens"
	ggufKeyScores = "tokenizer.ggml.scores"
	ggufKeyTypes  = "tokenizer.ggml.token_type"
	ggufKeyMerges = "tokenizer.ggml.merges"
	ggufKeyAddBOS = "tokenizer.ggml.add_bos_token"
	ggufKeyAddEOS = "tokenizer.ggml.add_eos_token"
	ggufKeyBOS    = "tokenizer.ggml.bos_token_id"
	ggufKeyEOS    = "tokenizer.ggml.eos_token_id"
	ggufKeyArch   = "general.architecture"
	ggufKeyPrefix = "tokenizer.ggml." // hiçbir anahtar bununla başlamıyorsa dosyada parçalayıcı yoktur (H5650)
)

// ggufDeclaredEOSKeys dosyanın bildirdiği durma kimliklerinin anahtarları, bu sırayla (16.1).
// eot ve eom dağarcıkla denetlenmez: dağarcık dışı kimlik örneklenmez, üretim onda durmaz (15.55).
var ggufDeclaredEOSKeys = []string{ggufKeyEOS, "tokenizer.ggml.eot_token_id", "tokenizer.ggml.eom_token_id"}

// ggufSpecialIDKeys sözlükte olması gereken özel kimliklerin anahtarlarıdır. "seperator" llama.cpp'nin yazımıdır.
var ggufSpecialIDKeys = []string{ggufKeyBOS, ggufKeyEOS, "tokenizer.ggml.unknown_token_id",
	"tokenizer.ggml.seperator_token_id", "tokenizer.ggml.padding_token_id"}

// Parça türleri (llama.h, llama_token_type): denetim parçası özeldir, kullanıcı parçası özel
// olmayan eklenmiş parçadır; ikisi de metinde tek kimlik olur.
const (
	ggufTokenControl     = 3
	ggufTokenUserDefined = 4
)

const ggufByteLevelNoRegex = `{"type":"ByteLevel","add_prefix_space":false,"trim_offsets":false,"use_regex":false}`

// ggufTag bir etiketin getirdikleridir.
type ggufTag struct {
	normalizer   string // tokenizer.json'un normalizer alanı; boşsa yok
	preTokenizer string
	ignoreMerges bool
	addBOS       bool // add_bos_token anahtarı yoksa geçerli
	onlyArch     []string
	notSpecial   []string // dosyada denetim türünde gelen, modelin özel saymadığı parçalar
}

// ggufTags 16.1'in etiketleri. Kalıplar modellerin tokenizer.json'undandır: Qwen3-0.6B, Llama 3,
// SmolLM2. qwen2 yalnız Qwen mimarilerinde kabul edilir: aynı etiketi NFC'siz modeller de taşır.
// Dönüştürücü bu modelin altı parçasını denetim türüyle yazar, tokenizer.json'da özel değildirler:
// aynı model iki dosyadan aynı metni vermelidir (çözme özel parçaları atlar, 16.3).
var ggufTags = map[string]ggufTag{
	"qwen2": {
		normalizer: `{"type":"NFC"}`,
		preTokenizer: `{"type":"Sequence","pretokenizers":[{"type":"Split","pattern":{"Regex":` +
			`"(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\\r\\n\\p{L}\\p{N}]?\\p{L}+|\\p{N}| ?[^\\s\\p{L}\\p{N}]+[\\r\\n]*|\\s*[\\r\\n]+|\\s+(?!\\S)|\\s+"` +
			`},"behavior":"Isolated","invert":false},` + ggufByteLevelNoRegex + `]}`,
		onlyArch:   []string{"qwen2", "qwen3"},
		notSpecial: []string{"<|fim_prefix|>", "<|fim_middle|>", "<|fim_suffix|>", "<|fim_pad|>", "<|repo_name|>", "<|file_sep|>"},
	},
	"llama-bpe": {
		preTokenizer: `{"type":"Sequence","pretokenizers":[{"type":"Split","pattern":{"Regex":` +
			`"(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\\r\\n\\p{L}\\p{N}]?\\p{L}+|\\p{N}{1,3}| ?[^\\s\\p{L}\\p{N}]+[\\r\\n]*|\\s*[\\r\\n]+|\\s+(?!\\S)|\\s+"` +
			`},"behavior":"Isolated","invert":false},{"type":"ByteLevel","add_prefix_space":false,"trim_offsets":true,"use_regex":false}]}`,
		ignoreMerges: true,
		addBOS:       true,
	},
	"smollm": {
		preTokenizer: `{"type":"Sequence","pretokenizers":[{"type":"Digits","individual_digits":true},` +
			`{"type":"ByteLevel","add_prefix_space":false,"trim_offsets":true,"use_regex":true}]}`,
	},
}

// ggufDocument tokenizer.json'un bellekte kurulan parçasıdır.
type ggufDocument struct {
	AddedTokens   []AddedToken    `json:"added_tokens"`
	Normalizer    json.RawMessage `json:"normalizer"`
	PreTokenizer  json.RawMessage `json:"pre_tokenizer"`
	PostProcessor json.RawMessage `json:"post_processor"`
	Decoder       json.RawMessage `json:"decoder"`
	Model         ggufModel       `json:"model"`
}

// ggufModel tokenizer.json'un BPE modelidir.
type ggufModel struct {
	Type         string           `json:"type"`
	IgnoreMerges bool             `json:"ignore_merges"`
	Vocab        map[string]int64 `json:"vocab"`
	Merges       []string         `json:"merges"`
}

// loadGGUF dosyanın başlığını çözer ve parçalayıcıyı kurar; tensör verisi okunmaz.
func loadGGUF(f *os.File) (*Tokenizer, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, messages.New(messages.H5272, err)
	}
	file, err := gguf.Decode(bufio.NewReaderSize(f, 1<<20), info.Size())
	if err != nil {
		return nil, err
	}
	return FromGGUF(file)
}

// FromGGUF parçalayıcıyı GGUF üstverisinden kurar.
func FromGGUF(f *gguf.File) (*Tokenizer, error) {
	if !hasTokenizerKey(f) {
		return nil, messages.New(messages.H5650, ggufKeyPrefix)
	}
	tag, err := ggufTagOf(f)
	if err != nil {
		return nil, err
	}
	tokens, ok := f.Values[ggufKeyTokens].Data.([]string)
	if !ok {
		return nil, messages.New(messages.H6681, ggufKeyTokens)
	}
	merges, ok := f.Values[ggufKeyMerges].Data.([]string)
	if !ok {
		return nil, messages.New(messages.H6681, ggufKeyMerges)
	}
	kinds, err := ggufTokenKinds(f, len(tokens))
	if err != nil {
		return nil, err
	}
	if err := checkGGUFSpecialIDs(f, len(tokens)); err != nil {
		return nil, err
	}
	doc := ggufDocument{
		Normalizer:   json.RawMessage(tag.normalizer),
		PreTokenizer: json.RawMessage(tag.preTokenizer),
		Decoder:      json.RawMessage(ggufByteLevelNoRegex),
		Model:        ggufModel{Type: "BPE", IgnoreMerges: tag.ignoreMerges, Vocab: make(map[string]int64, len(tokens)), Merges: merges},
	}
	if tag.normalizer == "" {
		doc.Normalizer = json.RawMessage("null")
	}
	for id, token := range tokens {
		if _, dup := doc.Model.Vocab[token]; dup {
			return nil, messages.New(messages.H6684, gguf.Clip(token))
		}
		doc.Model.Vocab[token] = int64(id)
		if kinds[id] == ggufTokenControl || kinds[id] == ggufTokenUserDefined {
			special := kinds[id] == ggufTokenControl && !slices.Contains(tag.notSpecial, token)
			doc.AddedTokens = append(doc.AddedTokens, AddedToken{ID: id, Content: token, Special: special})
		}
	}
	if doc.PostProcessor, err = ggufPostProcessor(f, tag, tokens); err != nil {
		return nil, err
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, messages.Internal("gguf tokenizer document: " + err.Error())
	}
	declared, err := ggufDeclaredEOS(f)
	if err != nil {
		return nil, err
	}
	p, err := Parse(data)
	if err != nil {
		return nil, err
	}
	p.pretokenizerTag, _ = f.Values[ggufKeyPre].Data.(string) // ggufTagOf denetledi
	p.declaredEOS = declared
	return p, nil
}

// hasTokenizerKey dosyada tokenizer.ggml. ile başlayan bir anahtar olup olmadığını söyler.
func hasTokenizerKey(f *gguf.File) bool {
	for key := range f.Values {
		if strings.HasPrefix(key, ggufKeyPrefix) {
			return true
		}
	}
	return false
}

// ggufDeclaredEOS dosyanın bildirdiği durma kimlikleridir: üç anahtardan dosyada olanlar, sırayla
// ve tekrarsız; hiçbiri yoksa nil.
func ggufDeclaredEOS(f *gguf.File) ([]int32, error) {
	var out []int32
	for _, key := range ggufDeclaredEOSKeys {
		v, ok := f.Values[key]
		if !ok {
			continue
		}
		id, isID := v.Data.(uint32)
		if !isID || id > uint32(1<<31-1) {
			return nil, messages.New(messages.H6681, key)
		}
		if !slices.Contains(out, int32(id)) {
			out = append(out, int32(id))
		}
	}
	return out, nil
}

// PretokenizerTag GGUF'tan yüklenen parçalayıcıda dosyanın ön bölme etiketidir (tokenizer.ggml.pre);
// tokenizer.json'dan yüklenende yoktur (16.1).
func (p *Tokenizer) PretokenizerTag() (string, bool) {
	return p.pretokenizerTag, p.pretokenizerTag != ""
}

// DeclaredEOSTokens GGUF'un bildirdiği durma kimlikleridir (ggufDeclaredEOSKeys); dosya hiçbirini
// bildirmiyorsa ya da parçalayıcı tokenizer.json'dan yüklendiyse yoktur (16.1). Boş liste dönmez.
func (p *Tokenizer) DeclaredEOSTokens() ([]int32, bool) {
	return slices.Clone(p.declaredEOS), len(p.declaredEOS) > 0
}

// ggufTagOf modeli ve etiketi denetler.
func ggufTagOf(f *gguf.File) (ggufTag, error) {
	model, ok := f.Values[ggufKeyModel].Data.(string)
	if !ok {
		return ggufTag{}, messages.New(messages.H6681, ggufKeyModel)
	}
	if model != "gpt2" {
		return ggufTag{}, messages.New(messages.H6678, gguf.Clip(model))
	}
	name, ok := f.Values[ggufKeyPre].Data.(string)
	if !ok {
		return ggufTag{}, messages.New(messages.H6681, ggufKeyPre)
	}
	tag, known := ggufTags[name]
	if !known {
		return ggufTag{}, messages.New(messages.H6679, gguf.Clip(name))
	}
	if tag.onlyArch != nil {
		arch, _ := f.Values[ggufKeyArch].Data.(string)
		if !slices.Contains(tag.onlyArch, arch) {
			return ggufTag{}, messages.New(messages.H6680, name, gguf.Clip(arch))
		}
	}
	return tag, nil
}

// ggufTokenKinds parça türlerini verir ve dizilerin boyunu denetler: boyu farklı dizi kimlikleri
// kaydırırdı (16.1; CVE-2025-49847'nin yüzeyi). Tür dizisi zorunludur: yoksa özel parça tanınmaz,
// `<|im_end|>` baytlarına bölünürdü.
func ggufTokenKinds(f *gguf.File, count int) ([]int32, error) {
	if v, ok := f.Values[ggufKeyScores]; ok {
		scores, isFloat := v.Data.([]float32)
		if !isFloat {
			return nil, messages.New(messages.H6681, ggufKeyScores)
		}
		if len(scores) != count {
			return nil, messages.New(messages.H6682, ggufKeyScores, len(scores), count)
		}
	}
	kinds, isInt := f.Values[ggufKeyTypes].Data.([]int32)
	if !isInt {
		return nil, messages.New(messages.H6681, ggufKeyTypes)
	}
	if len(kinds) != count {
		return nil, messages.New(messages.H6682, ggufKeyTypes, len(kinds), count)
	}
	return kinds, nil
}

// checkGGUFSpecialIDs özel kimliklerin sözlükte olduğunu denetler (GHSA-g4cc-763q-h9h6).
func checkGGUFSpecialIDs(f *gguf.File, count int) error {
	for _, key := range ggufSpecialIDKeys {
		v, ok := f.Values[key]
		if !ok {
			continue
		}
		id, isID := v.Data.(uint32)
		if !isID {
			return messages.New(messages.H6681, key)
		}
		if uint64(id) >= uint64(count) {
			return messages.New(messages.H6683, key, id, count)
		}
	}
	return nil
}

// ggufBool anahtar varsa onu, yoksa varsayılanı verir.
func ggufBool(f *gguf.File, key string, fallback bool) bool {
	if b, ok := f.Values[key].Data.(bool); ok {
		return b
	}
	return fallback
}

// ggufPostProcessor başa ve sona eklenen özel parçaların şablonudur; eklenecek yoksa null.
func ggufPostProcessor(f *gguf.File, tag ggufTag, tokens []string) (json.RawMessage, error) {
	type sequence struct {
		ID     string `json:"id"`
		TypeID int    `json:"type_id"`
	}
	type piece struct {
		SpecialToken *sequence `json:"SpecialToken,omitempty"`
		Sequence     *sequence `json:"Sequence,omitempty"`
	}
	type special struct {
		ID     string   `json:"id"`
		IDs    []int64  `json:"ids"`
		Tokens []string `json:"tokens"`
	}
	single := []piece{{Sequence: &sequence{ID: "A"}}}
	specials := map[string]special{}
	add := func(key string, front bool) error {
		id, ok := f.Values[key].Data.(uint32)
		if !ok {
			return messages.New(messages.H6681, key)
		}
		name := tokens[id]
		specials[name] = special{ID: name, IDs: []int64{int64(id)}, Tokens: []string{name}}
		p := piece{SpecialToken: &sequence{ID: name}}
		if front {
			single = append([]piece{p}, single...)
		} else {
			single = append(single, p)
		}
		return nil
	}
	if ggufBool(f, ggufKeyAddBOS, tag.addBOS) {
		if err := add(ggufKeyBOS, true); err != nil {
			return nil, err
		}
	}
	if ggufBool(f, ggufKeyAddEOS, false) {
		if err := add(ggufKeyEOS, false); err != nil {
			return nil, err
		}
	}
	if len(specials) == 0 {
		return json.RawMessage("null"), nil
	}
	pair := append(slices.Clone(single), piece{Sequence: &sequence{ID: "B", TypeID: 1}})
	out, err := json.Marshal(struct {
		Type          string             `json:"type"`
		Single        []piece            `json:"single"`
		Pair          []piece            `json:"pair"`
		SpecialTokens map[string]special `json:"special_tokens"`
	}{"TemplateProcessing", single, pair, specials})
	if err != nil {
		return nil, messages.Internal("gguf post processor: " + err.Error())
	}
	return out, nil
}
