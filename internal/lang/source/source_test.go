package source

import (
	"errors"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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
