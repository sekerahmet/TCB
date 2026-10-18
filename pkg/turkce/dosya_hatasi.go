package turkce

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// dosyaHatasi işletim sisteminin dosya hatasını Türkçe anlatır. Özgün hata Unwrap ile
// korunur: errors.Is(err, os.ErrNotExist) gibi denetimler çalışmaya devam eder.
type dosyaHatasi struct {
	mesaj string
	ozgun error
}

func (h *dosyaHatasi) Error() string { return h.mesaj }
func (h *dosyaHatasi) Unwrap() error { return h.ozgun }

// DosyaHatasi bir dosya işleminin hatasını kullanıcıya gösterilecek Türkçe mesaja çevirir;
// işletim sisteminin İngilizce metni ("The system cannot find the file specified")
// kullanıcıya sızmaz. Mesaj tek satırdır:
//
//	<fn>: "<yol>" <islem>: <neden>[; <ek>...]
//
// fn hatayı veren yerleşiktir (ör. "dosya_oku()"; boşsa ön ek yazılmaz), yol kullanıcının
// yazdığı yoldur, islem "okunamadı" ya da "yazılamadı" gibi sonuçtur. ekler nedenin
// ardına "; " ile eklenir (ör. "bakılan yerler: ..."). Bilinen durumlar: dosya ya
// da klasör yok, izin yok, yol bir klasör, disk dolu, dosya kullanımda, geçersiz ad, yol
// çok uzun, yazmaya karşı korumalı disk. Bilinmeyen durumda Türkçe bir giriş cümlesinin
// ardından işletim sisteminin hata kodu ve özgün metni verilir (aranabilsin diye).
//
// err nil ise nil döner. Motor paketleri (model_yükle, tensör_kaydet/yükle,
// parçalayıcı_yükle, veri_kümesi) de dosya hatalarını bununla anlatmalıdır.
func DosyaHatasi(fn, yol, islem string, err error, ekler ...string) error {
	if err == nil {
		return nil
	}
	var sb strings.Builder
	if fn != "" {
		sb.WriteString(fn)
		sb.WriteString(": ")
	}
	fmt.Fprintf(&sb, "%q %s: %s", yol, islem, DosyaHatasiNedeni(yol, err))
	for _, ek := range ekler {
		if ek != "" {
			sb.WriteString("; ")
			sb.WriteString(ek)
		}
	}
	return &dosyaHatasi{mesaj: sb.String(), ozgun: err}
}

// DosyaHatasiNedeni hatanın nedenini yalnızca Türkçe neden cümlesi olarak döndürür
// (ör. "böyle bir dosya yok"). Kendi mesajını kuran çağıranlar içindir.
func DosyaHatasiNedeni(yol string, err error) string {
	if err == nil {
		return ""
	}
	// Denetimler işletim sisteminin gerçekten denediği yolla yapılır (göreli yol programın
	// klasörüne göre çözülmüş olabilir).
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Path != "" {
		yol = pe.Path
	}
	// Yol var ve bir klasörse işletim sistemleri bunu farklı biçimlerde bildirir
	// (Windows'ta "Incorrect function", Unix'te EISDIR); hepsi aynı cümleye iner.
	if yol != "" {
		if bilgi, e := os.Stat(yol); e == nil && bilgi.IsDir() {
			return "bu bir klasör, dosya değil (klasörün içindeki bir dosyanın yolunu verin)"
		}
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if klasor := eksikKlasor(yol); klasor != "" {
			return fmt.Sprintf("%q klasörü yok", klasor)
		}
		return "böyle bir dosya yok (adını ve klasörünü denetleyin)"
	case errors.Is(err, fs.ErrPermission):
		return "erişim izni yok (dosya salt okunur, korumalı bir klasörde ya da başka bir kullanıcıya ait olabilir)"
	case errors.Is(err, fs.ErrExist):
		return "bu adda bir dosya zaten var"
	case errors.Is(err, syscall.EISDIR):
		return "bu bir klasör, dosya değil (klasörün içindeki bir dosyanın yolunu verin)"
	}
	var kod syscall.Errno
	if errors.As(err, &kod) {
		if neden := sistemHatasiNedeni(kod); neden != "" {
			return neden
		}
		return fmt.Sprintf("işletim sistemi beklenmeyen bir hata bildirdi (hata kodu %d: %s)", uintptr(kod), kod.Error())
	}
	ozgun := err.Error()
	if pe != nil {
		ozgun = pe.Err.Error()
	}
	return fmt.Sprintf("beklenmeyen bir hata oluştu (özgün hata: %s)", ozgun)
}

// eksikKlasor yolun klasör kısımlarından ilk olmayanını kullanıcının yazdığı biçimde
// döndürür; klasörler tamamsa (eksik olan dosyanın kendisiyse) "" döner.
func eksikKlasor(yol string) string {
	if yol == "" {
		return ""
	}
	klasor := filepath.Dir(yol)
	if klasor == "." || klasor == yol {
		return ""
	}
	if _, err := os.Stat(klasor); err == nil {
		return ""
	}
	// En üstteki eksik klasörü bul: "a/b/c.txt" için 'a' yoksa "a" söylenir.
	for {
		ust := filepath.Dir(klasor)
		if ust == "." || ust == klasor {
			break
		}
		if _, err := os.Stat(ust); err == nil {
			break
		}
		klasor = ust
	}
	return filepath.ToSlash(klasor)
}
