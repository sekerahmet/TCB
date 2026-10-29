package layoutrules

import "testing"

// Kural: istisnanın düzeltme payı tavanın %10'udur, en az 5 satır; tavan + pay istisnanın aşamayacağı boydur.
func TestFixMarginIsATenthAtLeastFive(t *testing.T) {
	for _, c := range []struct{ ceiling, margin int }{{40, 5}, {59, 5}, {60, 6}, {172, 17}, {915, 91}} {
		if got := FixMargin(c.ceiling); got != c.margin {
			t.Errorf("FixMargin(%d) = %d, want %d", c.ceiling, got, c.margin)
		}
	}
}
