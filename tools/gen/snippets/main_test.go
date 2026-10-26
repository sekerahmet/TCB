package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/tools/internal/genmain"
)

// Eklentinin şablon dosyası üretilenle aynı olmalı: bir ad ya da şablon değiştiğinde
// `go run ./tools/gen/snippets` çalıştırılmadıysa bu test düşer.
func TestSnippetsUpToDate(t *testing.T) {
	genmain.AssertUpToDate(t, reporoot.Root(), "snippets", snippetsPath, func(string) ([]byte, error) { return Generate() })
}

func generated(t *testing.T) map[string][]entry {
	t.Helper()
	data, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	var result map[string][]entry
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// Her gömülü yazım dilinin, her şablonun bir hâli vardır ve hiçbirinde çözülmemiş yer tutucu (ph_ ya da
// %sözcük%) kalmaz.
func TestSnippetsEveryLanguageEveryTemplate(t *testing.T) {
	result := generated(t)
	for _, lang := range codes.Languages() {
		entries := result[lang]
		if len(entries) != len(templates) {
			t.Errorf("%s: %d şablon üretildi; %d bekleniyordu", lang, len(entries), len(templates))
		}
		for _, e := range entries {
			text := strings.Join(e.Body, "\n") + strings.Join(e.Prefix, " ") + e.Label + e.Description
			if holeRe.MatchString(text) || wordRe.MatchString(text) {
				t.Errorf("%s: %q şablonunda çözülmemiş yer tutucu var:\n%s", lang, e.Label, text)
			}
			if len(e.Prefix) == 0 || e.Label == "" || e.Description == "" {
				t.Errorf("%s: %q şablonunun kısayolu, adı ya da açıklaması boş", lang, e.Label)
			}
		}
	}
}

var (
	defaultRe = regexp.MustCompile(`\$\{\d+:([^${}]*)\}`)
	emptyRe   = regexp.MustCompile(`\$\{\d+\}|\$\d+`)
)

// open bir düzenleyici şablonunu varsayılan değerleriyle açar (hiçbir durağa basılmamış gibi).
func open(body string) string {
	for {
		updated := defaultRe.ReplaceAllString(body, "$1")
		if updated == body {
			break
		}
		body = updated
	}
	return emptyRe.ReplaceAllString(body, "")
}

// Her dilin şablonu, varsayılanlarıyla açıldığında o dilin yazımında geçerli koddur: ayrıştırılır ve
// kayıpsız kodlanır (varsayılanlar tanımlı adlar aramaz; derleme sınaması Türkçe şablonlar için
// cmd/tr/e2e'dedir).
func TestSnippetsParseInEveryLanguage(t *testing.T) {
	for lang, entries := range generated(t) {
		language, err := codes.LoadLanguage(lang)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			code := open(strings.Join(e.Body, "\n"))
			if strings.HasPrefix(code, ".") {
				code = "list" + code // metot şablonları bir listenin ardına yazılır
			}
			if _, err := source.EncodeIn(code+"\n", language); err != nil {
				t.Errorf("%s: %q şablonu ayrıştırılamadı: %v\n%s", lang, e.Label, err, code)
			}
		}
	}
}
