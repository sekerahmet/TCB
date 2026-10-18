package turkce

import (
	"strings"
	"unicode"
)

// KucukHarf metni Türkçe kurala göre küçük harfe çevirir: I → ı, İ → i (IŞIK → ışık,
// İşler → işler). Dilin adlarını ve kullanıcı adlarını (tablo adı, renk adı) büyük/küçük
// harf duyarsız karşılaştıran her yer bu tek kuralı kullanır.
func KucukHarf(s string) string { return strings.ToLowerSpecial(unicode.TurkishCase, s) }

// BuyukHarf metni Türkçe kurala göre büyük harfe çevirir: i → İ, ı → I (seç → SEÇ,
// limit → LİMİT).
func BuyukHarf(s string) string { return strings.ToUpperSpecial(unicode.TurkishCase, s) }
