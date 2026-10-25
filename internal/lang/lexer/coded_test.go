package lexer

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// allOf bir çözümleyicinin bütün sözcük birimlerini (dosya sonu hariç) okur.
func allOf(l *Lexer) []Token {
	var t []Token
	for tok := l.NextToken(); tok.Type != TokEOF; tok = l.NextToken() {
		t = append(t, tok)
	}
	return t
}

// Kodlu gövdede kodlar etkin dilin adlarıyla, kod olmayan adlar kullanıcı adı olarak okunur (17.2).
func TestCodedTokens(t *testing.T) {
	l := NewBody("TR_kw_var eğer = TR_fn_print\nx.TR_mem_append TR_kw_true", true)
	toks := allOf(l)
	if len(l.Errors()) > 0 {
		t.Fatal(l.Errors())
	}
	expected := []struct {
		kind TokenType
		lit  string
		kd   codes.Code
	}{
		{TokVar, "değişken", codes.KwVar}, {TokIdent, "eğer", codes.None}, {TokAssign, "=", codes.None},
		{TokIdent, "yazdır", codes.FnPrint}, {TokIdent, "x", codes.None}, {TokDot, ".", codes.None},
		{TokIdent, "ekle", codes.MemAppend}, {TokTrue, "doğru", codes.KwTrue},
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
	if l := NewBody("TR_fn_yok", true); len(allOf(l)) != 1 || !hasCode(l.Errs(), messages.H0001) {
		t.Errorf("bilinmeyen kod hatası yok: %v", l.Errors())
	}
	if l := NewBody("TR_fn_print", false); allOf(l) == nil || !hasCode(l.Errs(), messages.H0004) {
		t.Errorf("Türkçe gövdede kod biçimli ad hatası yok: %v", l.Errors())
	}
}

// '$ad' kaçışı (2.11): ad anahtar kelime olsa da kullanıcı adıdır; ada bitişik '$' hatadır.
func TestEscapedName(t *testing.T) {
	l := New("$eğer + $yazdır")
	toks := allOf(l)
	if len(l.Errors()) > 0 || len(toks) != 3 || toks[0].Type != TokIdent || toks[0].Literal != "eğer" || toks[2].Literal != "yazdır" {
		t.Fatalf("kaçışlı adlar: %v %v", toks, l.Errors())
	}
	for _, k := range []string{"a$b", "1$x", "$TR_fn_print"} {
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

// Kodlu kipin ön taraması metin yerleştirmelerinin içine iner; derinlik sınırı olmadan çok iç içe
// metinli bir dosya dakikalar sürüyordu (her düzey altındaki metni kopyalar). Tarama ayrıştırıcının
// iç içelik sınırında durur (ayrıştırıcı orada zaten hata verir) ve bir iş bütçesi toplamı girdiyle
// doğrusal tutar. Duvar saati tavanı yerine büyüme ölçülür (testkit.ScalesLinearly: 5000 ve 20000
// düzey); yüklü bir makine tavanı aşıyordu.
func TestCodedPreScanDepthLimited(t *testing.T) {
	testkit.ScalesLinearly(t, "nested interpolations", 5000, func(n int) func() {
		source := "TR_kw_var s = " + strings.Repeat(`"{`, n) + "1" + strings.Repeat(`}"`, n) + "\n"
		return func() {
			l := NewBody(source, true)
			for l.NextToken().Type != TokEOF {
			}
		}
	})
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
