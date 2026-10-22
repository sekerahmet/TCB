package messages

import "testing"

// Kaynak dildeki her ileti kodunun İngilizce metni vardır: en.json'da olmayan kod hatadır
// (çevrilmemiş kod kullanıcıya Türkçe görünürdü). Yeni bir kod eklenince tr.json ile birlikte
// en.json'a da yazılır.
func TestEnglishCoversEveryCode(t *testing.T) {
	load()
	en := catalogs["en"]
	for _, code := range definitions.Codes {
		if _, has := en.templates[code]; !has {
			t.Errorf("%s: en.json'da çeviri yok", code)
		}
	}
}
