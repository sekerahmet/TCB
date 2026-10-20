package source

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// wantPos hatanın konumunu metinden değil yapıdan (messages.Pos) okuyup bekleneniyle karşılaştırır.
func wantPos(t *testing.T, err error, line, col int) {
	t.Helper()
	if l, c, ok := messages.Pos(err); !ok || l != line || c != col {
		t.Errorf("konum %d:%d bekleniyordu, gelen %d:%d (var: %v): %v", line, col, l, c, ok, err)
	}
}

// Kodlu dosya çevrilmeden hazırlanır; konumlar Türkçe gösterime göredir (17.5).
func TestCodedFilePrepares(t *testing.T) {
	s := Prepare(Header+"\nA030 x = 1\nG100(x, tanımsız)\n", Options{TypeCheck: true})
	if s.Stage != StageType || len(s.Errors) != 1 {
		t.Fatalf("tanımsız ad bildirilmeli: %v", s.Errors)
	}
	wantPos(t, s.Errors[0], 2, 11) // Türkçe gösterimdeki konum
	if s.Text != "değişken x = 1\nyazdır(x, tanımsız)\n" {
		t.Errorf("hata çerçevesinin metni Türkçe gösterim olmalı: %q", s.Text)
	}
	// Yerleştirmedeki kodlar da sütunu kaydırır.
	s = Prepare(Header+"\nG100(\"{G400([1])} {yok_ad}\")\n", Options{TypeCheck: true})
	if len(s.Errors) != 1 {
		t.Fatalf("yerleştirmedeki ad bildirilmeli: %v", s.Errors)
	}
	wantPos(t, s.Errors[0], 1, 24)
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
		if !strings.Contains(joinErrors(s.Errors), part) {
			t.Errorf("%q: %q içeren hata bekleniyordu: %v", src, part, s.Errors)
		}
	}
	// Türkçe yazılmış dosyada kod biçimli ad hatadır ve başlığı gösterir (2.3a).
	s := Prepare("G100(1)\n", Options{})
	if s.Stage != StageSyntax || !strings.Contains(s.Errors[0].Error(), Header) {
		t.Errorf("kod biçimli ad hatası başlığı göstermeli: %v", s.Errors)
	}
	// Ayrıştırılamayan kaynak çevrilmez.
	var sh *SyntaxError
	if _, err := Encode("eğer {\n"); !errors.As(err, &sh) {
		t.Errorf("sözdizimi hatası bekleniyordu: %v", err)
	}
}

// A file with thousands of errors (often not a TR file at all) reports the first MaxErrors and
// how many more there are; reporting every error with a code frame took minutes.
func TestPrepareLimitsErrors(t *testing.T) {
	r := Prepare(strings.Repeat("# yorum değil\n", 500), Options{})
	if len(r.Errors) != MaxErrors+1 {
		t.Fatalf("%d errors; %d expected", len(r.Errors), MaxErrors+1)
	}
	last := r.Errors[MaxErrors]
	if _, _, ok := messages.Pos(last); ok || !messages.Has(last, messages.H2513) {
		t.Errorf("summary line must be H2513 without a position: %v", last)
	}
	// Başka hatalar konumludur.
	wantPos(t, r.Errors[0], 1, 1)
}

func joinErrors(errs []error) string {
	parts := make([]string, len(errs))
	for i, e := range errs {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "\n")
}
