package checkpoint

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Kendini içeren liste açık bir döngü hatası verir (eskiden 100 düzey derinliğe inip
// "1.1.1..." yolunu yazıyordu); aynı alt listeyi çok kez içeren kısa bir değer üstel
// büyüyüp belleği tüketmez, öğe sınırında durur.
func TestPlainCyclesAndSharedLists(t *testing.T) {
	a := &interp.ListVal{Elements: []interp.Value{interp.NewInt(1)}}
	a.Elements = append(a.Elements, a, a)
	if _, err := writePlain(a); err == nil || !strings.Contains(err.Error(), "kendini içeriyor") {
		t.Fatalf("döngü hatası beklenirdi: %v", err)
	}
	old := plainNodeLimit
	plainNodeLimit = 1 << 16
	defer func() { plainNodeLimit = old }()
	l := interp.Value(&interp.ListVal{Elements: []interp.Value{interp.NewInt(1)}})
	for range 40 {
		l = &interp.ListVal{Elements: []interp.Value{l, l}} // 2⁴⁰ yaprak
	}
	if _, err := writePlain(l); err == nil || !strings.Contains(err.Error(), "çok büyük") {
		t.Fatalf("öğe sınırı hatası beklenirdi: %v", err)
	}
	// Paylaşılan ama küçük alt liste (döngü değil) yazılır.
	s := &interp.ListVal{Elements: []interp.Value{interp.NewInt(2)}}
	if b, err := writePlain(&interp.ListVal{Elements: []interp.Value{s, s}}); err != nil || string(b) != "[[2],[2]]" {
		t.Fatalf("paylaşılan alt liste: %s, %v", b, err)
	}
}
