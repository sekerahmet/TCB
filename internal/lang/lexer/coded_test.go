package lexer

import (
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// allOf bir çözümleyicinin bütün sözcük birimlerini (dosya sonu hariç) okur.
func allOf(l *Lexer) []Token {
	var t []Token
	for tok := l.NextToken(); tok.Type != TOKEN_EOF; tok = l.NextToken() {
		t = append(t, tok)
	}
	return t
}

// Kodlu gövdede kodlar etkin dilin adlarıyla, kod olmayan adlar kullanıcı adı olarak okunur (17.2).
func TestCodedTokens(t *testing.T) {
	l := NewBody("A030 eğer = G100\nx.Y004 A040", true)
	toks := allOf(l)
	if len(l.Errors()) > 0 {
		t.Fatal(l.Errors())
	}
	expected := []struct {
		kind TokenType
		lit  string
		kd   codes.Code
	}{
		{TOKEN_VAR, "değişken", codes.A030}, {TOKEN_IDENT, "eğer", codes.None}, {TOKEN_ASSIGN, "=", codes.None},
		{TOKEN_IDENT, "yazdır", codes.G100}, {TOKEN_IDENT, "x", codes.None}, {TOKEN_DOT, ".", codes.None},
		{TOKEN_IDENT, "ekle", codes.Y004}, {TOKEN_TRUE, "doğru", codes.A040},
	}
	if len(toks) != len(expected) {
		t.Fatalf("%d sözcük, beklenen %d: %v", len(toks), len(expected), toks)
	}
	for i, b := range expected {
		if toks[i].Type != b.kind || toks[i].Literal != b.lit || toks[i].Code != b.kd {
			t.Errorf("sözcük %d: %v (kod %v), beklenen %s %q %v", i, toks[i], toks[i].Code, b.kind, b.lit, b.kd)
		}
	}
	// Sütunlar Türkçe gösterime göredir (17.5): "değişken eğer = yazdır" → '=' 15. sütunda.
	if toks[2].Column != 15 || toks[3].Column != 17 {
		t.Errorf("gösterim sütunları: '=' %d, 'yazdır' %d (beklenen 15, 17)", toks[2].Column, toks[3].Column)
	}
	// Bilinmeyen kod hatadır; Türkçe yazılmış gövdede kodlar kod biçimli addır (2.3a).
	if l := NewBody("G999", true); len(allOf(l)) != 1 || !hasCode(l.Errs(), messages.H0001) {
		t.Errorf("bilinmeyen kod hatası yok: %v", l.Errors())
	}
	if l := NewBody("G100", false); allOf(l) == nil || !hasCode(l.Errs(), messages.H0004) {
		t.Errorf("Türkçe gövdede kod biçimli ad hatası yok: %v", l.Errors())
	}
}

// '$ad' kaçışı (2.11): ad anahtar kelime olsa da kullanıcı adıdır; ada bitişik '$' hatadır.
func TestEscapedName(t *testing.T) {
	l := New("$eğer + $yazdır")
	toks := allOf(l)
	if len(l.Errors()) > 0 || len(toks) != 3 || toks[0].Type != TOKEN_IDENT || toks[0].Literal != "eğer" || toks[2].Literal != "yazdır" {
		t.Fatalf("kaçışlı adlar: %v %v", toks, l.Errors())
	}
	for _, k := range []string{"a$b", "1$x", "$G100"} {
		if l := New(k); allOf(l) == nil || len(l.Errors()) == 0 {
			t.Errorf("%q hata vermeli", k)
		}
	}
}

// NUL karakteri dosyanın sonu sayılmaz, hatadır (2.1); ardından gelen kod okunur.
func TestNulCharacter(t *testing.T) {
	l := New("a\x00b")
	toks := allOf(l)
	if len(toks) != 3 || toks[2].Literal != "b" || len(l.Errs()) != 1 || !hasCode(l.Errs(), messages.H0003) {
		t.Errorf("NUL: %v %v", toks, l.Errors())
	}
}

// Yerleştirmedeki sözcükler dosyadaki konumlarıyla okunur: önündeki kaçış dizileri ve satır
// sonları konumu kaydırmaz; kayıt açıksa kaynak aralıkları dosya metnindedir.
func TestInterpolationPositions(t *testing.T) {
	source := "x = \"a\\n\\t{yaz}\"\n"
	l := New(source)
	l.StartRecording()
	toks := allOf(l)
	text := toks[2]
	y := l.Interpolations(text)
	if len(y) != 1 {
		t.Fatalf("yerleştirme aralıkları: %v", y)
	}
	value := []rune(text.Literal)
	expr, begin := InterpolationExpr(value, y[0][0]+1, y[0][1])
	sub := l.Interpolation(text, expr, begin)
	tok := sub.NextToken()
	if tok.Literal != "yaz" || tok.Line != 1 || tok.Column != 12 {
		t.Errorf("yerleştirmedeki ad: %v (beklenen 1:12)", tok)
	}
	records := l.Records()
	end := records[len(records)-1]
	if string([]rune(source)[end.Start:end.End]) != "yaz" {
		t.Errorf("kayıt aralığı [%d,%d) %q", end.Start, end.End, string([]rune(source)[end.Start:end.End]))
	}
}

// The coded-mode pre-scan descends into string interpolations; without a depth limit a file of
// deeply nested strings took minutes (each level copies the text below it). It stops at the
// parser's nesting limit, where the parser reports the error anyway, and a work budget keeps
// the total linear in the input.
func TestCodedPreScanDepthLimited(t *testing.T) {
	body := "1"
	for i := 0; i < 20000; i++ {
		body = `"{` + body + `}"`
	}
	begin := time.Now()
	l := NewBody("A030 s = "+body+"\n", true)
	for l.NextToken().Type != TOKEN_EOF {
	}
	if d := time.Since(begin); d > 5*time.Second { // linear: about 0.2 s on an idle machine
		t.Errorf("20000 nested interpolations took %v", d)
	}
}
