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
