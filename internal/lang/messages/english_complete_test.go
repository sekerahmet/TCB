package messages

import "testing"

// untranslatedEnglish en.json'da henüz çevirisi olmayan kod aralıklarıdır (iki uç dahil).
// Liste yalnızca küçülür; çeviri bitince boştur.
var untranslatedEnglish = [][2]Code{
	{5000, 9999},
}

func isUntranslatedEnglish(c Code) bool {
	for _, r := range untranslatedEnglish {
		if c >= r[0] && c <= r[1] {
			return true
		}
	}
	return false
}

// Kaynak dildeki her ileti kodunun İngilizce metni vardır; çevrilmemiş kod yalnızca
// untranslatedEnglish'te (gerekçeyle, küçülen listede) durabilir.
func TestEnglishCoversEveryCode(t *testing.T) {
	load()
	en := catalogs["en"]
	for _, code := range definitions.Codes {
		if _, has := en.templates[code]; !has && !isUntranslatedEnglish(code) {
			t.Errorf("%s: en.json'da çeviri yok", code)
		}
	}
}
