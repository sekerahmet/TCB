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

// Her gömülü yazım dilinin kendi metin dosyası (text/<dil>.json) vardır, yedek dil yoktur; her şablonun her
// dilde bir hâli vardır ve hiçbirinde çözülmemiş yer tutucu (ph_ ya da %sözcük%) kalmaz.
func TestSnippetsEveryLanguageEveryTemplate(t *testing.T) {
	result := generated(t)
	for _, lang := range codes.Languages() {
		if _, err := textFiles.ReadFile("text/" + lang + ".json"); err != nil {
			t.Errorf("%s: text/%s.json yok: %v", lang, lang, err)
		}
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

// Her dilin şablonu, varsayılanlarıyla açıldığında kimlik yazımıyla geçerli koddur: kodlu gövde olarak
// ayrıştırılır (varsayılanlar tanımlı adlar aramaz; derleme sınaması Türkçe şablonlar için cmd/tr/e2e'dedir).
func TestSnippetsParseInEveryLanguage(t *testing.T) {
	for lang, entries := range generated(t) {
		for _, e := range entries {
			code := open(strings.Join(e.Body, "\n"))
			if strings.HasPrefix(code, ".") {
				code = "list" + code // metot şablonları bir listenin ardına yazılır
			}
			if r := source.Prepare(code+"\n", source.Options{Body: true}); r.Stage == source.StageSyntax {
				t.Errorf("%s: %q şablonu ayrıştırılamadı: %v\n%s", lang, e.Label, r.Errors, code)
			}
		}
	}
}

// Kural (17.7): şablon verisi dilden bağımsızdır: gövdeler kodlu biçimdedir, data.json'da Türkçe harf ya da
// sözcük yoktur. Şablon kodlu dosyaya yazılır: dilin sözcükleri her dilde aynı kimliklerdir (TR_func), yalnız
// kullanıcının adları ve durakların varsayılan sözcükleri (ad, koşul) dildedir.
func TestSnippetDataIsLanguageNeutral(t *testing.T) {
	if turkish := regexp.MustCompile(`[çğıöşüÇĞİÖŞÜ]`).FindString(string(dataJSON)); turkish != "" {
		t.Errorf("data.json'da Türkçe harf var: %q", turkish)
	}
	for _, tpl := range templates {
		if b, err := source.Split(tpl.Source); err != nil || !b.Coded {
			t.Errorf("%s: gövde kodlu biçimde değil", tpl.ID)
		}
	}
	result := generated(t)
	identities := regexp.MustCompile(`TR_[\p{L}\p{N}_]+`)
	skeleton := func(e entry) string {
		return strings.Join(identities.FindAllString(strings.Join(e.Body, "\n"), -1), " ")
	}
	for i, tr := range result["tr"] {
		if en := result["en"][i]; skeleton(tr) != skeleton(en) || skeleton(tr) == "" && !strings.HasPrefix(tr.Body[0], "[") {
			t.Errorf("%s: gövde her dilde aynı kimlik yazımı olmalı:\n%s\n---\n%s", tr.Label, strings.Join(tr.Body, "\n"), strings.Join(en.Body, "\n"))
		}
	}
	if result["tr"][0].Label == result["en"][0].Label {
		t.Error("Türkçe ve İngilizce şablonun adı aynı çıktı")
	}
}
