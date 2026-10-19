package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/profiler"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// izleKomutu 'tr izle [--çıktı dosya] program.tr [argümanlar]': programı çalıştırır ve hızını
// canlı gösterir.
//
// Standart hata bir terminalse tablo ekranın üstünde saniyede bir yeniden çizilir;
// programın kendi çıktısı (yazdır, ilerleme çubuğu, hatalar) tabloyla karışmasın diye bir
// dosyaya yakalanır, son satırları tablonun altında görünür ve program bitince yeniden
// yazılır. Terminal değilse program çıktısı olduğu gibi akar, 10 saniyede bir düz özet
// satırı eklenir. Her iki durumda program bitince (hata ya da kesmeyle bitse de) son rapor
// standart hataya yazılır.
func izleKomutu(args []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") && !strings.EqualFold(filepath.Ext(args[0]), ".tr") {
		if _, err := os.Stat(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Hata: %q bir .tr dosyası değil. Çalışan bir arka plan işini izlemek (tr izle <iş_adı>) henüz yok; programı izleyerek çalıştırmak için: tr izle egit.tr\n", args[0])
			os.Exit(1)
		}
	}
	s, path, argumanlar := profilArgumanlari("izle", args, "--çıktı")
	machine, prog, code := programiHazirla(path, argumanlar)
	gercekCikis, gercekHata := os.Stdout, os.Stderr
	canli := profiler.YeniCanli(gercekHata, logging.TerminalMi(gercekHata), nil)
	var yak *profiler.Yakalama
	if canli.Terminal() {
		yol := s.cikti
		if yol == "" {
			ad := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			yol = filepath.Join(os.TempDir(), "tr-izle-"+ad+"-"+strconv.Itoa(os.Getpid())+".txt")
		}
		var err error
		if yak, err = profiler.YakalamaBaslat(yol); err != nil {
			canli.Kapat()
			fmt.Fprintf(os.Stderr, "Hata: programın çıktı dosyası açılamadı: %v\n", err)
			os.Exit(1)
		}
		canli.Yakala(yak)
		os.Stdout, os.Stderr = yak.Yazici(), yak.Yazici()
		machine.Out, machine.Err = yak.Yazici(), yak.Yazici()
	}
	p := olcumluProfilBaslat(machine, path, code, prog)
	p.Dinle(canli.Goster)
	cikisTemizligi = append(cikisTemizligi, func() {
		son := p.Bitir(cikisKodu)
		canli.Kapat()
		if yak != nil {
			os.Stdout, os.Stderr = gercekCikis, gercekHata
			yak.Kapat()
			fmt.Fprintf(gercekHata, "\nProgramın son çıktısı (tamamı: %s):\n", yak.Yol())
			for _, l := range yak.SonSatirlar(15) {
				fmt.Fprintln(gercekHata, "  │ "+l)
			}
		}
		fmt.Fprint(gercekHata, "\n"+profiler.RaporMetni(son))
	})
	programCalistir(machine, prog, code)
	temizle()
}
