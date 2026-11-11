// Package tokenize dil modelleri için metin <-> token kimliği dönüşümünü
// yapan genel amaçlı bir parçalayıcıdır (tokenizer). Yaygın "tokenizer.json"
// biçimindeki dosyaları ve GGUF dosyasının parçalayıcısını (gguf.go) okur; kodlama ve çözme
// sonuçları, dosyayı üreten başvuru uygulamasıyla birebir aynı olacak biçimde yazılmıştır.
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
	"io"
	"os"
	"sort"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
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
	// plain her ek token düzeyinde (addedSkip) modeldir: tanınmayan ek tokenların kimliklerini
	// bölütlemede kullanmayan kopya (bkz. plain.go); skipNone'da ve kopya gerekmezse model.
	plain [skipAll + 1]Model

	extras  []AddedToken // geçerli kimlikleriyle, dosyadaki sırayla
	addedID map[string]int32
	// addedNormID normalized=true ek tokenların normalleştirilmiş içeriğinden (IDToToken'ın verdiği
	// metin) kimliğe gider; aynı içerikte dosyadaki ilki.
	addedNormID map[string]int32
	rawMatch    *addedMatcher // normalized=false ek tokenlar
	nrmMatch    *addedMatcher // normalized=true ek tokenlar

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

	// Yalnız GGUF'tan yüklenende (16.1): dosyanın ön bölme etiketi ve bildirdiği durma kimlikleri.
	// tokenizer.json'da ikisi de boştur. declaredEOSErr anahtarlardan biri yanlış türdeyse H6681'dir.
	pretokenizerTag string
	declaredEOS     []int32
	declaredEOSErr  error
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

// Load verilen yoldaki parçalayıcıyı okur; biçim içerikten anlaşılır: GGUF ile başlayan dosya
// üstverisinden (16.1), öbürü tokenizer.json. GGUF'ta dosyanın kendisi belleğe alınmaz.
func Load(path string) (*Tokenizer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, messages.New(messages.H5272, err)
	}
	defer f.Close()
	head := make([]byte, len(gguf.Magic))
	if _, err := io.ReadFull(f, head); err == nil && string(head) == gguf.Magic {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, messages.New(messages.H5272, err)
		}
		return loadGGUF(f)
	}
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

	ids, _ := p.model.(idListModel)
	if err := p.addTokens(d.AddedTokens, vocab, ids); err != nil {
		return nil, err
	}
	nrmContent, specialContent, err := p.setupAddedMatchers()
	if err != nil {
		return nil, err
	}
	p.addedNormID = make(map[string]int32, len(nrmContent))
	for _, e := range p.extras {
		if c, ok := nrmContent[e.ID]; ok {
			if _, seen := p.addedNormID[c]; !seen {
				p.addedNormID[c] = int32(e.ID)
			}
		}
	}
	special, all := make(map[int32]bool), make(map[int32]bool)
	for _, e := range p.extras {
		if e.Special {
			special[int32(e.ID)] = true
		}
		all[int32(e.ID)] = true
	}
	p.plain[skipNone] = p.model
	p.plain[skipSpecial] = plainModel(p.model, special)
	p.plain[skipAll] = p.plain[skipSpecial] // özel olmayan ek token yoksa aynı kopya
	if len(all) > len(special) {
		p.plain[skipAll] = plainModel(p.model, all)
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

	p.buildIDTable(vocab, ids, nrmContent, specialContent)
	if l, ok := p.postProcessor.(specialIDLister); ok {
		for _, id := range l.specialIDs() {
			if _, ok := p.IDToToken(int(id)); !ok {
				return nil, messages.New(messages.H5278, id)
			}
		}
	}
	return p, nil
}

// maxTokenID dosyanın verebileceği en büyük parça kimliğidir.
const maxTokenID = 1 << 30

// addTokens ek tokenları kurar. Kural (16.1): ek tokenın kimliği dosyadaki kimliktir, hiç
// değiştirilmez: ya geçerlidir ya dosya reddedilir. İçerik sözlükte ya da daha önce eklenmişse
// kimlik onunla aynıdır (H5277); değilse 0 ile maxTokenID arasında (H5221) ve boştur (H5291).
// Kimlikler dosya sırasına bağlı değildir.
func (p *Tokenizer) addTokens(added []AddedToken, vocab map[string]int32, ids idListModel) error {
	fresh := make(map[int32]string) // sözlükte olmayan ek tokenlar: kimlik -> içerik
	for _, e := range added {
		if e.Content == "" {
			continue
		}
		prev, wasAdded := p.addedID[e.Content]
		known, inVocab := prev, wasAdded
		if !wasAdded {
			known, inVocab = vocab[e.Content]
		}
		switch {
		case inVocab && int(known) != e.ID:
			return messages.New(messages.H5277, e.Content, e.ID, known)
		case inVocab:
		case e.ID < 0 || e.ID > maxTokenID:
			return messages.New(messages.H5221, e.Content, e.ID, maxTokenID)
		default:
			if owner, taken := fresh[int32(e.ID)]; taken {
				return messages.New(messages.H5291, e.Content, e.ID, owner)
			}
			fresh[int32(e.ID)] = e.Content
		}
		if wasAdded {
			for k := range p.extras {
				if p.extras[k].Content == e.Content {
					p.extras[k] = e
				}
			}
		} else {
			p.extras = append(p.extras, e)
		}
		p.addedID[e.Content] = int32(e.ID)
	}
	owners := make(map[int32]string, len(fresh))
	if ids != nil {
		for id, tok := range ids.idTokens() {
			if _, ok := fresh[int32(id)]; ok {
				owners[int32(id)] = tok
			}
		}
	}
	for tok, id := range vocab {
		if _, ok := fresh[id]; !ok {
			continue
		}
		if owner, ok := owners[id]; !ok || tok < owner { // harita sırası değişir, ileti değişmesin
			owners[id] = tok
		}
	}
	for _, e := range p.extras {
		if owner, taken := owners[int32(e.ID)]; taken && fresh[int32(e.ID)] == e.Content {
			return messages.New(messages.H5291, e.Content, e.ID, owner)
		}
	}
	return nil
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

// VocabSize kimliklerin sınırıdır: ek tokenlar dahil en büyük kimlik + 1 (16.1). Kimlikler
// aralıksızsa token sayısıdır; seyrek sözlükte sayıdan büyüktür, gömme tablosu ve örnekleme
// sınırı en büyük kimliği de kapsamalıdır. Aradaki boş kimlik IDToToken'da yoktur.
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
	// normalized=true ek tokenın IDToToken'daki metni normalleştirilmiş içeriğidir; gidiş-dönüş
	// tutar (HF tokenizers: önce sözlük, sonra normalleştirilmiş sözlük).
	if id, ok := p.addedNormID[token]; ok {
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
	for _, m := range p.plain {
		switch m := m.(type) {
		case *bpeModel:
			m.cache = newCache(capacity)
		case *unigram:
			m.cache = newCache(capacity)
		}
	}
}
