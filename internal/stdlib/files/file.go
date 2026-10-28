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
// klasörü, sonra kodun modülünün ve programın klasörü; bkz. interp.(*VM).ResolvePath).
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package files

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	nameOverwrite    = bind.NamedArg(codes.ArgOverwrite)
	nameWithContents = bind.NamedArg(codes.ArgRecursive)
)

// Register dosya ve yol yerleşiklerini VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterNoNamed(codes.FnReadLines, safe(codes.FnReadLines, linesBuiltin))
	v.RegisterTypeCall(codes.TypePath, &interp.BuiltinVal{NoNamedArgs: true, Fn: safe(codes.TypePath, pathBuiltin)})
	v.RegisterNoNamed(codes.FnListDir, safe(codes.FnListDir, listBuiltin))
	v.RegisterNoNamed(codes.FnMakeDir, safe(codes.FnMakeDir, makeDirBuiltin))
	v.RegisterBuiltinCode(codes.FnRemove, safe(codes.FnRemove, deleteBuiltin))
	v.RegisterBuiltinCode(codes.FnCopy, safe(codes.FnCopy, copyBuiltin))
	v.RegisterBuiltinCode(codes.FnMove, safe(codes.FnMove, moveBuiltin))
	v.RegisterNoNamed(codes.FnTempDir, safe(codes.FnTempDir, tempDirBuiltin))
}

// safe yerleşiği Go paniğine karşı korur (panik dene/yakala ile yakalanan bir hataya döner).
func safe(kd codes.Code, fn interp.BuiltinFunc) interp.BuiltinFunc {
	return bind.SafeBuiltin(interp.CallPrefix(kd), fn)
}

// readPath yol alan bir yerleşiğin sira. argümanını (Metin ya da Yol) okur; yolda sekme ya da
// satır sonu varsa ham metni öneren hatayı, boş yolda açık bir hatayı verir.
func readPath(kd codes.Code, order int, d interp.Value) (string, error) {
	raw, ok, err := interp.PathArg(interp.CallPrefix(kd), d)
	if !ok {
		return "", interp.ArgTypeError(kd, order, d, codes.TypeString, codes.TypePath)
	}
	if err != nil {
		return "", err
	}
	if raw == "" {
		return "", messages.New(messages.H7124, interp.CallPrefix(kd))
	}
	return raw, nil
}
