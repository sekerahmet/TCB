package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Görünüm komutu (docs/spec/17_kodlar.md, 17.7):
//
//	tr göster <dosya.tr> [--dil tr]  dosyayı bir yazım dilinde standart çıktıya basar
//	tr göster - [--dil tr]           metni standart girişten okur, görünümünü çıkışa yazar
//
// Yazım dili verilmezse kişinin yazım dilidir (codes.Display, TR_DIL). Görünüm yalnız okunur:
// çalışmaz ve koda geri çevrilmez; kod yalnız kimliklerle yazılır.
//
// Standart giriş kipi ('-') düzenleyiciler içindir (VS Code eklentisi kodlu dosyanın görünümünü
// bununla gösterir): standart çıktıya yalnızca görünüm yazılır, hatalar standart hataya. Çıkış
// kodları: 0 gösterildi; exitSyntax metin ayrıştırılamadı (görünüm yazılmadı); 1 başka bir hata.

// exitSyntax standart giriş kipinde metnin ayrıştırılamadığını (sözdizimi hatası; görünüm
// yazılmadı) bildiren çıkış kodudur.
const exitSyntax = 2

// orDisplay verilmemiş ("") yazım dilinin yerine kişinin yazım dilidir (codes.Display).
func orDisplay(languageCode string) string {
	if languageCode == "" {
		return codes.Display().Code
	}
	return languageCode
}

// languageArgument standart giriş kipinin '--dil <kod>' argümanını okur (verilmemişse ""); başka
// bir argüman hatadır.
func languageArgument(args []string, command any) string {
	languageCode, extra := parseLanguageArgs(command, args, messages.Text(messages.H8700, codes.OptionFlag(codes.OptLang), command))
	if len(extra) > 0 {
		exitUsage(messages.Text(messages.H8701, command, extra[0]))
	}
	return languageCode
}

// parseLanguageArgs '--dil <kod>' argümanını (verilmemişse "") ve konumlu argümanları okur; dil
// kodunu ve konumlu argümanları döndürür (kaç tane alınacağı çağıranındır). Değersiz '--dil'
// missingValue iletisiyle (ileti çağıranındır), başka seçenek unknownOption ile çıkışa götürür.
// '--'dan sonraki argüman seçenek sayılmaz.
func parseLanguageArgs(command any, args []string, missingValue string) (languageCode string, files []string) {
	options := true
	for i := 0; i < len(args); i++ {
		switch {
		case options && args[i] == "--":
			options = false
		case options && isOption(args[i], codes.OptLang) && i+1 < len(args):
			languageCode = args[i+1]
			i++
		case options && isOption(args[i], codes.OptLang):
			exitUsage(missingValue)
		case options && looksLikeOption(args[i]):
			unknownOption(command, args[i], []codes.Code{codes.OptLang})
		default:
			files = append(files, args[i])
		}
	}
	return languageCode, files
}

// mustLoadLanguage yazım dili kodunun kod listesini yükler; bilinmeyen dil kullanım hatasıdır (2).
func mustLoadLanguage(languageCode string) *codes.Language {
	language, err := codes.LoadLanguage(languageCode)
	if err != nil {
		exitUsage(err)
	}
	return language
}

// convertFromInput standart girişin tamamını convert ile çevirip standart çıktıya yazar. Metin
// ayrıştırılamıyorsa hataları kod çerçeveleriyle (dosyanın kendi metniyle) standart hataya yazıp
// exitSyntax ile çıkar.
func convertFromInput(convert func(string) (string, error)) {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		exitWithError(messages.Text(messages.H8702, err))
	}
	text, err := convert(string(content))
	if err != nil {
		var sh *source.SyntaxError
		if errors.As(err, &sh) {
			r := source.Prepare(string(content), source.Options{})
			writeDiags(syntaxHeader, r.Errors, r.Text)
			os.Exit(exitSyntax)
		}
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		os.Exit(1)
	}
	fmt.Print(text)
}

// collectTrFiles komut satırı hedeflerinin (dosya ya da klasör) .tr dosyalarını sırayla toplar;
// ilk açılamayan hedefin hatasını komutun failed iletisiyle döndürür.
func collectTrFiles(targets []string, failed messages.Code) ([]string, error) {
	var files []string
	for _, h := range targets {
		d, err := trFiles(h, failed)
		if err != nil {
			return nil, err
		}
		files = append(files, d...)
	}
	return files, nil
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

// showCommand 'tr göster <dosya.tr> [--dil tr]' komutudur: dosya verilen yazım dilinde
// (varsayılan kişinin yazım dili) standart çıktıya yazılır; gösterim başlığı ve satırları tutar,
// başlıksız metin olduğu gibi kalır (source.Show). Dosya yerine '-' verilirse metin standart girişten
// okunur (düzenleyiciler için; çıkış kodları yukarıda).
func showCommand(args []string) {
	if len(args) > 0 && args[0] == "-" {
		language := mustLoadLanguage(orDisplay(languageArgument(args[1:], codes.NameRef(codes.CmdShow))))
		convertFromInput(func(text string) (string, error) { return source.Show(text, language) })
		return
	}
	showName := codes.NameRef(codes.CmdShow)
	languageCode, files := parseLanguageArgs(showName, args, messages.Text(messages.H8708, codes.OptionFlag(codes.OptLang), showName))
	if len(files) == 0 {
		exitUsage(messages.Text(messages.H8710, codes.NameRef(codes.CmdShow), codes.OptionFlag(codes.OptLang)))
	}
	if len(files) > 1 {
		exitUsage(messages.Text(messages.H8709, showName, files[1]))
	}
	file := files[0]
	language := mustLoadLanguage(orDisplay(languageCode))
	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, messages.Text(messages.H8657, turkish.FileError("", file, messages.H2100, err)))
		os.Exit(1)
	}
	text, err := source.Show(string(content), language)
	if err != nil {
		var sh *source.SyntaxError
		if errors.As(err, &sh) {
			r := source.Prepare(string(content), source.Options{})
			exitWithDiags(syntaxHeader, r.Errors, r.Text)
		}
		fmt.Fprintf(os.Stderr, "✗ %s: %v\n", file, err)
		os.Exit(1)
	}
	fmt.Print(text)
}
