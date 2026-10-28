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
//	tr kodla <dosya.tr | klasör>... [--dil tr]  bir yazım dilinde yazılmış dosyaları yerinde kodlu biçime çevirir
//	tr göster <dosya.tr> [--dil tr]             dosyayı bir yazım dilinde standart çıktıya basar
//	tr göster - [--dil tr]                      metni standart girişten okur, gösterimini çıkışa yazar
//	tr kodla - [--dil tr]                       metni standart girişten okur, kodlu biçimini çıkışa yazar
//
// Yazım dili verilmezse kişinin yazım dilidir (codes.Display, TR_DIL).
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

// encodeCommand 'tr kodla' komutudur. Görünümün ('tr göster'in çıktısı) dili başlığındadır
// ('# TR 4 view=tr'): dil oradan okunur, başka bir '--dil' kullanım hatasıdır (H2542). Başlıksız
// metin '--dil'in yazım dilinde (verilmezse kişinin yazım dili, codes.Display) yazılmıştır.
// İşaretsiz başlıklı dosya zaten kodludur (2.1), dokunulmaz (H8681): onu gösterim sayıp okumak, o
// dilin sözcüğüyle aynı adlı kullanıcı adını (eğer) kimliğe çevirirdi. Standart girişte işaretsiz
// metin gösterimdir.
func encodeCommand(args []string) {
	encodeName := codes.NameRef(codes.CmdEncode)
	if len(args) > 0 && args[0] == "-" {
		given := languageArgument(args[1:], encodeName)
		convertFromInput(mustLoadLanguage(orDisplay(given)), func(text string) (string, error) {
			return source.EncodeIn(text, encodingLanguage(text, given))
		})
		return
	}
	given, targets := parseLanguageArgs(encodeName, args, messages.Text(messages.H8708, codes.OptionFlag(codes.OptLang), encodeName))
	mustLoadLanguage(orDisplay(given))
	convertCommand(targets, encodeName, messages.Text(messages.H8704), messages.H2115, func(filePath, text string) (string, error) {
		if b, err := source.Split(text); err == nil && b.Coded {
			fmt.Println(messages.Text(messages.H8681, filePath))
			return text, nil
		}
		return source.EncodeIn(text, encodingLanguage(text, given))
	})
}

// encodingLanguage text'in kodlanacağı yazım dilidir: '--dil' verilmişse o (görünümün dili
// başka ise EncodeIn hata verir), yoksa görünümün başlığındaki dil, o da yoksa kişinin yazım dili.
func encodingLanguage(text, given string) *codes.Language {
	if b, _ := source.Split(text); given == "" && b.View != "" {
		given = b.View
	}
	return mustLoadLanguage(orDisplay(given))
}

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

// convertCommand hedeflerdeki (klasörlerde alt klasörler dahil) bütün .tr dosyalarını cevir ile
// dönüştürüp değişenleri yerine yazar. Hatalı dosyalar atlanır; biri bile varsa 1 koduyla çıkar.
// Açılamayan hedef failed iletisiyle bildirilir.
func convertCommand(targets []string, command any, verb string, failed messages.Code, convert func(filePath, text string) (string, error)) {
	if len(targets) == 0 {
		exitUsage(messages.Text(messages.H8703, command))
	}
	files, err := collectTrFiles(targets, failed)
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
		convertFromInput(source.Turkish(), func(text string) (string, error) { return source.Show(text, language) })
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
