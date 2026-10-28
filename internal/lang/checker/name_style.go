package checker

import (
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// nameKind bir adın tanımlandığı yerdir; yazılış kuralı (dil tanımı 2.3b) buna göre seçilir.
type nameKind int

const (
	kindVariable nameKind = iota // değişken (döngü ve yakala değişkeni dahil)
	kindValue                    // fonksiyon, parametre, alan, metot
	kindConst                    // sabit
	kindType                     // yapı
)

// varKind değişken ya da sabit tanımının yazılış kuralıdır: büyük harf yalnız dosyanın en dışındaki
// sabittedir; fonksiyonun ya da bloğun içindeki sabit öbür adlar gibi küçük harfle yazılır (2.3b).
func (c *Checker) varKind(isConst bool) nameKind {
	switch {
	case isConst && c.scope.parent == nil:
		return kindConst
	case isConst:
		return kindValue
	}
	return kindVariable
}

// mathSymbolLetters matematik adının en çok harf sayısıdır: W, X, Wq, dW, W1, QKV ve X_veri'nin
// X'i gibi adlar formüllerin yazımıdır, büyük harfli olabilir (PEP 8'in tersine; makine öğrenmesi
// kodunun yerleşik yazımı).
const mathSymbolLetters = 3

// checkNameStyle adın yazılışını tanımlandığı yere göre denetler ve kurala uymuyorsa önerisiyle
// bir uyarı ekler (PEP 8 düzeni, Rust'ın varsayılan uyarıları gibi hata değil): sabitler
// BÜYÜK_YILAN, yapılar BüyükDeve, geri kalan adlar küçük_yılan. Baştaki _ (modüle özel ad) ve
// $ (kaçış) yazılışa sayılmaz; TR_ ile başlayan adlar (2.3a) zaten hatadır.
func (c *Checker) checkNameStyle(name string, kind nameKind, at lexer.Token) {
	c.checkScripts(name, at)
	bare := strings.TrimLeft(name, "$_")
	if bare == "" || codes.IsReservedName(bare) || !strings.ContainsFunc(bare, unicode.IsLetter) {
		return
	}
	var warning error
	var suggestion string
	switch kind {
	case kindConst:
		if strings.ContainsFunc(bare, unicode.IsLower) {
			suggestion = keepPrefix(name, upperSnake(bare))
			warning = messages.New(messages.H1013, name, suggestion)
		}
	case kindType:
		if !startsUpper(bare) || strings.Contains(bare, "_") {
			suggestion = keepPrefix(name, upperCamel(bare))
			warning = messages.New(messages.H1014, name, suggestion)
		}
	default:
		if valueNameOK(bare) {
			return
		}
		suggestion = keepPrefix(name, snake(bare))
		if kind == kindVariable && !strings.ContainsFunc(bare, unicode.IsLower) {
			warning = messages.New(messages.H1015, name, c.word(codes.KwConst), suggestion)
		} else {
			warning = messages.New(messages.H1012, name, suggestion)
		}
	}
	if warning != nil {
		e := c.addWarningAtToken(at, warning)
		// The fix renames this one place. A name is right only when every use of it changes with
		// it, so the fix is a suggestion, never applicable as it stands.
		if at.Column > 0 {
			e.WithFix(replaceToken(at, suggestion, false))
		}
	}
}

// checkScripts tek bir adda birden çok yazı sisteminin harfi varsa uyarır (2.1a): θ_eski gibi
// formül adları geçerlidir ama başka bir adla karışabilir. Tek yazı sistemindeki ad uyarısızdır.
func (c *Checker) checkScripts(name string, at lexer.Token) {
	if first, second, mixed := lexer.MixedScripts(name); mixed {
		c.addWarningAtToken(at, messages.New(messages.H1019, name, first, second))
	}
}

// valueNameOK alt çizgiyle ayrılan her parça küçük harfli ya da bir matematik adı mı?
func valueNameOK(name string) bool {
	for _, part := range strings.Split(name, "_") {
		if !strings.ContainsFunc(part, unicode.IsUpper) {
			continue
		}
		letters := 0
		for _, r := range part {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters > mathSymbolLetters {
			return false
		}
	}
	return true
}

// keepPrefix önerinin başına adın baştaki $ ve _ işaretlerini geri koyar.
func keepPrefix(name, suggestion string) string {
	return name[:len(name)-len(strings.TrimLeft(name, "$_"))] + suggestion
}

func startsUpper(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

// words adı sözcüklerine ayırır: alt çizgide ve küçük harften (ya da rakamdan) büyük harfe
// geçişte (hazırModel → hazır, Model; SıçramaBekçisi → Sıçrama, Bekçisi).
func words(s string) []string {
	var out []string
	var cur []rune
	prev := rune(0)
	for _, r := range s {
		switch {
		case r == '_':
			if len(cur) > 0 {
				out = append(out, string(cur))
			}
			cur = cur[:0]
		case unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)):
			out = append(out, string(cur))
			cur = append(cur[:0], r)
		default:
			cur = append(cur, r)
		}
		prev = r
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

func snake(s string) string { return strings.ToLowerSpecial(caseOf(s), strings.Join(words(s), "_")) }

func upperSnake(s string) string {
	return strings.ToUpperSpecial(caseOf(s), strings.Join(words(s), "_"))
}

// caseOf öneride kullanılacak harf dönüşümüdür: adda Türkçeye özgü bir harf varsa ya da gösterim
// dili Türkçeyse Türkçe (I ↔ ı, İ ↔ i: ADIM → adım), değilse genel Unicode dönüşümü. Kodlu
// dosyayı başka dilde okuyan kullanıcının adları o dildedir: TOKENIZER'ın İngilizce önerisi
// tokenızer değil tokenizer olmalıdır.
func caseOf(s string) unicode.SpecialCase {
	if turkish.HasTurkishLetter(s) || codes.Display().Code == "tr" {
		return unicode.TurkishCase
	}
	return nil
}

func upperCamel(s string) string {
	var b strings.Builder
	for _, w := range words(s) {
		r := []rune(w)
		b.WriteString(strings.ToUpperSpecial(caseOf(s), string(r[0])))
		b.WriteString(string(r[1:]))
	}
	return b.String()
}
