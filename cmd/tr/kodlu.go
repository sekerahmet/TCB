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
// hataya. Çıkış kodları: 0 çevrildi; cikisSozdizimi metin ayrıştırılamadı (çevrilmedi,
// düzenleyici metni olduğu gibi saklar); 1 başka bir hata.

// cikisSozdizimi standart giriş kipinde metnin ayrıştırılamadığını (sözdizimi hatası; çeviri
// yapılmadı) bildiren çıkış kodudur.
const cikisSozdizimi = 2

// kodlaKomutu 'tr kodla' komutudur; zaten kodlu dosyalara dokunulmaz.
func kodlaKomutu(args []string) {
	if len(args) > 0 && args[0] == "-" {
		// Kodlanacak metin yazım dilinde yazılmıştır. Dilin sözcük tabloları süreç başında etkin
		// dilden kurulduğu için başka bir dil, süreci o dille yeniden çalıştırarak okunur.
		if dil := dilArgumani(args[1:], "kodla"); dil.Code != codes.Active().Code {
			dilleYenidenCalistir(dil.Code)
		}
		girdidenCevir(source.Encode)
		return
	}
	cevirKomutu(args, "kodla", "kodlandı", source.Encode)
}

// dilArgumani standart giriş kipinin '--dil <kod>' argümanını okur (yoksa etkin dil) ve yazım
// dilini doğrular; başka bir argüman ya da bilinmeyen dil hatadır.
func dilArgumani(args []string, komut string) *codes.Language {
	dilKodu := codes.Active().Code
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--dil" && i+1 < len(args):
			dilKodu = args[i+1]
			i++
		case args[i] == "--dil":
			fmt.Fprintf(os.Stderr, "Hata: '--dil' bir yazım dili bekler. Örn: tr %s - --dil tr\n", komut)
			os.Exit(1)
		default:
			fmt.Fprintf(os.Stderr, "Hata: 'tr %s -' metni standart girişten okur; fazladan argüman: %s\n", komut, args[i])
			os.Exit(1)
		}
	}
	dil, err := yazimDili(dilKodu)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	return dil
}

// girdidenCevir standart girişin tamamını cevir ile çevirip standart çıktıya yazar. Metin
// ayrıştırılamıyorsa hataları kod çerçeveleriyle standart hataya yazıp cikisSozdizimi ile çıkar.
func girdidenCevir(cevir func(string) (string, error)) {
	veri, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: standart giriş okunamadı: %v\n", err)
		os.Exit(1)
	}
	metin, err := cevir(string(veri))
	if err != nil {
		var sh *source.SyntaxError
		if errors.As(err, &sh) {
			b, _ := source.Split(string(veri))
			hatalariYaz(sozdizimiBasligi, sh.Errors, b.Body)
			os.Exit(cikisSozdizimi)
		}
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		os.Exit(1)
	}
	fmt.Print(metin)
}

// cozKomutu 'tr çöz' komutudur; Türkçe yazılmış dosyalara dokunulmaz.
func cozKomutu(args []string) {
	cevirKomutu(args, "çöz", "çözüldü", func(metin string) (string, error) {
		return source.Decode(metin, codes.Active())
	})
}

// cevirKomutu hedeflerdeki (klasörlerde alt klasörler dahil) bütün .tr dosyalarını cevir ile
// dönüştürüp değişenleri yerine yazar. Hatalı dosyalar atlanır; biri bile varsa 1 koduyla çıkar.
func cevirKomutu(args []string, komut, fiil string, cevir func(string) (string, error)) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Hata: Lütfen bir .tr dosyası ya da klasör belirtin. Örn: tr %s ornek.tr veya tr %s .\n", komut, komut)
		os.Exit(1)
	}
	var dosyalar []string
	for _, h := range args {
		d, err := trDosyalari(h)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
			os.Exit(1)
		}
		dosyalar = append(dosyalar, d...)
	}
	degisen, hatali := 0, 0
	for _, yol := range dosyalar {
		veri, err := os.ReadFile(yol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", turkish.FileError("", yol, messages.H2100, err))
			hatali++
			continue
		}
		yeni, err := cevir(string(veri))
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", yol, err)
			hatali++
			continue
		}
		if yeni == string(veri) {
			continue
		}
		if err := yerineYaz(yol, yeni); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", err)
			hatali++
			continue
		}
		degisen++
		fmt.Printf("%s: %s\n", fiil, yol)
	}
	fmt.Printf("%d dosya incelendi, %d dosya %s.\n", len(dosyalar), degisen, fiil)
	if hatali > 0 {
		fmt.Printf("%d dosya çevrilemedi (yukarıdaki hatalara bakın).\n", hatali)
		os.Exit(1)
	}
}

// yerineYaz dosyayı izinlerini koruyarak yeniden yazar.
func yerineYaz(yol, metin string) error {
	izin := fs.FileMode(0644)
	if bilgi, err := os.Stat(yol); err == nil {
		izin = bilgi.Mode().Perm()
	}
	if err := os.WriteFile(yol, []byte(metin), izin); err != nil {
		return turkish.FileError("", yol, messages.H2101, err)
	}
	return nil
}

// gosterKomutu 'tr göster <dosya.tr> [--dil tr]' komutudur: dosya kodluysa verilen yazım
// dilinde (varsayılan tr), değilse olduğu gibi standart çıktıya yazılır. Dosya yerine '-'
// verilirse metin standart girişten okunur (düzenleyiciler için; çıkış kodları yukarıda).
func gosterKomutu(args []string) {
	if len(args) > 0 && args[0] == "-" {
		dil := dilArgumani(args[1:], "göster")
		girdidenCevir(func(metin string) (string, error) { return source.Decode(metin, dil) })
		return
	}
	var dosya, dilKodu string
	dilKodu = codes.Active().Code
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--dil" && i+1 < len(args):
			dilKodu = args[i+1]
			i++
		case args[i] == "--dil":
			fmt.Fprintln(os.Stderr, "Hata: '--dil' bir yazım dili bekler. Örn: tr göster ornek.tr --dil tr")
			os.Exit(1)
		case dosya == "":
			dosya = args[i]
		default:
			fmt.Fprintf(os.Stderr, "Hata: 'tr göster' tek bir dosya alır; fazladan argüman: %s\n", args[i])
			os.Exit(1)
		}
	}
	if dosya == "" {
		fmt.Fprintln(os.Stderr, "Hata: Lütfen gösterilecek .tr dosyasını belirtin. Örn: tr göster ornek.tr --dil tr")
		os.Exit(1)
	}
	dil, err := yazimDili(dilKodu)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	veri, err := os.ReadFile(dosya)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası: %v\n", turkish.FileError("", dosya, messages.H2100, err))
		os.Exit(1)
	}
	metin, err := source.Decode(string(veri), dil)
	if err != nil {
		var sh *source.SyntaxError
		if errors.As(err, &sh) {
			b, _ := source.Split(string(veri))
			hatalarlaCik(sozdizimiBasligi, sh.Errors, b.Body)
		}
		fmt.Fprintf(os.Stderr, "✗ %s: %v\n", dosya, err)
		os.Exit(1)
	}
	fmt.Print(metin)
}

// yazimDili bir yazım dili kodunun (ISO 639-1) kod listesidir.
func yazimDili(kodu string) (*codes.Language, error) {
	return codes.LoadLanguage(kodu)
}

// dilleYenidenCalistir aynı komutu etkin dil olarak dilKodu ile yeniden çalıştırır (standart
// giriş, çıkış ve hata aynen geçer) ve onun çıkış koduyla çıkar.
func dilleYenidenCalistir(dilKodu string) {
	ben, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: '%s' yazım dili için tr yeniden çalıştırılamadı: %v\n", dilKodu, err)
		os.Exit(1)
	}
	c := exec.Command(ben, os.Args[1:]...)
	c.Env = append(os.Environ(), codes.LanguageEnvVar+"="+dilKodu)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		var ce *exec.ExitError
		if errors.As(err, &ce) {
			os.Exit(ce.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Hata: '%s' yazım dili için tr yeniden çalıştırılamadı: %v\n", dilKodu, err)
		os.Exit(1)
	}
	os.Exit(0)
}
