package messages

import (
	"fmt"
	"strings"
)

// segment bir ileti şablonunun parçasıdır: düz metin ya da bir parametre.
type segment struct {
	text  string
	arg   int  // parametrenin sırası; düz metinde -1
	quote bool // {ad|q}: Go'nun %q biçimiyle tırnaklı (denetim karakterleri kaçırılır)
}

// compile şablonu parçalara ayırır: {ad} ve {ad|q} parametredir, {{ ve }} düz parantezdir.
func compile(text string, params []string) ([]segment, []bool, error) {
	index := map[string]int{}
	for i, p := range params {
		index[p] = i
	}
	used := make([]bool, len(params))
	var segs []segment
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			segs = append(segs, segment{text: lit.String(), arg: -1})
			lit.Reset()
		}
	}
	for i := 0; i < len(text); i++ {
		switch ch := text[i]; {
		case ch == '{' && i+1 < len(text) && text[i+1] == '{', ch == '}' && i+1 < len(text) && text[i+1] == '}':
			lit.WriteByte(ch)
			i++
		case ch == '{':
			end := strings.IndexByte(text[i:], '}')
			if end < 0 {
				return nil, nil, fmt.Errorf("kapanmamış parametre: %q", text[i:])
			}
			name, mod, _ := strings.Cut(text[i+1:i+end], "|")
			n, ok := index[name]
			if !ok {
				return nil, nil, fmt.Errorf("{%s}: tanımda böyle bir parametre yok (parametreler: %v)", name, params)
			}
			if mod != "" && mod != "q" {
				return nil, nil, fmt.Errorf("{%s|%s}: bilinmeyen biçim (yalnızca |q)", name, mod)
			}
			flush()
			segs = append(segs, segment{arg: n, quote: mod == "q"})
			used[n] = true
			i += end
		case ch == '}':
			return nil, nil, fmt.Errorf("eşi olmayan '}' (düz parantez için }} yazın)")
		default:
			lit.WriteByte(ch)
		}
	}
	flush()
	return segs, used, nil
}

// render şablonu değerlerle doldurur. Eksik değer {ad?} olarak görünür (denetim testi çağrı
// yerlerindeki değer sayısını tanımla karşılaştırır; bu yalnızca bir güvence).
func (c *Catalog) render(code Code, args []any) string {
	segs, ok := c.templates[code]
	if !ok {
		if c.Language != "tr" {
			return catalogs["tr"].render(code, args)
		}
		return fmt.Sprintf("%s %v", code, args) // tanımsız kod: derleme sabitleriyle olmamalı
	}
	var b strings.Builder
	for _, s := range segs {
		switch {
		case s.arg < 0:
			b.WriteString(s.text)
		case s.arg >= len(args):
			fmt.Fprintf(&b, "{%s?}", definitions.Params[code][s.arg])
		case s.quote:
			fmt.Fprintf(&b, "%q", args[s.arg])
		default:
			fmt.Fprint(&b, args[s.arg])
		}
	}
	return b.String()
}
