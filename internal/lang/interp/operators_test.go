package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (4.3): H2603'ün önerdiği yazım kopyalanınca doğru sonucu verir. Eksi taban paranteze
// alınır: '-2.0 ** -2' 4.1'e göre '-(2.0 ** -2)' okunur ve -0.25 verirdi, doğrusu 0.25.
func TestNegativeIntExponentSuggestsAWorkingSpelling(t *testing.T) {
	for _, c := range []struct {
		a, b int64
		want string
	}{
		{2, -1, "(2 ** -1); for a decimal result write 2.0 ** -1"},
		{-2, -2, "((-2) ** -2); for a decimal result write (-2.0) ** -2"},
	} {
		_, _, err := intOp(opPow, "**", c.a, c.b, 1)
		if got := messages.ErrorIn(err, "en"); !messages.Has(err, messages.H2603) || !strings.Contains(got, c.want) {
			t.Errorf("%d ** %d: %q bekleniyordu, alınan: %s", c.a, c.b, c.want, got)
		}
	}
}

// Kural (4.10): listeye sayı eklemeye çalışan '+' listenin yazımını önerir (l + [x],
// l.TR_append(x)), metin yerleştirmesini değil; metinli durum yerleştirmeyi önerir.
func TestListPlusSuggestsTheListSpelling(t *testing.T) {
	for _, c := range []struct {
		left, right Value
		code        messages.Code
		want        string
	}{
		{&ListVal{Elements: []Value{NewInt(1)}}, NewInt(2), messages.H2614, "l + [x]"},
		{NewInt(2), &ListVal{Elements: []Value{NewInt(1)}}, messages.H2614, "l + [x]"},
		{&StringVal{Val: "a"}, NewInt(1), messages.H2609, `\(x)`},
		{&StringVal{Val: "a"}, &ListVal{}, messages.H2609, `\(x)`},
	} {
		_, ok, err := sequenceOp(opAdd, c.left, c.right, 1)
		if got := messages.ErrorIn(err, "en"); !ok || !messages.Has(err, c.code) || !strings.Contains(got, c.want) {
			t.Errorf("%s + %s: %v ve %q bekleniyordu, alınan: %s", c.left.Type(), c.right.Type(), c.code, c.want, got)
		}
	}
}
