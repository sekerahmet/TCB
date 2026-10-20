package messages

import "testing"

// Seçenek listesi dilin liste kalıbıyla kurulur: ikili kalıp, ayırıcı ve son kalıp ayrı kodlardır.
func TestOrList(t *testing.T) {
	for _, d := range []struct {
		items []string
		want  string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a ya da b"},
		{[]string{"a", "b", "c"}, "a, b ya da c"},
		{[]string{"a", "b", "c", "d"}, "a, b, c ya da d"},
	} {
		if got := OrList(d.items...); got != d.want {
			t.Errorf("%v: %q, %q bekleniyordu", d.items, got, d.want)
		}
	}
}
