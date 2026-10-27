package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// check kaynağı ayrıştırır (sözdizimi hatası testi düşürür) ve tip denetimi hatalarını döndürür.
func check(t *testing.T, source string) []string {
	t.Helper()
	errs, _ := checkErrs(t, source)
	return errs
}

// checkErrs check gibidir; ayrıca hataları kodlu (messages.Pos ve messages.Has ile sınanabilir) döndürür.
func checkErrs(t *testing.T, source string) ([]string, []error) {
	t.Helper()
	return checkErrsIn(t, "", source)
}

// checkErrsIn checkErrs gibidir; dosyanın yazım dili lang'dir (SetFileLanguage) ve hata metinleri
// lang'in iletisidir. lang boşsa dil bilinmez, metinler gösterim dilindedir. Kaynak kodlu gövdedir:
// program içeride kimliklerle çalışır (17.7), kullanıcı adları yazıldıkları gibi kalır.
func checkErrsIn(t *testing.T, lang, source string) ([]string, []error) {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	c.SetFileLanguage(lang)
	errs := c.CheckProgram(prog)
	if len(errs) != len(c.Errors()) || len(errs) != len(c.Errs()) {
		t.Errorf("CheckProgram, Errors() ve Errs() farklı sayıda hata döndürdü: %v / %v", errs, c.Errors())
	}
	if lang != "" {
		for i, err := range c.Errs() {
			errs[i] = messages.ErrorIn(err, lang)
		}
	}
	return errs, c.Errs()
}

func noErrors(t *testing.T, source string) {
	t.Helper()
	if errs := check(t, source); len(errs) > 0 {
		t.Errorf("hata beklenmiyordu, gelen: %v\nkaynak:\n%s", errs, source)
	}
}

// expectCode hatanın beklenen ileti kodunu taşımasını ve (line > 0 ise) satırının line olmasını sınar.
func expectCode(t *testing.T, err error, code messages.Code, line int) {
	t.Helper()
	if !messages.Has(err, code) {
		t.Errorf("hata %v kodunu taşımalı, gelen: %q", code, err)
	}
	if got, _, ok := messages.Pos(err); line > 0 && (!ok || got != line) {
		t.Errorf("hata %d. satırda beklenirdi, gelen: %d (konum var mı: %v): %q", line, got, ok, err)
	}
}

// singleError kaynağın tam olarak bir hata üretmesini; hatanın code kodunu ve (line > 0 ise) satırını
// taşımasını ve parts değerlerini (adlar, tip adları) içermesini bekler.
func singleError(t *testing.T, source string, code messages.Code, line int, parts ...string) {
	t.Helper()
	singleErrorIn(t, "", source, code, line, parts...)
}

// singleErrorIn singleError gibidir; dosya lang'de yazılmıştır, parts lang'in iletisinde aranır (checkErrsIn).
func singleErrorIn(t *testing.T, lang, source string, code messages.Code, line int, parts ...string) {
	t.Helper()
	errs, coded := checkErrsIn(t, lang, source)
	if len(errs) != 1 {
		t.Errorf("tek bir hata bekleniyordu, gelen %d: %v\nkaynak:\n%s", len(errs), errs, source)
		return
	}
	expectCode(t, coded[0], code, line)
	for _, p := range parts {
		if !strings.Contains(errs[0], p) {
			t.Errorf("hata %q parçasını içermeli, gelen: %q\nkaynak:\n%s", p, errs[0], source)
		}
	}
}
