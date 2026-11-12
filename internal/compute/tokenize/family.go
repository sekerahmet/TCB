package tokenize

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Model ailesinin biçim parçaları (16.2)
// ============================================================================
//
// TR_special_tokens: TR_false (Options.DontParseAdded) metindeki hiçbir eklenmiş parçayı tanımaz. Tek
// istisna, parçalayıcı bir model ailesinin dosyasından açılmışsa ailenin tanımının saydığı özel
// olmayan biçim parçalarıdır: onlar yalnız biçim metnidir (boşluk dizileri, HTML öğe adları) ve
// TR_false'ta da tek kimlik olurlar. Aile Go'daki tablodan, dosyanın üstverisiyle seçilir: GGUF'ta
// (mimari, ön bölme etiketi), klasörde (model_type, tokenizer_config.json'un işareti). Kitaplığın
// sohbet tablosu (lib/model_family.tr) aynı dört alanı taşır; iki tabloyu bir test karşılaştırır.
//
// Küme kapalıdır: ailenin tanımı, ailenin resmî dosyasında iki kümenin her biri için parça sayısını
// ve karmasını taşır. Yüklenen dosyanın kümesi farklıysa (ör. özel olmayan yeni bir etiket ekleyen
// ince ayar) parçalayıcı ailesiz açılır; fark FamilyMismatch'te kalır (H7184'ün anahtarları).

// FormatSet ailenin resmî dosyasındaki bir biçim kümesinin sayısı ve karmasıdır (setHash).
type FormatSet struct {
	Count int
	Hash  string
}

// Family Go'daki aile tablosunun bir satırıdır. ID kitaplığın sohbet tablosundaki aile kimliğidir;
// Arch, GGUFTag ve FolderMarker seçim anahtarlarıdır. Whitespace ve Pattern nil ise aile o kümeyi
// tanımaz (Llama 3, Qwen3: bugünkü davranış aynen sürer). Protocol ailenin protokol etiketleridir
// (sohbet, araç, görüntü): biçim kümesine giremezler (H7185).
type Family struct {
	ID           string
	Arch         string
	GGUFTag      string
	FolderMarker string
	Whitespace   *FormatSet
	Pattern      *FormatSet
	Protocol     []string
}

// gemma3Protocol Gemma 3'ün protokol etiketleridir (1.6.1); dosyanın özel parçaları da kümeye giremez.
var gemma3Protocol = []string{"<bos>", "<eos>", "<pad>", "<start_of_turn>", "<end_of_turn>", "<start_of_image>",
	"<end_of_image>", "<image_soft_token>"}

// gemma3Whitespace ve gemma3Pattern Gemma 3'ün resmî tokenizer.json'unun iki kümesidir: unsloth/gemma-3-1b-it,
// -4b-it ve -12b-it'in dosyası bayt bayt aynı (sha256 4667f2089529e8e7657cfb6d1c19910ae71ff5f28aa7ab2ff2763330affad795),
// 6406 özel olmayan eklenmiş parçadan 92'si boşluk, 59'u desen kümesinde; ikisi kesişmez, protokol etiketlerinin
// hepsi özel parçadır (Colab 3a, 2026-11-13; D1 Gemma 3 tasarımı 1.6.1).
var (
	gemma3Whitespace = FormatSet{92, "ec8b25bd363b59f46a2def2702b8327fa2ed1c50b7d8c829d73addf557ad7f1e"}
	gemma3Pattern    = FormatSet{59, "44b40d2140056d1c90b2ffc0ef450c18ab82b9994ead75383a8a2b4e6373e4c5"}
)

// families aile tablosudur. Sıra ve dört alan lib/model_family.tr'nin _sohbet_tablosu'yla aynıdır.
// gemma3'ün iki satırı: çok kipli klasör ve GGUF (mimari gemma3), 1B klasörü (gemma3_text).
var families = []Family{
	{ID: "llama3", Arch: "llama", GGUFTag: "llama-bpe", FolderMarker: "<|begin_of_text|>"},
	{ID: "qwen3", Arch: "qwen3", GGUFTag: "qwen2", FolderMarker: "Qwen2Tokenizer"},
	{ID: "gemma3", Arch: "gemma3", GGUFTag: "default", FolderMarker: "GemmaTokenizer", Whitespace: &gemma3Whitespace,
		Pattern: &gemma3Pattern, Protocol: gemma3Protocol},
	{ID: "gemma3", Arch: "gemma3_text", GGUFTag: "default", FolderMarker: "GemmaTokenizer", Whitespace: &gemma3Whitespace,
		Pattern: &gemma3Pattern, Protocol: gemma3Protocol},
}

// Families aile tablosunun bir kopyasıdır (tabloları karşılaştıran test için).
func Families() []Family { return append([]Family(nil), families...) }

// GGUFFamily (mimari, etiket) çiftinin ailesidir.
func GGUFFamily(arch, tag string) (Family, bool) {
	for _, f := range families {
		if f.Arch == arch && f.GGUFTag == tag {
			return f, true
		}
	}
	return Family{}, false
}

// FolderFamily (model_type, işaret) çiftinin ailesidir.
func FolderFamily(arch, marker string) (Family, bool) {
	for _, f := range families {
		if f.Arch == arch && f.FolderMarker == marker {
			return f, true
		}
	}
	return Family{}, false
}

// formatPattern desen kümesinin kuralıdır: yalnız küçük ASCII harfli HTML öğe adı (16.2).
var formatPattern = regexp.MustCompile(`^</?[a-z]+>$`)

// isWhitespacePiece içeriğin yalnız boşluk, ▁ (U+2581), sekme, satır sonu ve satır başından oluşup
// oluşmadığını söyler (dosyadaki içerik, normalleştirmeden önce).
func isWhitespacePiece(content string) bool {
	if content == "" {
		return false
	}
	for _, r := range content {
		switch r {
		case ' ', '▁', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return true
}

// setHash kümenin karmasıdır: parçalar kimliğe göre artan sırada, her biri için
// "<ondalık kimlik>:<içeriğin UTF-8 baytlarının küçük harf onaltılığı>\n"; birleşimin SHA-256'sı, küçük
// harf onaltılık. Python'da aynı yazılır (Colab 3a'nın betiği); JSON kullanılmaz.
func setHash(pieces []AddedToken) string {
	s := append([]AddedToken(nil), pieces...)
	sort.Slice(s, func(i, j int) bool { return s[i].ID < s[j].ID })
	h := sha256.New()
	for _, t := range s {
		fmt.Fprintf(h, "%d:%x\n", t.ID, t.Content)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// formatSets dosyanın özel olmayan eklenmiş parçalarından iki kümeyi seçer.
func (p *Tokenizer) formatSets() (whitespace, pattern []AddedToken) {
	for _, e := range p.extras {
		if e.Special {
			continue
		}
		if isWhitespacePiece(e.Content) {
			whitespace = append(whitespace, e)
		}
		if formatPattern.MatchString(e.Content) {
			pattern = append(pattern, e)
		}
	}
	return whitespace, pattern
}

// HasWhitespacePieces dosyada yalnız boşluk karakterlerinden oluşan özel olmayan eklenmiş parça
// olup olmadığını söyler (ailesiz yolun kuralı, 16.2).
func (p *Tokenizer) HasWhitespacePieces() bool {
	w, _ := p.formatSets()
	return len(w) > 0
}

// FamilyMismatch ailesi seçilen ama kümesi ailenin resmî dosyasınınkine uymayan parçalayıcının
// farkıdır (H7184'ün anahtarları): Set "whitespace" ya da "pattern"; Diff "count" (sayı farklı) ya da
// "content" (sayı aynı, karma farklı). İki küme birden ayrılırsa boşluk kümesi yazılır.
type FamilyMismatch struct {
	Family          string
	Set, Diff       string
	Count, Expected int
}

// FamilyOverlapError ailenin bir biçim parçasının ailenin protokol etiketiyle ya da dosyanın bir
// özel parçasıyla kesiştiğini söyler (H7185): Go tablosunun hatasıdır, yükleme durur.
type FamilyOverlapError struct {
	Family, Token, Tag string
}

// Message hatanın iletisidir (H7185); fn parçalayıcıyı açan işlevin adıdır (TR'de TR_load_tokenizer'ın adı).
func (e *FamilyOverlapError) Message(fn any) error {
	return messages.New(messages.H7185, fn, e.Family, e.Token, e.Tag)
}

// Error ve MessageText, çağıran adını vermeden (Go'dan Load ile) gelen hatanın iletisidir.
func (e *FamilyOverlapError) Error() string { return e.Message("tokenize.Load").Error() }

func (e *FamilyOverlapError) MessageText(lang string) string {
	return messages.ErrorIn(e.Message("tokenize.Load"), lang)
}

// Family parçalayıcının ailesidir (ailesizse boş).
func (p *Tokenizer) Family() string { return p.family }

// Mismatch ailesi seçilip kümesi uymadığı için ailesiz açılan parçalayıcının farkıdır (yoksa nil).
func (p *Tokenizer) Mismatch() *FamilyMismatch { return p.mismatch }

// applyFamily f'i parçalayıcıya uygular. Kümeler ailenin sayı ve karmasına uyarsa parçalayıcı aileyle
// açılır ve iki kümenin parçaları TR_false'ta da tanınır; uymazsa ailesiz kalır ve fark Mismatch'te
// durur. Kümelerden biri protokol etiketiyle ya da özel parçayla kesişirse *FamilyOverlapError döner.
// Yalnız yüklemede, parçalayıcı paylaşılmadan önce çağrılır.
func (p *Tokenizer) applyFamily(f Family) error {
	whitespace, pattern := p.formatSets()
	for _, c := range []struct {
		name   string
		want   *FormatSet
		pieces []AddedToken
	}{{"whitespace", f.Whitespace, whitespace}, {"pattern", f.Pattern, pattern}} {
		if c.want == nil {
			continue
		}
		if len(c.pieces) != c.want.Count {
			p.mismatch = &FamilyMismatch{f.ID, c.name, "count", len(c.pieces), c.want.Count}
			return nil
		}
		if setHash(c.pieces) != c.want.Hash {
			p.mismatch = &FamilyMismatch{f.ID, c.name, "content", len(c.pieces), c.want.Count}
			return nil
		}
	}
	format := make(map[int32]bool)
	if f.Whitespace != nil {
		for _, t := range whitespace {
			format[int32(t.ID)] = true
		}
	}
	if f.Pattern != nil {
		for _, t := range pattern {
			format[int32(t.ID)] = true
		}
	}
	if err := p.checkOverlap(f, format); err != nil {
		return err
	}
	p.family = f.ID
	if len(format) == 0 {
		return nil
	}
	return p.openFormatPieces(format)
}

// checkOverlap biçim parçalarının protokol etiketleriyle ve özel parçalarla kesişmediğini denetler.
func (p *Tokenizer) checkOverlap(f Family, format map[int32]bool) error {
	special := make(map[string]bool)
	for _, e := range p.extras {
		if e.Special {
			special[e.Content] = true
		}
	}
	for _, e := range p.extras {
		if !format[int32(e.ID)] {
			continue
		}
		for _, tag := range f.Protocol {
			if e.Content == tag {
				return &FamilyOverlapError{f.ID, e.Content, tag}
			}
		}
		if special[e.Content] {
			return &FamilyOverlapError{f.ID, e.Content, e.Content}
		}
	}
	return nil
}

// openFormatPieces biçim parçalarını TR_false düzeyinde (skipAll) tanınır yapar: eşleştiriciler
// işaretle yeniden kurulur, modelin düz kopyası bu kimlikleri yasaklamaz.
func (p *Tokenizer) openFormatPieces(format map[int32]bool) error {
	for k := range p.extras {
		p.extras[k].format = format[int32(p.extras[k].ID)]
	}
	if _, _, err := p.setupAddedMatchers(); err != nil {
		return err
	}
	barred := make(map[int32]bool)
	for _, e := range p.extras {
		if !e.format {
			barred[int32(e.ID)] = true
		}
	}
	p.plain[skipAll] = plainModel(p.model, barred)
	return nil
}
