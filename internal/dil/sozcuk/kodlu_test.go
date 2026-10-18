package sozcuk

import (
	"strings"
	"testing"

	"tr_dili/internal/dil/kod"
)

// tumu bir çözümleyicinin bütün sözcük birimlerini (dosya sonu hariç) okur.
func tumu(l *Lexer) []Token {
	var t []Token
	for tok := l.NextToken(); tok.Type != TOKEN_EOF; tok = l.NextToken() {
		t = append(t, tok)
	}
	return t
}

// Kodlu gövdede kodlar etkin dilin adlarıyla, kod olmayan adlar kullanıcı adı olarak okunur (17.2).
func TestKodluSozcukler(t *testing.T) {
	l := NewGovde("A030 eğer = G100\nx.Y004 A040", true)
	toks := tumu(l)
	if len(l.Errors()) > 0 {
		t.Fatal(l.Errors())
	}
	beklenen := []struct {
		tur TokenType
		lit string
		kd  kod.Kod
	}{
		{TOKEN_DEGISKEN, "değişken", kod.A030}, {TOKEN_IDENT, "eğer", kod.Yok}, {TOKEN_ASSIGN, "=", kod.Yok},
		{TOKEN_IDENT, "yazdır", kod.G100}, {TOKEN_IDENT, "x", kod.Yok}, {TOKEN_DOT, ".", kod.Yok},
		{TOKEN_IDENT, "ekle", kod.Y004}, {TOKEN_DOGRU, "doğru", kod.A040},
	}
	if len(toks) != len(beklenen) {
		t.Fatalf("%d sözcük, beklenen %d: %v", len(toks), len(beklenen), toks)
	}
	for i, b := range beklenen {
		if toks[i].Type != b.tur || toks[i].Literal != b.lit || toks[i].Kod != b.kd {
			t.Errorf("sözcük %d: %v (kod %v), beklenen %s %q %v", i, toks[i], toks[i].Kod, b.tur, b.lit, b.kd)
		}
	}
	// Sütunlar Türkçe gösterime göredir (17.5): "değişken eğer = yazdır" → '=' 15. sütunda.
	if toks[2].Column != 15 || toks[3].Column != 17 {
		t.Errorf("gösterim sütunları: '=' %d, 'yazdır' %d (beklenen 15, 17)", toks[2].Column, toks[3].Column)
	}
	// Bilinmeyen kod hatadır; Türkçe yazılmış gövdede kodlar kod biçimli addır (2.3a).
	if l := NewGovde("G999", true); len(tumu(l)) != 1 || !strings.Contains(strings.Join(l.Errors(), ""), "'G999' bilinen bir kod değil") {
		t.Errorf("bilinmeyen kod hatası yok: %v", l.Errors())
	}
	if l := NewGovde("G100", false); tumu(l) == nil || !strings.Contains(strings.Join(l.Errors(), ""), "kod biçiminde bir ad") {
		t.Errorf("Türkçe gövdede kod biçimli ad hatası yok: %v", l.Errors())
	}
}

// '$ad' kaçışı (2.11): ad anahtar kelime olsa da kullanıcı adıdır; ada bitişik '$' hatadır.
func TestKacisliAd(t *testing.T) {
	l := New("$eğer + $yazdır")
	toks := tumu(l)
	if len(l.Errors()) > 0 || len(toks) != 3 || toks[0].Type != TOKEN_IDENT || toks[0].Literal != "eğer" || toks[2].Literal != "yazdır" {
		t.Fatalf("kaçışlı adlar: %v %v", toks, l.Errors())
	}
	for _, k := range []string{"a$b", "1$x", "$G100"} {
		if l := New(k); tumu(l) == nil || len(l.Errors()) == 0 {
			t.Errorf("%q hata vermeli", k)
		}
	}
}

// NUL karakteri dosyanın sonu sayılmaz, hatadır (2.1); ardından gelen kod okunur.
func TestNulKarakteri(t *testing.T) {
	l := New("a\x00b")
	toks := tumu(l)
	if len(toks) != 3 || toks[2].Literal != "b" || len(l.Errors()) != 1 || !strings.Contains(l.Errors()[0], "NUL") {
		t.Errorf("NUL: %v %v", toks, l.Errors())
	}
}

// Yerleştirmedeki sözcükler dosyadaki konumlarıyla okunur: önündeki kaçış dizileri ve satır
// sonları konumu kaydırmaz; kayıt açıksa kaynak aralıkları dosya metnindedir.
func TestYerlestirmeKonumlari(t *testing.T) {
	kaynak := "x = \"a\\n\\t{yaz}\"\n"
	l := New(kaynak)
	l.Kaydet()
	toks := tumu(l)
	metin := toks[2]
	y := l.Yerlestirmeler(metin)
	if len(y) != 1 {
		t.Fatalf("yerleştirme aralıkları: %v", y)
	}
	deger := []rune(metin.Literal)
	ifade, bas := YerlestirmeIfadesi(deger, y[0][0]+1, y[0][1])
	alt := l.Yerlestirme(metin, ifade, bas)
	tok := alt.NextToken()
	if tok.Literal != "yaz" || tok.Line != 1 || tok.Column != 12 {
		t.Errorf("yerleştirmedeki ad: %v (beklenen 1:12)", tok)
	}
	kayitlar := l.Kayitlar()
	son := kayitlar[len(kayitlar)-1]
	if string([]rune(kaynak)[son.Bas:son.Son]) != "yaz" {
		t.Errorf("kayıt aralığı [%d,%d) %q", son.Bas, son.Son, string([]rune(kaynak)[son.Bas:son.Son]))
	}
}
