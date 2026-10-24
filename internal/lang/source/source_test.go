package source

import (
	"errors"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestHeaderRules(t *testing.T) {
	cases := map[string]string{Header + "tr\n": "tr", Header + "en": "en", "\xef\xbb\xbf" + Header + "en\r\nA030 x = 1": "en"}
	for m, language := range cases {
		if b, err := Split(m); err != nil || !b.Coded || b.Language != language || b.Join() != m {
			t.Errorf("%q: %s dilinde kodlu okunmalı ve geri birleşmeli (%+v, %v)", m, language, b, err)
		}
	}
	// Başlıksız dosya Türkçedir (17.7).
	if b, err := Split("A030 x = 1\n"); err != nil || b.Coded || b.Language != "tr" {
		t.Errorf("başlıksız dosya Türkçe olmalı: %+v, %v", b, err)
	}
	// Eski ve yeni biçim sürümleri, dilsiz ya da bilinmeyen dilli başlık okunmaz (17.1, 17.6); dilsiz
	// sürüm 2 başlığı sürümü değil eksik dili söyler (H2523).
	unreadable := map[string]messages.Code{
		"//! tr kodlu 1\nA030 x = 1": messages.H2512, "//! tr kodlu 3 dil=en\n": messages.H2512,
		"//! tr kodlu 2\n": messages.H2523, Header + "\n": messages.H2523, Header: messages.H2523, Header + "en \n": messages.H2508,
		"//! tr kodlu 2 \n": messages.H2512,
		Header + "fr\n":     messages.H2508, "//! tr kodlu\n": messages.H2512,
	}
	for m, code := range unreadable {
		var bh *HeaderError
		if _, err := Split(m); !errors.As(err, &bh) || !messages.Has(err, code) {
			t.Errorf("%q: %v başlık hatası bekleniyordu, alınan %v", m, code, err)
		}
		if s := Prepare(m, Options{}); s.Stage != StageSyntax || !hasPos(s.Errors[0], 1, 1) {
			t.Errorf("%q: hazırlık başlık hatasını sözdizimi hatası olarak bildirmeli: %+v", m, s.Errors)
		}
	}
	// İlk satır olmayan ya da başka bir '//!' yorumu sıradan yorumdur.
	for _, m := range []string{"\n" + Header + "\n", "//! tr notu\n"} {
		if b, err := Split(m); err != nil || b.Coded {
			t.Errorf("%q: Türkçe kaynak sayılmalı (%+v, %v)", m, b, err)
		}
	}
}

func hasPos(err error, line, col int) bool {
	l, c, ok := messages.Pos(err)
	return ok && l == line && c == col
}
