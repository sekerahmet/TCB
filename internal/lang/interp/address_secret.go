package interp

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Adresin kimlik kısmı (19.4): adı ne olursa olsun, ortam değişkeninin ve komut satırı
// argümanının değerindeki her adresin kimlik kısmı gizlidir. Kural burada tek yerde uygulanır:
// argümanları SetArgs, ortamı süreç motoru (internal/stdlib/process, Register) program başlarken
// tarar; TR_env: ile dış programa verilen değer de buradan geçer.

// addressSecrets returns the secret value of every address in value (19.4): after each "://", up to
// the first '/', '?', '#' or space (unicode.IsSpace) or the end, the text before the last '@' is the
// userinfo; the part after its first ':' is the secret, without a ':' the whole userinfo. Each is
// given as written and, when different, with its % escapes decoded (url.PathUnescape: '+' stays a
// '+'; an invalid escape gives only the written form). An empty secret is left out.
func addressSecrets(value string) []string {
	var out []string
	for i := 0; ; {
		k := strings.Index(value[i:], "://")
		if k < 0 {
			return out
		}
		start := i + k + len("://")
		end := start
		for end < len(value) {
			r, size := utf8.DecodeRuneInString(value[end:])
			if r == '/' || r == '?' || r == '#' || unicode.IsSpace(r) {
				break
			}
			end += size
		}
		// Bölge '/'da biter ve "://" '/' içerir: sonraki arama bölgenin içinden başlasa da bir bayt
		// en çok iki kez okunur, süre değerin uzunluğuyla doğrusaldır.
		i = start
		at := strings.LastIndexByte(value[start:end], '@')
		if at < 0 {
			continue
		}
		info := value[start : start+at]
		if c := strings.IndexByte(info, ':'); c >= 0 {
			info = info[c+1:]
		}
		if info == "" {
			continue
		}
		out = append(out, info)
		if decoded, err := url.PathUnescape(info); err == nil && decoded != info {
			out = append(out, decoded)
		}
	}
}

// AddAddressSecrets registers the secret value of every address in values with the secret-value
// filter (19.4), and with the part of the program-wide set that VMs register (TR_env: given to an
// external program). A value shorter than MinSecretLength characters is not registered; a number is.
// The filter is rebuilt once for the whole call. It may be called from any goroutine.
func (v *VM) AddAddressSecrets(values ...string) {
	v.addSecrets(addressSecretsOf(values), false)
}

// AddStartAddressSecrets is AddAddressSecrets for the values scanned when the program starts (the
// command line in SetArgs, the environment in the process engine's Register): they go to the start
// part of the program-wide set, which ClearRegisteredSecrets does not empty.
func (v *VM) AddStartAddressSecrets(values ...string) {
	v.addSecrets(addressSecretsOf(values), true)
}

// addressSecretsOf değerlerin hepsinin adres gizlileridir.
func addressSecretsOf(values []string) []string {
	var out []string
	for _, value := range values {
		out = append(out, addressSecrets(value)...)
	}
	return out
}
