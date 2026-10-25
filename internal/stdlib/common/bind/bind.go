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
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
		panic(messages.Internal(fmt.Sprintf("bind.NamedArg: %s is not a named-argument code (x9xx)", k)))
	}
	return Named{Code: k, TR: codes.Name(k), spellings: codes.Spellings(k)}
}

// Spellings adlı argümanın kabul edilen yazımlarıdır (önce adın kendisi). Dönen dilim
// değiştirilmemelidir.
func (a Named) Spellings() []string { return a.spellings }

// MessageText adlı argümanın iletinin dilindeki adıdır (messages.Localized): iletiye a verilir,
// a.TR değil.
func (a Named) MessageText(lang string) string { return codes.NameRef(a.Code).MessageText(lang) }

// Accepts verilen yazım bu adlı argümanın kabul edilen bir yazımı mı?
func (a Named) Accepts(spelling string) bool { return slices.Contains(a.spellings, spelling) }

// CheckNamed verilen adlı argümanların bilinen adlardan olduğunu denetler; yazım
// hatasında (ya da Türkçe karaktersiz yazılmış, kaldırılmış ya da başka dillerden alışılmış
// bir adda: ogrenme_orani, nerede, lr, axis...) en yakın TR adını önerir. Kaldırılmış ve
// alışılmış adlar kod listesindedir (tr.json'da "eski" ve "yabancı").
func CheckNamed(fn any, named map[string]interp.Value, names ...Named) error {
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
		var candidates, shown []string // eşleştirme kanonik adlarla, ileti yazım diliyle
		display := map[string]string{}
		for _, a := range names {
			candidates = append(candidates, a.TR)
			d := codes.DisplayName(a.Code)
			shown, display[a.TR] = append(shown, d), d
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
		switch {
		case o != "":
			return messages.New(messages.H7900, fn, given, display[o])
		case len(candidates) > 0:
			return messages.New(messages.H7901, fn, given, strings.Join(shown, ", "))
		}
		return messages.New(messages.H7902, fn, given)
	}
	return nil
}

// MemberError motor tiplerinde bulunamayan bir üye için hatadır: yazıma en yakın üyeyi, yakın
// bir üye yoksa tipin üyelerini önerir. ne tipin (ya da nesnenin) kullanıcıya görünen adıdır.
func MemberError(what, prop string, members []string) error {
	if o := turkish.Closest(prop, members); o != "" {
		return messages.New(messages.H7903, what, prop, o)
	}
	return messages.New(messages.H7904, what, prop, strings.Join(members, ", "))
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
func Bool(fn any, name any, v interp.Value) (bool, error) {
	b, ok := v.(*interp.BoolVal)
	if !ok {
		return false, messages.New(messages.H7905, fn, name, v.Type())
	}
	return b.Val, nil
}

// OptionalBool adlı bir Mantık argümanını okur; verilmemişse defaultVal döner. Hata iletisi
// argümanı adıyla (iletinin dilinde) anar.
func OptionalBool(fn any, named map[string]interp.Value, a Named, defaultVal bool) (bool, error) {
	v, ok := GetNamed(named, a)
	if !ok {
		return defaultVal, nil
	}
	return Bool(fn, a, v)
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

// Safe f'yi çalıştırır; Go paniğini iç hata olarak (messages.Internal: "lütfen bildirin")
// bildirir.
func Safe(fn any, f func() (interp.Value, error)) (interp.Value, error) {
	var ferr error
	result, err := Recover(func() interp.Value {
		v, e := f()
		ferr = e
		return v
	}, func(r any) error { return messages.New(messages.H2000, fn, messages.Internal(fmt.Sprint(r))) })
	if err != nil {
		return nil, err
	}
	return result, ferr
}

// IntList tam sayı dilimini TR tam sayı listesine çevirir.
func IntList[T ~int | ~int32 | ~int64](xs []T) *interp.ListVal {
	elems := make([]interp.Value, len(xs))
	for i, x := range xs {
		elems[i] = interp.NewInt(int64(x))
	}
	return &interp.ListVal{Elements: elems}
}

// SingleDictArg tek bir sözlük argümanı bekleyen yöntemin (kod kd) argümanını verir; sayı ya da tür
// yanlışsa interp.ArgCountError/ArgTypeError verir. example sayı hatasında kullanım ipucu olarak anılır.
func SingleDictArg(kd codes.Code, args []interp.Value, example string) (*interp.DictVal, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(kd, "1", len(args), example)
	}
	d, ok := args[0].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(kd, 1, args[0], codes.A206)
	}
	return d, nil
}

// SafeBuiltin fn yerleşiğini Go paniğine karşı korur: panik name adlı bir iç hataya döner (Safe).
func SafeBuiltin(name any, fn interp.BuiltinFunc) interp.BuiltinFunc {
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return Safe(name, func() (interp.Value, error) { return fn(m, args, named) })
	}
}

// Method nesne yöntemi değeri kurar: fn Go paniğine karşı korunur (panik name adlı bir hataya döner).
func Method(name string, fn interp.BuiltinFunc) *interp.BuiltinVal {
	return &interp.BuiltinVal{Name: name, Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return Safe(name, func() (interp.Value, error) { return fn(m, args, named) })
	}}
}

// CodeMethod üye koduyla (kd) adlı yöntem değeri kurar: değerin adı kodun kanonik adıdır, panik
// iletisindeki ad interp.CallName(kd)'dir. noNamedArgs doğruysa adlı argüman verilmesi hatadır.
func CodeMethod(kd codes.Code, noNamedArgs bool, fn interp.BuiltinFunc) *interp.BuiltinVal {
	name := interp.CallName(kd) // iletide iletinin dilinde (codes.Ref)
	return &interp.BuiltinVal{Name: codes.Name(kd), NoNamedArgs: noNamedArgs, Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return Safe(name, func() (interp.Value, error) { return fn(m, args, named) })
	}}
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

// ShortNumber büyük sayıyı kısa biçimde yazar, ileti parametresi olarak (messages.Localized):
// 999 → "999", 12 345 → "12,3 bin" (en: "12.3 thousand"), 56 900 000 000 → "56,9 milyar".
func ShortNumber(n int64) messages.Localized {
	return scaled{n, []scaleUnit{{1e12, messages.H7909, ""}, {1e9, messages.H7908, ""}, {1e6, messages.H7907, ""}, {1e3, messages.H7906, ""}}, 0}
}

// FormatBytes bayt sayısını okunur biçimde yazar, ileti parametresi olarak: "1,2 GB", "512 bayt"
// (en: "1.2 GB", "512 bytes").
func FormatBytes(n int64) messages.Localized {
	return scaled{n, []scaleUnit{{1 << 40, messages.H7910, "TB"}, {1 << 30, messages.H7910, "GB"}, {1 << 20, messages.H7910, "MB"}, {1 << 10, messages.H7910, "KB"}}, messages.H4020}
}

// scaleUnit bir ölçek basamağıdır: n en az threshold ise text iletisiyle (sayı, birim simgesi)
// yazılır.
type scaleUnit struct {
	threshold int64
	text      messages.Code
	symbol    string
}

// scaled n'yi en büyük uygun basamakta bir ondalıkla yazar (".0" atılır; sözcük ve ondalık ayırıcı
// iletinin dilinde); hiçbir basamağa ulaşmazsa sayının kendisini small iletisiyle (yoksa yalın).
type scaled struct {
	n     int64
	units []scaleUnit
	small messages.Code
}

func (s scaled) MessageText(lang string) string {
	for _, b := range s.units {
		if s.n >= b.threshold {
			v := strings.TrimSuffix(strconv.FormatFloat(float64(s.n)/float64(b.threshold), 'f', 1, 64), ".0")
			if b.symbol != "" {
				return messages.TextIn(lang, b.text, v, b.symbol)
			}
			return messages.TextIn(lang, b.text, v)
		}
	}
	if s.small == 0 {
		return strconv.FormatInt(s.n, 10)
	}
	return messages.TextIn(lang, s.small, s.n)
}
