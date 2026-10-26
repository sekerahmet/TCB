// Package bind, Go paketlerini TR diline bağlayan motor paketlerinin (tensör,
// parçalayıcı, veri) ortak yardımcılarıdır: adlı argüman denetimi, göreli yol çözme,
// paniği hataya çevirme ve sayıların Türkçe kısa yazımı. Kullanıcıya görünen hiçbir ad
// tanımlamaz: adlı argümanların adları kod listesindedir (internal/lang/codes, arg kodları).
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

// Named bir adlı argümandır: sözlükteki kodu (arg), adı ve kabul edilen yazımları. Adlar
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
		panic(messages.Internal(fmt.Sprintf("bind.NamedArg: %s is not a named-argument code (arg)", k)))
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
// bir adda: ogrenme_orani, nerede, lr, axis...) en yakın TR adını önerir. Alışılmış adlar
// kod listesindedir (dil dosyasının "foreign" alanı).
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
		var candidates []string // eşleştirme kanonik adlarla; iletide ad iletinin dilinde
		byName := map[string]Named{}
		for _, a := range names {
			candidates = append(candidates, a.TR)
			byName[a.TR] = a
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
			return messages.New(messages.H7900, fn, given, byName[o])
		case len(candidates) > 0:
			codeList := make(nameList, len(names))
			for i, a := range names {
				codeList[i] = a.Code
			}
			return messages.New(messages.H7901, fn, given, codeList)
		}
		return messages.New(messages.H7902, fn, given)
	}
	return nil
}

// nameList kodların virgülle ayrılmış adlarıdır, ileti parametresi olarak (messages.Localized):
// adlar iletinin dilinde.
type nameList []codes.Code

func (l nameList) MessageText(lang string) string {
	parts := make([]string, len(l))
	for i, k := range l {
		parts[i] = codes.NameRef(k).MessageText(lang)
	}
	return strings.Join(parts, ", ")
}

// MemberError motor tiplerinde bulunamayan bir üye için hatadır: yazıma en yakın üyeyi, yakın
// bir üye yoksa tipin üyelerini önerir (adlar iletinin dilinde). what tipin (ya da nesnenin)
// ileti parametresi olarak adıdır (interp.ValueType ya da kodlu ileti).
func MemberError(what any, prop string, members []codes.Code) error {
	if o, ok := ClosestMember(prop, members); ok {
		return messages.New(messages.H7903, what, prop, o)
	}
	return messages.New(messages.H7904, what, prop, nameList(members))
}

// ClosestMember yazıma (kanonik adlarıyla) en yakın üyedir, ileti parametresi olarak (codes.Ref:
// öneri iletinin dilinde).
func ClosestMember(prop string, members []codes.Code) (codes.Ref, bool) {
	names := codes.Names(members...)
	if o := turkish.Closest(prop, names); o != "" {
		return codes.NameRef(members[slices.Index(names, o)]), true
	}
	return codes.Ref{}, false
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
		return nil, interp.ArgCountError(kd, 1, len(args), example)
	}
	d, ok := args[0].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(kd, 1, args[0], codes.TypeDict)
	}
	return d, nil
}

// SafeBuiltin fn yerleşiğini Go paniğine karşı korur: panik name adlı bir iç hataya döner (Safe).
func SafeBuiltin(name any, fn interp.BuiltinFunc) interp.BuiltinFunc {
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return Safe(name, func() (interp.Value, error) { return fn(m, args, named) })
	}
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
func ShortNumber(n int64) messages.Localized { return shortNumber(n) }

// shortNumber n'yi en büyük uygun basamakta (bin, milyon, milyar, trilyon) bir ondalıkla yazar
// (".0" atılır; sözcük ve ondalık ayırıcı iletinin dilinde); bine ulaşmazsa sayının kendisini.
type shortNumber int64

// scales kısa sayının basamaklarıdır (büyükten küçüğe) ve iletileri.
var scales = []struct {
	threshold int64
	text      messages.Code
}{{1e12, messages.H7909}, {1e9, messages.H7908}, {1e6, messages.H7907}, {1e3, messages.H7906}}

func (n shortNumber) MessageText(lang string) string {
	for _, s := range scales {
		if int64(n) >= s.threshold {
			return messages.TextIn(lang, s.text, strings.TrimSuffix(strconv.FormatFloat(float64(n)/float64(s.threshold), 'f', 1, 64), ".0"))
		}
	}
	return strconv.FormatInt(int64(n), 10)
}
