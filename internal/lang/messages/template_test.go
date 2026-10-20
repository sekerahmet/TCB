package messages

import (
	"strings"
	"testing"
)

// mustCatalog tek kodlu (H0001) bir tanım ve katalog kurar.
func mustCatalog(t *testing.T, lang string, params, text string) *Catalog {
	t.Helper()
	d, err := ParseDefinitions([]byte(`{"H0001": { "param": [` + params + `] }}`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseCatalog(lang, []byte(`{"H0001": `+quoteJSON(text)+`}`), d)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return c
}

// Çoğul biçimi katalogdadır (ICU MessageFormat'ın plural alt kümesi): dil kendi çoğul kuralıyla
// (CLDR) biçimi seçer, Go kodu sayıya bakıp sözcük seçmez. '#' sayının kendisidir; "=N" tam
// eşleşme kategoriden önce gelir.
func TestPlural(t *testing.T) {
	en := mustCatalog(t, "en", `"n"`, "{n, plural, =0 {no arguments} one {# argument} other {# arguments}}")
	tr := mustCatalog(t, "tr", `"n"`, "{n, plural, one {zorunlu argümanı} other {zorunlu # argümanı}}")
	for _, d := range []struct {
		c    *Catalog
		n    any
		want string
	}{
		{en, 0, "no arguments"}, {en, 1, "1 argument"}, {en, int64(2), "2 arguments"}, {en, 1.5, "1.5 arguments"},
		{tr, 1, "zorunlu argümanı"}, {tr, 3, "zorunlu 3 argümanı"},
	} {
		if got := d.c.render(1, []any{d.n}); got != d.want {
			t.Errorf("%s %v: %q, %q bekleniyordu", d.c.Language, d.n, got, d.want)
		}
	}
}

// select bir metin değerine göre dal seçer; bilinmeyen değer "other" dalına düşer.
func TestSelect(t *testing.T) {
	c := mustCatalog(t, "tr", `"kind", "path"`, "{path|q} {kind, select, dir {bir klasör} other {bir dosya}}")
	if got := c.render(1, []any{"dir", "a"}); got != `"a" bir klasör` {
		t.Errorf("dir: %q", got)
	}
	if got := c.render(1, []any{"x", "a"}); got != `"a" bir dosya` {
		t.Errorf("other: %q", got)
	}
}

// Hâl eki şablondadır ({ad|dat}); eki dilin kayıtlı çekimcisi ekler. Çekimcisi olmayan dilde
// (İngilizce) ad olduğu gibi kalır; Go kodu ada ek eklemez.
func TestCaseSuffix(t *testing.T) {
	RegisterInflector("tr", func(word, grammarCase string) string { return word + "<" + grammarCase + ">" })
	defer RegisterInflector("tr", nil)
	tr := mustCatalog(t, "tr", `"name"`, "{name|dat} {name|acc} {name|loc} {name|abl} {name|gen}")
	if got := tr.render(1, []any{"f"}); got != "f<dat> f<acc> f<loc> f<abl> f<gen>" {
		t.Errorf("tr: %q", got)
	}
	en := mustCatalog(t, "en", `"name"`, "to {name|dat}")
	if got := en.render(1, []any{"f"}); got != "to f" {
		t.Errorf("en: %q", got)
	}
}

// Biçim hataları dil dosyası yüklenirken yakalanır.
func TestTemplateErrors(t *testing.T) {
	d, _ := ParseDefinitions([]byte(`{"H0001": { "param": ["n"] }}`))
	for _, text := range []string{
		"{n, plural, one {a}}",                 // other dalı yok
		"{n, plural, one {a} other {b}",        // kapanmamış
		"{n, plural, many {a} other {b} x {c}", // dalsız anahtar
		"{n, sayı, other {b}}",                 // bilinmeyen tür
		"{n|xyz}",                              // bilinmeyen biçim
		"{m, plural, other {b}}",               // tanımsız parametre
	} {
		if _, err := ParseCatalog("tr", []byte(`{"H0001": `+quoteJSON(text)+`}`), d); err == nil {
			t.Errorf("%q: hata bekleniyordu", text)
		}
	}
	// Çoğul dalındaki parametre "kullanılmış" sayılır.
	if _, err := ParseCatalog("tr", []byte(`{"H0001": "{n, plural, other {x}}"}`), d); err != nil {
		t.Errorf("çoğul parametresi kullanılmış sayılmalı: %v", err)
	}
	if _, err := ParseCatalog("tr", []byte(`{"H0001": "#{n}"}`), d); err != nil || !strings.Contains(mustCatalog(t, "tr", `"n"`, "#{n}").render(1, []any{2}), "#2") {
		t.Errorf("çoğul dışında # düz metindir: %v", err)
	}
}
