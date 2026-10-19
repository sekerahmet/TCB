package files

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// YolTipi Yol değerlerinin tip adıdır (kod listesinde A250).
var YolTipi = interp.TypeName(codes.A250)

// Yol temizlenmiş bir dosya ya da klasör yoludur. Değiştirilemez; ayırıcıyı her platformda
// / ile saklar ve gösterir (Windows'ta \ de kabul edilir, işletim sistemine giderken
// çevrilir). Göreli bir yol çözülmeden saklanır: kullanıldığı anda, kullanan kodun klasörüne
// göre 13.1'in tek kuralıyla çözülür (yurut.YolDegerli).
type Yol struct{ yol string }

var (
	_ interp.PathValuer     = (*Yol)(nil)
	_ interp.Equaler        = (*Yol)(nil)
	_ interp.TaskMemberHook = (*Yol)(nil)
	_ interp.PlainValuer    = (*Yol)(nil)
)

// YeniYol bir yol metnini temizleyip Yol yapar.
func YeniYol(s string) *Yol { return &Yol{yol: temizle(s)} }

func (y *Yol) Type() interp.ValueType { return YolTipi }
func (y *Yol) Inspect() string        { return y.yol }
func (y *Yol) IsTruthy() bool         { return true }

// FilePath yolun kendisidir (çözülmemiş; bkz. yurut.YolDegerli).
func (y *Yol) FilePath() string { return y.yol }

// Equal iki yolun temizlenmiş yazımları aynıysa doğrudur.
func (y *Yol) Equal(d interp.Value) bool {
	b, ok := d.(*Yol)
	return ok && b.yol == y.yol
}

// PlainValue yolun dış biçimlerdeki (JSON) karşılığıdır: yol metni.
func (y *Yol) PlainValue() interp.Value { return &interp.StringVal{Val: y.yol} }

// temizle yolu tek yazıma indirir: Windows'ta \ → /, a/./b/../c → a/c, sondaki / atılır
// (kök dışında).
func temizle(s string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(s)))
}

// mutlakParcaMi parça kendi başına bir kökten başlıyor mu (/x, C:/x, C:x, \\sunucu)?
func mutlakParcaMi(p string) bool {
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") {
		return true
	}
	return runtime.GOOS == "windows" && (strings.HasPrefix(p, `\`) || filepath.VolumeName(p) != "")
}

// yolYerlesik yol(...) yerleşiğidir: parçaları birleştirip temizler.
func yolYerlesik(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) == 0 {
		return nil, interp.ArgCountError(codes.G120, "en az 1", 0, "parça, ...")
	}
	fn := interp.CallPrefix(codes.G120)
	parcalar := make([]string, len(args))
	for i, a := range args {
		p, ok, err := interp.PathArg(fn, a)
		if !ok {
			return nil, interp.ArgTypeError(codes.G120, i+1, a, codes.A202, codes.A250)
		}
		if err != nil {
			return nil, err
		}
		if p == "" {
			return nil, fmt.Errorf("%s %d. parça boş metin; bulunulan klasör için \".\" yazın", fn, i+1)
		}
		if i > 0 && mutlakParcaMi(p) {
			return nil, fmt.Errorf("%s %d. parça %q mutlak bir yol; yalnızca ilk parça mutlak olabilir (dışarıdan gelen bir ad taban klasörü değiştiremesin). Göreli bir ad verin", fn, i+1, p)
		}
		parcalar[i] = filepath.FromSlash(p)
	}
	return YeniYol(filepath.Join(parcalar...)), nil
}

// Yol üyeleri (adları kod listesinde, internal/lang/codes Y31x).
var yolUyeKodlari = []codes.Code{codes.Y310, codes.Y311, codes.Y312, codes.Y313, codes.Y314, codes.Y315, codes.Y316, codes.Y317, codes.Y318}

// GetProperty üyeyi görevsiz okur (göreli yol yalnızca çalışma klasörüne göre çözülür); nokta
// erişimi GorevliUye'yi kullanır.
func (y *Yol) GetProperty(kd codes.Code, ad string) (interp.Value, error) {
	return y.TaskMember(nil, kd, ad)
}

// TaskMember yolun özelliğini verir; diske bakan özellikler yolu m'nin (çalışan kodun)
// klasörlerine göre çözer.
func (y *Yol) TaskMember(m *interp.VM, kd codes.Code, ad string) (interp.Value, error) {
	switch kd {
	case codes.Y310:
		return &interp.StringVal{Val: y.dosyaAdi()}, nil
	case codes.Y311:
		return &interp.StringVal{Val: uzanti(y.dosyaAdi())}, nil
	case codes.Y312:
		a := y.dosyaAdi()
		return &interp.StringVal{Val: a[:len(a)-len(uzanti(a))]}, nil
	case codes.Y313:
		return &Yol{yol: filepath.ToSlash(filepath.Dir(filepath.FromSlash(y.yol)))}, nil
	case codes.Y314:
		return y.mutlak(m), nil
	case codes.Y315, codes.Y316, codes.Y317:
		return y.soru(m, kd), nil
	case codes.Y318:
		return y.bayt(m)
	}
	return nil, bind.MemberError(string(YolTipi), ad, codes.Names(yolUyeKodlari...))
}

// dosyaAdi son bileşendir; . ve kök için boştur.
func (y *Yol) dosyaAdi() string {
	if y.yol == "." || strings.HasSuffix(y.yol, "/") {
		return ""
	}
	return y.yol[strings.LastIndexByte(y.yol, '/')+1:]
}

// uzanti adın noktasıyla son uzantısıdır; nokta ile başlayan adda ve noktayla bitende boştur.
func uzanti(ad string) string {
	i := strings.LastIndexByte(ad, '.')
	if i <= 0 || i == len(ad)-1 {
		return ""
	}
	return ad[i:]
}

// coz yolu m'nin klasörlerine göre çözer (m nil ise yalnızca çalışma klasörü).
func (y *Yol) coz(m *interp.VM) (string, []string) {
	return m.ResolvePath(filepath.FromSlash(y.yol))
}

func (y *Yol) mutlak(m *interp.VM) *Yol {
	c, _ := y.coz(m)
	if a, err := filepath.Abs(c); err == nil {
		c = a
	}
	return YeniYol(c)
}

// soru var_mı, dosya_mı ve klasör_mü'dür (bağlantı izlenir).
func (y *Yol) soru(m *interp.VM, kd codes.Code) interp.Value {
	c, _ := y.coz(m)
	b, err := os.Stat(c)
	switch {
	case err != nil:
		return interp.FALSE
	case kd == codes.Y316:
		return interp.BoolToVal(b.Mode().IsRegular())
	case kd == codes.Y317:
		return interp.BoolToVal(b.IsDir())
	}
	return interp.TRUE
}

func (y *Yol) bayt(m *interp.VM) (interp.Value, error) {
	fn := codes.MemberName(codes.Y318)
	c, bakilanlar := y.coz(m)
	b, err := os.Stat(c)
	if err != nil {
		return nil, interp.ReadError(fn, y.yol, "okunamadı", err, bakilanlar)
	}
	if b.IsDir() {
		return nil, fmt.Errorf("%s: %q bir klasör; klasörün boyutu yoktur (içindeki dosyaların boyutunu toplayın: %s(...))", fn, y.yol, codes.Name(codes.G121))
	}
	return interp.NewInt(b.Size()), nil
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.Equaler        = (*Yol)(nil)
	_ interp.PathValuer     = (*Yol)(nil)
	_ interp.PropertyGetter = (*Yol)(nil)
	_ interp.PlainValuer    = (*Yol)(nil)
	_ interp.TaskMemberHook = (*Yol)(nil)
)
