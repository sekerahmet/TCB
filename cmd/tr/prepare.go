package main

import (
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"os"
	"strings"
)

// preparation bir kaynağın çalıştırılmadan önce hangi aşamalardan geçeceğini ve hata
// başlıklarını belirler. Komutlar arasındaki tek fark bu başlıklar ve tip denetiminin
// yapılıp yapılmamasıdır.
type preparation struct {
	// syntaxHeader sözdizimi hatalarının üstüne yazılır. Boşsa hatalar başlıksız ve
	// code çerçevesi olmadan, her biri tek satır olarak yazılır.
	syntaxHeader string
	// typeHeader boş değilse tip denetimi yapılır ve hataları bu başlıkla yazılır.
	typeHeader string
}

// Dosya çalıştırmada ve etkileşimli kabukta (REPL) ortak kullanılan hata başlıkları. Metinler
// süreç başında etkin dilde kurulur (dil süreç boyunca değişmez).
var (
	syntaxHeader = "❌ " + messages.Text(messages.H8650)
	typeHeader   = "❌ " + messages.Text(messages.H8651)
	runHeader    = "❌ " + messages.Text(messages.H8126)
	// warningHeader denetleyicinin uyarılarının (ör. yerleşik ad gölgeleme) başlığıdır.
	// Uyarılar programı durdurmaz ve çıkış kodunu etkilemez.
	warningHeader = "⚠️ " + messages.Text(messages.H8122)
)

// Komutların kullandığı hazırlık biçimleri.
var (
	runPreparation = preparation{
		syntaxHeader: syntaxHeader,
		typeHeader:   typeHeader,
	}
	debugPreparation = preparation{
		syntaxHeader: syntaxHeader,
	}
	buildPreparation = preparation{
		syntaxHeader: "❌ " + messages.Text(messages.H8654),
		typeHeader:   "❌ " + messages.Text(messages.H8655),
	}
	embeddedPreparation = preparation{
		syntaxHeader: "❌ " + messages.Text(messages.H8656),
	}
)

// readSource dosyanın metnini okur; okunamazsa hatayı yazıp çıkar. BOM ve kodlu biçim
// başlığı internal/lang/source'ta çözülür.
func readSource(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, messages.Text(messages.H8657, turkish.FileError("", path, messages.H2100, err)))
		os.Exit(1)
	}
	return string(data)
}

// prepareFile dosyayı okuyup prepareSource'dan geçirir; dosyanın metnini (paket ve modül
// denetimi için) ve hazırlığın sonucunu verir.
func prepareFile(path string, h preparation) (string, *source.Result) {
	text := readSource(path)
	return text, prepareSource(text, h)
}

// prepareSource kodu ayrıştırır ve istenmişse tip denetiminden geçirir (source.Prepare). Hata
// varsa hepsini yazıp 1 koduyla çıkar. Hata çerçeveleri ve yorumlayıcının kaynak metni
// sonucun gövdesidir (kodlu dosyada başlıktan sonrası).
func prepareSource(text string, h preparation) *source.Result {
	s := source.Prepare(text, source.Options{TypeCheck: h.typeHeader != ""})
	writeWarnings(s.Warnings, s.Text)
	switch s.Stage {
	case source.StageSyntax:
		if h.syntaxHeader == "" {
			for _, e := range s.Errors {
				fmt.Fprintln(os.Stderr, e)
			}
			os.Exit(1)
		}
		exitWithDiags(h.syntaxHeader, s.Errors, s.Text)
	case source.StageType:
		exitWithDiags(h.typeHeader, s.Errors, s.Text)
	}
	return s
}

// writeWarnings denetleyicinin uyarılarını (varsa) hatalardan ayrı bir başlıkla standart
// hataya yazar; çıkış kodunu etkilemez.
func writeWarnings(warnings []error, code string) {
	if len(warnings) > 0 {
		writeDiags(warningHeader, warnings, code)
	}
}

// writeDiags başlığı ve her hatayı kod çerçevesiyle standart hataya yazar.
func writeDiags(header string, errs []error, code string) {
	fmt.Fprintln(os.Stderr, header)
	for _, e := range errs {
		printError(e, code)
	}
}

// exitWithDiags hataları writeDiags ile yazıp (temizlik işlerinden sonra) 1 koduyla çıkar.
func exitWithDiags(header string, errs []error, code string) {
	writeDiags(header, errs, code)
	exit(1)
}

// runProgram programı çalıştırır, çalışma zamanı hatasını yazıp çıkar; başarılıysa
// eşzamansız işlerin bitmesini bekler. Her iki durumda da motorların açık kaynakları
// (veri okuyucuları, önbellekler) çıkmadan önce kapatılır.
//
// Ctrl+C ve sonlandırma istekleri yakalanır (interrupt.go): program nazikçe durur, ertelenen
// işler çalışır ve 3 koduyla çıkılır.
func runProgram(machine *interp.VM, prog *ast.Program, code string) {
	catchInterrupt(machine)
	if _, err := machine.Run(prog); err != nil {
		if c, ok := interp.IsExitRequest(err); ok {
			machine.WaitAsync()
			closeEngine()
			programExit(machine, c)
		}
		if errors.Is(err, interp.ErrInterrupted) {
			machine.WaitAsync() // eşzamanlı görevler de kesildi; ertelenen işleri bitsin
			closeEngine()
			reportInterrupt(machine, err, code)
		}
		closeEngine()
		writeRunError(machine, err, code)
		exit(1)
	}
	machine.WaitAsync()
	closeEngine()
	if machine.Interrupted() {
		// Kesme isteği programın son beklemesinde (ör. bekle) geldi; program yine de durduruldu.
		reportInterrupt(machine, nil, code)
	}
	if u := machine.UnusedArgsWarning(); u != "" {
		machine.WriteOutput(os.Stderr, u+"\n")
	}
}

// beforeClose program bittiğinde, motorlar kapanmadan önce çalışacak işlerdir (ör. tr izle
// ve tr profil GPU'nun son ölçümlerini arka uç kapanmadan alır).
var beforeClose []func()

// closeEngine beforeClose işlerini (bir kez) çalıştırır ve motorları kapatır.
func closeEngine() {
	jobs := beforeClose
	beforeClose = nil
	for _, f := range jobs {
		f()
	}
	stdlib.Close()
}

// programExit bir yerleşiğin istediği program çıkışını yapar (ör. komut satırı
// seçeneklerinde --yardım: 0, hatalı argüman: 2): mesaj 0 kodunda standart çıktıya, öbür
// kodlarda standart hataya yazılır.
func programExit(machine *interp.VM, c *interp.ExitRequest) {
	if c.Message != "" {
		w := os.Stderr
		if c.Code == 0 {
			w = os.Stdout
		}
		machine.WriteOutput(w, c.Message+"\n")
	}
	exit(c.Code)
}

// writeRunError bir çalışma zamanı hatasını başlığıyla standart hataya yazar. Bir modüle
// ait hatanın (ya da modülün denetim hatalarının) çerçevesi o modülün kaynağından çizilir.
// Metin VM'nin çıktı noktasından geçer (canlı satır, gizli değerler).
func writeRunError(machine *interp.VM, err error, code string) {
	var sb strings.Builder
	sb.WriteString(runHeader + "\n")
	for _, m := range interp.ErrorItems(err, code) {
		fmt.Fprintf(&sb, "  • %s\n", m.Message)
		sb.WriteString(indentFrame(m.Frame))
	}
	machine.WriteOutput(os.Stderr, turkish.Printable(sb.String()))
}

// moduleHeader bir hata başlığına modülün yolunu ekler: "❌ X:" → "❌ X (yol modülünde):".
func moduleHeader(header, filePath string) string {
	return messages.Text(messages.H8658, strings.TrimSuffix(header, ":"), filePath)
}

// checkModules programın 'aktar' ile aldığı bütün modülleri (özyinelemeli) bulur ve
// çalıştırmadan denetler; 'tr kontrol' ve 'tr derle' kullanır. Hata varsa yazıp 1 koduyla
// çıkar. globalHeader bulunamayan/okunamayan modül hatalarının başlığıdır.
func checkModules(path, code string, h preparation, globalHeader string) *pkg {
	pk, err := preparePackage(path, code)
	if err == nil {
		return pk
	}
	var moduleErr *moduleCheckError
	if errors.As(err, &moduleErr) {
		header := h.typeHeader
		if moduleErr.kind == "sözdizimi" {
			header = h.syntaxHeader
		}
		exitWithDiags(moduleHeader(header, moduleErr.filePath), moduleErr.diags, moduleErr.code)
	}
	fmt.Fprintf(os.Stderr, "%s %v\n", globalHeader, err)
	exit(1)
	return nil
}
