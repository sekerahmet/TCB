package logging

import (
	"io"
	"time"
)

// Testlerin terminali ve saati taklit etmesi için (yalnızca testlerde derlenir).

// TerminalTaklidi çubukların çıkışı terminal sanmasını sağlar; geri alan fonksiyonu döndürür.
func TerminalTaklidi(genislik int) func() {
	eski := terminalSecimi
	terminalSecimi = func(io.Writer) (bool, int) { return true, genislik }
	return func() { terminalSecimi = eski }
}

// SaatTaklidi şimdiki zamanı verilen fonksiyondan okutur; geri alan fonksiyonu döndürür.
func SaatTaklidi(f func() time.Time) func() {
	eski := saat
	saat = f
	return func() { saat = eski }
}

// Cubuk çubuk görünümünün iç yardımcısıdır (SayiYazimi ve SureYazimi dışa açıktır:
// disa_acik.go).
var Cubuk = cubuk
