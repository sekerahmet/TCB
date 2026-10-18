package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"tr_dili/internal/dil/kaynak"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
)

// Kodlu saklama komutları (docs/dil/17_kodlar.md, 17.1-17.6):
//
//	tr kodla <dosya.tr | klasör>...       Türkçe yazılmış dosyaları yerinde kodlu biçime çevirir
//	tr çöz <dosya.tr | klasör>...         kodlu dosyaları yerinde Türkçe yazıma geri çevirir
//	tr göster <dosya.tr> [--dil tr]       dosyayı bir yazım dilinde standart çıktıya basar
//
// Çeviri yalnızca dilin sözcüklerinin yerini değiştirir; çevirici kendi sonucunu denetler ve
// kayıp görürse dosyaya dokunmaz (kaynak.ErrKayip).

// kodlaKomutu 'tr kodla' komutudur; zaten kodlu dosyalara dokunulmaz.
func kodlaKomutu(args []string) {
	cevirKomutu(args, "kodla", "kodlandı", kaynak.Kodla)
}

// cozKomutu 'tr çöz' komutudur; Türkçe yazılmış dosyalara dokunulmaz.
func cozKomutu(args []string) {
	cevirKomutu(args, "çöz", "çözüldü", func(metin string) (string, error) {
		return kaynak.Coz(metin, kod.Etkin())
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
			fmt.Fprintf(os.Stderr, "✗ %v\n", turkce.DosyaHatasi("", yol, "okunamadı", err))
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
		return turkce.DosyaHatasi("", yol, "yazılamadı", err)
	}
	return nil
}

// gosterKomutu 'tr göster <dosya.tr> [--dil tr]' komutudur: dosya kodluysa verilen yazım
// dilinde (varsayılan tr), değilse olduğu gibi standart çıktıya yazılır.
func gosterKomutu(args []string) {
	var dosya, dilKodu string
	dilKodu = kod.Etkin().Kodu
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
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası: %v\n", turkce.DosyaHatasi("", dosya, "okunamadı", err))
		os.Exit(1)
	}
	metin, err := kaynak.Coz(string(veri), dil)
	if err != nil {
		var sh *kaynak.SozdizimiHatasi
		if errors.As(err, &sh) {
			b, _ := kaynak.Ayir(string(veri))
			hatalarlaCik(sozdizimiBasligi, sh.Hatalar, b.Govde)
		}
		fmt.Fprintf(os.Stderr, "✗ %s: %v\n", dosya, err)
		os.Exit(1)
	}
	fmt.Print(metin)
}

// yazimDili bir yazım dili kodunun (ISO 639-1) kod listesidir. Bugün yalnızca Türkçe vardır.
func yazimDili(kodu string) (*kod.Dil, error) {
	if kodu == kod.Etkin().Kodu {
		return kod.Etkin(), nil
	}
	return nil, fmt.Errorf("'%s' yazım dili yok; bugün yalnızca '%s' var", kodu, kod.Etkin().Kodu)
}
