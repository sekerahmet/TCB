// Package parcalayici dil modelleri için metin <-> token kimliği dönüşümünü
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

// Parcalayici yüklenmiş bir parçalayıcıdır. Yüklendikten sonra salt
// okunurdur; birden çok goroutine'den aynı anda kullanılabilir.
type Parcalayici struct {
	normallestirici Normallestirici
	onBolucu        OnBolucu
	model           Model
	sonIsleyici     SonIsleyici
	cozucu          Cozucu

	ekler      []EkToken // geçerli kimlikleriyle, dosyadaki sırayla
	ekKimlik   map[string]int32
	hamEslesme *ekEslestirici // normalized=false ek tokenlar
	nrmEslesme *ekEslestirici // normalized=true ek tokenlar

	idToken      []string // kimlik -> token metni
	idVar        []bool
	idOzel       []bool   // kimlik -> özel token mı (çözmede atlanabilir)
	idBayt       [][]byte // yalnızca bayt düzeyi çözücüde: kimlik -> baytlar
	baytCozucu   bool
	sozlukBoyutu int

	// Düzenli ifade kullanan adım sayısı: kodlamada (normalleştirici ve ön
	// bölücü) ve çözmede. Bir çağrının adım bütçesi bununla ölçeklenir.
	kodlaIfade, cozIfade int
}

// ifadeSay bir bileşen (zinciri) içindeki düzenli ifadeleri sayar.
func ifadeSay(b any) int {
	n := 0
	ifadeleriGez(b, func(*DuzIfade) { n++ })
	return n
}

// kodlaButcesi bayt uzunluğundaki bir metnin kodlanması için paylaşılan adım
// bütçesini açar (düzenli ifade yoksa nil).
func (p *Parcalayici) kodlaButcesi(bayt int) *dfButce {
	if p.kodlaIfade == 0 {
		return nil
	}
	return yeniButce(p.kodlaIfade * bayt)
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

func bosMu(r json.RawMessage) bool {
	t := bytes.TrimSpace(r)
	return len(t) == 0 || bytes.Equal(t, []byte("null"))
}

// tembelBaytAyarla zincirin son adımı bayt düzeyi ön bölücüyse dönüşümü
// modele bırakır (bayt -> karakter çevirisi ve fazladan bellek ayırma olmaz).
func tembelBaytAyarla(o OnBolucu) {
	switch x := o.(type) {
	case *baytOnBol:
		x.tembel = true
	case OnBolucuSirasi:
		if len(x) > 0 {
			tembelBaytAyarla(x[len(x)-1])
		}
	}
}

// Ayristir tokenizer.json içeriğinden bir parçalayıcı kurar.
func Ayristir(veri []byte) (*Parcalayici, error) {
	var d dosyaBicimi
	if err := json.Unmarshal(veri, &d); err != nil {
		return nil, fmt.Errorf("parçalayıcı JSON'u çözümlenemedi: %w", err)
	}
	if !bosMu(d.Truncation) {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: truncation (kırpma) tanımlı; kodlama çıktısını değiştireceği için desteklenmiyor")
	}
	if !bosMu(d.Padding) {
		return nil, fmt.Errorf("desteklenmeyen yapılandırma: padding (dolgu) tanımlı; kodlama çıktısını değiştireceği için desteklenmiyor")
	}
	p := &Parcalayici{ekKimlik: make(map[string]int32)}
	var err error
	if p.normallestirici, err = NormallestiriciKur(d.Normalizer); err != nil {
		return nil, err
	}
	if p.onBolucu, err = OnBolucuKur(d.PreTokenizer); err != nil {
		return nil, err
	}
	if p.onBolucu != nil {
		tembelBaytAyarla(p.onBolucu)
	}
	if p.model, err = ModelKur(d.Model); err != nil {
		return nil, err
	}
	if p.cozucu, err = CozucuKur(d.Decoder); err != nil {
		return nil, err
	}
	_, p.baytCozucu = p.cozucu.(baytCozucu)
	p.kodlaIfade = ifadeSay(p.normallestirici) + ifadeSay(p.onBolucu)
	p.cozIfade = ifadeSay(p.cozucu)
	sozluk := p.model.Sozluk()

	// Ek tokenlar: kimlik, içerik sözlükteyse sözlükteki kimlik; değilse
	// sözlük boyutundan başlayarak sırayla verilen kimliktir. Dosyadaki
	// kimlik bununla tutarsızsa dosya reddedilir.
	boyut := int64(len(sozluk))
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
		} else if v, ok := sozluk[e.Icerik]; ok {
			id = int64(v)
		} else if enBuyukEk < 0 || enBuyukEk < boyut {
			id = boyut
		} else {
			id = enBuyukEk + 1
		}
		if id != int64(e.ID) {
			return nil, fmt.Errorf("ek token %q dosyada %d kimliğiyle tanımlı, ancak hesaplanan kimlik %d; tutarsız dosya", e.Icerik, e.ID, id)
		}
		if id > enBuyukEk {
			enBuyukEk = id
		}
		e.ID = int(id)
		if eski, ok := p.ekKimlik[e.Icerik]; ok {
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
	var ham, nrm []EkToken
	ozelIcerik := make(map[string]bool)
	enBuyuk := int64(-1)
	for tok, id := range sozluk {
		_ = tok
		if int64(id) > enBuyuk {
			enBuyuk = int64(id)
		}
	}
	// normalized=true ek tokenlar normalleştirilmiş metinde aranır; bu yüzden
	// içerikleri de parçalayıcının normalleştiricisinden geçirilir (ör.
	// küçük harfe çeviren bir normalleştiricide "ÇAY" -> "çay"). Kimliğin
	// token metni de (çözmede kullanılan) bu normalleştirilmiş içeriktir;
	// özel token kümesi ise özgün içeriklerdir (başvurudaki gibi: içeriği
	// normalleştirmeyle değişen özel token çözmede atlanmaz). İki ek tokenın
	// içeriği normalleştirmeden sonra aynıysa dosyadaki ilki kazanır (başvuru
	// uygulamasında bu durumda sonuç çalıştırmadan çalıştırmaya değişiyor).
	nrmIcerik := make(map[int]string)
	for _, e := range p.ekler {
		if e.Ozel {
			ozelIcerik[e.Icerik] = true
		}
		if e.Normalize {
			if p.normallestirici != nil {
				var icerik string
				if err := adimHatasiniYakala(func() { icerik = p.normallestirici.Normallestir(e.Icerik) }); err != nil {
					return nil, fmt.Errorf("ek token %q normalleştirilemedi: %w", e.Icerik, err)
				}
				nrmIcerik[e.ID] = icerik
				e.Icerik = icerik
			}
			if e.Icerik != "" {
				nrm = append(nrm, e)
			}
		} else {
			ham = append(ham, e)
		}
		if int64(e.ID) > enBuyuk {
			enBuyuk = int64(e.ID)
		}
	}
	p.hamEslesme = yeniEkEslestirici(ham)
	p.nrmEslesme = yeniEkEslestirici(nrm)

	// Son işleyici, özel token kimliklerine erişebilsin diye en son kurulur.
	if p.sonIsleyici, err = SonIsleyiciKur(d.PostProcessor, func(s string) (int32, bool) {
		id, ok := p.TokenKimligi(s)
		return int32(id), ok
	}); err != nil {
		return nil, err
	}
	if _, ok := p.sonIsleyici.(etkisizIsleyici); ok {
		p.sonIsleyici = nil
	}

	n := int(enBuyuk) + 1
	p.idToken = make([]string, n)
	p.idVar = make([]bool, n)
	p.idOzel = make([]bool, n)
	for tok, id := range sozluk {
		p.idToken[id] = tok
		p.idVar[id] = true
	}
	for _, e := range p.ekler {
		p.idToken[e.ID] = e.Icerik
		if s, ok := nrmIcerik[e.ID]; ok {
			p.idToken[e.ID] = s
		}
		p.idVar[e.ID] = true
	}
	if p.baytCozucu {
		p.idBayt = make([][]byte, n)
	}
	for id := 0; id < n; id++ {
		if p.idVar[id] {
			p.idOzel[id] = ozelIcerik[p.idToken[id]]
			if p.baytCozucu {
				p.idBayt[id] = tokenBaytlari(p.idToken[id])
			}
		}
	}
	p.sozlukBoyutu = len(sozluk)
	for _, e := range p.ekler {
		if _, ok := sozluk[e.Icerik]; !ok {
			p.sozlukBoyutu++
		}
	}
	return p, nil
}

// SozlukBoyutu ek tokenlar dahil farklı token sayısını verir.
func (p *Parcalayici) SozlukBoyutu() int { return p.sozlukBoyutu }

// BirlesmeSayisi BPE modelindeki birleştirme kuralı sayısını verir (BPE değilse 0).
func (p *Parcalayici) BirlesmeSayisi() int {
	if b, ok := p.model.(*bpeModel); ok {
		return b.birlesmeSayisi
	}
	return 0
}

// Model parçalayıcının modelini verir.
func (p *Parcalayici) Model() Model { return p.model }

// TokenKimligi bir token metninin kimliğini verir (ek tokenlar önceliklidir).
func (p *Parcalayici) TokenKimligi(token string) (int, bool) {
	if id, ok := p.ekKimlik[token]; ok {
		return int(id), true
	}
	if id, ok := p.model.Sozluk()[token]; ok {
		return int(id), true
	}
	return 0, false
}

// KimlikToken bir kimliğin token metnini (modelin gösterimiyle) verir.
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

// OnbellekKapasitesiAyarla modelin sözcük önbelleğini verilen kapasiteyle
// yeniden kurar (0: önbellek kapalı). Kodlama sürerken çağrılmamalıdır.
func (p *Parcalayici) OnbellekKapasitesiAyarla(kapasite int) {
	switch m := p.model.(type) {
	case *bpeModel:
		m.onbellek = yeniOnbellek(kapasite)
	case *unigram:
		m.onbellek = yeniOnbellek(kapasite)
	}
}
