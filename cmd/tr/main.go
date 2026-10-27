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
	// hesap çekirdeği kimlik adını (TR_env_GPU_SYNC) okur. İki ad ayrı değerle verilmişse durulur.
	if err := codes.ExportEnvironment(); err != nil {
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
	command := commandCode(args[0])
	if run, ok := commands[command]; ok {
		run(args[1:])
		return
	}
	if replacement, ok := oldCommand(args[0]); ok {
		exitWithError(messages.Text(messages.H8602, args[0], replacement))
	}
	if looksLikeOption(args[0]) {
		unknownTopLevelOption(args[0])
	}
	if args[0] == "--" {
		args = args[1:] // adı '-' ile başlayan dosya: tr -- -x.tr
		if len(args) == 0 {
			startREPL()
			return
		}
	}
	runFile(args[0], args[1:])
}

// commandCode komut satırının ilk argümanını bir tr komutunun koduna çevirir: kısa bayraklar
// (-h, -v) simgedir; adlar her yazım dilinde aranır (tr çalıştır da tr run da her makinede
// çalışır, betikler TR_DIL'den bağımsızdır). Komut değilse codes.None (dosya adıdır).
func commandCode(arg string) codes.Code {
	switch arg {
	case "-h":
		return codes.CmdHelp
	case "-v":
		return codes.CmdVersion
	}
	if k, ok := codes.FindCommand(arg); ok {
		return k
	}
	return codes.None
}

// oldCommand bir komutun eski yazımını (Türkçe harfsiz biçim) tanır ve asıl adını verir. Eski
// yazım KABUL EDİLMEZ: dosya adı sanılıp çalıştırılmaya çalışılmaz. Komut adının seçenek
// biçimi (--help, --yardım) unknownTopLevelOption'dadır.
func oldCommand(arg string) (string, bool) {
	if k, ok := codes.Display().Suggest(codes.NamespaceCommand, arg, nil); ok {
		return codes.DisplayName(k), true
	}
	return "", false
}

// commands tr'nin bütün alt komutlarıdır; her biri komut adından sonraki argümanları alır.
var commands = map[codes.Code]func([]string){
	codes.CmdHelp: helpCommand, codes.CmdVersion: versionCommand,
	codes.CmdCheck: checkCommand, codes.CmdBuild: buildCommand, codes.CmdDap: dapCommand, codes.CmdRun: runCommand,
	codes.CmdDebug: debugCommand, codes.CmdTest: testCommand, codes.CmdFormat: formatCommand, codes.CmdEncode: encodeCommand,
	codes.CmdShow: showCommand, codes.CmdWatch: watchCommand, codes.CmdProfile: profileCommand,
	codes.CmdIndex: indexCommand,
}

// helpCommand 'tr yardım' komutudur.
func helpCommand(args []string) {
	noOptions(codes.CmdHelp, args)
	printHelp()
}

// versionCommand 'tr sürüm' komutudur.
func versionCommand(args []string) {
	noOptions(codes.CmdVersion, args)
	fmt.Println(messages.Text(messages.H8603, Version))
}

// checkCommand 'tr kontrol dosya.tr' komutudur.
func checkCommand(args []string) {
	file, _, ok := programFile(codes.NameRef(codes.CmdCheck), args)
	if !ok {
		exitWithError(messages.Text(messages.H8605, codes.NameRef(codes.CmdCheck), "ornek.tr"))
	}
	checkFile(file)
}

// buildCommand 'tr derle dosya.tr [çıktı]' komutudur.
func buildCommand(args []string) {
	files := fileArguments(codes.NameRef(codes.CmdBuild), args, nil, nil)
	if len(files) < 1 {
		exitWithError(messages.Text(messages.H8606, codes.NameRef(codes.CmdBuild)))
	}
	outPath := ""
	if len(files) >= 2 {
		outPath = files[1]
	}
	compileToExe(files[0], outPath)
}

// dapCommand düzenleyicinin hata ayıklama oturumudur: iletiler standart giriş/çıkış üzerinden
// gelir. Oturum bitince motorların açık kaynakları kapanır, geçici klasörler silinir.
func dapCommand(args []string) {
	noOptions(codes.CmdDap, args)
	err := debugger.ServeDAP(os.Stdin, os.Stdout)
	stdlib.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8607, err))
		os.Exit(1)
	}
}

// runCommand 'tr çalıştır dosya.tr [argümanlar]' komutudur.
func runCommand(args []string) {
	file, rest, ok := programFile(codes.NameRef(codes.CmdRun), args)
	if !ok {
		exitWithError(messages.Text(messages.H8605, codes.NameRef(codes.CmdRun), "ornek.tr"))
	}
	runFile(file, rest)
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
	lang := codes.OptionFlag(codes.OptLang)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, row := range []string{
		messages.Text(messages.H8612),
		messages.Text(messages.H8613),
		messages.Text(messages.H8614, codes.NameRef(codes.CmdRun)),
		messages.Text(messages.H8615, codes.NameRef(codes.CmdDebug)),
		messages.Text(messages.H8616, codes.NameRef(codes.CmdCheck)),
		messages.Text(messages.H8617, codes.NameRef(codes.CmdBuild)),
		messages.Text(messages.H8618, codes.NameRef(codes.CmdTest)),
		messages.Text(messages.H8619, codes.NameRef(codes.CmdFormat), codes.OptionFlag(codes.OptCheck)),
		messages.Text(messages.H8620, codes.NameRef(codes.CmdEncode), lang),
		messages.Text(messages.H8622, codes.NameRef(codes.CmdShow), lang),
		messages.Text(messages.H8623, codes.NameRef(codes.CmdShow), lang),
		messages.Text(messages.H8624, codes.NameRef(codes.CmdEncode), lang),
		messages.Text(messages.H8625, codes.NameRef(codes.CmdWatch)),
		messages.Text(messages.H8626, codes.NameRef(codes.CmdProfile), codes.OptionFlag(codes.OptJson)),
		messages.Text(messages.H8627, codes.NameRef(codes.CmdDap)),
		messages.Text(messages.H8830, codes.NameRef(codes.CmdIndex), codes.NameRef(codes.SubDefinition), codes.NameRef(codes.SubReferences), codes.NameRef(codes.SubTestedBy), codes.NameRef(codes.SubImpact)),
		messages.Text(messages.H8863, codes.NameRef(codes.CmdIndex), codes.NameRef(codes.SubRename)),
		messages.Text(messages.H8628, codes.NameRef(codes.CmdHelp)),
		messages.Text(messages.H8629, codes.NameRef(codes.CmdVersion)),
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
	file, rest, ok := programFile(codes.NameRef(codes.CmdDebug), args)
	if !ok {
		exitWithError(messages.Text(messages.H8608, codes.NameRef(codes.CmdDebug)))
	}
	debugFile(file, rest)
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
	fmt.Println(messages.Text(messages.H8634, codes.NameRef(codes.ReplExit)))
	machine := interp.New()
	// Kabuğa yazılan satırlar kişinin yazım dilindedir (source.Options.Typed); değerlerin metni
	// dilden bağımsızdır (3.14).
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
		if k, ok := codes.FindCommand(line); ok && k == codes.ReplExit {
			fmt.Println(messages.Text(messages.H8635))
			break
		}
		if exitHint(line) {
			fmt.Fprintln(os.Stderr, messages.Text(messages.H8636, line, codes.NameRef(codes.ReplExit)))
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
		s := source.Prepare(line, source.Options{TypeCheck: true, Checker: chk, Typed: true, GlobalNames: machine.GlobalEnv.Names()})
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

// startsWithPrint satır yazdır yerleşiğinin (TR_fn_print) kanonik ya da kişinin yazım dilindeki bir
// yazımıyla mı başlıyor (kabuk satırı kişinin dilinde yazılır)?
func startsWithPrint(ln string) bool {
	for _, y := range append(codes.Spellings(codes.FnPrint), codes.Display().Spellings(codes.FnPrint)...) {
		if strings.HasPrefix(ln, y) {
			return true
		}
	}
	return false
}

// exitHint kabukta çıkmak isteyen ama kabuğun çıkış komutunu (TR_repl_exit) yazmayan bir satırı tanır:
// adın Türkçe harfsiz biçimi (cikis) ya da hata ayıklayıcının çıkış komutu (TR_dbg_quit, quit).
// Böyle bir satır program sanılıp çalıştırılmaz; asıl komut önerilir.
func exitHint(line string) bool {
	if k, ok := codes.Display().Suggest(codes.NamespaceCommand, line, nil); ok && k == codes.ReplExit {
		return true
	}
	k, ok := codes.FindDebuggerCommand(line)
	return ok && k == codes.DbgQuit
}
