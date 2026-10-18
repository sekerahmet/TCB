package vm

import "fmt"

// kaldirilanYerlesikler TR'den kaldırılmış eş anlamlı yerleşik fonksiyon adlarıdır: her
// işin tek yazımı olsun diye yalnızca asıl ad (ve Türkçe karaktersiz karşılığı) kayıtlıdır.
// Kaldırılan adlar artık sıradan adlardır (ör. değişken boy = 180); yalnızca tanımsız ad
// hatasında, eski alışkanlıkla yazılmış olabilecekleri için asıl ad önerilir.
var kaldirilanYerlesikler = map[string]string{
	"yaz":          "yazdır",
	"oku":          "girdi",
	"boy":          "uzunluk",
	"tur":          "tip",
	"donustur":     "haritala",
	"ozetle":       "indirge",
	"en_büyük":     "maks",
	"en_buyuk":     "maks",
	"en_küçük":     "min",
	"en_kucuk":     "min",
	"uyut":         "bekle",
	"std_sapma":    "standart_sapma",
	"medyan":       "ortanca",
	"json_yaz":     "json_metin",
	"tablo_oku":    "csv_oku",
	"http_al":      "web_al",
	"web_sunucusu": "web_sunucu",
}

// kaldirilanUyeler Metin ve Liste üzerinde kaldırılmış eş anlamlı özellik ve metot adlarıdır.
var kaldirilanUyeler = map[string]string{
	"boy":      "uzunluk",
	"dönüştür": "haritala",
	"donustur": "haritala",
}

// KaldirilanYerlesikIpucu ad kaldırılmış bir yerleşik adıysa hata mesajına eklenecek
// ipucunu, değilse boş metni döndürür.
func KaldirilanYerlesikIpucu(ad string) string {
	return kaldirilanIpucu(kaldirilanYerlesikler, ad)
}

// kaldirilanUyeIpucu kaldırılmış bir metot ya da özellik adı için ipucunu döndürür.
func kaldirilanUyeIpucu(ad string) string {
	return kaldirilanIpucu(kaldirilanUyeler, ad)
}

func kaldirilanIpucu(tablo map[string]string, ad string) string {
	if asil, ok := tablo[ad]; ok {
		return fmt.Sprintf(" ('%s' TR'de yok; '%s' yazmak mı istediniz?)", ad, asil)
	}
	return ""
}
