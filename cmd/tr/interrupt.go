package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Nazik çıkış kodları: 3 "durduruldu" (ertelenen işler çalıştı; kontrol noktası kullanan
// bir eğitim kaldığı yerden sürdürülebilir), 130 ikinci kesme isteğiyle hemen çıkış.
const (
	interruptExitCode = 3
	immediateExitCode = 130
)

// catchInterrupt Ctrl+C'yi ve sonlandırma isteklerini (Unix'te SIGTERM ve SIGHUP; Windows'ta
// Ctrl+Break ile konsol kapatma, oturum kapatma ve bilgisayarı kapatma olayları) yakalar.
// İlk istek programın nazikçe durmasını ister (VM.RequestInterrupt): program bir kontrol noktası
// kurduysa istek adım sınırına ertelenir (nokta kaydedilip öyle durulur), kurmadıysa program
// hemen durur. İkinci istek süreci hemen bitirir. Sınır: Windows konsol penceresi kapatılınca
// işletim sistemi süreci birkaç saniye içinde öldürür; ertelenen işler ve uzun bir adım bu
// sürede bitmeyebilir (gerçek güvence kontrol noktasının düzenli kaydıdır).
func catchInterrupt(machine *interp.VM) {
	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-channel
		if machine.RequestInterrupt() {
			machine.WriteOutput(os.Stderr, "\n⏹️ "+messages.Text(messages.H8660)+"\n")
		} else {
			machine.WriteOutput(os.Stderr, "\n⏹️ "+messages.Text(messages.H8661)+"\n")
		}
		<-channel
		fmt.Fprintln(os.Stderr, "⏹️ "+messages.Text(messages.H8662))
		os.Exit(immediateExitCode)
	}()
}

// reportInterrupt program kesme isteğiyle durduysa bildirimi yazar ve 3 koduyla çıkar.
// err programın hatasıdır (kesilmeyse ertelenen işlerin hataları da içinde olabilir).
func reportInterrupt(machine *interp.VM, err error, code string) {
	if err != nil && err != interp.ErrInterrupted {
		writeRunError(machine, err, code) // ertelenen işlerin hataları
	}
	machine.WriteOutput(os.Stderr, "⏹️ "+messages.Text(messages.H8663, interruptExitCode)+"\n")
	exit(interruptExitCode)
}
