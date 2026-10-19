package formatter

import (
	"errors"
	"testing"
)

// Kodlu dosya kodlu biçimde biçimlenir (17): başlık ve BOM korunur, kodlar olduğu gibi kalır,
// satır sonları (başlığınki de) LF olur.
func TestCodedFileFormats(t *testing.T) {
	src := "\xef\xbb\xbf//! tr kodlu 1\r\nA020   f(a){A021 a+1}\r\nG100( f(1) )\r\n"
	got, err := Format(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := "\xef\xbb\xbf//! tr kodlu 1\nA020 f(a) { A021 a + 1 }\nG100(f(1))\n"; got != want {
		t.Errorf("kodlu biçimleme:\n%q\nbeklenen:\n%q", got, want)
	}
	// Biçimin değişmesi (kodlu ↔ Türkçe) anlam denetiminde reddedilir.
	var meaning *MeaningChangedError
	if err := SameMeaning(got, "fonksiyon f(a) { sonuç a + 1 }\nyazdır(f(1))\n"); !errors.As(err, &meaning) {
		t.Errorf("kodlu ve Türkçe biçim aynı sayıldı: %v", err)
	}
	// Kodlu sözcük kodla, kullanıcı adı metinle karşılaştırılır: G100 ile adı 'yazdır' olan
	// kullanıcı değişkeni aynı değildir.
	if err := SameMeaning("//! tr kodlu 1\nG100\n", "//! tr kodlu 1\nyazdır\n"); !errors.As(err, &meaning) {
		t.Errorf("kod ile aynı adlı kullanıcı adı aynı sayıldı: %v", err)
	}
}
