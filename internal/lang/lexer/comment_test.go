package lexer

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Yorumlar
// ---------------------------------------------------------------------------

func TestYorumlar(t *testing.T) {
	testler := []struct {
		ad, girdi string
		want      []beklenen
	}{
		{"çift bölü", "a // yorum {\nb", []beklenen{{TOKEN_IDENT, "a"}, {TOKEN_IDENT, "b"}}},
		{"blok", "a /* çok\nsatırlı ' yorum */ b", []beklenen{{TOKEN_IDENT, "a"}, {TOKEN_IDENT, "b"}}},
		{"dosya sonunda", "a // son", []beklenen{{TOKEN_IDENT, "a"}}},
		{"art arda", "// bir\n// iki\n/* üç */\nx", []beklenen{{TOKEN_IDENT, "x"}}},
		{"blok içinde yıldız", "/* a * b ** c */x", []beklenen{{TOKEN_IDENT, "x"}}},
		{"bölme yorum değildir", "a / b", []beklenen{{TOKEN_IDENT, "a"}, {TOKEN_SLASH, "/"}, {TOKEN_IDENT, "b"}}},
		{"metin içindeki # yorum değildir", `"# değil" x`, []beklenen{{TOKEN_STRING, "# değil"}, {TOKEN_IDENT, "x"}}},
		{"metin içindeki // yorum değildir", `"http://a" x`, []beklenen{{TOKEN_STRING, "http://a"}, {TOKEN_IDENT, "x"}}},
		// Mevcut davranış: blok yorumlar iç içe geçmez; ilk '*/' yorumu bitirir
		{"iç içe blok yok", "/* a /* b */ c */", []beklenen{
			{TOKEN_IDENT, "c"}, {TOKEN_ASTERISK, "*"}, {TOKEN_SLASH, "/"},
		}},
	}
	for _, tt := range testler {
		t.Run(tt.ad, func(t *testing.T) {
			tokenlariDogrula(t, tt.girdi, append(tt.want, beklenen{TOKEN_EOF, ""}))
		})
	}
}

// '#' yorum değildir: açık bir hata verir, satırın geri kalanı atlanır (tek hata).
func TestDiyezYorumDegil(t *testing.T) {
	for _, c := range []struct {
		girdi, hata string
		ad          []string
	}{
		{"a # yorum ** \"\n b", "Satır 1, Sütun 3: '#' TR'de yorum değil; yorum için '//' yazın", []string{"a", "b"}},
		{"# bir\nx", "Satır 1, Sütun 1: '#' TR'de yorum değil; yorum için '//' yazın", []string{"x"}},
	} {
		toks, hatalar := tumTokenlar(t, c.girdi)
		if len(hatalar) != 1 || hatalar[0] != c.hata {
			t.Errorf("%q: tek hata %q bekleniyordu, gelen: %v", c.girdi, c.hata, hatalar)
		}
		var adlar []string
		for _, tk := range toks {
			if tk.Type == TOKEN_IDENT {
				adlar = append(adlar, tk.Literal)
			}
		}
		if strings.Join(adlar, " ") != strings.Join(c.ad, " ") {
			t.Errorf("%q: adlar %v, %v bekleniyordu", c.girdi, adlar, c.ad)
		}
	}
}
func TestYorumSonrasiSatirNumarasi(t *testing.T) {
	toks := hatasizTokenlar(t, "/* bir\niki\nüç */ x // yorum\n  y")
	if toks[0].Line != 3 || toks[0].Column != 7 {
		t.Errorf("'x' konumu 3:7 bekleniyordu, %d:%d geldi", toks[0].Line, toks[0].Column)
	}
	if toks[1].Line != 4 || toks[1].Column != 3 {
		t.Errorf("'y' konumu 4:3 bekleniyordu, %d:%d geldi", toks[1].Line, toks[1].Column)
	}
}
func TestKapatilmamisBlokYorum(t *testing.T) {
	toks, errs := tumTokenlar(t, "a = 1\n  /* yorum\nb = 2\n")
	if len(toks) != 4 || toks[3].Type != TOKEN_EOF {
		t.Errorf("yorum dosya sonuna kadar sürmeli, gelen: %v", toks)
	}
	if len(errs) != 1 || !hataIcerirMi(errs, "Satır 2, Sütun 3:", "kapatılmamış") {
		t.Errorf("'Satır 2, Sütun 3' konumlu kapatılmamış yorum hatası bekleniyordu, gelen: %v", errs)
	}

	// Yalnızca '/*' ve tek '*' ile biten girdiler de hata vermeli, çökmemeli
	for _, s := range []string{"/*", "/**", "/* *"} {
		_, errs := tumTokenlar(t, s)
		if len(errs) != 1 {
			t.Errorf("%q: tek bir hata bekleniyordu, gelen: %v", s, errs)
		}
	}
	// '/**/' boş ama kapalı bir yorumdur
	tokenlariDogrula(t, "/**/x", []beklenen{{TOKEN_IDENT, "x"}, {TOKEN_EOF, ""}})
}
