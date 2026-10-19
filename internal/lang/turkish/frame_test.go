package turkish

import "testing"

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
