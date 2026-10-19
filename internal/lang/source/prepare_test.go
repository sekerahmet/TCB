package source

import (
	"errors"
	"strings"
	"testing"
)

// Kodlu dosya çevrilmeden hazırlanır; konumlar Türkçe gösterime göredir (17.5).
func TestCodedFilePrepares(t *testing.T) {
	s := Prepare(Header+"\nA030 x = 1\nG100(x, tanımsız)\n", Options{TypeCheck: true})
	if s.Stage != StageType || len(s.Errors) != 1 || !strings.HasPrefix(s.Errors[0], "Satır 2, Sütun 11:") {
		t.Fatalf("tanımsız ad Türkçe gösterimdeki konumla bildirilmeli: %v", s.Errors)
	}
	if s.Text != "değişken x = 1\nyazdır(x, tanımsız)\n" {
		t.Errorf("hata çerçevesinin metni Türkçe gösterim olmalı: %q", s.Text)
	}
	// Yerleştirmedeki kodlar da sütunu kaydırır.
	s = Prepare(Header+"\nG100(\"{G400([1])} {yok_ad}\")\n", Options{TypeCheck: true})
	if len(s.Errors) != 1 || !strings.HasPrefix(s.Errors[0], "Satır 1, Sütun 24:") {
		t.Errorf("yerleştirmedeki ad Türkçe gösterimdeki konumla bildirilmeli: %v", s.Errors)
	}
	// Kod olmayan her ad kullanıcının adıdır, anahtar kelime yazımında bile.
	s = Prepare(Header+"\nA030 eğer = 1\nG100(eğer)\n", Options{TypeCheck: true})
	if s.Stage != "" {
		t.Errorf("kodlu dosyada 'eğer' bir kullanıcı adıdır: %v", s.Errors)
	}
}

func TestCodedFileErrors(t *testing.T) {
	cases := map[string]string{
		Header + "\nG999(1)\n":            "'G999' bilinen bir kod değil",
		Header + "\nA030 x = G200([1])\n": "'uzunluk(...)' TR'de bir fonksiyon değil",
	}
	for src, part := range cases {
		s := Prepare(src, Options{TypeCheck: true})
		if !strings.Contains(strings.Join(s.Errors, "\n"), part) {
			t.Errorf("%q: %q içeren hata bekleniyordu: %v", src, part, s.Errors)
		}
	}
	// Türkçe yazılmış dosyada kod biçimli ad hatadır ve başlığı gösterir (2.3a).
	s := Prepare("G100(1)\n", Options{})
	if s.Stage != StageSyntax || !strings.Contains(s.Errors[0], Header) {
		t.Errorf("kod biçimli ad hatası başlığı göstermeli: %v", s.Errors)
	}
	// Ayrıştırılamayan kaynak çevrilmez.
	var sh *SyntaxError
	if _, err := Encode("eğer {\n"); !errors.As(err, &sh) {
		t.Errorf("sözdizimi hatası bekleniyordu: %v", err)
	}
}
