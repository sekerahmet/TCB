package main

import "strings"

// Eski tırnaklı metin (çift ya da tek tırnak) bugünkü sözcük çözümleyicinin readString'i gibi
// okunur: kaçış dizileri önce çözülür, kaçışsız her '{' bir yerleştirme başlatır, yerleştirmenin
// içindeki tırnaklı alt metin onu kapatmaz, yerleştirme aynı satırda kapanır. Yeni metin çift
// tırnaklıdır (2.6): '\{' ve '\}' düz süslü parantez, '\'' tek tırnak olur, tek tırnaklı metindeki
// çift tırnak kaçışla yazılır; '{ifade}' yerleştirmesi '\(ifade)' olur (2.7), ifadenin kendisi
// convertCode ile çevrilir.

// str c.i'deki tırnakla başlayan metni çevirip yazar.
func (c *converter) str(quote rune) {
	startLine := c.line
	c.i++ // açılış tırnağı
	var out strings.Builder
	out.WriteByte('"')
	for {
		if c.i >= len(c.src) {
			c.fail("unclosed string")
			c.out.WriteString(out.String())
			return
		}
		r := c.src[c.i]
		switch {
		case r == quote:
			c.i++
			out.WriteByte('"')
			c.out.WriteString(out.String())
			return
		case r == '\\':
			c.escape(&out)
		case r == '{':
			if !c.interpolation(&out) {
				c.line = startLine + strings.Count(out.String(), "\n")
				c.out.WriteString(out.String())
				return
			}
		case r == '"':
			out.WriteString(`\"`) // tek tırnaklı metindeki çift tırnak
			c.i++
		default:
			if r == '\n' {
				c.line++
			}
			out.WriteRune(r)
			c.i++
		}
	}
}

// escape metnin düz kısmındaki bir kaçış dizisini yeni yazımına çevirir.
func (c *converter) escape(out *strings.Builder) {
	next := c.peek(1)
	switch next {
	case '\'', '{', '}':
		out.WriteRune(next)
	case 'n', 't', 'r', '"', '\\':
		out.WriteRune('\\')
		out.WriteRune(next)
	case 0:
		c.i++
		return
	default:
		// Bilinmeyen kaçış eskide de hatadır (bilerek bozuk fikstürler); olduğu gibi kalır.
		c.fail("unknown escape \\" + string(next))
		out.WriteRune('\\')
		out.WriteRune(next)
		if next == '\n' {
			c.line++
		}
	}
	c.i += 2
}

// interpolation c.i'deki '{' ile başlayan yerleştirmeyi okur ve '\(...)' olarak yazar. Kapanmayan
// yerleştirmede (eskide de hata) false döner; okunan kısım olduğu gibi yazılmıştır.
func (c *converter) interpolation(out *strings.Builder) bool {
	start := c.i
	c.i++ // '{'
	var expr strings.Builder
	depth := 1
	var inner rune
	for {
		if c.i >= len(c.src) || c.src[c.i] == '\n' {
			c.fail("unclosed interpolation")
			out.WriteString(string(c.src[start:c.i]))
			return false
		}
		r := c.src[c.i]
		if r == '\\' {
			// Kaçış dizileri yerleştirmenin içinde de önce çözülür ("{d[\"k\"]}" → d["k"]).
			switch n := c.peek(1); n {
			case '"', '\'', '\\':
				expr.WriteRune(n)
			case '{', '}':
				c.fail("escaped brace inside an interpolation")
				expr.WriteRune(n)
			default:
				expr.WriteRune('\\')
				expr.WriteRune(n)
			}
			c.i += 2
			continue
		}
		switch {
		case inner != 0:
			if r == inner {
				inner = 0
			}
		case r == '"' || r == '\'':
			inner = r
		case r == '{':
			depth++
		case r == '}':
			depth--
		}
		if depth == 0 {
			c.i++ // '}'
			break
		}
		expr.WriteRune(r)
		c.i++
	}
	// Yerleştirmenin ifadesi bloğun içindedir: bloktaki sabitlerin yeni adları orada da geçerlidir.
	sub := &converter{src: []rune(expr.String()), line: 1, lineBase: c.lineBase + c.line - 1, v: c.v,
		words: c.words, renames: c.renames, depth: c.depth}
	sub.run()
	code, probs := sub.out.String(), sub.problems
	c.problems = append(c.problems, probs...)
	if strings.TrimSpace(code) == "" {
		c.fail("empty interpolation")
	}
	out.WriteString(`\(` + code + `)`)
	return true
}
