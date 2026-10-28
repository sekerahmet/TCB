package parser

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// A syntax error is at the token it is about, from its first character to the one after its last
// (2.13), and carries the innermost code, not the H0100 every syntax error is wrapped in.
func TestSyntaxErrorRangeIsTheToken(t *testing.T) {
	cases := []struct {
		name string
		code string
		want messages.Range
	}{
		{"number where a name is expected", "TR_var 12 = 1", messages.Range{Line: 1, Col: 8, EndLine: 1, EndCol: 10}},
		{"bracket on the second line", "TR_var a = 1\nTR_var ağırlık = )", messages.Range{Line: 2, Col: 18, EndLine: 2, EndCol: 19}},
	}
	for _, c := range cases {
		errs := parseErrs(c.code)
		if len(errs) == 0 {
			t.Errorf("%s: no error", c.name)
			continue
		}
		p, ok := messages.Find(errs[0])
		if !ok {
			t.Errorf("%s: no position in %v", c.name, errs[0])
			continue
		}
		if got := p.Range(); got != c.want {
			t.Errorf("%s: range %+v, want %+v (%v)", c.name, got, c.want, errs[0])
		}
		if code, _ := messages.InnerCode(errs[0]); code == messages.H0100 {
			t.Errorf("%s: the record would carry the wrapper's code H0100", c.name)
		}
	}
}
