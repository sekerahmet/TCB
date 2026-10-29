package codes

import (
	"fmt"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Dil dosyası program her başladığında okunur. encoding/json'un yansıma (reflect) ile
// çalışan çözümleyicisi ilk çağrıda yaklaşık bir milisaniye harcadığı için dosya bu küçük,
// yalnızca dil dosyasının biçimini (nesnelerden oluşan bir nesne; alanları metin, metin
// listesi ya da doğru/yanlış) tanıyan çözümleyiciyle okunur. Girdi yine standart JSON'dur;
// testler dosyanın encoding/json ile de geçerli olduğunu denetler.

// rawEntry dosyadaki bir satırın çözülmüş biçimidir.
type rawEntry struct {
	key         string
	name        string
	family      string
	description string
}

type jsonParser struct {
	data []byte
	i    int
}

func (c *jsonParser) errorf(form string, a ...any) error {
	line, col := 1, 1
	for _, b := range c.data[:min(c.i, len(c.data))] {
		if b == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return messages.Internalf("line %d, column %d: %s", line, col, fmt.Sprintf(form, a...))
}

func (c *jsonParser) skipSpace() {
	for c.i < len(c.data) {
		switch c.data[c.i] {
		case ' ', '\t', '\n', '\r':
			c.i++
		default:
			return
		}
	}
}

// expect boşluktan sonra b karakterini bekler.
func (c *jsonParser) expect(b byte) error {
	c.skipSpace()
	if c.i >= len(c.data) || c.data[c.i] != b {
		return c.errorf("'%c' expected", b)
	}
	c.i++
	return nil
}

// next boşluktan sonraki karakterdir (dosya bittiyse 0).
func (c *jsonParser) next() byte {
	c.skipSpace()
	if c.i >= len(c.data) {
		return 0
	}
	return c.data[c.i]
}

// str bir JSON metnini (kaçış dizileriyle) okur.
func (c *jsonParser) str() (string, error) {
	if err := c.expect('"'); err != nil {
		return "", err
	}
	start := c.i
	escaped := false
	for c.i < len(c.data) {
		switch b := c.data[c.i]; {
		case b == '"':
			raw := c.data[start:c.i]
			c.i++
			if !escaped {
				if !utf8.Valid(raw) {
					return "", c.errorf("string is not valid UTF-8")
				}
				return string(raw), nil
			}
			s, err := strconv.Unquote(`"` + string(raw) + `"`)
			if err != nil {
				return "", c.errorf("invalid escape sequence in string")
			}
			return s, nil
		case b == '\\':
			escaped = true
			c.i += 2
		case b < 0x20:
			return "", c.errorf("control character in string")
		default:
			c.i++
		}
	}
	return "", c.errorf("unterminated string")
}

// entry { "name": ..., "family": ..., "description": ... }
// nesnesini okur.
func (c *jsonParser) entry(g *rawEntry) error {
	if err := c.expect('{'); err != nil {
		return err
	}
	if c.next() == '}' {
		c.i++
		return nil
	}
	var seen []string
	for {
		field, err := c.str()
		if err != nil {
			return err
		}
		if slices.Contains(seen, field) {
			return c.errorf("%s entry has field %q twice", g.key, field)
		}
		seen = append(seen, field)
		if err := c.expect(':'); err != nil {
			return err
		}
		switch field {
		case "name":
			g.name, err = c.str()
		case "family":
			g.family, err = c.str()
		case "description":
			g.description, err = c.str()
		default:
			return c.errorf("%s entry has unknown field %q (valid fields: name, family, description; familiar names and examples are in definitions.json)", g.key, field)
		}
		if err != nil {
			return err
		}
		switch c.next() {
		case ',':
			c.i++
		case '}':
			c.i++
			return nil
		default:
			return c.errorf("',' or '}' expected")
		}
	}
}

// parseFile dil dosyasını dosyadaki sırasıyla girdilere çevirir. Yinelenen anahtarlar
// olduğu gibi döner (Yukle bildirir).
func parseFile(data []byte) ([]rawEntry, error) {
	c := &jsonParser{data: data}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		c.i = 3 // UTF-8 BOM
	}
	if err := c.expect('{'); err != nil {
		return nil, err
	}
	var out []rawEntry
	if c.next() == '}' {
		c.i++
	} else {
		for {
			key, err := c.str()
			if err != nil {
				return nil, err
			}
			if err := c.expect(':'); err != nil {
				return nil, err
			}
			g := rawEntry{key: key}
			if err := c.entry(&g); err != nil {
				return nil, err
			}
			out = append(out, g)
			b := c.next()
			if b == ',' {
				c.i++
				continue
			}
			if b == '}' {
				c.i++
				break
			}
			return nil, c.errorf("',' or '}' expected")
		}
	}
	if c.next() != 0 {
		return nil, c.errorf("extra content after the closing '}'")
	}
	return out, nil
}
