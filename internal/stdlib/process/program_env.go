package process

import (
	"os"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Dış programın ortamı (21.6): TR programının ortamı, gizli adlı değişkenler olmadan. TR_env: ekler ya
// da değiştirir, geri kalanı silmez; aynı adın son yazımı geçerlidir (os/exec, Windows'ta harf ayırmadan).

// programEnv programın ortamını ve süzülen adları verir. TR_env: ile açıkça verilen gizli değişken
// süzülmüş sayılmaz; değeri gizli değer süzgecine girer: dosyadan okunan anahtar süzgeçte yoktur ve
// programın standart hatası onu iletiye taşırdı.
func programEnv(m *interp.VM, fn any, named map[string]interp.Value) (env, filtered []string, err error) {
	given, err := givenEnv(fn, named)
	if err != nil {
		return nil, nil, err
	}
	env = make([]string, 0, len(given))
	for _, entry := range os.Environ() {
		name := envEntryName(entry)
		if !secretEntry(name, entry) {
			env = append(env, entry)
		} else if !slices.ContainsFunc(given, func(g [2]string) bool { return sameEnvName(g[0], name) }) {
			filtered = append(filtered, name)
		}
	}
	values := make([]string, 0, len(given))
	for _, g := range given {
		if IsSecretName(g[0]) {
			addSecret(m, g[1])
		}
		values = append(values, g[1])
		env = append(env, g[0]+"="+g[1])
	}
	if m != nil {
		m.AddAddressSecrets(values...) // adresin kimlik kısmı, adı ne olursa olsun (19.4)
	}
	slices.Sort(filtered)
	return env, filtered, nil
}

// givenEnv TR_env: argümanının ad ve değer çiftleridir. Ad 19.1'in kuralına uyar (H7231): boş ad, '='
// içeren ad başka bir değişkeni değiştirirdi. Boş bayt ad ve değerde dizeyi keserdi.
func givenEnv(fn any, named map[string]interp.Value) ([][2]string, error) {
	v, ok := bind.GetNamed(named, nameEnv)
	if !ok {
		return nil, nil
	}
	dict, ok := v.(*interp.DictVal)
	if !ok {
		return nil, messages.New(messages.H7507, fn, nameEnv, v.Type())
	}
	out := make([][2]string, 0, len(dict.Order))
	for _, k := range dict.Order {
		key, ok := dict.KeyValue(k).(*interp.StringVal)
		if !ok {
			return nil, messages.New(messages.H7507, fn, nameEnv, dict.KeyValue(k).Type())
		}
		if key.Val == "" || strings.ContainsAny(key.Val, "=\x00") {
			return nil, messages.New(messages.H7231, fn, key.Val)
		}
		value, ok := dict.Pairs[k].(*interp.StringVal)
		if !ok {
			return nil, messages.New(messages.H7507, fn, nameEnv, dict.Pairs[k].Type())
		}
		if strings.IndexByte(value.Val, 0) >= 0 {
			return nil, messages.New(messages.H7505, fn, key.Val)
		}
		out = append(out, [2]string{key.Val, value.Val})
	}
	return out, nil
}

// secretEntry girdinin aktarılmayacağını söyler: ad ile değere birlikte bakılır, gizli adlı değişkenin
// yalnız sayı ya da mantık olan değeri gizli sayılmaz (19.4): MAX_TOKENS=4096 programa gider.
func secretEntry(name, entry string) bool {
	return IsSecretName(name) && !isNumberOrBool(entry[min(len(name)+1, len(entry)):])
}

// envEntryName AD=değer girdisinin adıdır. Windows'un sürücü klasörü girdileri (=C:=C:\x) '=' ile başlar.
func envEntryName(entry string) string {
	if i := strings.IndexByte(entry[min(1, len(entry)):], '='); i >= 0 {
		return entry[:i+1]
	}
	return entry
}

// sameEnvName iki adın aynı değişken olduğunu söyler: Windows'ta ad büyük küçük harf ayırmaz.
func sameEnvName(a, b string) bool {
	if onWindows {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// filteredHint başarısız bitişin iletisine süzülen adları (değerlerini değil) ve TR_env: yazımını
// ekler (21.6). Süzülen yoksa ipucu yoktur.
func filteredHint(filtered []string) any {
	if len(filtered) == 0 {
		return ""
	}
	return messages.New(messages.H7509, strings.Join(filtered, ", "), filtered[0])
}
