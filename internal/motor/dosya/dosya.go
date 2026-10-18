// Package dosya, standart kitaplığın dosya ve yol işlerini TR diline bağlar: Yol tipi ve
// yol(...) kurucusu, yola soru soran özellikler (dosya_adı, uzantı, var_mı, bayt ...),
// klasörü_listele, klasör_oluştur, sil, kopyala, taşı ve dosyayı akışla satır satır okuyan
// dosya_satırları.
//
// Güvenli varsayılanlar: var olan bir hedefin üzerine yazmak (üzerine_yaz:) ve dolu bir
// klasörü silmek (içindekilerle:) açık onay ister; sil kökü, ev klasörünü, çalışma ve
// program klasörlerini ve onların üstlerini silmez; bağlantılar (sembolik bağlantılar ve
// Windows dizin bağlantıları) hiçbir işte izlenmez: sil bağlantının kendisini siler,
// kopyala onları atlar, klasörü_listele ** ile içlerine girmez. Klasör silme ve kopyalama
// os.Root ile yapılır: gezinme sırasında bir alt klasör bağlantıyla değiştirilse de işlem
// kökün dışına çıkmaz. Ağır işler (kopyalama, silme, listeleme, satır okuma) yorumlayıcı
// kilidi bırakılarak yapılır.
//
// Göreli yollar her yerde 13.1'in tek kuralıyla, kullanıldıkları anda çözülür (önce çalışma
// klasörü, sonra kodun modülünün ve programın klasörü; bkz. yurut.(*VM).YolCoz).
//
// Katman: motor (internal/motor); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package dosya

import (
	"fmt"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/ortak/baglama"
)

// Adlı argümanlar; adları kod listesindedir (internal/dil/kod).
var (
	adUzerineYaz    = baglama.KodAd(kod.G900)
	adIcindekilerle = baglama.KodAd(kod.G901)
)

// Kaydet dosya ve yol yerleşiklerini VM'ye kaydeder.
func Kaydet(v *yurut.VM) {
	v.AdlisizKaydet(kod.G112, guvenli(kod.G112, satirlarYerlesik))
	v.AdlisizKaydet(kod.G120, guvenli(kod.G120, yolYerlesik))
	v.AdlisizKaydet(kod.G121, guvenli(kod.G121, listeleYerlesik))
	v.AdlisizKaydet(kod.G122, guvenli(kod.G122, klasorOlusturYerlesik))
	v.RegisterBuiltinKod(kod.G123, guvenli(kod.G123, silYerlesik))
	v.RegisterBuiltinKod(kod.G124, guvenli(kod.G124, kopyalaYerlesik))
	v.RegisterBuiltinKod(kod.G125, guvenli(kod.G125, tasiYerlesik))
}

// guvenli yerleşiği Go paniğine karşı korur (panik dene/yakala ile yakalanan bir hataya döner).
func guvenli(kd kod.Kod, fn yurut.BuiltinFunc) yurut.BuiltinFunc {
	ad := yurut.Cagri(kd)
	return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
		return baglama.Guvenli(ad, func() (yurut.Value, error) { return fn(m, args, named) })
	}
}

// yolOku yol alan bir yerleşiğin sira. argümanını (Metin ya da Yol) okur; yolda sekme ya da
// satır sonu varsa ham metni öneren hatayı, boş yolda açık bir hatayı verir.
func yolOku(kd kod.Kod, sira int, d yurut.Value) (string, error) {
	ham, ok, err := yurut.YolArgumani(yurut.Cagri(kd), d)
	if !ok {
		return "", yurut.ArgumanTipHatasi(kd, sira, d, kod.A202, kod.A250)
	}
	if err != nil {
		return "", err
	}
	if ham == "" {
		return "", fmt.Errorf("%s: yol boş olamaz", yurut.Cagri(kd))
	}
	return ham, nil
}

// mantikAdli adlı bir Mantık argümanını okur (verilmemişse yanlış).
func mantikAdli(kd kod.Kod, named map[string]yurut.Value, a baglama.Ad) (bool, error) {
	d, ok := baglama.AdliAl(named, a)
	if !ok {
		return false, nil
	}
	return baglama.Mantik(yurut.Cagri(kd), a.TR+":", d)
}
