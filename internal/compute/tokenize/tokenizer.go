// Package tokenize dil modelleri için metin <-> token kimliği dönüşümünü
// yapan genel amaçlı bir parçalayıcıdır (tokenizer). Yaygın "tokenizer.json"
// biçimindeki dosyaları okur; kodlama ve çözme sonuçları, dosyayı üreten
// başvuru uygulamasıyla birebir aynı olacak biçimde yazılmıştır.
//
// Parçalayıcı bağımsız bileşenlerin zinciridir (bkz. bilesenler.go):
// normalleştirici, ön bölücü, model, son işleyici ve çözücü. Her bileşen
// dosyadaki "type" alanına göre kayıt tablosundan seçilir; desteklenmeyen bir
// tür görülürse hangi bileşenin hangi türünün desteklenmediği açıkça bildirilir.
//
// Desteklenen türler:
//   - normalleştirici: NFC, NFD, NFKC, NFKD, Lowercase, Strip, StripAccents,
//     Replace, Prepend, Nmt, ByteLevel, BertNormalizer, Precompiled, Sequence
//   - ön bölücü: ByteLevel, Split, Whitespace, WhitespaceSplit, Metaspace,
//     BertPreTokenizer, Punctuation, Digits, CharDelimiterSplit, Sequence
//   - model: BPE (byte_fallback, unk_token, fuse_unk, önek/sonek, ignore_merges),
//     WordPiece, WordLevel, Unigram
//   - son işleyici: TemplateProcessing, BertProcessing, RobertaProcessing,
//     ByteLevel, Sequence
//   - çözücü: ByteLevel, Metaspace, WordPiece, BPEDecoder, ByteFallback, Fuse,
//     Strip, Replace, Sequence
//
// Layer: compute core (internal/compute); yalnızca internal/compute paketlerini içe aktarır.
package tokenize

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// AddedToken sözlüğe sonradan eklenmiş (özel ya da sıradan) bir tokendır.
type AddedToken struct {
	ID         int    `json:"id"`
	Content    string `json:"content"`
	SingleWord bool   `json:"single_word"`
	LStrip     bool   `json:"lstrip"`
	RStrip     bool   `json:"rstrip"`
	Normalize  bool   `json:"normalized"`
	Special    bool   `json:"special"`
}

// Tokenizer yüklenmiş bir parçalayıcıdır. Yüklendikten sonra salt
// okunurdur; birden çok goroutine'den aynı anda kullanılabilir.
type Tokenizer struct {
	normalizer    Normalizer
	preTokenizer  PreTokenizer
	model         Model
	postProcessor PostProcessor
	decoder       Decoder

	extras   []AddedToken // geçerli kimlikleriyle, dosyadaki sırayla
	addedID  map[string]int32
	rawMatch *addedMatcher // normalized=false ek tokenlar
	nrmMatch *addedMatcher // normalized=true ek tokenlar

	// Kimlik -> token bilgisi. Kimlikler sıkıysa (en büyük kimlik, girdi
	// sayısının küçük bir katından küçükse) doğrudan dizi, değilse seyrek
	// haritayla dizideki sıraya bakılır: bellek, dosyadaki kimliklerin
	// büyüklüğüyle değil gerçek girdi sayısıyla orantılıdır.
	idValues    []idInfo
	sparse      map[int32]int32 // nil değilse kimlik -> kimlikler'deki sıra
	byteDecoder bool
	vocabSize   int

	// Düzenli ifade kullanan adım sayısı: kodlamada (normalleştirici ve ön
	// bölücü) ve çözmede. Bir çağrının adım bütçesi bununla ölçeklenir.
	encodeRegex, parseRegex int
}

// exprCount bir bileşen (zinciri) içindeki düzenli ifadeleri sayar.
func exprCount(b any) int {
	n := 0
	walkExprs(b, func(*Regex) { n++ })
	return n
}

// encodeBudget bayt uzunluğundaki bir metnin kodlanması için paylaşılan adım
// bütçesini açar (düzenli ifade yoksa nil).
func (p *Tokenizer) encodeBudget(byteVal int) *reBudget {
	// Düzenli ifade olmasa da bütçe vardır: zincirlerin çıktı büyütmesi de onunla sınırlanır.
	return newBudget((p.encodeRegex + 1) * byteVal)
}

// DefaultCacheCapacity sözcük önbelleğinin varsayılan girdi sayısıdır.
const DefaultCacheCapacity = 1 << 17

// Load verilen yoldaki tokenizer.json dosyasını okur.
func Load(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, messages.New(messages.H5272, err)
	}
	return Parse(data)
}

type fileFormat struct {
	Truncation    json.RawMessage `json:"truncation"`
	Padding       json.RawMessage `json:"padding"`
	AddedTokens   []AddedToken    `json:"added_tokens"`
	Normalizer    json.RawMessage `json:"normalizer"`
	PreTokenizer  json.RawMessage `json:"pre_tokenizer"`
	PostProcessor json.RawMessage `json:"post_processor"`
	Decoder       json.RawMessage `json:"decoder"`
	Model         json.RawMessage `json:"model"`
}

func isEmpty(r json.RawMessage) bool {
	t := bytes.TrimSpace(r)
	return len(t) == 0 || bytes.Equal(t, []byte("null"))
}

// setLazyBytes zincirin son adımı bayt düzeyi ön bölücüyse dönüşümü
// modele bırakır (bayt -> karakter çevirisi ve fazladan bellek ayırma olmaz).
func setLazyBytes(o PreTokenizer) {
	switch x := o.(type) {
	case *bytePreTok:
		x.lazy = true
	case PreTokenizerSequence:
		if len(x) > 0 {
			setLazyBytes(x[len(x)-1])
		}
	}
}

// Parse tokenizer.json içeriğinden bir parçalayıcı kurar.
func Parse(data []byte) (*Tokenizer, error) {
	var d fileFormat
	if err := exactUnmarshal(data, &d); err != nil {
		return nil, messages.New(messages.H5274, err)
	}
	if !isEmpty(d.Truncation) {
		return nil, messages.New(messages.H5275)
	}
	if !isEmpty(d.Padding) {
		return nil, messages.New(messages.H5276)
	}
	p := &Tokenizer{addedID: make(map[string]int32)}
	var err error
	if p.normalizer, err = NewNormalizer(d.Normalizer); err != nil {
		return nil, err
	}
	if p.preTokenizer, err = NewPreTokenizer(d.PreTokenizer); err != nil {
		return nil, err
	}
	if p.preTokenizer != nil {
		setLazyBytes(p.preTokenizer)
	}
	if p.model, err = NewModel(d.Model); err != nil {
		return nil, err
	}
	if p.decoder, err = NewDecoder(d.Decoder); err != nil {
		return nil, err
	}
	_, p.byteDecoder = p.decoder.(byteDecoder)
	p.encodeRegex = exprCount(p.normalizer) + exprCount(p.preTokenizer)
	p.parseRegex = exprCount(p.decoder)
	vocab := p.model.Vocab()

	// Ek tokenlar: kimlik, içerik sözlükteyse sözlükteki kimlik; değilse
	// sözlük boyutundan başlayarak sırayla verilen kimliktir. Dosyadaki
	// kimlik bununla tutarsızsa dosya reddedilir.
	dim := int64(len(vocab))
	largestAdded := int64(-1)
	seen := make(map[AddedToken]bool)
	for _, e := range d.AddedTokens {
		if e.Content == "" {
			continue
		}
		key := e
		key.ID = 0
		if seen[key] {
			continue
		}
		seen[key] = true
		var id int64
		if v, ok := p.addedID[e.Content]; ok {
			id = int64(v)
		} else if v, ok := vocab[e.Content]; ok {
			id = int64(v)
		} else if largestAdded < 0 || largestAdded < dim {
			id = dim
		} else {
			id = largestAdded + 1
		}
		if id != int64(e.ID) {
			return nil, messages.New(messages.H5277, e.Content, e.ID, id)
		}
		if id > largestAdded {
			largestAdded = id
		}
		e.ID = int(id)
		if prevVal, ok := p.addedID[e.Content]; ok {
			for k := range p.extras {
				if p.extras[k].Content == e.Content && int32(p.extras[k].ID) == prevVal {
					p.extras[k] = e
				}
			}
		} else {
			p.extras = append(p.extras, e)
		}
		p.addedID[e.Content] = int32(id)
	}
	nrmContent, specialContent, err := p.setupAddedMatchers()
	if err != nil {
		return nil, err
	}

	// Son işleyici, özel token kimliklerine erişebilsin diye en son kurulur.
	if p.postProcessor, err = NewPostProcessor(d.PostProcessor, func(s string) (int32, bool) {
		id, ok := p.TokenToID(s)
		return int32(id), ok
	}); err != nil {
		return nil, err
	}
	if _, ok := p.postProcessor.(noopProcessor); ok {
		p.postProcessor = nil
	}

	ids, _ := p.model.(idListModel)
	p.buildIDTable(vocab, ids, nrmContent, specialContent)
	p.vocabSize = len(vocab)
	if ids != nil {
		p.vocabSize = len(ids.idTokens())
	}
	for _, e := range p.extras {
		if _, ok := vocab[e.Content]; !ok {
			p.vocabSize++
		}
	}
	if l, ok := p.postProcessor.(specialIDLister); ok {
		for _, id := range l.specialIDs() {
			if _, ok := p.IDToToken(int(id)); !ok {
				return nil, messages.New(messages.H5278, id)
			}
		}
	}
	return p, nil
}

// setupAddedMatchers ek tokenların eşleştiricilerini kurar; normalleştirilmiş içerikleri
// (kimliğe göre) ve özel tokenların özgün içeriklerini verir.
func (p *Tokenizer) setupAddedMatchers() (map[int]string, map[string]bool, error) {
	var raw, nrm []AddedToken
	specialContent := make(map[string]bool)
	// normalized=true ek tokenlar normalleştirilmiş metinde aranır; bu yüzden
	// içerikleri de parçalayıcının normalleştiricisinden geçirilir (ör.
	// küçük harfe çeviren bir normalleştiricide "ÇAY" -> "çay"). Kimliğin
	// token metni de (çözmede kullanılan) bu normalleştirilmiş içeriktir;
	// özel token kümesi ise özgün içeriklerdir (başvurudaki gibi: içeriği
	// normalleştirmeyle değişen özel token çözmede atlanmaz). İki ek tokenın
	// içeriği normalleştirmeden sonra aynıysa dosyadaki ilki kazanır (başvuru
	// uygulamasında bu durumda sonuç çalıştırmadan çalıştırmaya değişiyor).
	nrmContent := make(map[int]string)
	// Ek tokenların normalleştirilmesi tek bir adım bütçesini paylaşır (bütün içeriklerin
	// uzunluğuyla orantılı): her biri ayrı bir taban bütçe alsaydı çok sayıda ek token içeren
	// kötü niyetli bir dosya yüklemeyi token sayısı kadar uzatabilirdi.
	var extraBytes int
	for _, e := range p.extras {
		if e.Normalize {
			extraBytes += len(e.Content)
		}
	}
	extraBudget := newBudget(extraBytes)
	for _, e := range p.extras {
		if e.Special {
			specialContent[e.Content] = true
		}
		if e.Normalize {
			if p.normalizer != nil {
				var content string
				if err := catchStepError(func() { content = normalizeBudgeted(p.normalizer, e.Content, extraBudget) }); err != nil {
					return nil, nil, messages.New(messages.H5279, e.Content, err)
				}
				nrmContent[e.ID] = content
				e.Content = content
			}
			if e.Content != "" {
				nrm = append(nrm, e)
			}
		} else {
			raw = append(raw, e)
		}
	}
	p.rawMatch = newAddedMatcher(raw)
	p.nrmMatch = newAddedMatcher(nrm)
	return nrmContent, specialContent, nil
}

// VocabSize ek tokenlar dahil farklı token sayısını verir.
func (p *Tokenizer) VocabSize() int { return p.vocabSize }

// MergeCount BPE modelindeki birleştirme kuralı sayısını verir (BPE değilse 0).
func (p *Tokenizer) MergeCount() int {
	if b, ok := p.model.(*bpeModel); ok {
		return b.mergeCount
	}
	return 0
}

// Model parçalayıcının modelini verir.
func (p *Tokenizer) Model() Model { return p.model }

// TokenToID bir token metninin kimliğini verir (ek tokenlar önceliklidir).
func (p *Tokenizer) TokenToID(token string) (int, bool) {
	if id, ok := p.addedID[token]; ok {
		return int(id), true
	}
	if id, ok := p.model.Vocab()[token]; ok {
		return int(id), true
	}
	return 0, false
}

// IDToToken bir kimliğin token metnini (modelin gösterimiyle) verir.
func (p *Tokenizer) IDToToken(id int) (string, bool) {
	k := p.tokID(id)
	if k == nil {
		return "", false
	}
	return k.token, true
}

// IsSpecial kimliğin özel bir token olup olmadığını söyler.
func (p *Tokenizer) IsSpecial(id int) bool {
	k := p.tokID(id)
	return k != nil && k.special
}

// AddedTokens ek tokenların bir kopyasını kimlik sırasıyla verir.
func (p *Tokenizer) AddedTokens() []AddedToken {
	out := append([]AddedToken(nil), p.extras...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SetCacheCapacity modelin sözcük önbelleğini verilen kapasiteyle
// yeniden kurar (0: önbellek kapalı). Kodlama sürerken çağrılmamalıdır.
func (p *Tokenizer) SetCacheCapacity(capacity int) {
	switch m := p.model.(type) {
	case *bpeModel:
		m.cache = newCache(capacity)
	case *unigram:
		m.cache = newCache(capacity)
	}
}
