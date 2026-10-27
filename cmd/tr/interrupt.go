package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// catchStop Ctrl+C'yi ve sonlandırma isteklerini yakalar (Unix'te SIGTERM, SIGHUP, SIGUSR1;
// Windows'ta Ctrl+Break ve konsol penceresinin kapanması; sinyaller signals_*.go'da). Windows
// oturum ve bilgisayar kapanma olaylarını yalnız hizmetlere gönderir; etkileşimli program bu
// olayları almadan sonlandırılır. İlk istek VM'ye iletilir (RequestStop): program nazikçe durur,
// üstlendiyse kaydedilir, kontrol noktası kurduysa adım sınırına ertelenir, biterken geldiyse
// ertelenen işleri kesmez (1.5, 1.6, 1.15). İkinci istek süreci hemen bitirir (1.7). Sınır:
// Windows konsol penceresi kapatılınca işletim sistemi süreci varsayılan olarak 5 saniye içinde
// öldürür; ertelenen işler ve uzun bir adım bu sürede bitmeyebilir (gerçek güvence kontrol
// noktasının düzenli kaydıdır, 1.8).
func catchStop(machine *interp.VM) {
	channel := make(chan os.Signal, 2)
	signal.Notify(channel, requestSignals...)
	go func() {
		first := <-channel
		machine.WriteOutput(os.Stderr, "\n⏹️ "+stopNotice(machine.RequestStop(signalNumber(first)))+"\n")
		second := <-channel
		fmt.Fprintln(os.Stderr, "⏹️ "+messages.Text(messages.H8662))
		// 1.7 "hemen" der: exitCleanup işleri (profilin son raporu, geçici klasörler) bilerek
		// çalıştırılmaz, takılan bir temizlik ikinci isteği de bekletirdi. Biten süreç gömülü
		// uygulamanın geçici klasörünü bırakmış olabilir; ilk istekten sonra normal yoldan
		// çıkılsaydı finishProgram temizliği yapardı.
		endBySignal(signalNumber(second))
	}()
}

// stopNotice isteğin nasıl karşılandığını kullanıcıya söyler.
func stopNotice(d interp.StopDisposition) string {
	switch d {
	case interp.StopHandled:
		return messages.Text(messages.H8666)
	case interp.StopDeferred:
		return messages.Text(messages.H8660)
	}
	return messages.Text(messages.H8661)
}
