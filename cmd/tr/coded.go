package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Kodlu saklama komutları (docs/spec/17_kodlar.md, 17.1-17.6 ve "Düzenleyicide"):
//
//	tr kodla <dosya.tr | klasör>...       Türkçe yazılmış dosyaları yerinde kodlu biçime çevirir
//	tr çöz <dosya.tr | klasör>...         kodlu dosyaları yerinde Türkçe yazıma geri çevirir
//	tr göster <dosya.tr> [--dil tr]       dosyayı bir yazım dilinde standart çıktıya basar
//	tr göster - [--dil tr]                metni standart girişten okur, gösterimini çıkışa yazar
//	tr kodla - [--dil tr]                 metni standart girişten okur, kodlu biçimini çıkışa yazar
//
// Çeviri yalnızca dilin sözcüklerinin yerini değiştirir; çevirici kendi sonucunu denetler ve
// kayıp görürse dosyaya dokunmaz (kaynak.ErrKayip).
//
// Standart giriş kipi ('-') düzenleyiciler içindir (VS Code eklentisi kodlu dosyayı bununla
// gösterir ve kaydeder): standart çıktıya yalnızca çevrilmiş metin yazılır, hatalar standart
// hataya. Çıkış kodları: 0 çevrildi; exitSyntax metin ayrıştırılamadı (çevrilmedi,
// düzenleyici metni olduğu gibi saklar); 1 başka bir hata.

// exitSyntax standart giriş kipinde metnin ayrıştırılamadığını (sözdizimi hatası; çeviri
// yapılmadı) bildiren çıkış kodudur.
const exitSyntax = 2

// encodeCommand 'tr kodla' komutudur; zaten kodlu dosyalara dokunulmaz.
func encodeCommand(args []string) {
	if len(args) > 0 && args[0] == "-" {
		// Kodlanacak metin yazım dilinde yazılmıştır. Dilin sözcük tabloları süreç başında etkin
		// dilden kurulduğu için başka bir dil, süreci o dille yeniden çalıştırarak okunur.
		if language := languageArgument(args[1:], "kodla"); language.Code != codes.Active().Code {
			rerunWithLanguage(language.Code)
		}
		convertFromInput(source.Encode)
		return
	}
	convertCommand(args, "kodla", messages.Text(messages.H8704), source.Encode)
}

// languageArgument standart giriş kipinin '--dil <kod>' argümanını okur (yoksa etkin dil) ve yazım
// dilini doğrular; başka bir argüman ya da bilinmeyen dil hatadır.
func languageArgument(args []string, command string) *codes.Language {
	languageCode := codes.Active().Code
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--dil" && i+1 < len(args):
			languageCode = args[i+1]
			i++
		case args[i] == "--dil":
			exitWithError(messages.Text(messages.H8700, command))
		default:
			exitWithError(messages.Text(messages.H8701, command, args[i]))
		}
	}
	language, err := writingLanguage(languageCode)
	if err != nil {
		exitWithError(err)
	}
	return language
}

// convertFromInput standart girişin tamamını cevir ile çevirip standart çıktıya yazar. Metin
// ayrıştırılamıyorsa hataları kod çerçeveleriyle standart hataya yazıp exitSyntax ile çıkar.
func convertFromInput(convert func(string) (string, error)) {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		exitWithError(messages.Text(messages.H8702, err))
	}
	text, err := convert(string(content))
	if err != nil {
		var sh *source.SyntaxError
		if errors.As(err, &sh) {
			b, _ := source.Split(string(content))
			writeDiags(syntaxHeader, sh.Errors, b.Body)
			os.Exit(exitSyntax)
		}
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		os.Exit(1)
	}
	fmt.Print(text)
}

// decodeCommand 'tr çöz' komutudur; Türkçe yazılmış dosyalara dokunulmaz.
func decodeCommand(args []string) {
	convertCommand(args, "çöz", messages.Text(messages.H8705), func(text string) (string, error) {
		return source.Decode(text, codes.Active())
	})
}

// convertCommand hedeflerdeki (klasörlerde alt klasörler dahil) bütün .tr dosyalarını cevir ile
// dönüştürüp değişenleri yerine yazar. Hatalı dosyalar atlanır; biri bile varsa 1 koduyla çıkar.
func convertCommand(args []string, command, verb string, convert func(string) (string, error)) {
	if len(args) == 0 {
		exitWithError(messages.Text(messages.H8703, command))
	}
	var files []string
	for _, h := range args {
		d, err := trFiles(h)
		if err != nil {
			exitWithError(err)
		}
		files = append(files, d...)
	}
	changed, faulty := 0, 0
	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", turkish.FileError("", filePath, messages.H2100, err))
			faulty++
			continue
		}
		updated, err := convert(string(content))
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", filePath, err)
			faulty++
			continue
		}
		if updated == string(content) {
			continue
		}
		if err := writeInPlace(filePath, updated); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", err)
			faulty++
			continue
		}
		changed++
		fmt.Printf("%s: %s\n", verb, filePath)
	}
	fmt.Println(messages.Text(messages.H8706, len(files), changed, verb))
	if faulty > 0 {
		fmt.Println(messages.Text(messages.H8707, faulty))
		os.Exit(1)
	}
}

// writeInPlace dosyayı izinlerini koruyarak yeniden yazar.
func writeInPlace(filePath, text string) error {
	perm := fs.FileMode(0644)
	if info, err := os.Stat(filePath); err == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(filePath, []byte(text), perm); err != nil {
		return turkish.FileError("", filePath, messages.H2101, err)
	}
	return nil
}

// showCommand 'tr göster <dosya.tr> [--dil tr]' komutudur: dosya kodluysa verilen yazım
// dilinde (varsayılan tr), değilse olduğu gibi standart çıktıya yazılır. Dosya yerine '-'
// verilirse metin standart girişten okunur (düzenleyiciler için; çıkış kodları yukarıda).
func showCommand(args []string) {
	if len(args) > 0 && args[0] == "-" {
		language := languageArgument(args[1:], "göster")
		convertFromInput(func(text string) (string, error) { return source.Decode(text, language) })
		return
	}
	var file, languageCode string
	languageCode = codes.Active().Code
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--dil" && i+1 < len(args):
			languageCode = args[i+1]
			i++
		case args[i] == "--dil":
			exitWithError(messages.Text(messages.H8708))
		case file == "":
			file = args[i]
		default:
			exitWithError(messages.Text(messages.H8709, args[i]))
		}
	}
	if file == "" {
		exitWithError(messages.Text(messages.H8710))
	}
	language, err := writingLanguage(languageCode)
	if err != nil {
		exitWithError(err)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, messages.Text(messages.H8657, turkish.FileError("", file, messages.H2100, err)))
		os.Exit(1)
	}
	text, err := source.Decode(string(content), language)
	if err != nil {
		var sh *source.SyntaxError
		if errors.As(err, &sh) {
			b, _ := source.Split(string(content))
			exitWithDiags(syntaxHeader, sh.Errors, b.Body)
		}
		fmt.Fprintf(os.Stderr, "✗ %s: %v\n", file, err)
		os.Exit(1)
	}
	fmt.Print(text)
}

// writingLanguage bir yazım dili kodunun (ISO 639-1) kod listesidir.
func writingLanguage(langCode string) (*codes.Language, error) {
	return codes.LoadLanguage(langCode)
}

// rerunWithLanguage aynı komutu etkin dil olarak languageCode ile yeniden çalıştırır (standart
// giriş, çıkış ve hata aynen geçer) ve onun çıkış koduyla çıkar.
func rerunWithLanguage(languageCode string) {
	self, err := os.Executable()
	if err != nil {
		exitWithError(messages.Text(messages.H8711, languageCode, err))
	}
	c := exec.Command(self, os.Args[1:]...)
	c.Env = append(os.Environ(), codes.LanguageEnvVar+"="+languageCode)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		exitWithError(messages.Text(messages.H8711, languageCode, err))
	}
	os.Exit(0)
}
