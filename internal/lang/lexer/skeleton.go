package lexer

import (
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Adların biçimi ve benzerliği (dil tanımı 2.3, 2.1a). Adlar NFC biçimine göre karşılaştırılır: aynı
// görünen iki yazım (birleşik ö ile o ve birleşen iki nokta) aynı addır; sözcük çözümleyici adı NFC
// biçimiyle verir, kodlama ve gösterim de bu biçimi yazar. Görünüşü aynı olan iki ayrı ad (yetki ile
// son harfi Kiril і olan yetkі) Unicode'un benzerlik iskeletiyle (UTS #39) bulunur: iskelet aynıysa
// okuyan birini öbürü sanar. Benzerlik çizelgesi confusables_generated.go'dadır (Unicode 17.0.0).

// NFC returns the text's NFC form (itself when it already is; no copy for ASCII names): names are
// compared and written in this form (2.3). Round-trip checks compare texts by it.
func NFC(name string) string {
	if norm.NFC.IsNormalString(name) {
		return name
	}
	return norm.NFC.String(name)
}

// Skeleton returns the UTS #39 skeleton of s: s in NFD, every code point replaced by its prototype
// from the confusables table, NFD again. Two names that look alike have the same skeleton.
func Skeleton(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if p, ok := confusables[r]; ok {
			b.WriteString(p)
		} else {
			b.WriteRune(r)
		}
	}
	return norm.NFD.String(b.String())
}

// identityLookalike adın kimliğe benzediğidir: TR_ ile başlamadığı hâlde iskeleti TR_'nin
// iskeletiyle başlar (Kiril Т ile yazılmış ТR_print, 2.1a). Yalnız ASCII olan ad kimliğe ancak
// TR_ ile başlayarak benzer; onun iskeleti hesaplanmaz (sözcük çözümleyicinin sıcak yolu).
func identityLookalike(name string) bool {
	if codes.IsReservedName(name) || isASCII(name) {
		return false
	}
	return strings.HasPrefix(Skeleton(name), identitySkeleton)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// identitySkeleton kimlik önekinin iskeletidir.
var identitySkeleton = Skeleton(codes.IdentityPrefix)
