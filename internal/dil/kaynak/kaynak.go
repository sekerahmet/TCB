// Package kaynak TR'nin "kaynaktan programa" giriş noktasıdır: bir .tr dosyasının metnini
// bölümlerine ayırır (BOM, kodlu biçim başlığı, gövde), doğru sözcük çözümleyiciyi seçer,
// ayrıştırır, isteğe göre tip denetiminden geçirir ve hataları ile uyarıları tek biçimde
// döndürür. Komut satırı, etkileşimli kabuk, test koşucusu, hata ayıklama sunucusu ve modül
// yükleyici dosyaları yalnızca buradan okur. Kodlu biçim (docs/dil/17_kodlar.md, 17.1-17.7)
// ve onun çeviricisi (Kodla, Coz) da yalnızca bu pakettedir.
//
// Katman: dil çekirdeği (internal/dil); yalnızca internal/dil paketlerini içe aktarır.
package kaynak

import (
	"fmt"
	"strings"

	"tr_dili/internal/dil/sozcuk"
)

// Baslik kodlu bir dosyanın ilk satırıdır (17.1): biçim sürümü 1. Görüntüleme dili dosyada
// saklanmaz; her kişi kendi yazım dilini seçer.
const Baslik = sozcuk.KodluBaslik

// baslikOneki kodlu biçim başlıklarının ortak başıdır; bu önekle başlayıp Baslik olmayan
// bir ilk satır, bu sürümün okuyamadığı bir biçimdir (sessizce Türkçe kaynak sayılmaz).
const baslikOneki = "//! tr kodlu"

const bom = "\xef\xbb\xbf"

// Bolumler bir kaynak dosyanın bölümleridir. Birlestir bölümleri bayt bayt geri birleştirir.
type Bolumler struct {
	// BOM dosya UTF-8 BOM'uyla başlıyor mu?
	BOM bool
	// Kodlu dosya kodlu biçimde mi (ilk satırı Baslik)?
	Kodlu bool
	// SatirSonu başlık satırının sonudur ("\n" ya da "\r\n"; başlık dosyanın tek satırıysa boş).
	SatirSonu string
	// Govde başlıktan (varsa) sonraki metindir. Satır numaraları gövdeden sayılır (17.5): kodlu
	// dosyanın 1. satırı başlığın altındaki satırdır, Türkçe gösterimdeki 1. satırla aynıdır.
	Govde string
}

// BaslikHatasi dosyanın ilk satırı bir kodlu biçim başlığına benzediği hâlde bu sürümün
// okuyabildiği başlık olmadığında döner.
type BaslikHatasi struct {
	Satir string
}

func (e *BaslikHatasi) Error() string {
	return fmt.Sprintf("Satır 1, Sütun 1: '%s' bu yorumlayıcının tanımadığı bir kodlu biçim başlığı; yalnızca '%s' okunur (dosya daha yeni bir TR sürümüyle yazılmış olabilir; '%s' ile başlayan ilk satır kodlu biçim için ayrılmıştır)",
		e.Satir, Baslik, baslikOneki)
}

// Ayir metni bölümlerine ayırır. İlk satır (BOM'dan sonra) tam olarak Baslik ise dosya
// kodludur; değilse Türkçe yazılmış kaynaktır (geriye uyum). İlk satır başlık önekiyle başlayıp
// Baslik değilse *BaslikHatasi döner.
func Ayir(metin string) (Bolumler, error) {
	var b Bolumler
	if strings.HasPrefix(metin, bom) {
		b.BOM = true
		metin = metin[len(bom):]
	}
	ilk, kalan, satirSonu := ilkSatir(metin)
	switch {
	case ilk == Baslik:
		b.Kodlu, b.SatirSonu, b.Govde = true, satirSonu, kalan
	case strings.HasPrefix(ilk, baslikOneki):
		b.Govde = metin
		return b, &BaslikHatasi{Satir: ilk}
	default:
		b.Govde = metin
	}
	return b, nil
}

// ilkSatir metnin ilk satırını, ondan sonrasını ve aradaki satır sonunu verir.
func ilkSatir(metin string) (ilk, kalan, satirSonu string) {
	i := strings.IndexByte(metin, '\n')
	if i < 0 {
		return metin, "", ""
	}
	ilk, satirSonu = metin[:i], "\n"
	if strings.HasSuffix(ilk, "\r") {
		ilk, satirSonu = ilk[:len(ilk)-1], "\r\n"
	}
	return ilk, metin[i+1:], satirSonu
}

// Birlestir bölümleri dosya metnine çevirir (Ayir'in tersi).
func (b Bolumler) Birlestir() string {
	var sb strings.Builder
	if b.BOM {
		sb.WriteString(bom)
	}
	if b.Kodlu {
		sb.WriteString(Baslik)
		sb.WriteString(b.SatirSonu)
	}
	sb.WriteString(b.Govde)
	return sb.String()
}

// Sozcukler gövdenin sözcük çözümleyicisidir: kodlu dosyada kodları tanıyan çözümleyici.
func (b Bolumler) Sozcukler() *sozcuk.Lexer {
	return sozcuk.NewGovde(b.Govde, b.Kodlu)
}

// satirSonuBul bir gövdenin satır sonu biçimidir: ilk satır sonu CRLF ise "\r\n", yoksa "\n".
// Kodla başlığı gövdenin biçimiyle yazar.
func satirSonuBul(govde string) string {
	if i := strings.IndexByte(govde, '\n'); i > 0 && govde[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}
