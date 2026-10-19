package logging

import (
	"io"
	"time"
)

// Testlerin terminali ve saati taklit etmesi için (yalnızca testlerde derlenir).

// FakeTerminal çubukların çıkışı terminal sanmasını sağlar; geri alan fonksiyonu döndürür.
func FakeTerminal(width int) func() {
	old := terminalChoice
	terminalChoice = func(io.Writer) (bool, int) { return true, width }
	return func() { terminalChoice = old }
}

// FakeClock şimdiki zamanı verilen fonksiyondan okutur; geri alan fonksiyonu döndürür.
func FakeClock(f func() time.Time) func() {
	old := clock
	clock = f
	return func() { clock = old }
}

// Bar çubuk görünümünün iç yardımcısıdır (FormatNumber ve FormatDuration dışa açıktır:
// disa_acik.go).
var Bar = bar
