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
		// Kodlanacak metin verilen yazım dilinde yazılmıştır (varsayılan kişinin dili).
		language := languageArgument(args[1:], "kodla")
		convertFromInput(language, func(text string) (string, error) { return source.EncodeIn(text, language) })
		return
	}
	convertCommand(args, "kodla", messages.Text(messages.H8704), func(_, text string) (string, error) { return source.Encode(text) })
}

// languageArgument standart giriş kipinin '--dil <kod>' argümanını okur (yoksa kişinin yazım
// dili, codes.Display) ve yazım dilini doğrular; başka bir argüman ya da bilinmeyen dil hatadır.
func languageArgument(args []string, command any) *codes.Language {
	languageCode, _ := parseLanguageArgs(args, false, messages.Text(messages.H8700, codes.OptionFlag(codes.K100), command),
		func(arg string) string { return messages.Text(messages.H8701, command, arg) })
	return mustLoadLanguage(languageCode)
}

// parseLanguageArgs '--dil <kod>' argümanını (yoksa kişinin yazım dili) ve allowFile ise tek bir
// konumlu dosya argümanını okur; dil kodunu ve dosyayı döndürür. Değersiz '--dil' missingValue
// iletisiyle, başka argüman unexpected(arg) iletisiyle çıkışa götürür (iletiler çağıranındır).
func parseLanguageArgs(args []string, allowFile bool, missingValue string, unexpected func(arg string) string) (languageCode, file string) {
	languageCode = codes.Display().Code
	for i := 0; i < len(args); i++ {
		switch {
		case isOption(args[i], codes.K100) && i+1 < len(args):
			languageCode = args[i+1]
			i++
		case isOption(args[i], codes.K100):
			exitWithError(missingValue)
		case allowFile && file == "":
			file = args[i]
		default:
			exitWithError(unexpected(args[i]))
		}
	}
	return languageCode, file
}

// mustLoadLanguage yazım dili kodunun kod listesini yükler; bilinmeyen dil hatadır.
func mustLoadLanguage(languageCode string) *codes.Language {
	language, err := codes.LoadLanguage(languageCode)
	if err != nil {
		exitWithError(err)
	}
	return language
}

// convertFromInput standart girişin tamamını cevir ile çevirip standart çıktıya yazar. Metin
// ayrıştırılamıyorsa hataları kod çerçeveleriyle standart hataya yazıp exitSyntax ile çıkar;
// kodsuz girdi language'de yazılmıştır.
func convertFromInput(language *codes.Language, convert func(string) (string, error)) {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		exitWithError(messages.Text(messages.H8702, err))
	}
	text, err := convert(string(content))
	if err != nil {
		var sh *source.SyntaxError
		if errors.As(err, &sh) {
			// Çerçeve kişinin gördüğü metni gösterir: kodsuz girdide kendisini, kodlu girdide
			// Türkçe gösterimini; konumlar da ona göre (17.5).
			if b, _ := source.Split(string(content)); !b.Coded {
				writeDiags(syntaxHeader, sh.Errors, b.Body)
				os.Exit(exitSyntax)
			}
			r := source.Prepare(string(content), source.Options{})
			writeDiags(syntaxHeader, r.Errors, r.Text)
			os.Exit(exitSyntax)
		}
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		os.Exit(1)
	}
	fmt.Print(text)
}

// decodeCommand 'tr çöz' komutudur; Türkçe yazılmış dosyalara dokunulmaz. Kodsuz dosya Türkçedir
// (17.7): dili Türkçe olmayan kodlu dosya çözülünce dili değişir, bu yüzden uyarılır (17.1).
func decodeCommand(args []string) {
	convertCommand(args, codes.NameRef(codes.K010), messages.Text(messages.H8705), func(filePath, text string) (string, error) {
		out, err := source.Decode(text, source.Turkish())
		if b, _ := source.Split(text); err == nil && b.Coded && b.Language != codes.SourceLanguage {
			fmt.Fprintln(os.Stderr, "⚠️ "+messages.Text(messages.H8711, filePath, b.Language))
		}
		return out, err
	})
}

// convertCommand hedeflerdeki (klasörlerde alt klasörler dahil) bütün .tr dosyalarını cevir ile
// dönüştürüp değişenleri yerine yazar. Hatalı dosyalar atlanır; biri bile varsa 1 koduyla çıkar.
func convertCommand(args []string, command any, verb string, convert func(filePath, text string) (string, error)) {
	if len(args) == 0 {
		exitWithError(messages.Text(messages.H8703, command))
	}
	files, err := collectTrFiles(args)
	if err != nil {
		exitWithError(err)
	}
	changed, faulty := 0, 0
	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", turkish.FileError("", filePath, messages.H2100, err))
			faulty++
			continue
		}
		updated, err := convert(filePath, string(content))
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

// collectTrFiles komut satırı hedeflerinin (dosya ya da klasör) .tr dosyalarını sırayla toplar;
// ilk açılamayan hedefin hatasını döndürür.
func collectTrFiles(targets []string) ([]string, error) {
	var files []string
	for _, h := range targets {
		d, err := trFiles(h)
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
// (varsayılan kişinin yazım dili) standart çıktıya yazılır; kodsuz dosya Türkçedir, Türkçede
// olduğu gibi kalır (source.Show). Dosya yerine '-' verilirse metin standart girişten okunur
// (düzenleyiciler için; çıkış kodları yukarıda).
func showCommand(args []string) {
	if len(args) > 0 && args[0] == "-" {
		language := languageArgument(args[1:], codes.NameRef(codes.K011))
		convertFromInput(source.Turkish(), func(text string) (string, error) { return source.Show(text, language) })
		return
	}
	showName := codes.NameRef(codes.K011)
	languageCode, file := parseLanguageArgs(args, true, messages.Text(messages.H8708, codes.OptionFlag(codes.K100), showName),
		func(arg string) string { return messages.Text(messages.H8709, showName, arg) })
	if file == "" {
		exitWithError(messages.Text(messages.H8710, codes.NameRef(codes.K011), codes.OptionFlag(codes.K100)))
	}
	language := mustLoadLanguage(languageCode)
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
