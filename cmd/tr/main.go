package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tr_dili/pkg/checker"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

const VERSION = "1.0.0"
const BUNDLE_MAGIC = "TR_EXE_1"

func main() {
	// Eğer bu .exe dosyası 'tr derle' ile üretilmiş bağımsız bir uygulama ise gömülü kodu çalıştır:
	if embeddedCode, ok := readEmbeddedBundle(); ok {
		runEmbeddedCode(embeddedCode)
		return
	}

	args := os.Args[1:]

	if len(args) == 0 {
		startREPL()
		return
	}

	switch args[0] {
	case "yardım", "yardim", "--help", "-h":
		printHelp()
	case "sürüm", "surum", "--version", "-v":
		fmt.Printf("TR Programlama ve Yapay Zeka Dili v%s\n", VERSION)
	case "bytecode", "baytkod":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Hata: Lütfen bir .tr dosyası belirtin. Örn: tr bytecode ornek.tr")
			os.Exit(1)
		}
		showBytecode(args[1])
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
	case "debug", "hata_ayıkla", "hata_ayikla", "abap":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Hata: Lütfen hata ayıklanacak .tr dosyasını belirtin. Örn: tr debug ornek.tr veya tr debug --web ornek.tr")
			os.Exit(1)
		}
		if args[1] == "--web" || args[1] == "-w" {
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Hata: Lütfen bir .tr dosyası belirtin. Örn: tr debug --web ornek.tr")
				os.Exit(1)
			}
			debugFile(args[2], true)
		} else {
			webMode := len(args) >= 3 && (args[2] == "--web" || args[2] == "-w")
			debugFile(args[1], webMode)
		}
	case "dap":
		// VS Code hata ayıklama oturumu: iletiler standart giriş/çıkış üzerinden gelir.
		if err := vm.ServeDAP(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Hata ayıklama sunucusu: %v\n", err)
			os.Exit(1)
		}
	case "çalıştır", "calistir":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Hata: Lütfen bir .tr dosyası belirtin. Örn: tr çalıştır ornek.tr")
			os.Exit(1)
		}
		runFile(args[1])
	default:
		runFile(args[0])
	}
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
	fmt.Println("  tr <dosya.tr>               .tr kaynak dosyasını derler ve çalıştırır")
	fmt.Println("  tr debug <dosya.tr>         SAP ABAP tarzı etkileşimli Hata Ayıklayıcıyı (Debugger) başlatır")
	fmt.Println("  tr debug --web <dosya.tr>   Tarayıcı tabanlı Görsel ABAP Debugger Studio'yu başlatır")
	fmt.Println("  tr kontrol <dosya.tr>       Sözdizimi ve akıllı tip kontrolü yapar")
	fmt.Println("  tr bytecode <dosya.tr>      Derlenen TR Sanal Makinesi (VM) bytecode komutlarını gösterir")
	fmt.Println("  tr derle <dosya.tr>         .tr dosyasını tek başına çalışan bağımsız .exe dosyasına dönüştürür")
}

func runFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası (%s): %v\n", path, err)
		os.Exit(1)
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")

	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Sözdizimi Hataları:")
		for _, e := range p.Errors() {
			printError(e, code)
		}
		os.Exit(1)
	}

	chk := checker.New()
	typeErrors := chk.CheckProgram(prog)
	if len(typeErrors) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Tip ve Güvenlik Denetimi Hataları:")
		for _, e := range typeErrors {
			printError(e, code)
		}
		os.Exit(1)
	}

	machine := vm.New()
	machine.SourceFile = path
	machine.SourceCode = code
	_, err = machine.Run(prog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ Çalışma Zamanı Hatası:")
		printError(err.Error(), code)
		os.Exit(1)
	}
	machine.WaitAsync()
}

func debugFile(path string, webMode bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası (%s): %v\n", path, err)
		os.Exit(1)
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")

	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Sözdizimi Hataları:")
		for _, e := range p.Errors() {
			printError(e, code)
		}
		os.Exit(1)
	}

	machine := vm.New()
	machine.SourceFile = path
	machine.SourceCode = code
	dbg := vm.NewDebugger(path, code, true)
	machine.Debugger = dbg

	if webMode {
		if err := dbg.StartWebServer(machine, 8090); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
	}

	_, err = machine.Run(prog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ Çalışma Zamanı Hatası:")
		printError(err.Error(), code)
		os.Exit(1)
	}
	machine.WaitAsync()
}

func checkFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası (%s): %v\n", path, err)
		os.Exit(1)
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")

	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Sözdizimi Hataları:")
		for _, e := range p.Errors() {
			printError(e, code)
		}
		os.Exit(1)
	}

	chk := checker.New()
	typeErrors := chk.CheckProgram(prog)
	if len(typeErrors) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Tip ve Güvenlik Denetimi Hataları:")
		for _, e := range typeErrors {
			printError(e, code)
		}
		os.Exit(1)
	}

	fmt.Printf("✅ %s başarıyla denetlendi: Sözdizimi ve tip güvenliği hatasız!\n", path)
}

func showBytecode(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası (%s): %v\n", path, err)
		os.Exit(1)
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")

	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}

	chunk := vm.CompileToBytecode(prog)
	fmt.Print(chunk.Disassemble())
}

func readEmbeddedBundle() (string, bool) {
	exePath, err := os.Executable()
	if err != nil {
		return "", false
	}
	f, err := os.Open(exePath)
	if err != nil {
		return "", false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.Size() < 16 {
		return "", false
	}

	footer := make([]byte, 16)
	if _, err := f.ReadAt(footer, info.Size()-16); err != nil {
		return "", false
	}
	if string(footer[8:16]) != BUNDLE_MAGIC {
		return "", false
	}

	codeLen := int64(binary.LittleEndian.Uint64(footer[0:8]))
	if codeLen <= 0 || codeLen > info.Size()-16 {
		return "", false
	}

	codeBytes := make([]byte, codeLen)
	if _, err := f.ReadAt(codeBytes, info.Size()-16-codeLen); err != nil {
		return "", false
	}
	return string(codeBytes), true
}

func runEmbeddedCode(code string) {
	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Gömülü Uygulama Sözdizimi Hataları:")
		for _, e := range p.Errors() {
			printError(e, code)
		}
		os.Exit(1)
	}
	machine := vm.New()
	machine.SourceFile = "<derlenmiş-uygulama>"
	machine.SourceCode = code
	if _, err := machine.Run(prog); err != nil {
		fmt.Fprintln(os.Stderr, "❌ Çalışma Zamanı Hatası:")
		printError(err.Error(), code)
		os.Exit(1)
	}
	machine.WaitAsync()
}

func compileToExe(srcPath string, outPath string) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası (%s): %v\n", srcPath, err)
		os.Exit(1)
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")

	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Derleme Öncesi Sözdizimi Hataları:")
		for _, e := range p.Errors() {
			printError(e, code)
		}
		os.Exit(1)
	}

	chk := checker.New()
	if errs := chk.CheckProgram(prog); len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Derleme Öncesi Tip Güvenliği Hataları:")
		for _, e := range errs {
			printError(e, code)
		}
		os.Exit(1)
	}

	if outPath == "" {
		base := strings.TrimSuffix(srcPath, filepath.Ext(srcPath))
		outPath = base + ".exe"
	}

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Derleyici ikili dosyası okunamadı: %v\n", err)
		os.Exit(1)
	}
	baseExeBytes, err := os.ReadFile(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Derleyici şablonu okunamadı: %v\n", err)
		os.Exit(1)
	}

	// Eğer şablon binary sonunda zaten gömülü bir paket varsa temizle
	if len(baseExeBytes) >= 16 && string(baseExeBytes[len(baseExeBytes)-8:]) == BUNDLE_MAGIC {
		prevLen := int(binary.LittleEndian.Uint64(baseExeBytes[len(baseExeBytes)-16 : len(baseExeBytes)-8]))
		if prevLen > 0 && prevLen+16 < len(baseExeBytes) {
			baseExeBytes = baseExeBytes[:len(baseExeBytes)-16-prevLen]
		}
	}

	codeBytes := []byte(code)
	footer := make([]byte, 16)
	binary.LittleEndian.PutUint64(footer[0:8], uint64(len(codeBytes)))
	copy(footer[8:16], []byte(BUNDLE_MAGIC))

	outBuf := make([]byte, 0, len(baseExeBytes)+len(codeBytes)+16)
	outBuf = append(outBuf, baseExeBytes...)
	outBuf = append(outBuf, codeBytes...)
	outBuf = append(outBuf, footer...)

	if err := os.WriteFile(outPath, outBuf, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ .exe oluşturma hatası: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🚀 Başarıyla derlendi! Bağımsız ve taşınabilir .exe hazır: %s\n", outPath)
}

func startREPL() {
	fmt.Printf("🇹🇷 TR Programlama ve Yapay Zeka Dili (v%s) — Etkileşimli Kabuk\n", VERSION)
	fmt.Println("Çıkmak için 'çıkış', ABAP Hata Ayıklayıcıyı açmak için '/h' yazın.")
	scanner := bufio.NewScanner(os.Stdin)
	machine := vm.New()
	chk := checker.New()

	for {
		fmt.Print("tr> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "çıkış" || line == "cikis" || line == "exit" {
			fmt.Println("Görüşmek üzere!")
			break
		}
		if line == "/h" || line == "/H" {
			machine.Debugger = vm.NewDebugger("<etkileşimli-kabuk>", "", true)
			fmt.Println("🐞 [ABAP /h] Hata ayıklama modu etkinleştirildi! Bir sonraki komut adım adım (F5/F6/F7/F8) çalışacak.")
			continue
		}

		if machine.Debugger != nil {
			machine.Debugger.SourceLines = strings.Split(line, "\n")
		}

		l := lexer.New(line)
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			for _, e := range p.Errors() {
				fmt.Println("  Hata:", e)
				fmt.Print(indentFrame(turkce.KodCercevesi(line, e)))
			}
			continue
		}

		errs := chk.CheckProgram(prog)
		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Println("  Tip Hatası:", e)
			}
			continue
		}

		val, err := machine.Run(prog)
		if err != nil {
			fmt.Println("  Çalışma Hatası:", err)
			fmt.Print(indentFrame(turkce.KodCercevesi(line, err.Error())))
			continue
		}
		machine.WaitAsync()
		if val != nil && val.Type() != vm.VAL_NULL {
			if !strings.HasPrefix(line, "yazdır") && !strings.HasPrefix(line, "yazdir") && !strings.HasPrefix(line, "yaz(") {
				fmt.Println("=>", val.Inspect())
			}
		}
	}
}
