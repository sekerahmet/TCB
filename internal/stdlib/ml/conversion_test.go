package ml

import (
	"strings"
	"testing"
)

// Kendini içeren liste tensöre çevrilirken sonsuz şekil üretmez (eskiden bellek bitene dek
// eksen ekleniyordu); açık bir hata verir.
func TestTensorFromCyclicList(t *testing.T) {
	_, err := run(t, "değişken a = []\na.ekle(a)\ntensör(a)")
	if err == nil || !strings.Contains(err.Error(), "kendini içeriyor") {
		t.Fatalf("döngüsel liste hatası beklenirdi: %v", err)
	}
	if out := mustRun(t, "değişken a = [[1.0]]\ndöngü i içinde 0..20 { a = [a] }\nyazdır(tensör(a).şekil.uzunluk)"); strings.TrimSpace(out) != "22" {
		t.Fatalf("derin ama düzgün liste: %q", out)
	}
}
