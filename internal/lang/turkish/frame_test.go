package turkish

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestErrorFrame(t *testing.T) {
	source := "TR_var toplam = 1\n\tTR_print(toplm + 1)\nson"
	at := func(line, col int, text string) error { return messages.At(line, col, errors.New(text)) }
	got := ErrorFrame(source, at(2, 9, "'toplm' adında bir değişken yok"))
	want := " 2 | \tyazdır(toplm + 1)\n   | \t       ^^^^^\n"
	if got != want {
		t.Fatalf("çerçeve beklenenden farklı:\n%q\n%q", got, want)
	}

	// Sütunsuz konum: yalnızca satır gösterilir
	if got := ErrorFrame(source, at(3, 0, "hata")); got != " 3 | son\n" {
		t.Fatalf("sütunsuz çerçeve hatalı: %q", got)
	}
	// Tırnaklı ad o konumda değilse tek işaretçi
	if got := ErrorFrame(source, at(1, 10, "'başka' sorunu")); got != " 1 | değişken toplam = 1\n   |          ^\n" {
		t.Fatalf("tek işaretçili çerçeve hatalı: %q", got)
	}
	// Satır aralık dışında ya da kaynak boş: çerçeve yok
	if got := ErrorFrame(source, at(99, 1, "x")); got != "" {
		t.Errorf("aralık dışı satır için çerçeve beklenmiyordu: %q", got)
	}
	if ErrorFrame("", at(1, 0, "x")) != "" {
		t.Error("boş kaynakta çerçeve beklenmiyordu")
	}
}

// Konum hatanın alanından gelir, metinden okunmaz: "Satır 3" içeren konumsuz bir kullanıcı
// metni (ör. fırlat "Satır 3 yanlış") o satırı çerçeveletmez.
func TestErrorFrameIgnoresText(t *testing.T) {
	source := "bir\niki\nüç\n"
	for _, text := range []string{"Satır 3 yanlış", "Satır 2, Sütun 1: konum gibi görünen metin"} {
		if got := ErrorFrame(source, errors.New(text)); got != "" {
			t.Errorf("konumsuz hata %q çerçevesiz olmalı: %q", text, got)
		}
	}
}

func TestLineAt(t *testing.T) {
	src := "bir\r\niki\nüç"
	for n, want := range map[int]string{1: "bir", 2: "iki", 3: "üç"} {
		if got, ok := lineAt(src, n); !ok || got != want {
			t.Errorf("lineAt(%d) = %q, %v; %q expected", n, got, ok, want)
		}
	}
	for _, n := range []int{0, 4} {
		if _, ok := lineAt(src, n); ok {
			t.Errorf("lineAt(%d) should not exist", n)
		}
	}
}

// ErrorFrame konumu hatanın alanından okur: metni "Satır" ile başlamasa da (başka dil) çizer.
func TestErrorFrameFromPosition(t *testing.T) {
	src := "a = 1\nTR_print(toplm + 1)\n"
	err := messages.At(2, 8, messages.New(messages.H2000, "x", "'toplm'"))
	got := ErrorFrame(src, err)
	if !strings.Contains(got, "2 | yazdır(toplm + 1)") || !strings.Contains(got, "^^^^^") {
		t.Fatalf("çerçeve:\n%s", got)
	}
	if ErrorFrame(src, nil) != "" {
		t.Fatal("nil hata çerçevesiz olmalı")
	}
}

// Kural: IndentLines her satırın başına öneki koyar, sondaki boş satırları atar ve her satırı satır
// sonuyla bitirir; boş metin boş kalır.
// Neden: hata çerçevesi girintileme üç pakette aynı döngüyle ayrı ayrı yazılmıştı.
func TestIndentLinesPrefixesEveryLine(t *testing.T) {
	for _, c := range []struct{ in, prefix, want string }{
		{"", "  ", ""},
		{"a", "  ", "  a\n"},
		{"a\nb\n\n", "> ", "> a\n> b\n"},
		{"a\n\nb", "--", "--a\n--\n--b\n"},
	} {
		if got := IndentLines(c.in, c.prefix); got != c.want {
			t.Errorf("IndentLines(%q, %q) = %q, beklenen %q", c.in, c.prefix, got, c.want)
		}
	}
}
