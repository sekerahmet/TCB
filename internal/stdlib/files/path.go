package files

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// PathType Yol değerlerinin tip adıdır (kod listesinde TR_type_Path).
var PathType = interp.TypeName(codes.TypePath)

// Path temizlenmiş bir dosya ya da klasör yoludur. Değiştirilemez; ayırıcıyı her platformda
// / ile saklar ve gösterir (Windows'ta \ de kabul edilir, işletim sistemine giderken
// çevrilir). Göreli bir yol çözülmeden saklanır: kullanıldığı anda, kullanan kodun klasörüne
// göre 13.1'in tek kuralıyla çözülür (interp.PathValuer).
type Path struct{ fpath string }

// NewPath bir yol metnini temizleyip Yol yapar.
func NewPath(s string) *Path { return &Path{fpath: clean(s)} }

func (y *Path) Type() interp.ValueType { return PathType }
func (y *Path) Inspect() string        { return y.fpath }
func (y *Path) IsTruthy() bool         { return true }

// FilePath yolun kendisidir (çözülmemiş; bkz. interp.PathValuer).
func (y *Path) FilePath() string { return y.fpath }

// Equal iki yolun temizlenmiş yazımları aynıysa doğrudur.
func (y *Path) Equal(d interp.Value) bool {
	b, ok := d.(*Path)
	return ok && b.fpath == y.fpath
}

// PlainValue yolun dış biçimlerdeki (JSON) karşılığıdır: yol metni.
func (y *Path) PlainValue() interp.Value { return &interp.StringVal{Val: y.fpath} }

// clean yolu tek yazıma indirir: Windows'ta \ → /, a/./b/../c → a/c, sondaki / atılır
// (kök dışında).
func clean(s string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(s)))
}

// isAbsPart parça kendi başına bir kökten başlıyor mu (/x, C:/x, C:x, \\sunucu)?
func isAbsPart(p string) bool {
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") {
		return true
	}
	return runtime.GOOS == "windows" && (strings.HasPrefix(p, `\`) || filepath.VolumeName(p) != "")
}

// pathBuiltin yol(...) yerleşiğidir: parçaları birleştirip temizler.
func pathBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) == 0 {
		return nil, interp.ArgCountError(codes.FnPath, interp.AtLeast(1), 0, messages.Text(messages.H2320))
	}
	fn := interp.CallPrefix(codes.FnPath)
	parts := make([]string, len(args))
	for i, a := range args {
		p, ok, err := interp.PathArg(fn, a)
		if !ok {
			return nil, interp.ArgTypeError(codes.FnPath, i+1, a, codes.TypeString, codes.TypePath)
		}
		if err != nil {
			return nil, err
		}
		if p == "" {
			return nil, messages.New(messages.H7138, fn, i+1)
		}
		if i > 0 && isAbsPart(p) {
			return nil, messages.New(messages.H7139, fn, i+1, p)
		}
		parts[i] = filepath.FromSlash(p)
	}
	return NewPath(filepath.Join(parts...)), nil
}

// Yol üyeleri (adları kod listesinde, internal/lang/codes mem).
var pathMemberCodes = []codes.Code{codes.MemName, codes.MemExtension, codes.MemStem, codes.MemParent, codes.MemAbsolute, codes.MemExists, codes.MemIsFile, codes.MemIsDir, codes.MemSize}

// GetProperty üyeyi görevsiz okur (göreli yol yalnızca çalışma klasörüne göre çözülür); nokta
// erişimi GorevliUye'yi kullanır.
func (y *Path) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	return y.TaskMember(nil, kd, name)
}

// TaskMember yolun özelliğini verir; diske bakan özellikler yolu m'nin (çalışan kodun)
// klasörlerine göre çözer.
func (y *Path) TaskMember(m *interp.VM, kd codes.Code, name string) (interp.Value, error) {
	switch kd {
	case codes.MemName:
		return &interp.StringVal{Val: y.fileName()}, nil
	case codes.MemExtension:
		return &interp.StringVal{Val: ext(y.fileName())}, nil
	case codes.MemStem:
		a := y.fileName()
		return &interp.StringVal{Val: a[:len(a)-len(ext(a))]}, nil
	case codes.MemParent:
		return &Path{fpath: filepath.ToSlash(filepath.Dir(filepath.FromSlash(y.fpath)))}, nil
	case codes.MemAbsolute:
		return y.abs(m), nil
	case codes.MemExists, codes.MemIsFile, codes.MemIsDir:
		return y.query(m, kd), nil
	case codes.MemSize:
		return y.size(m)
	}
	return nil, bind.MemberError(PathType, name, pathMemberCodes)
}

// fileName son bileşendir; . ve kök için boştur.
func (y *Path) fileName() string {
	if y.fpath == "." || strings.HasSuffix(y.fpath, "/") {
		return ""
	}
	return y.fpath[strings.LastIndexByte(y.fpath, '/')+1:]
}

// ext adın noktasıyla son uzantısıdır; nokta ile başlayan adda ve noktayla bitende boştur.
func ext(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || i == len(name)-1 {
		return ""
	}
	return name[i:]
}

// resolve yolu m'nin klasörlerine göre çözer (m nil ise yalnızca çalışma klasörü).
func (y *Path) resolve(m *interp.VM) (string, []string) {
	return m.ResolvePath(filepath.FromSlash(y.fpath))
}

func (y *Path) abs(m *interp.VM) *Path {
	c, _ := y.resolve(m)
	if a, err := filepath.Abs(c); err == nil {
		c = a
	}
	return NewPath(c)
}

// query var_mı, dosya_mı ve klasör_mü'dür (bağlantı izlenir).
func (y *Path) query(m *interp.VM, kd codes.Code) interp.Value {
	c, _ := y.resolve(m)
	b, err := os.Stat(c)
	switch {
	case err != nil:
		return interp.False
	case kd == codes.MemIsFile:
		return interp.BoolToVal(b.Mode().IsRegular())
	case kd == codes.MemIsDir:
		return interp.BoolToVal(b.IsDir())
	}
	return interp.True
}

func (y *Path) size(m *interp.VM) (interp.Value, error) {
	fn := codes.CallNameRef(codes.MemSize)
	c, tried := y.resolve(m)
	b, err := os.Stat(c)
	if err != nil {
		return nil, interp.ReadError(fn, y.fpath, messages.H2100, err, tried)
	}
	if b.IsDir() {
		return nil, messages.New(messages.H7140, fn, y.fpath, codes.NameRef(codes.FnListDir))
	}
	return interp.NewInt(b.Size()), nil
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.Equaler        = (*Path)(nil)
	_ interp.PathValuer     = (*Path)(nil)
	_ interp.PropertyGetter = (*Path)(nil)
	_ interp.PlainValuer    = (*Path)(nil)
	_ interp.TaskMemberHook = (*Path)(nil)
)
