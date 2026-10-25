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
	"text/tabwriter"
)

// Dilin TR ile yazılı kitaplığı ikiliye gömülüdür ve 'aktar "tr/..."' ile yüklenir (11.6);
// test koşucusu ve hata ayıklama sunucusu da bu ikilinin içinde çalıştığından onu görür.
func init() { interp.SetLibrary(lib.Files) }

const Version = "1.0.0"

// gcTarget çöp toplayıcının varsayılan hedefi (yığın, canlı belleğin bu yüzdesi kadar
// büyüyünce toplama başlar; Go'nun varsayılanı 100). Eğitimde her adım çok sayıda
// büyük ara tensör ayırır; 100'de karakter modeli adım başına ~0,8 toplama yapıyordu.
// Ölçüm (examples/tr/36_karakter_modeli.tr, GOMAXPROCS=2, eşli dönüşümlü 26 tur): 200 ile 20/26 turda
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
	// Ortam değişkenleri her yazım dilinin adıyla verilebilir (TR_GPU_SYNC = TR_GPU_ESZAMANLI);
	// hesap çekirdeği kaynak dildeki adı okur.
	codes.ExportEnvironment()
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
	command := commandCode(args[0])
	if command == codes.None {
		if replacement, ok := oldCommand(args[0]); ok {
			exitWithError(messages.Text(messages.H8602, args[0], replacement))
		}
	}
	if run, ok := commandsWithArguments[command]; ok {
		run(args[1:])
		return
	}
	switch command {
	case codes.K001:
		printHelp()
	case codes.K002:
		fmt.Println(messages.Text(messages.H8603, Version))
	case codes.K004:
		if len(args) < 2 {
			exitWithError(messages.Text(messages.H8605, codes.DisplayName(codes.K004), "ornek.tr"))
		}
		checkFile(args[1])
	case codes.K005:
		if len(args) < 2 {
			exitWithError(messages.Text(messages.H8606, codes.DisplayName(codes.K005)))
		}
		outPath := ""
		if len(args) >= 3 {
			outPath = args[2]
		}
		compileToExe(args[1], outPath)
	case codes.K014:
		// Düzenleyicinin hata ayıklama oturumu: iletiler standart giriş/çıkış üzerinden gelir.
		// Oturum bitince motorların açık kaynakları kapanır, geçici klasörler silinir.
		err := debugger.ServeDAP(os.Stdin, os.Stdout)
		stdlib.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8607, err))
			os.Exit(1)
		}
	case codes.K003:
		if len(args) < 2 {
			exitWithError(messages.Text(messages.H8605, codes.DisplayName(codes.K003), "ornek.tr"))
		}
		runFile(args[1], args[2:])
	default:
		runFile(args[0], args[1:])
	}
}

// commandCode komut satırının ilk argümanını bir tr komutunun koduna çevirir: kısa bayraklar
// (-h, -v) simgedir; adlar her yazım dilinde aranır (tr çalıştır da tr run da her makinede
// çalışır, betikler TR_DIL'den bağımsızdır). Komut değilse codes.None (dosya adıdır).
func commandCode(arg string) codes.Code {
	switch arg {
	case "-h":
		return codes.K001
	case "-v":
		return codes.K002
	}
	if k, ok := codes.FindCommand(arg); ok {
		return k
	}
	return codes.None
}

// oldCommand bir komutun eski yazımını (Türkçe harfsiz biçim, --help ve --version) tanır ve
// asıl adını verir. Eski yazım KABUL EDİLMEZ: dosya adı sanılıp çalıştırılmaya çalışılmaz.
func oldCommand(arg string) (string, bool) {
	switch arg {
	case "--help":
		return codes.DisplayName(codes.K001), true
	case "--version":
		return codes.DisplayName(codes.K002), true
	}
	if k, ok := codes.Display().Suggest(codes.NamespaceCommand, arg, nil); ok {
		return codes.DisplayName(k), true
	}
	return "", false
}

// commandsWithArguments argümanlarını kendisi ayrıştıran alt komutlardır (komut adından
// sonrakiler verilir).
var commandsWithArguments = map[codes.Code]func([]string){
	codes.K006: debugCommand, codes.K007: testCommand, codes.K008: formatCommand, codes.K009: encodeCommand,
	codes.K010: decodeCommand, codes.K011: showCommand, codes.K012: watchCommand, codes.K013: profileCommand,
	codes.K015: indexCommand,
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
	fmt.Fprint(os.Stderr, turkish.Printable(turkish.IndentLines(turkish.ErrorFrame(source, err), "    ")))
}

func printHelp() {
	fmt.Println(messages.Text(messages.H8610, Version))
	fmt.Println(messages.Text(messages.H8611))
	// Her satır tek iletidir: kullanım ve açıklama arasında sekme; komut ve seçenek adları
	// koddan, etkin dilde gelir. Sütunu tabwriter hizalar (adların uzunluğu dile göre değişir).
	lang := codes.OptionFlag(codes.K100)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, row := range []string{
		messages.Text(messages.H8612),
		messages.Text(messages.H8613),
		messages.Text(messages.H8614, codes.DisplayName(codes.K003)),
		messages.Text(messages.H8615, codes.DisplayName(codes.K006)),
		messages.Text(messages.H8616, codes.DisplayName(codes.K004)),
		messages.Text(messages.H8617, codes.DisplayName(codes.K005)),
		messages.Text(messages.H8618, codes.DisplayName(codes.K007)),
		messages.Text(messages.H8619, codes.DisplayName(codes.K008), codes.OptionFlag(codes.K101)),
		messages.Text(messages.H8620, codes.DisplayName(codes.K009)),
		messages.Text(messages.H8621, codes.DisplayName(codes.K010)),
		messages.Text(messages.H8622, codes.DisplayName(codes.K011), lang),
		messages.Text(messages.H8623, codes.DisplayName(codes.K011), lang),
		messages.Text(messages.H8624, codes.DisplayName(codes.K009), lang),
		messages.Text(messages.H8625, codes.DisplayName(codes.K012)),
		messages.Text(messages.H8626, codes.DisplayName(codes.K013), codes.OptionFlag(codes.K103)),
		messages.Text(messages.H8627, codes.DisplayName(codes.K014)),
		messages.Text(messages.H8830, codes.DisplayName(codes.K015), codes.DisplayName(codes.K016), codes.DisplayName(codes.K017), codes.DisplayName(codes.K018), codes.DisplayName(codes.K019)),
		messages.Text(messages.H8863, codes.DisplayName(codes.K015), codes.DisplayName(codes.K020)),
		messages.Text(messages.H8628, codes.DisplayName(codes.K001)),
		messages.Text(messages.H8629, codes.DisplayName(codes.K002)),
	} {
		fmt.Fprintln(w, "  "+row)
	}
	w.Flush()
}

// runFile programı çalıştırır; argumanlar dosya adından sonra yazılanlardır ve programa
// iletilir (tr betik.tr --adım 5).
func runFile(path string, arguments []string) {
	machine, program, text := prepareProgram(path, arguments, runPreparation)
	startJobProfile(machine, path, text, program)
	runProgram(machine, program, text)
	cleanup()
}

// debugCommand 'tr hata_ayıkla dosya.tr [argümanlar]' komutudur; dosyadan sonraki argümanlar
// programındır.
func debugCommand(args []string) {
	if len(args) < 1 {
		exitWithError(messages.Text(messages.H8608, codes.DisplayName(codes.K006)))
	}
	debugFile(args[0], args[1:])
}

func debugFile(path string, arguments []string) {
	machine, program, text := prepareProgram(path, arguments, debugPreparation)
	machine.Debugger = debugger.NewDebugger(path, text, true)
	runProgram(machine, program, text)
}

func checkFile(path string) {
	text, _ := prepareFile(path, runPreparation)
	// Programın aktardığı modüller de (çalıştırılmadan) aynı denetimden geçer.
	checkModules(path, text, runPreparation, "❌ "+messages.Text(messages.H8632))
	fmt.Println("✅ " + messages.Text(messages.H8631, path))
}

func startREPL() {
	defer stdlib.Close() // açık dosyalar kapanır, geçici klasörler silinir
	fmt.Println(messages.Text(messages.H8633, Version))
	fmt.Println(messages.Text(messages.H8634, codes.DisplayName(codes.K200)))
	machine := interp.New()
	// Kabuğa yazılan satırlar kişinin yazım dilindedir (source.Options.Typed); değerler de o
	// dilde metne çevrilir (3).
	machine.SourceLanguage = codes.Display().Code
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
		if k, ok := codes.FindCommand(line); ok && k == codes.K200 {
			fmt.Println(messages.Text(messages.H8635))
			break
		}
		if exitHint(line) {
			fmt.Fprintln(os.Stderr, messages.Text(messages.H8636, line, codes.DisplayName(codes.K200)))
			continue
		}
		if line == "/h" || line == "/H" {
			machine.Debugger = debugger.NewDebugger(messages.Text(messages.H8639), "", true)
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
		s := source.Prepare(line, source.Options{TypeCheck: true, Checker: chk, Typed: true})
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
		if val != nil && val.Type() != interp.ValNull {
			if !startsWithPrint(line) {
				fmt.Println("=>", interp.DisplayText(val)) // kabuğun yankısı görüntüdür (TR_DIL)
			}
		}
	}
}

// startsWithPrint satır yazdır yerleşiğinin (G100) kanonik ya da kişinin yazım dilindeki bir
// yazımıyla mı başlıyor (kabuk satırı kişinin dilinde yazılır)?
func startsWithPrint(ln string) bool {
	for _, y := range append(codes.Spellings(codes.G100), codes.Display().Spellings(codes.G100)...) {
		if strings.HasPrefix(ln, y) {
			return true
		}
	}
	return false
}

// exitHint kabukta çıkmak isteyen ama kabuğun çıkış komutunu (K200) yazmayan bir satırı tanır:
// adın Türkçe harfsiz biçimi (cikis) ya da hata ayıklayıcının çıkış komutu (K313, quit).
// Böyle bir satır program sanılıp çalıştırılmaz; asıl komut önerilir.
func exitHint(line string) bool {
	if k, ok := codes.Display().Suggest(codes.NamespaceCommand, line, nil); ok && k == codes.K200 {
		return true
	}
	k, ok := codes.FindDebuggerCommand(line)
	return ok && k == codes.K313
}
