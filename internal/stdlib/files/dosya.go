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
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
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
	v.RegisterNoNamed(codes.G112, guvenli(codes.G112, satirlarYerlesik))
	v.RegisterNoNamed(codes.G120, guvenli(codes.G120, yolYerlesik))
	v.RegisterNoNamed(codes.G121, guvenli(codes.G121, listeleYerlesik))
	v.RegisterNoNamed(codes.G122, guvenli(codes.G122, klasorOlusturYerlesik))
	v.RegisterBuiltinCode(codes.G123, guvenli(codes.G123, silYerlesik))
	v.RegisterBuiltinCode(codes.G124, guvenli(codes.G124, kopyalaYerlesik))
	v.RegisterBuiltinCode(codes.G125, guvenli(codes.G125, tasiYerlesik))
	v.RegisterNoNamed(codes.G126, guvenli(codes.G126, geciciKlasorYerlesik))
}

// guvenli yerleşiği Go paniğine karşı korur (panik dene/yakala ile yakalanan bir hataya döner).
func guvenli(kd codes.Code, fn interp.BuiltinFunc) interp.BuiltinFunc {
	ad := interp.CallPrefix(kd)
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(ad, func() (interp.Value, error) { return fn(m, args, named) })
	}
}

// yolOku yol alan bir yerleşiğin sira. argümanını (Metin ya da Yol) okur; yolda sekme ya da
// satır sonu varsa ham metni öneren hatayı, boş yolda açık bir hatayı verir.
func yolOku(kd codes.Code, sira int, d interp.Value) (string, error) {
	ham, ok, err := interp.PathArg(interp.CallPrefix(kd), d)
	if !ok {
		return "", interp.ArgTypeError(kd, sira, d, codes.A202, codes.A250)
	}
	if err != nil {
		return "", err
	}
	if ham == "" {
		return "", fmt.Errorf("%s: yol boş olamaz", interp.CallPrefix(kd))
	}
	return ham, nil
}

// mantikAdli adlı bir Mantık argümanını okur (verilmemişse yanlış).
func mantikAdli(kd codes.Code, named map[string]interp.Value, a bind.Ad) (bool, error) {
	d, ok := bind.AdliAl(named, a)
	if !ok {
		return false, nil
	}
	return bind.Mantik(interp.CallPrefix(kd), a.TR+":", d)
}
