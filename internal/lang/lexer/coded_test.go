package lexer

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// allOf bir çözümleyicinin bütün sözcük birimlerini (dosya sonu hariç) okur.
func allOf(l *Lexer) []Token {
	var t []Token
	for tok := l.NextToken(); tok.Type != TokEOF; tok = l.NextToken() {
		t = append(t, tok)
	}
	return t
}

// Kural (17.2, 17.5): kodlu gövdede kodlar okunduğu dilin adlarıyla, kod olmayan adlar kullanıcı adı
// olarak okunur. Program kimlik listesinde okunur (NewBody): sözcüğün yazımı kimliğidir; bir yazım
// dilinde okunan gövdede (NewBodyIn) o dilin adıdır. Kod her dilde aynıdır; konumlar diskteki
// metne göredir (kişinin görünümünün konumlarını NewBodyView verir).
func TestCodedTokens(t *testing.T) {
	const src = "TR_var eğer = TR_print\nx.TR_append TR_true"
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		lexer *Lexer
		words [4]string // var, print, append, true
	}{
		{"kimlik", NewBody(src, true), [4]string{"TR_var", "TR_print", "TR_append", "TR_true"}},
		{"tr", NewBodyIn(src, true, codes.Source()), [4]string{"değişken", "yazdır", "ekle", "doğru"}},
		{"en", NewBodyIn(src, true, en), [4]string{"var", "print", "append", "true"}},
	} {
		toks := allOf(c.lexer)
		if len(c.lexer.Errors()) > 0 {
			t.Fatalf("%s: %v", c.name, c.lexer.Errors())
		}
		expected := []struct {
			kind TokenType
			lit  string
			kd   codes.Code
		}{
			{TokVar, c.words[0], codes.KwVar}, {TokIdent, "eğer", codes.None}, {TokAssign, "=", codes.None},
			{TokIdent, c.words[1], codes.FnPrint}, {TokIdent, "x", codes.None}, {TokDot, ".", codes.None},
			{TokIdent, c.words[2], codes.MemAppend}, {TokTrue, c.words[3], codes.KwTrue},
		}
		if len(toks) != len(expected) {
			t.Fatalf("%s: %d sözcük, beklenen %d: %v", c.name, len(toks), len(expected), toks)
		}
		for i, b := range expected {
			if toks[i].Type != b.kind || toks[i].Literal != b.lit || toks[i].Code != b.kd {
				t.Errorf("%s: sözcük %d: %v (kod %v), beklenen %s %q %v", c.name, i, toks[i], toks[i].Code, b.kind, b.lit, b.kd)
			}
		}
		if toks[2].Column != 16 || toks[3].Column != 18 {
			t.Errorf("%s: disk sütunları: '=' %d, yazdır %d (beklenen 16, 18)", c.name, toks[2].Column, toks[3].Column)
		}
	}
	// Bilinmeyen kimlik hatadır; Türkçe yazılmış gövdede kimlik ayrılmış addır (2.3a).
	if l := NewBody("TR_fn_yok", true); len(allOf(l)) != 1 || !hasCode(l.Errs(), messages.H0001) {
		t.Errorf("bilinmeyen kimlik hatası yok: %v", l.Errors())
	}
	if l := NewBodyIn("TR_print", false, codes.Source()); allOf(l) == nil || !hasCode(l.Errs(), messages.H0004) {
		t.Errorf("Türkçe gövdede ayrılmış ad hatası yok: %v", l.Errors())
	}
	// Kodlu dosyanın başlığı yalnız ad bilinen bir kimlikse (başlıksız yapıştırılmış kodlu metin)
	// anılır; başlığın dili okunan metnin dilidir.
	if l := NewBodyIn("değişken TR_x = 1", false, codes.Source()); allOf(l) == nil || !hasCode(l.Errs(), messages.H0004) || hasCode(l.Errs(), messages.H0014) {
		t.Errorf("kimlik olmayan ayrılmış ad başlığı anmamalı: %v", l.Errors())
	}
	for _, c := range []struct{ lang, text string }{{"tr", "değişken TR_sum = 1"}, {"en", "var TR_sum = 1"}} {
		lang, err := codes.LoadLanguage(c.lang)
		if err != nil {
			t.Fatal(err)
		}
		l := NewBodyIn(c.text, false, lang)
		allOf(l)
		if !hasCode(l.Errs(), messages.H0014) || !strings.Contains(l.Errors()[0], "dil="+c.lang+"'") {
			t.Errorf("%s: kimlik olan ayrılmış ad '%s%s' başlığını anmalı: %v", c.lang, CodedHeader, c.lang, l.Errors())
		}
	}
}

// '$ad' kaçışı (2.11): ad anahtar kelime olsa da kullanıcı adıdır; ada bitişik '$' hatadır.
func TestEscapedName(t *testing.T) {
	l := New("$eğer + $yazdır")
	toks := allOf(l)
	if len(l.Errors()) > 0 || len(toks) != 3 || toks[0].Type != TokIdent || toks[0].Literal != "eğer" || toks[2].Literal != "yazdır" {
		t.Fatalf("kaçışlı adlar: %v %v", toks, l.Errors())
	}
	for _, k := range []string{"a$b", "1$x", "$TR_print"} {
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

// Kodsuz gövde verilen dilde okunur (NewBodyIn): Türkçe yazılmış dosya etkin dil ne olursa
// olsun Türkçe okunur (17.7). Etkin dil Türkçeyken İngilizce gövde İngilizce sözcükleriyle,
// Türkçe sözcükler ise ad olarak okunur (yerleştirmeler: parser.TestContextWordsFollowLexer).
func TestBodyInLanguage(t *testing.T) {
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	l := NewBodyIn("if eğer {", false, en)
	if l.Language() != en || NewBody("", false).Language() != codes.Active() {
		t.Fatal("çözümleyicinin dili yanlış")
	}
	got := allOf(l)
	want := []TokenType{TokIf, TokIdent, TokLBrace}
	if len(got) != len(want) {
		t.Fatalf("sözcük birimleri %v, beklenen türler %v", got, want)
	}
	for i, w := range want {
		if got[i].Type != w {
			t.Fatalf("sözcük birimleri %v, beklenen türler %v", got, want)
		}
	}
}
