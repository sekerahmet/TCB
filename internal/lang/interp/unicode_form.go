package interp

import (
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// unicodeFormBytes, .TR_unicode_normalize'ın ASCII olmayan metinde yorumlayıcı kilidini bıraktığı
// girdi boyudur (longTextBytes'tan küçük): karakter karakter kurulan biçim ve eşitlik denetimi
// (lexer.Normalize) bu boyda ~1 ms'lik iştir.
const unicodeFormBytes = 512

// setupUnicodeFormMethod metnin .TR_unicode_normalize metodunu kaydeder (13.2). Biçim, adıyla dört
// Unicode biçiminden biridir; harf büyüklüğü tutmayan ad hatadır, ileti doğru yazımı verir (H2671).
func setupUnicodeFormMethod() {
	addMethod(typeString, codes.MemUnicodeNormalize, signature{1, 1, []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		form := lexer.Form(args[0].(*StringVal).Val)
		if !slices.Contains(lexer.Forms, form) {
			return nil, unknownForm(string(form))
		}
		s := inst.(*StringVal).Val
		var out string
		work := func() { out = lexer.Normalize(form, s) }
		if len(s) >= longTextBytes || len(s) >= unicodeFormBytes && !isASCIIText(s) {
			y.Blocking(work)
		} else {
			work()
		}
		return &StringVal{Val: out}, nil
	})
}

// unknownForm H2671'dir; given metin de olabilir (metinle biçim yer değiştirmişse), kısaltılır.
func unknownForm(given string) error {
	near := ""
	for _, f := range lexer.Forms {
		if strings.EqualFold(given, string(f)) {
			near = string(f)
		}
	}
	return messages.New(messages.H2671, CallName(codes.MemUnicodeNormalize), shorten(programSecrets.mask(given), summaryLimit), near)
}

func isASCIIText(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
