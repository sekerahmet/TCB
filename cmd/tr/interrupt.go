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
	kesilmeCikisKodu = 3
	hemenCikisKodu   = 130
)

// kesmeyiYakala Ctrl+C'yi ve sonlandırma isteklerini (Unix'te SIGTERM ve SIGHUP; Windows'ta
// Ctrl+Break ile konsol kapatma, oturum kapatma ve bilgisayarı kapatma olayları) yakalar.
// İlk istek programın nazikçe durmasını ister (VM.KesmeIste): program bir kontrol noktası
// kurduysa istek adım sınırına ertelenir (nokta kaydedilip öyle durulur), kurmadıysa program
// hemen durur. İkinci istek süreci hemen bitirir. Sınır: Windows konsol penceresi kapatılınca
// işletim sistemi süreci birkaç saniye içinde öldürür; ertelenen işler ve uzun bir adım bu
// sürede bitmeyebilir (gerçek güvence kontrol noktasının düzenli kaydıdır).
func kesmeyiYakala(machine *interp.VM) {
	kanal := make(chan os.Signal, 2)
	signal.Notify(kanal, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-kanal
		if machine.RequestInterrupt() {
			machine.WriteOutput(os.Stderr, "\n⏹️ "+messages.Text(messages.H8660)+"\n")
		} else {
			machine.WriteOutput(os.Stderr, "\n⏹️ "+messages.Text(messages.H8661)+"\n")
		}
		<-kanal
		fmt.Fprintln(os.Stderr, "⏹️ "+messages.Text(messages.H8662))
		os.Exit(hemenCikisKodu)
	}()
}

// kesilmeyiBildir program kesme isteğiyle durduysa bildirimi yazar ve 3 koduyla çıkar.
// err programın hatasıdır (kesilmeyse ertelenen işlerin hataları da içinde olabilir).
func kesilmeyiBildir(machine *interp.VM, err error, code string) {
	if err != nil && err != interp.ErrInterrupted {
		calismaHatasiYaz(machine, err, code) // ertelenen işlerin hataları
	}
	machine.WriteOutput(os.Stderr, "⏹️ "+messages.Text(messages.H8663, kesilmeCikisKodu)+"\n")
	cik(kesilmeCikisKodu)
}
