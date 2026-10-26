package codes

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// İki dil vardır (dil tanımı 17.7):
//   - active, programın okunduğu ve çalıştığı dildir: her zaman Türkçe (kanonik). Sözcük
//     çözümleyicinin anahtar kelimeleri, yerleşiklerin ve üyelerin kayıt adları, kurucu,
//     'kendi', örtük blok adları, tür adları ve çalışma anı değerlerinin adları (doğru, tip())
//     buradan gelir. Böylece bir programın anlamı ve çıktısı onu çalıştıranın yazım diline
//     bağlı değildir (aynı program her makinede aynı adlara, sözlük anahtarlarına ve dosyalara
//     ulaşır).
//   - display, kişinin yazım dilidir (LanguageEnvVar; varsayılan Türkçe): tanı iletileri,
//     yardım ve komut satırı çıktısı ile kodlu dosyanın görünümü bu dildedir (DisplayName).
// Karşılaştırma: Python ve Swift çalışma anı değerlerini (True, tür adları) hiç yerelleştirmez;
// Excel formülü kanonik saklar, yalnızca ekranda yerelleştirir. TR de anlamı kanonik tutar,
// yalnızca gösterimi yerelleştirir. Başka bir yazım dilinde yazılmış metin (tr kodla - --dil en,
// kabuk satırı) dili açıkça verilerek okunur (lexer.NewBodyIn) ve kanonik biçime çevrilir.

// display LanguageEnvVar'ın seçtiği gösterim dilidir. Değişken geçersiz bir dil seçiyorsa paket
// süreci kapatmaz (paketi kullanan dil sunucusu, testler ya da TR'yi gömen bir program da
// kapanırdı): Türkçeye düşülür ve hata LanguageError ile okunur; tr komutu başlarken onu
// bildirip çıkar.
var display = func() *Language {
	langCode := messages.ActiveLanguage()
	d, err := buildLanguage(langCode)
	if err != nil {
		displayErr = messages.New(messages.H2509, LanguageEnvVar, langCode, err)
		if d, err = buildLanguage(SourceLanguage); err != nil {
			panic(fmt.Sprintf("codes: the embedded Turkish code list cannot be built: %v", err))
		}
	}
	return d
}()

// active programın okunduğu ve çalıştığı kanonik dildir: her zaman Türkçe.
var active = func() *Language {
	if display.Code == SourceLanguage {
		return display
	}
	d, err := buildLanguage(SourceLanguage)
	if err != nil {
		panic(fmt.Sprintf("codes: the embedded Turkish code list cannot be built: %v", err))
	}
	return d
}()

// displayErr LanguageEnvVar'ın seçtiği dil kurulamadıysa hatasıdır (dil Türkçeye düşmüştür).
var displayErr error

// LanguageError reports why the language selected by LanguageEnvVar could not be built (the
// display language then fell back to Turkish); nil when the selection is valid.
func LanguageError() error { return displayErr }
