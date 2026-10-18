package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"tr_dili/internal/dil/yurut"
)

// Nazik çıkış kodları: 3 "durduruldu" (ertelenen işler çalıştı; kontrol noktası kullanan
// bir eğitim kaldığı yerden sürdürülebilir), 130 ikinci kesme isteğiyle hemen çıkış.
const (
	kesilmeCikisKodu = 3
	hemenCikisKodu   = 130
)

// kesmeyiYakala Ctrl+C'yi ve sonlandırma isteklerini (Unix'te SIGTERM ve SIGHUP; Windows'ta
// Ctrl+Break ile konsol kapatma, oturum kapatma ve bilgisayarı kapatma olayları) yakalar.
// İlk istek programın nazikçe durmasını ister (VM.Kes); ikinci istek süreci hemen bitirir.
// Sınır: Windows konsol penceresi kapatılınca işletim sistemi süreci birkaç saniye içinde
// öldürür; ertelenen işler bu sürede bitmeyebilir.
func kesmeyiYakala(machine *yurut.VM) {
	kanal := make(chan os.Signal, 2)
	signal.Notify(kanal, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-kanal
		machine.Kes()
		machine.CiktiYaz(os.Stderr, "\n⏹️ Durduruluyor: ertelenen işler çalıştırılıp çıkılacak (hemen çıkmak için Ctrl+C'ye yeniden basın)\n")
		<-kanal
		fmt.Fprintln(os.Stderr, "⏹️ Hemen çıkıldı.")
		os.Exit(hemenCikisKodu)
	}()
}

// kesilmeyiBildir program kesme isteğiyle durduysa bildirimi yazar ve 3 koduyla çıkar.
// err programın hatasıdır (kesilmeyse ertelenen işlerin hataları da içinde olabilir).
func kesilmeyiBildir(machine *yurut.VM, err error, code string) {
	if err != nil && err != yurut.ErrKesildi {
		calismaHatasiYaz(machine, err, code) // ertelenen işlerin hataları
	}
	machine.CiktiYaz(os.Stderr, "⏹️ Program durduruldu (kesme isteği); ertelenen işler çalıştı. Çıkış kodu 3.\n")
	cik(kesilmeCikisKodu)
}
