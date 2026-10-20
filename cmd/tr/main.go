// tr, TR dilinin tek ikilisidir: alt komutları (çalıştır, kabuk, kontrol, derle, test,
// biçimle, kodla, çöz, göster, izle, profil, hata_ayıkla, dap) ayrıştırır ve işi internal/
// altındaki paketlere bırakır.
//
// Layer: command (cmd); dil, motor, hesap ve araç katmanlarını içe aktarabilir.
package main

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/devtools/debugger"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/lib"
	"os"
	"runtime/debug"
	"strings"
)

// Dilin TR ile yazılı kitaplığı ikiliye gömülüdür ve 'aktar "tr/..."' ile yüklenir (11.6);
// test koşucusu ve hata ayıklama sunucusu da bu ikilinin içinde çalıştığından onu görür.
func init() { interp.SetLibrary(lib.Dosyalar) }

const VERSION = "1.0.0"

// gcTarget çöp toplayıcının varsayılan hedefi (yığın, canlı belleğin bu yüzdesi kadar
// büyüyünce toplama başlar; Go'nun varsayılanı 100). Eğitimde her adım çok sayıda
// büyük ara tensör ayırır; 100'de karakter modeli adım başına ~0,8 toplama yapıyordu.
// Ölçüm (examples/07, GOMAXPROCS=2, eşli dönüşümlü 26 tur): 200 ile 20/26 turda
// hızlı, ortanca −0,9…−1,5 ms/adım (~%6); tepe bellek 71 → 105 MB. Kullanıcının GOGC
// env değişkeni her zaman önceliklidir.
const gcTarget = 200

func setGCTarget() {
	if _, given := os.LookupEnv("GOGC"); !given {
		debug.SetGCPercent(gcTarget)
	}
}

func main() {
	// Geçersiz TR_DIL: codes paketi Türkçeye düştü; komut yanlış dille çalışmasın.
	if err := codes.LanguageError(); err != nil {
		fmt.Fprintln(os.Stderr, messages.Text(messages.H8060, err))
		os.Exit(2)
	}
	setGCTarget()
	// Bu .exe 'tr derle' ile üretilmiş bağımsız bir uygulama ise gömülü paketi çalıştır.
	if pk, found, err := findEmbeddedPackage(); found {
		if err != nil {
			fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8601, err))
			os.Exit(1)
		}
		runEmbeddedApp(pk)
		return
	}

	args := os.Args[1:]

	if len(args) == 0 {
		startREPL()
		return
	}
	if replacement, ok := removedCommands[args[0]]; ok {
		exitWithError(messages.Text(messages.H8602, args[0], replacement))
	}

	if command, ok := commandsWithArguments[args[0]]; ok {
		command(args[1:])
		return
	}
	switch args[0] {
	case "yardım", "-h":
		printHelp()
	case "sürüm", "-v":
		fmt.Println(messages.Text(messages.H8603, VERSION))
	case "bytecode", "baytkod":
		exitWithError(messages.Text(messages.H8604))
	case "kontrol":
		if len(args) < 2 {
			exitWithError(messages.Text(messages.H8605, "kontrol", "ornek.tr"))
		}
		checkFile(args[1])
	case "derle":
		if len(args) < 2 {
			exitWithError(messages.Text(messages.H8606))
		}
		outPath := ""
		if len(args) >= 3 {
			outPath = args[2]
		}
		compileToExe(args[1], outPath)
	case "dap":
		// Düzenleyicinin hata ayıklama oturumu: iletiler standart giriş/çıkış üzerinden gelir.
		// Oturum bitince motorların açık kaynakları kapanır, geçici klasörler silinir.
		err := debugger.ServeDAP(os.Stdin, os.Stdout)
		stdlib.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8607, err))
			os.Exit(1)
		}
	case "çalıştır":
		if len(args) < 2 {
			exitWithError(messages.Text(messages.H8605, "çalıştır", "ornek.tr"))
		}
		runFile(args[1], args[2:])
	default:
		runFile(args[0], args[1:])
	}
}

// commandsWithArguments argümanlarını kendisi ayrıştıran alt komutlardır (komut adından
// sonrakiler verilir).
var commandsWithArguments = map[string]func([]string){
	"hata_ayıkla": debugCommand, "test": testCommand, "biçimle": formatCommand, "kodla": encodeCommand,
	"çöz": decodeCommand, "göster": showCommand, "izle": watchCommand, "profil": profileCommand,
}

// removedCommands kaldırılan komut adlarının (Türkçe karaktersiz ve İngilizce yazımlar)
// asıl adlarıdır. Her komutun tek Türkçe adı vardır; kısa bayraklar -h ve -v simgedir, kalır.
// Bu adlar KABUL EDİLMEZ: dosya adı sanılıp çalıştırılmaya çalışılmaz, asıl komut önerilir.
var removedCommands = map[string]string{
	"calistir": "çalıştır", "bicimle": "biçimle", "hata_ayikla": "hata_ayıkla", "debug": "hata_ayıkla",
	"yardim": "yardım", "--help": "yardım", "help": "yardım", "surum": "sürüm", "--version": "sürüm",
	"version": "sürüm", "coz": "çöz", "goster": "göster", "watch": "izle", "profile": "profil",
}

// exitWithError iletiyi "Hata: ..." olarak standart hataya yazar ve 1 koduyla çıkar.
func exitWithError(message any) {
	fmt.Fprintln(os.Stderr, turkish.Printable(messages.Text(messages.H8060, message)))
	os.Exit(1)
}

// printError hatayı madde olarak yazar; konumu biliniyorsa kaynak satırını işaretçiyle gösterir.
func printError(err error, source string) {
	// Denetim karakterleri görünür yazılır: iletideki dış veri terminali bozamaz (8.4).
	fmt.Fprintf(os.Stderr, "  • %s\n", turkish.Printable(err.Error()))
	fmt.Fprint(os.Stderr, turkish.Printable(indentFrame(turkish.ErrorFrame(source, err))))
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
	fmt.Println("🇹🇷 " + messages.Text(messages.H8610, VERSION))
	fmt.Println(messages.Text(messages.H8611))
	fmt.Println("  " + messages.Text(messages.H8612))
	fmt.Println("  " + messages.Text(messages.H8613))
	fmt.Println("  " + messages.Text(messages.H8614))
	fmt.Println("  " + messages.Text(messages.H8615))
	fmt.Println("  " + messages.Text(messages.H8616))
	fmt.Println("  " + messages.Text(messages.H8617))
	fmt.Println("  " + messages.Text(messages.H8618))
	fmt.Println("  " + messages.Text(messages.H8619))
	fmt.Println("  " + messages.Text(messages.H8620))
	fmt.Println("  " + messages.Text(messages.H8621))
	fmt.Println("  " + messages.Text(messages.H8622))
	fmt.Println("  " + messages.Text(messages.H8623))
	fmt.Println("  " + messages.Text(messages.H8624))
	fmt.Println("  " + messages.Text(messages.H8625))
	fmt.Println("  " + messages.Text(messages.H8626))
	fmt.Println("  " + messages.Text(messages.H8627))
	fmt.Println("  " + messages.Text(messages.H8628))
	fmt.Println("  " + messages.Text(messages.H8629))
}

// runFile programı çalıştırır; argumanlar dosya adından sonra yazılanlardır ve programa
// iletilir (tr betik.tr --adım 5).
func runFile(path string, arguments []string) {
	_, s := prepareFile(path, runPreparation)
	machine := interp.New()
	machine.SourceFile = path
	machine.SourceCode = s.Text
	machine.SetArgs(arguments)
	startJobProfile(machine, path, s.Text, s.Program)
	runProgram(machine, s.Program, s.Text)
	cleanup()
}

// debugCommand 'tr hata_ayıkla dosya.tr [argümanlar]' komutudur. Kaldırılan tarayıcı
// failure ayıklayıcısının bayrağı dosyadan önce ya da hemen sonra yazılırsa ipucu verilir;
// sonraki argümanlar programındır.
func debugCommand(args []string) {
	if len(args) < 1 {
		exitWithError(messages.Text(messages.H8608))
	}
	for _, a := range args[:min(2, len(args))] {
		if a == "--web" || a == "-w" {
			exitWithError(messages.Text(messages.H8609))
		}
	}
	debugFile(args[0], args[1:])
}

func debugFile(path string, arguments []string) {
	_, s := prepareFile(path, debugPreparation)
	machine := interp.New()
	machine.SourceFile = path
	machine.SourceCode = s.Text
	machine.SetArgs(arguments)
	dbg := debugger.NewDebugger(path, s.Text, true)
	machine.Debugger = dbg

	runProgram(machine, s.Program, s.Text)
}

func checkFile(path string) {
	text, _ := prepareFile(path, runPreparation)
	// Programın aktardığı modüller de (çalıştırılmadan) aynı denetimden geçer.
	checkModules(path, text, runPreparation, "❌ "+messages.Text(messages.H8632))
	fmt.Println("✅ " + messages.Text(messages.H8631, path))
}

func startREPL() {
	defer stdlib.Close() // açık dosyalar kapanır, geçici klasörler silinir
	fmt.Println("🇹🇷 " + messages.Text(messages.H8633, VERSION))
	fmt.Println(messages.Text(messages.H8634))
	machine := interp.New()
	// Satırlar girdi() ile aynı tamponlu okuyucudan okunur; ayrı bir okuyucu, programın
	// okuyacağı satırları kendi tamponuna çekip yutardı.
	reader := machine.InputReader()
	chk := checker.New()
	chk.SetInteractive()
	previousTypeErrors, previousWarnings := 0, 0

	for {
		fmt.Print("tr> ")
		ln, readError := reader.ReadString('\n')
		if ln == "" && readError != nil {
			break
		}
		line := strings.TrimSpace(ln)
		if line == "" {
			continue
		}
		if line == "çıkış" {
			fmt.Println(messages.Text(messages.H8635))
			break
		}
		if line == "cikis" || line == "exit" || line == "quit" {
			fmt.Fprintln(os.Stderr, messages.Text(messages.H8636, line))
			continue
		}
		if line == "/h" || line == "/H" {
			machine.Debugger = debugger.NewDebugger("<etkileşimli-kabuk>", "", true)
			fmt.Println("🐞 " + messages.Text(messages.H8637))
			continue
		}

		// kesme_noktası deyimi de hata ayıklayıcıyı açmış olabilir
		if dbg, ok := machine.Debugger.(*debugger.Debugger); ok {
			dbg.SourceLines = strings.Split(line, "\n")
		}

		// Hatalar dosya çalıştırmadakiyle aynı başlık ve biçimle standart hataya yazılır.
		// Denetleyici satırlar arasında durumunu (ve hata listesini) korur; yalnızca bu
		// satırın getirdiği yeni hatalar gösterilir.
		s := source.Prepare(line, source.Options{TypeCheck: true, Checker: chk})
		if s.Stage == source.StageSyntax {
			writeDiags(syntaxHeader, s.Errors, s.Text)
			continue
		}
		if u := s.Warnings; len(u) > previousWarnings {
			writeWarnings(u[previousWarnings:], s.Text)
			previousWarnings = len(u)
		}
		if errs := s.Errors; len(errs) > previousTypeErrors {
			updated := errs[previousTypeErrors:]
			previousTypeErrors = len(errs)
			writeDiags(typeHeader, updated, s.Text)
			continue
		}

		val, err := machine.Run(s.Program)
		if err != nil {
			writeRunError(machine, err, line)
			continue
		}
		machine.WaitAsync()
		if val != nil && val.Type() != interp.VAL_NULL {
			if !startsWithPrint(line) {
				fmt.Println("=>", val.Inspect())
			}
		}
	}
}

// startsWithPrint satır yazdır yerleşiğinin (G100) bir yazımıyla mı başlıyor?
func startsWithPrint(ln string) bool {
	for _, y := range codes.Spellings(codes.G100) {
		if strings.HasPrefix(ln, y) {
			return true
		}
	}
	return false
}
