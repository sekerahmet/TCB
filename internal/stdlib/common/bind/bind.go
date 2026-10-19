// Package bind, Go paketlerini TR diline bağlayan motor paketlerinin (tensör,
// parçalayıcı, veri) ortak yardımcılarıdır: adlı argüman denetimi, göreli yol çözme,
// paniği hataya çevirme ve sayıların Türkçe kısa yazımı. Kullanıcıya görünen hiçbir ad
// tanımlamaz: adlı argümanların adları kod listesindedir (internal/lang/codes, x9xx kodları).
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package bind

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Named bir adlı argümandır: sözlükteki kodu (x9xx), adı ve kabul edilen yazımları. Adlar
// kod listesinden gelir (internal/lang/codes); Go kodu yalnızca kodu bilir.
type Named struct {
	Code codes.Code
	// TR adın kendisidir (hata mesajlarında ve önerilerde gösterilir).
	TR        string
	spellings []string
}

// NamedArg sözlükteki bir adlı argüman kodunun Ad'ıdır. Paket düzeyinde (başlangıçta) bir
// kez çağrılır; kod bir adlı argüman kodu değilse panikler.
func NamedArg(k codes.Code) Named {
	if k.Namespace() != codes.NamespaceParam {
		panic(fmt.Sprintf("bind.NamedArg: %s bir adlı argüman kodu (x9xx) değil", k))
	}
	return Named{Code: k, TR: codes.Name(k), spellings: codes.Spellings(k)}
}

// Spellings adlı argümanın kabul edilen yazımlarıdır (önce adın kendisi). Dönen dilim
// değiştirilmemelidir.
func (a Named) Spellings() []string { return a.spellings }

// Accepts verilen yazım bu adlı argümanın kabul edilen bir yazımı mı?
func (a Named) Accepts(spelling string) bool { return slices.Contains(a.spellings, spelling) }

// CheckNamed verilen adlı argümanların bilinen adlardan olduğunu denetler; yazım
// hatasında (ya da Türkçe karaktersiz yazılmış, kaldırılmış ya da başka dillerden alışılmış
// bir adda: ogrenme_orani, nerede, lr, axis...) en yakın TR adını önerir. Kaldırılmış ve
// alışılmış adlar kod listesindedir (tr.json'da "eski" ve "yabancı").
func CheckNamed(fn string, named map[string]interp.Value, names ...Named) error {
	for given := range named {
		known := false
		for _, a := range names {
			if a.Accepts(given) {
				known = true
				break
			}
		}
		if known {
			continue
		}
		var candidates []string
		for _, a := range names {
			candidates = append(candidates, a.TR)
		}
		o := ""
		if k, ok := codes.Suggest(codes.NamespaceParam, given, nil); ok && slices.Contains(candidates, codes.Name(k)) {
			o = codes.Name(k)
		}
		if k, ok := codes.Suggest(codes.NamespaceParam, strings.ToLower(given), nil); o == "" && ok && slices.Contains(candidates, codes.Name(k)) {
			o = codes.Name(k) // LR, Seed: alışılmış adlar büyük harfle de tanınır
		}
		if o == "" {
			o = turkish.Closest(given, candidates)
		}
		suggestion := ""
		if o != "" {
			suggestion = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
		} else if len(candidates) > 0 {
			suggestion = fmt.Sprintf(" (geçerli adlar: %s)", strings.Join(candidates, ", "))
		} else {
			suggestion = " (adlı argüman almaz)"
		}
		return fmt.Errorf("%s '%s' adında bir parametre almaz%s", fn, given, suggestion)
	}
	return nil
}

// MemberError motor tiplerinde bulunamayan bir üye için hatadır: yazıma en yakın üyeyi, yakın
// bir üye yoksa tipin üyelerini önerir. ne tipin (ya da nesnenin) kullanıcıya görünen adıdır.
func MemberError(what, prop string, members []string) error {
	suggestion := ""
	if o := turkish.Closest(prop, members); o != "" {
		suggestion = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	} else {
		suggestion = fmt.Sprintf(" (üyeler: %s)", strings.Join(members, ", "))
	}
	return fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", what, prop, suggestion)
}

// GetNamed adlı argümanı kabul edilen yazımlarıyla (önce adın kendisiyle) arar.
func GetNamed(named map[string]interp.Value, a Named) (interp.Value, bool) {
	for _, y := range a.spellings {
		if v, ok := named[y]; ok {
			return v, true
		}
	}
	return nil, false
}

// Int bir TR değerini tam sayıya çevirir (tam değerli ondalıklar da kabul edilir).
func Int(v interp.Value) (int64, bool) {
	switch n := v.(type) {
	case *interp.IntVal:
		return n.Val, true
	case *interp.FloatVal:
		if n.Val == float64(int64(n.Val)) {
			return int64(n.Val), true
		}
	}
	return 0, false
}

// Bool bir TR değerinin doğru/yanlış olmasını bekler.
func Bool(fn, name string, v interp.Value) (bool, error) {
	b, ok := v.(*interp.BoolVal)
	if !ok {
		return false, fmt.Errorf("%s: %s doğru ya da yanlış olmalıdır, %s verildi", fn, name, v.Type())
	}
	return b.Val, nil
}

// Recover fn'i çalıştırır; fn panik yaparsa programı çökertmek yerine paniği cevir ile
// TR'nin dene/yakala ile yakalanabilen bir hataya çevirir.
func Recover[T any](fn func() T, convert func(r any) error) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			if interp.IsInterrupt(r) {
				panic(r) // kesilme (Ctrl+C) hata değildir; yığın çözülmeye devam eder
			}
			var zero T
			result, err = zero, convert(r)
		}
	}()
	return fn(), nil
}

// Safe f'yi çalıştırır; Go paniğini "beklenmeyen iç hata" olarak bildirir.
func Safe(fn string, f func() (interp.Value, error)) (interp.Value, error) {
	var ferr error
	result, err := Recover(func() interp.Value {
		v, e := f()
		ferr = e
		return v
	}, func(r any) error { return fmt.Errorf("%s: beklenmeyen iç hata: %v", fn, r) })
	if err != nil {
		return nil, err
	}
	return result, ferr
}

// ResolvePath göreli bir yolu TR'nin tek yol kuralıyla çözer (bkz. interp.(*VM).ResolvePath: önce
// çalışma klasörü, sonra programın klasörü). Hiçbir yerde yoksa hata mesajı kullanıcının
// yazdığı yolu gösterebilsin diye çalışma klasörüne göre yol döner.
func ResolvePath(m *interp.VM, path string) string {
	resolved, _ := ResolvePathTried(m, path)
	return resolved
}

// ResolvePathTried ResolvePath gibidir; ayrıca hata mesajları için bakılan yerleri döndürür.
func ResolvePathTried(m *interp.VM, path string) (string, []string) {
	return m.ResolvePath(path) // m nil olabilir: o zaman yalnızca çalışma klasörüne bakılır
}

// IsNotExist hatanın "dosya ya da klasör yok" anlamına gelip gelmediğini söyler.
func IsNotExist(err error) bool { return errors.Is(err, os.ErrNotExist) }

// ShortNumber büyük sayıları Türkçe kısa biçimde yazar: 999 → "999", 12 345 → "12,3 bin",
// 56 900 000 000 → "56,9 milyar".
func ShortNumber(n int64) string {
	units := []struct {
		threshold int64
		name      string
	}{{1e12, "trilyon"}, {1e9, "milyar"}, {1e6, "milyon"}, {1e3, "bin"}}
	for _, b := range units {
		if n >= b.threshold {
			x := float64(n) / float64(b.threshold)
			s := strconv.FormatFloat(x, 'f', 1, 64)
			s = strings.TrimSuffix(s, ".0")
			return strings.Replace(s, ".", ",", 1) + " " + b.name
		}
	}
	return strconv.FormatInt(n, 10)
}

// FormatBytes bayt sayısını okunur biçimde yazar (ör. "1,2 GB").
func FormatBytes(n int64) string {
	units := []struct {
		threshold int64
		name      string
	}{{1 << 40, "TB"}, {1 << 30, "GB"}, {1 << 20, "MB"}, {1 << 10, "KB"}}
	for _, b := range units {
		if n >= b.threshold {
			s := strconv.FormatFloat(float64(n)/float64(b.threshold), 'f', 1, 64)
			s = strings.TrimSuffix(s, ".0")
			return strings.Replace(s, ".", ",", 1) + " " + b.name
		}
	}
	return strconv.FormatInt(n, 10) + " bayt"
}
