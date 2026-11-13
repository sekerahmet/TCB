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
		_, start, end, _, ok := addressRegion(value, i)
		if !ok {
			return out
		}
		i = start
		info, _, _, ok := userinfoSecret(value[start:end])
		if !ok || info == "" {
			continue
		}
		out = append(out, info)
		if decoded, err := url.PathUnescape(info); err == nil && decoded != info {
			out = append(out, decoded)
		}
	}
}

// span, metnin SecretMask'la değişecek bayt aralığıdır: [start, end).
type span struct{ start, end int }

// contextSpans is the contextual mask's part of text (19.4): in each address region the bytes of
// the userinfo's secret, found by the same walk as addressSecrets. With a ':' the part after the
// first ':' is covered whatever its length (an empty one is left alone); without a ':' the whole
// userinfo, when it has at least MinSecretLength characters. Nothing is registered: the spans are
// positions in this text only. atEnd: the end of text closes a region; otherwise the walk stops at
// the first region with no boundary yet, whose "://" offset is open (-1 when every region closed).
func contextSpans(text string, atEnd bool) (spans []span, open int) {
	for i := 0; ; {
		p, start, end, closed, ok := addressRegion(text, i)
		if !ok {
			return spans, -1
		}
		if !closed && !atEnd {
			return spans, p
		}
		i = start
		secret, off, colon, ok := userinfoSecret(text[start:end])
		if !ok || secret == "" || !colon && utf8.RuneCountInString(secret) < MinSecretLength {
			continue
		}
		spans = append(spans, span{start + off, start + off + len(secret)})
	}
}

// addressRegion finds the first address region at or after i: p is the offset of its "://", start
// the byte after it, end the first '/', '?', '#' or space (unicode.IsSpace) from start, or len(text)
// with closed false. ok is false when there is no "://" from i on.
func addressRegion(text string, i int) (p, start, end int, closed, ok bool) {
	k := strings.Index(text[i:], "://")
	if k < 0 {
		return 0, 0, 0, false, false
	}
	p = i + k
	start = p + len("://")
	for end = start; end < len(text); {
		r, size := utf8.DecodeRuneInString(text[end:])
		if r == '/' || r == '?' || r == '#' || unicode.IsSpace(r) {
			// Bölge '/'da biter ve "://" '/' içerir: sonraki arama bölgenin içinden (start'tan)
			// başlasa da bir bayt en çok iki kez okunur, süre metnin uzunluğuyla doğrusaldır.
			return p, start, end, true, true
		}
		end += size
	}
	return p, start, end, false, true
}

// userinfoSecret is the secret of a region's userinfo (the text before the region's last '@'): the
// part after its first ':' (colon true), or the whole userinfo; off is its offset in region. ok is
// false when region has no '@'.
func userinfoSecret(region string) (secret string, off int, colon, ok bool) {
	at := strings.LastIndexByte(region, '@')
	if at < 0 {
		return "", 0, false, false
	}
	info := region[:at]
	if c := strings.IndexByte(info, ':'); c >= 0 {
		return info[c+1:], c + 1, true, true
	}
	return info, 0, false, true
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
