package parser

import (
	"regexp"
	"strings"
	"testing"

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
		"yazdır(1":                    {messages.H0140, "')' bekleniyordu, ancak dosya sonu bulundu"},
		"değişken = 4":                {messages.H0140, "bir ad bekleniyordu, ancak '=' bulundu"},
		"eğer x = 5 { }":              {messages.H0140, "'{' bekleniyordu, ancak '=' bulundu (karşılaştırma için '==' yazın"},
		"değişken d = [1, 2\nsonuç 3": {messages.H0140, "']' bekleniyordu, ancak 'sonuç' anahtar kelimesi bulundu"},
		"fonksiyon f(a b) { }":        {messages.H0140, "')' bekleniyordu, ancak 'b' adı bulundu"},
		"eşle 1 { 1 \"bir\" }":        {messages.H0140, "'=>' bekleniyordu, ancak \"bir\" metni bulundu"},
		")":                           {messages.H0141, "burada bir değer ya da ifade bekleniyordu, ancak ')' bulundu"},
		"değişken a, b 5":             {messages.H0142, "çoklu değişken tanımında '=' bekleniyordu, ancak 5 sayısı bulundu"},
		"döngü i içinde [1] değilse":  {messages.H0140, "'{' bekleniyordu, ancak 'değilse' anahtar kelimesi bulundu"},
		"x = 99999999999999999999":    {messages.H0165, "99999999999999999999 tam sayı olarak çok büyük"},
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
		// '=' hatası; gövde ve 'değilse' kolu hatalı deyimin parçasıdır
		"değişken x = 5\neğer x = 5 {\n    yazdır(\"a\")\n    yazdır(\"b\")\n} değilse {\n    yazdır(\"c\")\n}\nyazdır(x)": 1,
		// Parametre listesindeki eksik virgül; gövde atlanır
		"fonksiyon f(a b) {\n    sonuç a\n}\nyazdır(1)": 1,
		// Bloğu kapatan '}' hatalı deyimde yutulmaz: "'{' kapatılmadı" hatası çıkmaz
		"fonksiyon g() {\n    eğer doğru { yazdır( }\n    sonuç 3\n}\nyazdır(g())": 1,
		// Eşle bloğundaki hata kalan durumları da bozmaz
		"eşle 3 {\n    1 \"bir\"\n    2 => \"iki\"\n    _ => \"çok\"\n}\nyazdır(1)": 1,
		// Ayrı satırlardaki bağımsız hatalar ayrı ayrı bildirilir
		"değişken = 1\nyazdır(2)\ndeğişken = 3": 2,
	}
	for codeVal, wantSub := range caseList {
		if errList := parseErrors(codeVal); len(errList) != wantSub {
			t.Errorf("%q: %d hata bekleniyordu, %d alındı:\n%s", codeVal, wantSub, len(errList), strings.Join(errList, "\n"))
		}
	}
}

// Anahtar kelimenin Türkçe harfsiz yazımının ipucu (hint.KeywordSpellingHint) hatalı satırda korunur.
func TestKeywordSpellingHintKept(t *testing.T) {
	const src = "eger topla(a, b) -> Tam { sonuc a + b }"
	errs := parseErrs(src)
	compound := strings.Join(parseErrors(src), "\n")
	for _, name := range []string{"eger", "sonuc"} {
		// İpucu metni tek kaynaktan (hint) gelir; hata satırına eklenmiş olmalı.
		if want := hint.KeywordSpellingHint(name); want == "" || !strings.Contains(compound, want) {
			t.Errorf("%q için ipucu %q bekleniyordu, alınan:\n%s", name, want, compound)
		}
	}
	if !hasCode(errs, messages.H0164) {
		t.Errorf("ipucu iletisi (H0164) bekleniyordu, alınan: %v", errs)
	}
}
