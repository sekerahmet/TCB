package yurut

import (
	"fmt"
	"strings"
)

// YolDegerli bir dosya yolunu taşıyan dış değerlerdir (ileride standart kitaplığın Yol
// tipi). Yol alan yerleşikler argümanı YolArgumani ile okur; böylece Metin ve YolDegerli
// her yerde aynı biçimde kabul edilir, yeni bir yol tipi gelince yerleşikler değişmez.
type YolDegerli interface {
	// DosyaYolu değerin gösterdiği yoldur (çözülmemiş; göreli olabilir, bkz. YolCoz).
	DosyaYolu() string
}

// YolArgumani yol alan bir yerleşiğin argümanını okur: Metin ya da YolDegerli. Değer bir
// yol değilse ok=false döner (çağıran kendi tip hatasını verir). Yolda sekme ya da satır
// sonu varsa yol gösteren bir hata döner: bunlar dosya adında neredeyse hiç bulunmaz,
// çoğunlukla "C:\tokenizer.json" gibi bir Windows yolunda '\t' kaçış sayılmıştır.
func YolArgumani(fn string, d Value) (yol string, ok bool, err error) {
	switch x := d.(type) {
	case *StringVal:
		yol = x.Val
	case YolDegerli:
		yol = x.DosyaYolu()
	default:
		return "", false, nil
	}
	return yol, true, YolDenetle(fn, yol)
}

// YolDenetle yolda kaçış dizisinden gelmiş olabilecek denetim karakterlerini (sekme, satır
// sonu) arar; bulursa ham metni öneren bir hata döndürür.
func YolDenetle(fn, yol string) error {
	if !strings.ContainsAny(yol, "\t\n\r") {
		return nil
	}
	ne := "satır sonu"
	if strings.Contains(yol, "\t") {
		ne = "sekme"
	}
	gorunen := strings.NewReplacer("\t", `\t`, "\n", `\n`, "\r", `\r`).Replace(yol)
	return fmt.Errorf("%s: dosya yolunda %s karakteri var; metindeki ters bölü bir kaçış dizisi sayıldı (\"\\t\" sekme, \"\\n\" satır sonudur). "+
		"Windows yolu için ham metin kullanın: `%s` (ya da ters bölüleri ikileyin veya / yazın)", fn, ne, gorunen)
}
