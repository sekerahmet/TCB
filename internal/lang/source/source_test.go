package source

import (
	"errors"
	"strings"
	"testing"
)

func TestHeaderRules(t *testing.T) {
	for _, m := range []string{Header + "\n", Header, "\xef\xbb\xbf" + Header + "\r\nA030 x = 1"} {
		if b, err := Split(m); err != nil || !b.Coded || b.Join() != m {
			t.Errorf("%q: kodlu okunmalı ve geri birleşmeli (%+v, %v)", m, b, err)
		}
	}
	for _, m := range []string{"//! tr kodlu 2\nA030 x = 1", Header + " \n", "//! tr kodlu\n"} {
		var bh *HeaderError
		if _, err := Split(m); !errors.As(err, &bh) {
			t.Errorf("%q: tanınmayan başlık hatası bekleniyordu, alınan %v", m, err)
		}
		if s := Prepare(m, Options{}); s.Stage != StageSyntax || !strings.Contains(s.Errors[0], "Satır 1, Sütun 1:") {
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
