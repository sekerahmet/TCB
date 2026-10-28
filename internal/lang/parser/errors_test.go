package parser

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// İç token adları (DOSYA_SONU, TANIMLAYICI, SONUÇ...) kullanıcıya sızmamalı.
var innerNameRe = regexp.MustCompile(`DOSYA_SONU|TANIMLAYICI|TAM_SAYI|ONDALIK_SAYI|METİN|SONUÇ|DEĞİŞKEN|DEĞİLSE|EĞER|FONKSİYON|GEÇERSİZ`)

func TestErrorMessagesUseTurkishDescriptions(t *testing.T) {
	// Her durumun ileti kodu ve (bu testin asıl konusu olan) Türkçe anlatım metni.
	type want struct {
		code messages.Code
		text string
	}
	caseList := map[string]want{
		"TR_print(1":                    {messages.H0140, "')' bekleniyordu, ancak dosya sonu bulundu"},
		"TR_var = 4":                    {messages.H0140, "bir ad bekleniyordu, ancak '=' bulundu"},
		"TR_if x = 5 { }":               {messages.H0140, "'{' bekleniyordu, ancak '=' bulundu (karşılaştırma için '==' yazın"},
		"TR_var d = [1, 2\nTR_return 3": {messages.H0140, "']' bekleniyordu, ancak 'TR_return' anahtar kelimesi bulundu"},
		"TR_func f(a b) { }":            {messages.H0140, "')' bekleniyordu, ancak 'b' adı bulundu"},
		"TR_match 1 { 1 \"bir\" }":      {messages.H0140, "'=>' bekleniyordu, ancak \"bir\" metni bulundu"},
		")":                             {messages.H0141, "burada bir değer ya da ifade bekleniyordu, ancak ')' bulundu"},
		"TR_var a, b 5":                 {messages.H0142, "çoklu değişken tanımında '=' bekleniyordu, ancak 5 sayısı bulundu"},
		"TR_for i TR_in [1] TR_else":    {messages.H0140, "'{' bekleniyordu, ancak 'TR_else' anahtar kelimesi bulundu"},
		"x = 99999999999999999999":      {messages.H0165, "99999999999999999999 tam sayı olarak çok büyük"},
	}
	for codeVal, w := range caseList {
		errs := parseErrs(codeVal)
		if len(errs) == 0 || !messages.Has(errs[0], w.code) {
			t.Errorf("%q: %v bekleniyordu, alınan: %v", codeVal, w.code, errs)
		}
		errList := parseErrors(codeVal)
		if len(errList) == 0 || !strings.Contains(errList[0], w.text) {
			t.Errorf("%q: %q içeren hata bekleniyordu, alınan: %v", codeVal, w.text, errList)
		}
		for _, h := range errList {
			if innerNameRe.MatchString(h) {
				t.Errorf("%q: iç token adı sızdı: %s", codeVal, h)
			}
		}
	}
}

// Tek bir yanlış, ardından gelen geçerli satırlarda art arda hata üretmemeli.
func TestSingleErrorNoCascade(t *testing.T) {
	caseList := map[string]int{
		// '=' hatası; gövde ve 'TR_else' kolu hatalı deyimin parçasıdır
		"TR_var x = 5\nTR_if x = 5 {\n    TR_print(\"a\")\n    TR_print(\"b\")\n} TR_else {\n    TR_print(\"c\")\n}\nTR_print(x)": 1,
		// Parametre listesindeki eksik virgül; gövde atlanır
		"TR_func f(a b) {\n    TR_return a\n}\nTR_print(1)": 1,
		// Bloğu kapatan '}' hatalı deyimde yutulmaz: "'{' kapatılmadı" hatası çıkmaz
		"TR_func g() {\n    TR_if TR_true { TR_print( }\n    TR_return 3\n}\nTR_print(g())": 1,
		// Eşle bloğundaki hata kalan durumları da bozmaz
		"TR_match 3 {\n    1 \"bir\"\n    2 => \"iki\"\n    _ => \"çok\"\n}\nTR_print(1)": 1,
		// Ayrı satırlardaki bağımsız hatalar ayrı ayrı bildirilir
		"TR_var = 1\nTR_print(2)\nTR_var = 3": 2,
	}
	for codeVal, wantSub := range caseList {
		if errList := parseErrors(codeVal); len(errList) != wantSub {
			t.Errorf("%q: %d hata bekleniyordu, %d alındı:\n%s", codeVal, wantSub, len(errList), strings.Join(errList, "\n"))
		}
	}
}

// Anahtar kelimenin Türkçe harfsiz yazımının ipucu (hint.KeywordSpellingHint) hatalı satırda korunur.
// Bilerek Türkçe gösterim: ipucu Türkçe yazılmış metnin (kabuk satırı) sözcüğünü önerir.
func TestKeywordSpellingHintKept(t *testing.T) {
	const src = "eger topla(a, b) -> Tam { sonuc a + b }"
	errs := parseErrsIn(src, codes.Source())
	compound := strings.Join(messages.Texts(errs), "\n")
	for _, name := range []string{"eger", "sonuc"} {
		// İpucu metni tek kaynaktan (hint) gelir; hata satırına eklenmiş olmalı.
		if want := hint.KeywordSpellingHint(name, nil).Error(); want == "" || !strings.Contains(compound, want) {
			t.Errorf("%q için ipucu %q bekleniyordu, alınan:\n%s", name, want, compound)
		}
	}
	if !hasCode(errs, messages.H0164) {
		t.Errorf("ipucu iletisi (H0164) bekleniyordu, alınan: %v", errs)
	}
}

// A syntax error is at the token it is about, from its first character to the one after its last
// (2.13), and carries the innermost code, not the H0100 every syntax error is wrapped in.
func TestSyntaxErrorRangeIsTheToken(t *testing.T) {
	cases := []struct {
		name string
		code string
		want messages.Range
	}{
		{"number where a name is expected", "TR_var 12 = 1", messages.Range{Line: 1, Col: 8, EndLine: 1, EndCol: 10}},
		{"bracket on the second line", "TR_var a = 1\nTR_var ağırlık = )", messages.Range{Line: 2, Col: 18, EndLine: 2, EndCol: 19}},
	}
	for _, c := range cases {
		errs := parseErrs(c.code)
		if len(errs) == 0 {
			t.Errorf("%s: no error", c.name)
			continue
		}
		p, ok := messages.Find(errs[0])
		if !ok {
			t.Errorf("%s: no position in %v", c.name, errs[0])
			continue
		}
		if got := p.Range(); got != c.want {
			t.Errorf("%s: range %+v, want %+v (%v)", c.name, got, c.want, errs[0])
		}
		if code, _ := messages.InnerCode(errs[0]); code == messages.H0100 {
			t.Errorf("%s: the record would carry the wrapper's code H0100", c.name)
		}
	}
}

// A syntax error records the fixes it is given with its range; without any it has none.
func TestSyntaxErrorFixIsRecorded(t *testing.T) {
	p := New(codedLexer("x"))
	tok := p.curToken()
	fix := messages.Fix{Range: tok.Range(), Replacement: "#", Applicable: true}
	p.syntaxErrorFix(tok, []messages.Fix{fix}, messages.H0025)
	pos, ok := messages.Find(p.Errs()[0])
	if !ok || len(pos.Fixes) != 1 || pos.Fixes[0] != fix {
		t.Errorf("record %+v", pos)
	}
	q := New(codedLexer("x"))
	q.syntaxError(q.curToken(), messages.H0025)
	if pos, _ := messages.Find(q.Errs()[0]); len(pos.Fixes) != 0 {
		t.Errorf("fixes %+v on an error given none", pos.Fixes)
	}
}
