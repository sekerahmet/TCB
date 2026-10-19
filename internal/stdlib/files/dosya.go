// Package files, standart kitaplığın dosya ve yol işlerini TR diline bağlar: Yol tipi ve
// yol(...) kurucusu, yola soru soran özellikler (dosya_adı, uzantı, var_mı, bayt ...),
// klasörü_listele, klasör_oluştur, sil, kopyala, taşı, program bitince silinen geçici_klasör ve
// dosyayı akışla satır satır okuyan dosya_satırları.
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
// Katman: motor (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package files

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	adUzerineYaz    = bind.KodAd(codes.G900)
	adIcindekilerle = bind.KodAd(codes.G901)
)

// Kaydet dosya ve yol yerleşiklerini VM'ye kaydeder.
func Kaydet(v *interp.VM) {
	v.AdlisizKaydet(codes.G112, guvenli(codes.G112, satirlarYerlesik))
	v.AdlisizKaydet(codes.G120, guvenli(codes.G120, yolYerlesik))
	v.AdlisizKaydet(codes.G121, guvenli(codes.G121, listeleYerlesik))
	v.AdlisizKaydet(codes.G122, guvenli(codes.G122, klasorOlusturYerlesik))
	v.RegisterBuiltinKod(codes.G123, guvenli(codes.G123, silYerlesik))
	v.RegisterBuiltinKod(codes.G124, guvenli(codes.G124, kopyalaYerlesik))
	v.RegisterBuiltinKod(codes.G125, guvenli(codes.G125, tasiYerlesik))
	v.AdlisizKaydet(codes.G126, guvenli(codes.G126, geciciKlasorYerlesik))
}

// guvenli yerleşiği Go paniğine karşı korur (panik dene/yakala ile yakalanan bir hataya döner).
func guvenli(kd codes.Kod, fn interp.BuiltinFunc) interp.BuiltinFunc {
	ad := interp.Cagri(kd)
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(ad, func() (interp.Value, error) { return fn(m, args, named) })
	}
}

// yolOku yol alan bir yerleşiğin sira. argümanını (Metin ya da Yol) okur; yolda sekme ya da
// satır sonu varsa ham metni öneren hatayı, boş yolda açık bir hatayı verir.
func yolOku(kd codes.Kod, sira int, d interp.Value) (string, error) {
	ham, ok, err := interp.YolArgumani(interp.Cagri(kd), d)
	if !ok {
		return "", interp.ArgumanTipHatasi(kd, sira, d, codes.A202, codes.A250)
	}
	if err != nil {
		return "", err
	}
	if ham == "" {
		return "", fmt.Errorf("%s: yol boş olamaz", interp.Cagri(kd))
	}
	return ham, nil
}

// mantikAdli adlı bir Mantık argümanını okur (verilmemişse yanlış).
func mantikAdli(kd codes.Kod, named map[string]interp.Value, a bind.Ad) (bool, error) {
	d, ok := bind.AdliAl(named, a)
	if !ok {
		return false, nil
	}
	return bind.Mantik(interp.Cagri(kd), a.TR+":", d)
}
