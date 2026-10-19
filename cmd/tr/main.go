// tr, TR dilinin tek ikilisidir: alt komutları (çalıştır, kabuk, kontrol, derle, test,
// biçimle, kodla, çöz, göster, hata_ayıkla, dap) ayrıştırır ve işi internal/ altındaki
// paketlere bırakır.
//
// Katman: araç (cmd); dil, motor, hesap ve araç katmanlarını içe aktarabilir.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"tr_dili/internal/arac/ayikla"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/kaynak"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor"
	"tr_dili/kitaplik"
)

// Dilin TR ile yazılı kitaplığı ikiliye gömülüdür ve 'aktar "tr/..."' ile yüklenir (11.6);
// test koşucusu ve hata ayıklama sunucusu da bu ikilinin içinde çalıştığından onu görür.
func init() { yurut.KitaplikBagla(kitaplik.Dosyalar) }

const VERSION = "1.0.0"

// gcHedefi çöp toplayıcının varsayılan hedefi (yığın, canlı belleğin bu yüzdesi kadar
// büyüyünce toplama başlar; Go'nun varsayılanı 100). Eğitimde her adım çok sayıda
// büyük ara tensör ayırır; 100'de karakter modeli adım başına ~0,8 toplama yapıyordu.
// Ölçüm (ornekler/07, GOMAXPROCS=2, eşli dönüşümlü 26 tur): 200 ile 20/26 turda
// hızlı, ortanca −0,9…−1,5 ms/adım (~%6); tepe bellek 71 → 105 MB. Kullanıcının GOGC
// ortam değişkeni her zaman önceliklidir.
const gcHedefi = 200

func gcHedefiniAyarla() {
	if _, verildi := os.LookupEnv("GOGC"); !verildi {
		debug.SetGCPercent(gcHedefi)
	}
}

func main() {
	gcHedefiniAyarla()
	// Bu .exe 'tr derle' ile üretilmiş bağımsız bir uygulama ise gömülü paketi çalıştır.
	if pk, bulundu, err := gomuluPaketiBul(); bulundu {
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Gömülü uygulama okunamadı: %v\n", err)
			os.Exit(1)
		}
		gomuluUygulamayiCalistir(pk)
		return
	}

	args := os.Args[1:]

	if len(args) == 0 {
		startREPL()
		return
	}
	if asil, ok := kaldirilanKomutlar[args[0]]; ok {
		fmt.Fprintf(os.Stderr, "Hata: '%s' TR'de bir komut değil; 'tr %s' yazmak mı istediniz?\n", args[0], asil)
		os.Exit(1)
	}

	switch args[0] {
	case "yardım", "-h":
		printHelp()
	case "sürüm", "-v":
		fmt.Printf("TR Programlama ve Yapay Zeka Dili v%s\n", VERSION)
	case "bytecode", "baytkod":
		fmt.Fprintln(os.Stderr, "Hata: 'tr bytecode' komutu kaldırıldı.")
		os.Exit(1)
	case "kontrol":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Hata: Lütfen bir .tr dosyası belirtin. Örn: tr kontrol ornek.tr")
			os.Exit(1)
		}
		checkFile(args[1])
	case "derle":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Hata: Lütfen derlenecek .tr dosyasını belirtin. Örn: tr derle uygulama.tr")
			os.Exit(1)
		}
		outPath := ""
		if len(args) >= 3 {
			outPath = args[2]
		}
		compileToExe(args[1], outPath)
	case "hata_ayıkla":
		hataAyiklaKomutu(args[1:])
	case "test":
		testKomutu(args[1:])
	case "biçimle":
		bicimleKomutu(args[1:])
	case "kodla":
		kodlaKomutu(args[1:])
	case "çöz":
		cozKomutu(args[1:])
	case "göster":
		gosterKomutu(args[1:])
	case "dap":
		// Düzenleyicinin hata ayıklama oturumu: iletiler standart giriş/çıkış üzerinden gelir.
		// Oturum bitince motorların açık kaynakları kapanır, geçici klasörler silinir.
		err := ayikla.ServeDAP(os.Stdin, os.Stdout)
		motor.Kapat()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Hata ayıklama sunucusu: %v\n", err)
			os.Exit(1)
		}
	case "çalıştır":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Hata: Lütfen bir .tr dosyası belirtin. Örn: tr çalıştır ornek.tr")
			os.Exit(1)
		}
		runFile(args[1], args[2:])
	default:
		runFile(args[0], args[1:])
	}
}

// kaldirilanKomutlar kaldırılan komut adlarının (Türkçe karaktersiz ve İngilizce yazımlar)
// asıl adlarıdır. Her komutun tek Türkçe adı vardır; kısa bayraklar -h ve -v simgedir, kalır.
// Bu adlar KABUL EDİLMEZ: dosya adı sanılıp çalıştırılmaya çalışılmaz, asıl komut önerilir.
var kaldirilanKomutlar = map[string]string{
	"calistir": "çalıştır", "bicimle": "biçimle", "hata_ayikla": "hata_ayıkla", "debug": "hata_ayıkla",
	"yardim": "yardım", "--help": "yardım", "help": "yardım", "surum": "sürüm", "--version": "sürüm",
	"version": "sürüm", "coz": "çöz", "goster": "göster",
}

// printError hatayı madde olarak yazar; konumu biliniyorsa kaynak satırını işaretçiyle gösterir.
func printError(msg, source string) {
	fmt.Fprintf(os.Stderr, "  • %s\n", msg)
	fmt.Fprint(os.Stderr, indentFrame(turkce.KodCercevesi(source, msg)))
}

// indentFrame kod çerçevesini madde işaretinin altına hizalar.
func indentFrame(frame string) string {
	if frame == "" {
		return ""
	}
	var sb strings.Builder
	for _, l := range strings.Split(strings.TrimRight(frame, "\n"), "\n") {
		sb.WriteString("    " + l + "\n")
	}
	return sb.String()
}

func printHelp() {
	fmt.Println("🇹🇷 TR (.tr) Programlama ve Yapay Zeka Dili v" + VERSION)
	fmt.Println("Kullanım:")
	fmt.Println("  tr                          Etkileşimli TR kabuğunu (REPL) başlatır")
	fmt.Println("  tr <dosya.tr> [argümanlar]  .tr kaynak dosyasını derler ve çalıştırır (argümanlar programa iletilir)")
	fmt.Println("  tr çalıştır <dosya.tr>      Aynısı (komut adıyla)")
	fmt.Println("  tr hata_ayıkla <dosya.tr>   Etkileşimli TR Hata Ayıklayıcısını başlatır")
	fmt.Println("  tr kontrol <dosya.tr>       Sözdizimi ve akıllı tip kontrolü yapar")
	fmt.Println("  tr derle <dosya.tr>         .tr dosyasını (aktardığı modüllerle) tek başına çalışan bağımsız .exe dosyasına dönüştürür")
	fmt.Println("  tr test [dosya.tr|klasör]   Test bloklarını çalıştırır (klasörde tüm *_test.tr dosyaları)")
	fmt.Println("  tr biçimle [--kontrol] <dosya.tr|klasör>  Kodu standart biçime getirir (--kontrol: yalnızca denetler)")
	fmt.Println("  tr kodla <dosya.tr|klasör>  Dosyaları yerinde kodlu biçime çevirir (dilin sözcükleri kodlarla saklanır)")
	fmt.Println("  tr çöz <dosya.tr|klasör>    Kodlu dosyaları yerinde Türkçe yazıma geri çevirir")
	fmt.Println("  tr göster <dosya.tr> [--dil tr]  Dosyayı bir yazım dilinde gösterir (kodlu dosya da)")
	fmt.Println("  tr göster - [--dil tr]      Metni standart girişten okur, gösterimini çıkışa yazar (düzenleyiciler için)")
	fmt.Println("  tr kodla - [--dil tr]       Metni standart girişten okur, kodlu biçimini çıkışa yazar (düzenleyiciler için)")
	fmt.Println("  tr dap                      düzenleyicinin hata ayıklama sunucusu (eklenti kendisi başlatır)")
	fmt.Println("  tr yardım | -h              Bu yardımı gösterir")
	fmt.Println("  tr sürüm | -v               Sürümü gösterir")
}

// runFile programı çalıştırır; argumanlar dosya adından sonra yazılanlardır ve programa
// iletilir (tr betik.tr --adım 5).
func runFile(path string, argumanlar []string) {
	_, s := dosyaHazirla(path, calistirmaHazirligi)
	machine := yurut.New()
	machine.SourceFile = path
	machine.SourceCode = s.Metin
	machine.ArgumanlariAyarla(argumanlar)
	programCalistir(machine, s.Program, s.Metin)
}

// hataAyiklaKomutu 'tr hata_ayıkla dosya.tr [argümanlar]' komutudur. Kaldırılan tarayıcı
// hata ayıklayıcısının bayrağı dosyadan önce ya da hemen sonra yazılırsa ipucu verilir;
// sonraki argümanlar programındır.
func hataAyiklaKomutu(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Hata: Lütfen hata ayıklanacak .tr dosyasını belirtin. Örn: tr hata_ayıkla ornek.tr")
		os.Exit(1)
	}
	for _, a := range args[:min(2, len(args))] {
		if a == "--web" || a == "-w" {
			fmt.Fprintln(os.Stderr, "Hata: Tarayıcıdaki hata ayıklayıcı kaldırıldı. Düzenleyicide F5 ile hata ayıklayın (İzleme Masası) ya da terminalde: tr hata_ayıkla ornek.tr")
			os.Exit(1)
		}
	}
	debugFile(args[0], args[1:])
}

func debugFile(path string, argumanlar []string) {
	_, s := dosyaHazirla(path, hataAyiklamaHazirligi)
	machine := yurut.New()
	machine.SourceFile = path
	machine.SourceCode = s.Metin
	machine.ArgumanlariAyarla(argumanlar)
	dbg := ayikla.NewDebugger(path, s.Metin, true)
	machine.Debugger = dbg

	programCalistir(machine, s.Program, s.Metin)
}

func checkFile(path string) {
	metin, _ := dosyaHazirla(path, calistirmaHazirligi)
	// Programın aktardığı modüller de (çalıştırılmadan) aynı denetimden geçer.
	modulleriDenetle(path, metin, calistirmaHazirligi, "❌ Modül hatası:")
	fmt.Printf("✅ %s başarıyla denetlendi: Sözdizimi ve tip güvenliği hatasız!\n", path)
}

func startREPL() {
	defer motor.Kapat() // açık dosyalar kapanır, geçici klasörler silinir
	fmt.Printf("🇹🇷 TR Programlama ve Yapay Zeka Dili (v%s) — Etkileşimli Kabuk\n", VERSION)
	fmt.Println("Çıkmak için 'çıkış', hata ayıklayıcıyı açmak için '/h' yazın.")
	machine := yurut.New()
	// Satırlar girdi() ile aynı tamponlu okuyucudan okunur; ayrı bir okuyucu, programın
	// okuyacağı satırları kendi tamponuna çekip yutardı.
	okuyucu := machine.GirdiOkuyucu()
	chk := denetci.New()
	oncekiTipHatalari, oncekiUyarilar := 0, 0

	for {
		fmt.Print("tr> ")
		satir, okumaHatasi := okuyucu.ReadString('\n')
		if satir == "" && okumaHatasi != nil {
			break
		}
		line := strings.TrimSpace(satir)
		if line == "" {
			continue
		}
		if line == "çıkış" {
			fmt.Println("Görüşmek üzere!")
			break
		}
		if line == "cikis" || line == "exit" || line == "quit" {
			fmt.Fprintf(os.Stderr, "'%s' TR'de bir komut değil; çıkmak için 'çıkış' yazın.\n", line)
			continue
		}
		if line == "/h" || line == "/H" {
			machine.Debugger = ayikla.NewDebugger("<etkileşimli-kabuk>", "", true)
			fmt.Println("🐞 [/h] Hata ayıklama modu etkinleştirildi! Bir sonraki komut adım adım çalışacak (F11 adım, F10 üstten geç, Shift+F11 dön, F5 devam).")
			continue
		}

		// kesme_noktası deyimi de hata ayıklayıcıyı açmış olabilir
		if dbg, ok := machine.Debugger.(*ayikla.Debugger); ok {
			dbg.SourceLines = strings.Split(line, "\n")
		}

		// Hatalar dosya çalıştırmadakiyle aynı başlık ve biçimle standart hataya yazılır.
		// Denetleyici satırlar arasında durumunu (ve hata listesini) korur; yalnızca bu
		// satırın getirdiği yeni hatalar gösterilir.
		s := kaynak.Hazirla(line, kaynak.Secenekler{TipDenetimi: true, Denetleyici: chk})
		if s.Asama == kaynak.AsamaSozdizimi {
			hatalariYaz(sozdizimiBasligi, s.Hatalar, s.Metin)
			continue
		}
		if u := s.Uyarilar; len(u) > oncekiUyarilar {
			uyarilariYaz(u[oncekiUyarilar:], s.Metin)
			oncekiUyarilar = len(u)
		}
		if errs := s.Hatalar; len(errs) > oncekiTipHatalari {
			yeni := errs[oncekiTipHatalari:]
			oncekiTipHatalari = len(errs)
			hatalariYaz(tipBasligi, yeni, s.Metin)
			continue
		}

		val, err := machine.Run(s.Program)
		if err != nil {
			calismaHatasiYaz(machine, err, line)
			continue
		}
		machine.WaitAsync()
		if val != nil && val.Type() != yurut.VAL_NULL {
			if !yazdirIleBaslar(line) {
				fmt.Println("=>", val.Inspect())
			}
		}
	}
}

// yazdirIleBaslar satır yazdır yerleşiğinin (G100) bir yazımıyla mı başlıyor?
func yazdirIleBaslar(satir string) bool {
	for _, y := range kod.Yazimlar(kod.G100) {
		if strings.HasPrefix(satir, y) {
			return true
		}
	}
	return false
}
