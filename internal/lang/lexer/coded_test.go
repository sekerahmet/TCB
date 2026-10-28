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

// Kural (2.1, 2.3a): kodlu gövdede dilin sözcükleri kimlikle, kod olmayan adlar kullanıcı adı olarak
// okunur. Program kimlik listesinde okunur (NewBody): sözcüğün yazımı kimliğidir; bir yazım dilinde
// okunan gövdede (NewBodyIn) o dilin adıdır. Kod her dilde aynıdır; konumlar dosyanın metnine göredir
// (kişinin görünümünün konumlarını NewBodyView verir).
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
		if toks[2].Column != 13 || toks[3].Column != 15 {
			t.Errorf("%s: dosyanın sütunları: '=' %d, TR_print %d (beklenen 13, 15)", c.name, toks[2].Column, toks[3].Column)
		}
	}
	// Bilinmeyen kimlik hatadır (2.3a); kimlik bir dildeki gösterimde de kimliktir (bir dilde yazılmış
	// metne yapıştırılan kod çalışır).
	if l := NewBody("TR_yok", true); len(allOf(l)) != 1 || !hasCode(l.Errs(), messages.H0001) {
		t.Errorf("bilinmeyen kimlik hatası yok: %v", l.Errors())
	}
	if l := NewBodyIn("TR_print", false, codes.Source()); len(l.Errs()) != 0 || allOf(l)[0].Code != codes.FnPrint {
		t.Errorf("Türkçe gösterimde de TR_print yazdırdır: %v", l.Errors())
	}
	// TR_ ile başlayan ad tanımlanamaz: ileti başka bir ad önerir.
	for _, l := range []*Lexer{NewBody("TR_var TR_x = 1", true), NewBodyIn("değişken TR_x = 1", false, codes.Source())} {
		if allOf(l) == nil || len(l.Errs()) != 1 || !hasCode(l.Errs(), messages.H0004) || !strings.Contains(l.Errors()[0], "tr_x") {
			t.Errorf("TR_ ile başlayan tanım H0004 vermeli, tr_x önermeli: %v", l.Errors())
		}
	}
}

// '$ad' kaçışı bir dildeki gösterimdedir (17): ad o dilin sözcüğü olsa da kullanıcı adıdır; ada
// bitişik '$' hatadır. Dosyada (kodlu gövde) '$' yoktur (2.3): ileti adı '$'sız önerir.
func TestEscapedName(t *testing.T) {
	l := New("$eğer + $yazdır")
	toks := allOf(l)
	if len(l.Errors()) > 0 || len(toks) != 3 || toks[0].Type != TokIdent || toks[0].Literal != "eğer" || toks[2].Literal != "yazdır" {
		t.Fatalf("kaçışlı adlar: %v %v", toks, l.Errors())
	}
	for k, want := range map[string]messages.Code{"a$b": messages.H0002, "1$x": messages.H0002, "$TR_print": messages.H0004} {
		if l := New(k); allOf(l) == nil || !hasCode(l.Errs(), want) {
			t.Errorf("%q %s vermeli: %v", k, want, l.Errors())
		}
	}
	if l := NewBody("TR_var a = 1\nTR_print($a)", true); allOf(l) == nil || len(l.Errs()) != 1 || !hasCode(l.Errs(), messages.H0031) {
		t.Errorf("dosyadaki '$a' H0031 vermeli: %v", l.Errors())
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
	source := "x = \"a\\n\\t\\(yaz)\"\n"
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
	if tok.Literal != "yaz" || tok.Line != 1 || tok.Column != 13 {
		t.Errorf("yerleştirmedeki ad: %v (beklenen 1:13)", tok)
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
