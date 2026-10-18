package kod

import (
	"fmt"
	"slices"
	"strconv"
	"unicode/utf8"

	"tr_dili/internal/dil/turkce"
)

// asciiBicim adın Türkçe karakterleri ASCII'ye çevrilmiş biçimidir (turkce.ASCII); ASCII
// adlarda ayırma yapmadan adın kendisini döndürür.
func asciiBicim(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return turkce.ASCII(s)
		}
	}
	return s
}

// Dil dosyası program her başladığında okunur. encoding/json'un yansıma (reflect) ile
// çalışan çözümleyicisi ilk çağrıda yaklaşık bir milisaniye harcadığı için dosya bu küçük,
// yalnızca dil dosyasının biçimini (nesnelerden oluşan bir nesne; alanları metin, metin
// listesi ya da doğru/yanlış) tanıyan çözümleyiciyle okunur. Girdi yine standart JSON'dur;
// testler dosyanın encoding/json ile de geçerli olduğunu denetler.

// hamGirdi dosyadaki bir satırın çözülmüş biçimidir.
type hamGirdi struct {
	anahtar         string
	ad              string
	eski, yabanci   []string
	aile            string
	aciklama, ornek string
}

type cozumleyici struct {
	veri []byte
	i    int
}

func (c *cozumleyici) hata(bicim string, a ...any) error {
	satir, sutun := 1, 1
	for _, b := range c.veri[:min(c.i, len(c.veri))] {
		if b == '\n' {
			satir, sutun = satir+1, 1
		} else {
			sutun++
		}
	}
	return fmt.Errorf("satır %d, sütun %d: %s", satir, sutun, fmt.Sprintf(bicim, a...))
}

func (c *cozumleyici) bosluk() {
	for c.i < len(c.veri) {
		switch c.veri[c.i] {
		case ' ', '\t', '\n', '\r':
			c.i++
		default:
			return
		}
	}
}

// bekle boşluktan sonra b karakterini bekler.
func (c *cozumleyici) bekle(b byte) error {
	c.bosluk()
	if c.i >= len(c.veri) || c.veri[c.i] != b {
		return c.hata("'%c' bekleniyordu", b)
	}
	c.i++
	return nil
}

// sonraki boşluktan sonraki karakterdir (dosya bittiyse 0).
func (c *cozumleyici) sonraki() byte {
	c.bosluk()
	if c.i >= len(c.veri) {
		return 0
	}
	return c.veri[c.i]
}

// metin bir JSON metnini (kaçış dizileriyle) okur.
func (c *cozumleyici) metin() (string, error) {
	if err := c.bekle('"'); err != nil {
		return "", err
	}
	bas := c.i
	kacisli := false
	for c.i < len(c.veri) {
		switch b := c.veri[c.i]; {
		case b == '"':
			ham := c.veri[bas:c.i]
			c.i++
			if !kacisli {
				if !utf8.Valid(ham) {
					return "", c.hata("metin geçerli UTF-8 değil")
				}
				return string(ham), nil
			}
			s, err := strconv.Unquote(`"` + string(ham) + `"`)
			if err != nil {
				return "", c.hata("metindeki kaçış dizisi geçersiz")
			}
			return s, nil
		case b == '\\':
			kacisli = true
			c.i += 2
		case b < 0x20:
			return "", c.hata("metin içinde denetim karakteri")
		default:
			c.i++
		}
	}
	return "", c.hata("metin kapanmadı")
}

func (c *cozumleyici) metinListesi() ([]string, error) {
	if err := c.bekle('['); err != nil {
		return nil, err
	}
	var out []string
	if c.sonraki() == ']' {
		c.i++
		return out, nil
	}
	for {
		s, err := c.metin()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
		switch c.sonraki() {
		case ',':
			c.i++
		case ']':
			c.i++
			return out, nil
		default:
			return nil, c.hata("',' ya da ']' bekleniyordu")
		}
	}
}

// girdi { "ad": ..., "eski": [...], "yabancı": [...], "aile": ..., "açıklama": ..., "örnek": ... }
// nesnesini okur.
func (c *cozumleyici) girdi(g *hamGirdi) error {
	if err := c.bekle('{'); err != nil {
		return err
	}
	if c.sonraki() == '}' {
		c.i++
		return nil
	}
	var gorulen []string
	for {
		alan, err := c.metin()
		if err != nil {
			return err
		}
		if slices.Contains(gorulen, alan) {
			return c.hata("%s girdisinde %q alanı iki kez var", g.anahtar, alan)
		}
		gorulen = append(gorulen, alan)
		if err := c.bekle(':'); err != nil {
			return err
		}
		switch alan {
		case "ad":
			g.ad, err = c.metin()
		case "aile":
			g.aile, err = c.metin()
		case "eski":
			g.eski, err = c.metinListesi()
		case "yabancı":
			g.yabanci, err = c.metinListesi()
		case "açıklama":
			g.aciklama, err = c.metin()
		case "örnek":
			g.ornek, err = c.metin()
		default:
			return c.hata("%s girdisinde bilinmeyen alan %q (geçerli alanlar: ad, eski, yabancı, aile, açıklama, örnek)", g.anahtar, alan)
		}
		if err != nil {
			return err
		}
		switch c.sonraki() {
		case ',':
			c.i++
		case '}':
			c.i++
			return nil
		default:
			return c.hata("',' ya da '}' bekleniyordu")
		}
	}
}

// dosyaCoz dil dosyasını dosyadaki sırasıyla girdilere çevirir. Yinelenen anahtarlar
// olduğu gibi döner (Yukle bildirir).
func dosyaCoz(veri []byte) ([]hamGirdi, error) {
	c := &cozumleyici{veri: veri}
	if len(veri) >= 3 && veri[0] == 0xEF && veri[1] == 0xBB && veri[2] == 0xBF {
		c.i = 3 // UTF-8 BOM
	}
	if err := c.bekle('{'); err != nil {
		return nil, err
	}
	var out []hamGirdi
	if c.sonraki() == '}' {
		c.i++
	} else {
		for {
			anahtar, err := c.metin()
			if err != nil {
				return nil, err
			}
			if err := c.bekle(':'); err != nil {
				return nil, err
			}
			g := hamGirdi{anahtar: anahtar}
			if err := c.girdi(&g); err != nil {
				return nil, err
			}
			out = append(out, g)
			b := c.sonraki()
			if b == ',' {
				c.i++
				continue
			}
			if b == '}' {
				c.i++
				break
			}
			return nil, c.hata("',' ya da '}' bekleniyordu")
		}
	}
	if c.sonraki() != 0 {
		return nil, c.hata("kapanan '}' sonrasında fazladan içerik var")
	}
	return out, nil
}
