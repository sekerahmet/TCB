package turkish

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestCodeFrame(t *testing.T) {
	source := "değişken toplam = 1\n\tyazdır(toplm + 1)\nson"
	got := CodeFrame(source, "Satır 2, Sütun 9: 'toplm' adında bir değişken yok")
	want := " 2 | \tyazdır(toplm + 1)\n   | \t       ^^^^^\n"
	if got != want {
		t.Fatalf("çerçeve beklenenden farklı:\n%q\n%q", got, want)
	}

	// Sütunsuz konum: yalnızca satır gösterilir
	if got := CodeFrame(source, "Satır 3: hata"); got != " 3 | son\n" {
		t.Fatalf("sütunsuz çerçeve hatalı: %q", got)
	}
	// Tırnaklı ad o konumda değilse tek işaretçi
	if got := CodeFrame(source, "Satır 1, Sütun 10: 'başka' sorunu"); got != " 1 | değişken toplam = 1\n   |          ^\n" {
		t.Fatalf("tek işaretçili çerçeve hatalı: %q", got)
	}
	// Konum yok, kaynak boş ya da satır aralık dışında: çerçeve yok
	for _, msg := range []string{"konumsuz hata", "Satır 99, Sütun 1: x"} {
		if got := CodeFrame(source, msg); got != "" {
			t.Errorf("%q için çerçeve beklenmiyordu: %q", msg, got)
		}
	}
	if CodeFrame("", "Satır 1: x") != "" {
		t.Error("boş kaynakta çerçeve beklenmiyordu")
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
	src := "a = 1\nyazdır(toplm + 1)\n"
	err := messages.At(2, 8, messages.New(messages.H2000, "x", "'toplm'"))
	got := ErrorFrame(src, err)
	if !strings.Contains(got, "2 | yazdır(toplm + 1)") || !strings.Contains(got, "^^^^^") {
		t.Fatalf("çerçeve:\n%s", got)
	}
	if ErrorFrame(src, nil) != "" {
		t.Fatal("nil hata çerçevesiz olmalı")
	}
}
