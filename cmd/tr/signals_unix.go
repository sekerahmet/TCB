//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// requestSignals durdurma isteği sayılan sinyallerdir (1.5): Ctrl+C, SIGTERM, SIGHUP ve
// kümelerin erken uyarısı SIGUSR1 (1.8).
var requestSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1}

// signalNumber isteğin sinyalinin numarasıdır (süreç bununla biter, 1.9).
func signalNumber(s os.Signal) int {
	if n, ok := s.(syscall.Signal); ok {
		return int(n)
	}
	return int(syscall.SIGINT)
}

// endBySignal süreci verilen sinyalle bitirir: kabuk sürecin sinyalle öldüğünü görür (Ctrl+C'de
// 130, SIGTERM'de 143). Go çalışma zamanı SIGUSR1'in varsayılan eylemini (süreci öldürmek)
// yerine getiremez; orada kabuğun göreceği kodla (128 + numara) çıkılır.
func endBySignal(number int) {
	sig := syscall.Signal(number)
	if sig != syscall.SIGUSR1 {
		signal.Reset(sig)
		_ = syscall.Kill(syscall.Getpid(), sig)
		time.Sleep(time.Second) // sinyalin gelmesini bekle; gelmezse aşağıdaki çıkış
	}
	os.Exit(128 + number)
}
