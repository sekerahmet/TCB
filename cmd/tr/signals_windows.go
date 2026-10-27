//go:build windows

package main

import (
	"os"
	"syscall"
)

// requestSignals durdurma isteği sayılan olaylardır (1.5): Ctrl+C ve Ctrl+Break (os.Interrupt),
// konsolun kapanması (SIGTERM). Oturum ve bilgisayar kapanma olayları yalnız hizmetlere gelir.
var requestSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// signalNumber isteğin sinyalinin numarasıdır.
func signalNumber(s os.Signal) int {
	if n, ok := s.(syscall.Signal); ok {
		return int(n)
	}
	return int(syscall.SIGINT)
}

// statusControlCExit Windows'ta Ctrl+C ve Ctrl+Break'le biten sürecin çıkış kodudur
// (STATUS_CONTROL_C_EXIT, 1.9).
const statusControlCExit uint32 = 0xC000013A

// endBySignal süreci Windows'un Ctrl+C sonu koduyla bitirir; Windows'ta sinyalle ölme yoktur.
func endBySignal(int) {
	code := statusControlCExit // sabit değil değişken: 32 bitlik int'te derleme hatası olmasın
	os.Exit(int(code))
}
