package codeindex

import "testing"

// Ek ayrımı: sözcüğün ekli biçimleri sayılır, aynı harflerle başlayan başka sözcükler sayılmaz.
func TestSuffixRecognisesTurkishInflections(t *testing.T) {
	cases := map[string]struct{ yes, no []string }{
		"baş": {
			yes: []string{"ı", "ın", "a", "ta", "tan", "ları", "lar", "ımız", "ında", "larında", "larının", "lara", "ımızı", "ıdır", "tır", "lardır"},
			no:  []string{"ka", "la", "lık", "arı", "vuru", "kan", "lat", "arılı", "ça", "sız", "ı1"},
		},
		"kafa": {
			yes: []string{"m", "sı", "yı", "ya", "dan", "lar", "ların", "mız", "sında"},
			no:  []string{"lık", "ya1", "sız"},
		},
		"göz": {
			yes: []string{"ü", "üm", "ler", "leri", "lerinde", "ün"},
			no:  []string{"lük", "cü", "lem"},
		},
	}
	for stem, c := range cases {
		tails := suffixTails(stem)
		for _, y := range c.yes {
			if !tails[y] {
				t.Errorf("%s+%s should be an inflection", stem, y)
			}
		}
		for _, n := range c.no {
			if tails[n] {
				t.Errorf("%s+%s must not be an inflection", stem, n)
			}
		}
	}
	if suffixTails("st") != nil {
		t.Error("a word without a vowel has no harmony: no inflections")
	}
}

// Ünsüz yumuşaması: çok heceli sözcüğün sondaki sert ünsüzü ünlüyle başlayan ekten önce yumuşar
// (çekirdek → çekirdeği); tek heceliler (ok → oku) yumuşamaz.
func TestSoftenedStems(t *testing.T) {
	for word, want := range map[string]string{"çekirdek": "çekirdeğ", "kitap": "kitab", "ağaç": "ağac", "kanat": "kanad", "ahenk": "aheng"} {
		if got, ok := soften(word); !ok || got != want {
			t.Errorf("%s -> %q %v, want %q", word, got, ok, want)
		}
	}
	for _, word := range []string{"ok", "baş", "kernel", "göz"} {
		if got, ok := soften(word); ok {
			t.Errorf("%s must not soften: %q", word, got)
		}
	}
	m := newWordMatcher("çekirdek", "kernel")
	for token, suggestion := range map[string]string{"çekirdeği": "kerneli", "çekirdeğe": "kernele", "çekirdekler": "kernelzler"} {
		match, got, _, _ := m.classify(token)
		if token == "çekirdekler" {
			if match != MatchSuffixed || got != "kerneller" {
				t.Errorf("%s: %s %q", token, match, got)
			}
			continue
		}
		if match != MatchSuffixed || got != suggestion {
			t.Errorf("%s: %s %q, want %q", token, match, got, suggestion)
		}
	}
	if match, _, _, other := m.classify("çekirdeksiz"); match != MatchDerived && !other {
		t.Errorf("çekirdeksiz is a derived form: %s", match)
	}
}
