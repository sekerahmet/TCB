package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/profiler"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// 'tr profil' ve 'tr izle' komutlarının ortak parçaları ve 'tr profil'. Profilin çekirdeği
// internal/devtools/profiler'dedir; burada yalnızca argümanlar, çıktı yerleri ve çıkış vardır.

// profilSecenekleri komutların program dosyasından önce yazılan seçenekleridir.
type profilSecenekleri struct {
	json     string // --json: raporun JSON yolu (klasörse içine speed.json)
	goProfil string // --go-profili: yorumlayıcının Go CPU profili (geliştiriciler için)
	cikti    string // --çıktı: tr izle'de programın çıktısının dosyası
}

// profilArgumanlari seçenekleri, program dosyasını ve programın argümanlarını ayırır.
// izinli, komutun kabul ettiği seçeneklerdir. Hata varsa yazıp 1 ile çıkar.
func profilArgumanlari(komut string, args []string, izinli ...string) (profilSecenekleri, string, []string) {
	var s profilSecenekleri
	hedef := map[string]*string{"--json": &s.json, "--go-profili": &s.goProfil, "--çıktı": &s.cikti}
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		ad, deger, esitli := strings.Cut(args[0], "=")
		p, ok := hedef[ad]
		if !ok || !icinde(izinli, ad) {
			hataylaCik(messages.Text(messages.H8674, komut, ad, strings.Join(izinli, ", ")))
		}
		args = args[1:]
		if !esitli {
			if len(args) == 0 {
				hataylaCik(messages.Text(messages.H8675, ad, komut))
			}
			deger, args = args[0], args[1:]
		}
		*p = deger
	}
	if len(args) == 0 {
		hataylaCik(messages.Text(messages.H8605, komut, "egit.tr"))
	}
	return s, args[0], args[1:]
}

func icinde(liste []string, s string) bool {
	for _, x := range liste {
		if x == s {
			return true
		}
	}
	return false
}

// programiHazirla dosyayı hazırlar ve bir VM kurar (runFile ile aynı adımlar).
func programiHazirla(path string, argumanlar []string) (*interp.VM, *ast.Program, string) {
	_, s := dosyaHazirla(path, calistirmaHazirligi)
	machine := interp.New()
	machine.SourceFile = path
	machine.SourceCode = s.Text
	machine.SetArgs(argumanlar)
	return machine, s.Program, s.Text
}

// uyariYazici profil uyarılarını programın çıktı noktasından (canlı satırla eşgüdümlü)
// standart hataya yazar.
func uyariYazici(machine *interp.VM) func(string) {
	return func(m string) { machine.WriteOutput(os.Stderr, m+"\n") }
}

// olcumluProfilBaslat tr izle ve tr profil'in profilini başlatır: satır örneklemesi ve GPU
// çekirdek ölçümü açık; arka plan işiyse durum dosyası da yazılır. GPU'nun son ölçümleri
// motorlar kapanmadan alınır (kapanmadanOnce).
func olcumluProfilBaslat(machine *interp.VM, path, code string, prog *ast.Program) *profiler.Profile {
	p := profiler.Start(machine, profiler.Options{
		Program: path, Source: code, SampleLines: true, GPUMeasure: true,
		StatusFile: profiler.JobStatusFile(), Warning: uyariYazici(machine),
	})
	kapanmadanOnce = append(kapanmadanOnce, p.BeforeClose)
	return p
}

// profilKomutu 'tr profil [--json yol] [--go-profili dosya] program.tr [argümanlar]':
// programı satır örneklemesiyle çalıştırır, bitince (hata ya da kesmeyle bitse de) metin
// raporu standart çıktıya, istenmişse JSON raporu dosyaya yazar.
func profilKomutu(args []string) {
	s, path, argumanlar := profilArgumanlari("profil", args, "--json", "--go-profili")
	machine, prog, code := programiHazirla(path, argumanlar)
	if s.goProfil != "" {
		goProfiliniBaslat(s.goProfil)
	}
	p := olcumluProfilBaslat(machine, path, code, prog)
	cikisTemizligi = append(cikisTemizligi, func() {
		son := p.Finish(cikisKodu)
		fmt.Print("\n" + profiler.ReportText(son))
		if s.json != "" {
			jsonRaporuYaz(s.json, son)
		}
	})
	programCalistir(machine, prog, code)
	temizle()
}

// jsonRaporuYaz raporu yola (klasörse içine speed.json) yazar; yazılamazsa bildirir.
func jsonRaporuYaz(yol string, g *profiler.Snapshot) {
	if b, err := os.Stat(yol); (err == nil && b.IsDir()) || strings.HasSuffix(yol, "/") || strings.HasSuffix(yol, string(os.PathSeparator)) {
		if err := os.MkdirAll(yol, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "⚠️ "+messages.Text(messages.H8219, messages.Text(messages.H8676, yol, err)))
			return
		}
		yol = filepath.Join(yol, profiler.ReportFileName)
	}
	if err := profiler.WriteFile(yol, g, func(m string) { fmt.Fprintln(os.Stderr, m) }); err != nil {
		fmt.Fprintln(os.Stderr, "⚠️ "+messages.Text(messages.H8219, messages.Text(messages.H8677, err)))
		return
	}
	fmt.Fprintln(os.Stderr, messages.Text(messages.H8678, yol))
}

// goProfiliniBaslat yorumlayıcının kendi Go CPU profilini yazmaya başlar (geliştiriciler
// için; 'go tool pprof' ile okunur).
func goProfiliniBaslat(yol string) {
	f, err := os.Create(yol)
	if err != nil {
		hataylaCik(messages.Text(messages.H8679, err))
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		hataylaCik(messages.Text(messages.H8680, err))
	}
	cikisTemizligi = append(cikisTemizligi, func() {
		pprof.StopCPUProfile()
		f.Close()
	})
}

// isProfiliniBaslat program bir arka plan işi olarak çalışıyorsa (TR_İŞ) iş klasörüne
// saniyede bir profil.json yazar (yalnızca her zaman açık sayaçlar; satır örneklemesi
// yok: ek maliyet saniyede bir dosya yazmasıdır). İş değilse hiçbir şey yapmaz.
func isProfiliniBaslat(machine *interp.VM, path, code string, prog *ast.Program) {
	yol := profiler.JobStatusFile()
	if yol == "" {
		return
	}
	p := profiler.Start(machine, profiler.Options{
		Program: path, Source: code, StatusFile: yol, Warning: uyariYazici(machine),
	})
	cikisTemizligi = append(cikisTemizligi, func() { p.Finish(cikisKodu) })
}
