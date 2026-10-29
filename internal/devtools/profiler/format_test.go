package profiler

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (4D): yüzde işaretinin yeri ve ondalık ayırıcı kişinin dilindedir (H8293): Türkçede
// işaret sayıdan önce (%5,2), İngilizcede sonra (5.2%).
func TestPercentFollowsTheLanguage(t *testing.T) {
	for f, want := range map[float64][2]string{0.0001: {"%0", "0%"}, 0.052: {"%5,2", "5.2%"}, 0.41: {"%41", "41%"}, 1: {"%100", "100%"}} {
		if tr, en := messages.TextIn("tr", messages.H8293, percentNumber(f)), messages.TextIn("en", messages.H8293, percentNumber(f)); tr != want[0] || en != want[1] {
			t.Errorf("%v: %q / %q; %q / %q olmalı", f, tr, en, want[0], want[1])
		}
	}
	if got := percent(0.052); got != "%5,2" { // sınama süreci Türkçedir (TR_LANG=tr)
		t.Errorf("percent: %q", got)
	}
}
