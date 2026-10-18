// Package parcalayici dil modelleri için metin <-> token kimliği dönüşümünü
// yapan bayt düzeyi BPE parçalayıcısıdır. Yaygın "tokenizer.json" biçimindeki
// dosyaları okur; kodlama ve çözme sonuçları, dosyayı üreten başvuru
// uygulamasıyla birebir aynı olacak biçimde yazılmıştır.
//
// Desteklenen yapılandırma (başkası görülürse açık bir hata döner):
//   - model: BPE (birleştirmeler "a b" metni ya da ["a","b"] çifti olarak)
//   - normalizer: yok
//   - pre_tokenizer: Sequence[Split(BolmeDeseni, Isolated), ByteLevel(önek boşluğu yok, desen yok)]
//     ya da yalnızca bu Split + ByteLevel çifti
//   - decoder: ByteLevel
//   - post_processor: yok ya da ByteLevel (yalnızca konumları etkiler)
//   - added_tokens: özel/ek tokenlar; single_word, lstrip, rstrip, normalized seçenekleriyle
package parcalayici

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// EkToken sözlüğe sonradan eklenmiş (özel ya da sıradan) bir tokendır.
type EkToken struct {
	ID        int    `json:"id"`
	Icerik    string `json:"content"`
	TekSozcuk bool   `json:"single_word"`
	SolKirp   bool   `json:"lstrip"`
	SagKirp   bool   `json:"rstrip"`
	Normalize bool   `json:"normalized"`
	Ozel      bool   `json:"special"`
}

// Parcalayici yüklenmiş bir bayt düzeyi BPE parçalayıcısıdır. Yüklendikten
// sonra salt okunurdur; birden çok goroutine'den aynı anda kullanılabilir.
type Parcalayici struct {
	sozluk     map[string]int32 // token metni -> kimlik (model sözlüğü)
	baytSozluk map[string]int32 // tokenın temsil ettiği ham baytlar -> kimlik
	baytID     [256]int32       // tek baytlık tokenlar (-1: yok)

	tablo          *birlesmeTablosu
	birlesmeSayisi int

	bilinmeyen          int32 // bilinmeyen token kimliği (-1: yok)
	bilinmeyenBirlestir bool
	birlesmeYoksay      bool

	ekler      []EkToken // geçerli kimlikleriyle, dosyadaki sırayla
	ekKimlik   map[string]int32
	hamEslesme *ekEslestirici // normalized=false ek tokenlar
	nrmEslesme *ekEslestirici // normalized=true ek tokenlar

	idToken    []string // kimlik -> token metni ("" ve varYok=false: yok)
	idVar      []bool
	idBayt     [][]byte // kimlik -> çözmede üretilecek baytlar
	idOzel     []bool   // kimlik -> özel token mı (çözmede atlanabilir)
	ozelIcerik map[string]bool

	onbellek *onbellek
}

// VarsayilanOnbellekKapasitesi sözcük önbelleğinin varsayılan girdi sayısıdır.
const VarsayilanOnbellekKapasitesi = 1 << 17

// Yukle verilen yoldaki tokenizer.json dosyasını okur.
func Yukle(yol string) (*Parcalayici, error) {
	veri, err := os.ReadFile(yol)
	if err != nil {
		return nil, fmt.Errorf("parçalayıcı dosyası okunamadı: %w", err)
	}
	return Ayristir(veri)
}

// Oku bir okuyucudan tokenizer.json içeriğini okur.
func Oku(r io.Reader) (*Parcalayici, error) {
	veri, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parçalayıcı verisi okunamadı: %w", err)
	}
	return Ayristir(veri)
}

// Dosya biçiminin yalnızca gereken alanları.
type dosyaBicimi struct {
	Truncation    json.RawMessage `json:"truncation"`
	Padding       json.RawMessage `json:"padding"`
	AddedTokens   []EkToken       `json:"added_tokens"`
	Normalizer    json.RawMessage `json:"normalizer"`
	PreTokenizer  json.RawMessage `json:"pre_tokenizer"`
	PostProcessor json.RawMessage `json:"post_processor"`
	Decoder       json.RawMessage `json:"decoder"`
	Model         json.RawMessage `json:"model"`
}

type modelBicimi struct {
	Type                    *string           `json:"type"`
	Dropout                 *float64          `json:"dropout"`
	UnkToken                *string           `json:"unk_token"`
	ContinuingSubwordPrefix *string           `json:"continuing_subword_prefix"`
	EndOfWordSuffix         *string           `json:"end_of_word_suffix"`
	FuseUnk                 bool              `json:"fuse_unk"`
	ByteFallback            bool              `json:"byte_fallback"`
	IgnoreMerges            bool              `json:"ignore_merges"`
	Vocab                   map[string]int64  `json:"vocab"`
	Merges                  []json.RawMessage `json:"merges"`
}

type bilesenBicimi struct {
	Type           string            `json:"type"`
	Pretokenizers  []json.RawMessage `json:"pretokenizers"`
	Pattern        map[string]string `json:"pattern"`
	Behavior       string            `json:"behavior"`
	Invert         bool              `json:"invert"`
	AddPrefixSpace *bool             `json:"add_prefix_space"`
	UseRegex       *bool             `json:"use_regex"`
}

func bosMu(r json.RawMessage) bool {
	t := bytes.TrimSpace(r)
	return len(t) == 0 || bytes.Equal(t, []byte("null"))
}

func bilesenCoz(r json.RawMessage, ad string) (*bilesenBicimi, error) {
	var b bilesenBicimi
	if err := json.Unmarshal(r, &b); err != nil {
		return nil, fmt.Errorf("%s alanı çözümlenemedi: %w", ad, err)
	}
	return &b, nil
}

// onBolmeDenetle ön bölme yapılandırmasının desteklenen biçimde olduğunu doğrular.
func onBolmeDenetle(r json.RawMessage) error {
	if bosMu(r) {
		return fmt.Errorf("desteklenmeyen yapılandırma: pre_tokenizer boş; bayt düzeyi bölme (Split + ByteLevel) gerekli")
	}
	b, err := bilesenCoz(r, "pre_tokenizer")
	if err != nil {
		return err
	}
	var adimlar []json.RawMessage
	if b.Type == "Sequence" {
		adimlar = b.Pretokenizers
	} else {
		return fmt.Errorf("desteklenmeyen yapılandırma: pre_tokenizer türü %q (yalnızca Sequence[Split, ByteLevel] destekleniyor)", b.Type)
	}
	if len(adimlar) != 2 {
		return fmt.Errorf("desteklenmeyen yapılandırma: pre_tokenizer dizisinde %d adım var, 2 (Split, ByteLevel) bekleniyordu", len(adimlar))
	}
	sp, err := bilesenCoz(adimlar[0], "pre_tokenizer[0]")
	if err != nil {
		return err
	}
	if sp.Type != "Split" {
		return fmt.Errorf("desteklenmeyen yapılandırma: ilk ön bölme adımı %q, Split bekleniyordu", sp.Type)
	}
	desen, ok := sp.Pattern["Regex"]
	if !ok || len(sp.Pattern) != 1 {
		return fmt.Errorf("desteklenmeyen yapılandırma: Split deseni düzenli ifade (Regex) olmalı")
	}
	if desen != BolmeDeseni {
		return fmt.Errorf("desteklenmeyen yapılandırma: Split deseni %q desteklenmiyor; yalnızca %q elle uygulanmıştır", desen, BolmeDeseni)
	}
	if sp.Behavior != "Isolated" || sp.Invert {
		return fmt.Errorf("desteklenmeyen yapılandırma: Split davranışı %q, ters=%v (yalnızca Isolated, ters=false)", sp.Behavior, sp.Invert)
	}
	bl, err := bilesenCoz(adimlar[1], "pre_tokenizer[1]")
	if err != nil {
		return err
	}
	if bl.Type != "ByteLevel" {
		return fmt.Errorf("desteklenmeyen yapılandırma: ikinci ön bölme adımı %q, ByteLevel bekleniyordu", bl.Type)
	}
	if bl.AddPrefixSpace != nil && *bl.AddPrefixSpace {
		return fmt.Errorf("desteklenmeyen yapılandırma: ByteLevel add_prefix_space=true desteklenmiyor")
	}
	if bl.UseRegex == nil || *bl.UseRegex {
		return fmt.Errorf("desteklenmeyen yapılandırma: ByteLevel ön bölmesinde use_regex=false olmalı")
	}
	return nil
}

// birlesmeCiftiCoz bir birleştirme girdisini ("a b" ya da ["a","b"]) çözer.
func birlesmeCiftiCoz(r json.RawMessage) (string, string, error) {
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		for i := 0; i < len(s); i++ {
			if s[i] == ' ' {
				return s[:i], s[i+1:], nil
			}
		}
		return "", "", fmt.Errorf("birleştirme %q iki parçaya ayrılamadı", s)
	}
	var c []string
	if err := json.Unmarshal(r, &c); err != nil || len(c) != 2 {
		return "", "", fmt.Errorf("birleştirme girdisi çözümlenemedi: %s", string(r))
	}
	return c[0], c[1], nil
}

// Ayristir tokenizer.json içeriğinden bir parçalayıcı kurar.
func Ayristir(veri []byte) (*Parcalayici, error) {
	var d dosyaBicimi
	if err := json.Unmarshal(veri, &d); err != nil {
		return nil, fmt.Errorf("parçalayıcı JSON'u çözümlenemedi: %w", err)
	}
	if !bosMu(d.Normalizer) {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: normalizer tanımlı (%s); yalnızca normalizer=null destekleniyor", string(bytes.TrimSpace(d.Normalizer)))
	}
	if !bosMu(d.Truncation) {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: truncation tanımlı; kodlama çıktısını değiştireceği için desteklenmiyor")
	}
	if !bosMu(d.Padding) {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: padding tanımlı; kodlama çıktısını değiştireceği için desteklenmiyor")
	}
	if err := onBolmeDenetle(d.PreTokenizer); err != nil {
		return nil, err
	}
	if !bosMu(d.PostProcessor) {
		b, err := bilesenCoz(d.PostProcessor, "post_processor")
		if err != nil {
			return nil, err
		}
		if b.Type != "ByteLevel" {
			return nil, fmt.Errorf("desteklenmeyen yapılandırma: post_processor türü %q (yalnızca null ya da ByteLevel)", b.Type)
		}
	}
	if bosMu(d.Decoder) {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: decoder boş; ByteLevel çözücü gerekli")
	}
	if b, err := bilesenCoz(d.Decoder, "decoder"); err != nil {
		return nil, err
	} else if b.Type != "ByteLevel" {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: decoder türü %q (yalnızca ByteLevel)", b.Type)
	}

	if bosMu(d.Model) {
		return nil, fmt.Errorf("parçalayıcı dosyasında model yok")
	}
	var m modelBicimi
	if err := json.Unmarshal(d.Model, &m); err != nil {
		return nil, fmt.Errorf("model alanı çözümlenemedi: %w", err)
	}
	if m.Type != nil && *m.Type != "BPE" {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: model türü %q (yalnızca BPE)", *m.Type)
	}
	if m.Type == nil && m.Merges == nil {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: model türü belirtilmemiş ve birleştirme listesi yok (yalnızca BPE)")
	}
	if m.Dropout != nil && *m.Dropout != 0 {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: BPE dropout=%v (rastgele birleştirme desteklenmiyor)", *m.Dropout)
	}
	if m.ContinuingSubwordPrefix != nil && *m.ContinuingSubwordPrefix != "" {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: continuing_subword_prefix=%q desteklenmiyor", *m.ContinuingSubwordPrefix)
	}
	if m.EndOfWordSuffix != nil && *m.EndOfWordSuffix != "" {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: end_of_word_suffix=%q desteklenmiyor", *m.EndOfWordSuffix)
	}
	if m.ByteFallback {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: byte_fallback=true desteklenmiyor")
	}
	if len(m.Vocab) == 0 {
		return nil, fmt.Errorf("model sözlüğü boş")
	}

	p := &Parcalayici{
		sozluk:              make(map[string]int32, len(m.Vocab)),
		baytSozluk:          make(map[string]int32, len(m.Vocab)),
		bilinmeyen:          -1,
		bilinmeyenBirlestir: m.FuseUnk,
		birlesmeYoksay:      m.IgnoreMerges,
		ekKimlik:            make(map[string]int32),
		ozelIcerik:          make(map[string]bool),
		onbellek:            yeniOnbellek(VarsayilanOnbellekKapasitesi),
	}
	enBuyuk := int64(-1)
	for tok, id := range m.Vocab {
		if id < 0 || id > 1<<30 {
			return nil, fmt.Errorf("sözlükte geçersiz kimlik: %q -> %d", tok, id)
		}
		p.sozluk[tok] = int32(id)
		if id > enBuyuk {
			enBuyuk = id
		}
	}
	// Tokenların bayt karşılıkları: yalnızca tamamen bayt eşlemesindeki
	// karakterlerden oluşan tokenlar BPE tarafından üretilebilir.
	for tok, id := range p.sozluk {
		ok := true
		bs := make([]byte, 0, len(tok))
		for _, r := range tok {
			b, var_ := karakterBayt[r]
			if !var_ {
				ok = false
				break
			}
			bs = append(bs, b)
		}
		if ok {
			p.baytSozluk[string(bs)] = id
		}
	}
	for b := 0; b < 256; b++ {
		if id, ok := p.sozluk[string(baytKarakter[b])]; ok {
			p.baytID[b] = id
		} else {
			p.baytID[b] = -1
		}
	}
	if m.UnkToken != nil {
		id, ok := p.sozluk[*m.UnkToken]
		if !ok {
			return nil, fmt.Errorf("bilinmeyen token %q sözlükte yok", *m.UnkToken)
		}
		p.bilinmeyen = id
	}
	birlesmeler := make(map[uint64]birlesme, len(m.Merges))
	for i, r := range m.Merges {
		a, b, err := birlesmeCiftiCoz(r)
		if err != nil {
			return nil, fmt.Errorf("%d. birleştirme: %w", i, err)
		}
		ia, ok1 := p.sozluk[a]
		ib, ok2 := p.sozluk[b]
		yeni, ok3 := p.sozluk[a+b]
		if !ok1 || !ok2 || !ok3 {
			return nil, fmt.Errorf("%d. birleştirme (%q, %q) sözlükte olmayan bir token içeriyor", i, a, b)
		}
		// Aynı çift birden çok kez geçerse sonuncusu geçerlidir.
		birlesmeler[ciftAnahtar(ia, ib)] = birlesme{sira: int32(i), yeni: yeni}
	}
	p.birlesmeSayisi = len(m.Merges)
	p.tablo = yeniBirlesmeTablosu(birlesmeler)

	// Ek tokenlar: kimlik, içerik sözlükteyse sözlükteki kimlik; değilse
	// sözlük boyutundan başlayarak sırayla verilen kimliktir (dosyadaki
	// kimlik yalnızca bu değerle tutarlıysa aynıdır).
	sozlukBoyutu := int64(len(p.sozluk))
	enBuyukEk := int64(-1)
	gorulen := make(map[EkToken]bool)
	for _, e := range d.AddedTokens {
		if e.Icerik == "" {
			continue
		}
		anahtar := e
		anahtar.ID = 0
		if gorulen[anahtar] {
			continue
		}
		gorulen[anahtar] = true
		var id int64
		if v, ok := p.ekKimlik[e.Icerik]; ok {
			id = int64(v)
		} else if v, ok := p.sozluk[e.Icerik]; ok {
			id = int64(v)
		} else if enBuyukEk < 0 {
			id = sozlukBoyutu
		} else if enBuyukEk >= sozlukBoyutu {
			id = enBuyukEk + 1
		} else {
			id = sozlukBoyutu
		}
		if id != int64(e.ID) {
			return nil, fmt.Errorf("ek token %q dosyada %d kimliğiyle tanımlı, ancak hesaplanan kimlik %d; tutarsız dosya", e.Icerik, e.ID, id)
		}
		if id > enBuyukEk {
			enBuyukEk = id
		}
		e.ID = int(id)
		if eski, ok := p.ekKimlik[e.Icerik]; ok {
			// Aynı içerik farklı seçeneklerle yeniden tanımlanmış: sonuncusu geçerli.
			for k := range p.ekler {
				if p.ekler[k].Icerik == e.Icerik && int32(p.ekler[k].ID) == eski {
					p.ekler[k] = e
				}
			}
		} else {
			p.ekler = append(p.ekler, e)
		}
		p.ekKimlik[e.Icerik] = int32(id)
	}
	for _, e := range p.ekler {
		if e.Ozel {
			p.ozelIcerik[e.Icerik] = true
		}
		if int64(e.ID) > enBuyuk {
			enBuyuk = int64(e.ID)
		}
	}
	var ham, nrm []EkToken
	for _, e := range p.ekler {
		if e.Normalize {
			nrm = append(nrm, e)
		} else {
			ham = append(ham, e)
		}
	}
	p.hamEslesme = yeniEkEslestirici(ham)
	p.nrmEslesme = yeniEkEslestirici(nrm)

	// Kimlik tabloları (çözme için).
	n := int(enBuyuk) + 1
	p.idToken = make([]string, n)
	p.idVar = make([]bool, n)
	p.idBayt = make([][]byte, n)
	p.idOzel = make([]bool, n)
	for tok, id := range p.sozluk {
		p.idToken[id] = tok
		p.idVar[id] = true
	}
	for _, e := range p.ekler {
		p.idToken[e.ID] = e.Icerik
		p.idVar[e.ID] = true
	}
	for id := 0; id < n; id++ {
		if p.idVar[id] {
			p.idBayt[id] = tokenBaytlari(p.idToken[id])
			p.idOzel[id] = p.ozelIcerik[p.idToken[id]]
		}
	}
	return p, nil
}

// SozlukBoyutu ek tokenlar dahil farklı token sayısını verir.
func (p *Parcalayici) SozlukBoyutu() int {
	n := len(p.sozluk)
	for _, e := range p.ekler {
		if _, ok := p.sozluk[e.Icerik]; !ok {
			n++
		}
	}
	return n
}

// BirlesmeSayisi modeldeki birleştirme kuralı sayısını verir.
func (p *Parcalayici) BirlesmeSayisi() int { return p.birlesmeSayisi }

// TokenKimligi bir token metninin kimliğini verir (ek tokenlar önceliklidir).
func (p *Parcalayici) TokenKimligi(token string) (int, bool) {
	if id, ok := p.ekKimlik[token]; ok {
		return int(id), true
	}
	if id, ok := p.sozluk[token]; ok {
		return int(id), true
	}
	return 0, false
}

// KimlikToken bir kimliğin token metnini (bayt düzeyi gösterimiyle) verir.
func (p *Parcalayici) KimlikToken(id int) (string, bool) {
	if id < 0 || id >= len(p.idVar) || !p.idVar[id] {
		return "", false
	}
	return p.idToken[id], true
}

// OzelMi kimliğin özel bir token olup olmadığını söyler.
func (p *Parcalayici) OzelMi(id int) bool {
	return id >= 0 && id < len(p.idOzel) && p.idOzel[id]
}

// EkTokenlar ek tokenların bir kopyasını kimlik sırasıyla verir.
func (p *Parcalayici) EkTokenlar() []EkToken {
	out := append([]EkToken(nil), p.ekler...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// OnbellekKapasitesiAyarla sözcük önbelleğini verilen kapasiteyle yeniden
// kurar (0: önbellek kapalı). Kodlama sürerken çağrılmamalıdır.
func (p *Parcalayici) OnbellekKapasitesiAyarla(kapasite int) {
	p.onbellek = yeniOnbellek(kapasite)
}
