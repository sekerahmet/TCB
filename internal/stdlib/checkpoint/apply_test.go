package checkpoint

import (
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Fark satırı ayarın adını yazar, sözlüğün iç anahtarını değil: NUL ile başlayan ad sözlükte
// kodlanmış durur (interp.DictKey).
func TestSettingDifferenceWritesTheName(t *testing.T) {
	old, fresh := interp.NewDictVal(), interp.NewDictVal()
	old.Set("\x00a", interp.NewInt(1))
	old.Set("\x00b", interp.NewInt(2))
	fresh.Set("\x00a", interp.NewInt(3))
	diffs := settingsDiffs(old, fresh)
	if want := []string{"\x00a: 1 → 3", "\x00b: 2 → yok"}; !slices.Equal(diffs, want) {
		t.Fatalf("%q, %q bekleniyordu", diffs, want)
	}
}
