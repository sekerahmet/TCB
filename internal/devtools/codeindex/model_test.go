package codeindex

import "testing"

func TestNodeID(t *testing.T) {
	cases := []struct {
		kind       Kind
		file, name string
		want       string
	}{
		{KindFunction, "lib/katman.tr", "cevre", "function:lib/katman.tr#cevre"},
		{KindMethod, "lib/katman.tr", "Doğrusal.ileri", "method:lib/katman.tr#Doğrusal.ileri"},
		{KindModule, "lib/katman.tr", "", "module:lib/katman.tr"},
		{KindBuiltin, "", "TR_time_ms", "builtin:TR_time_ms"},
	}
	for _, c := range cases {
		if got := NodeID(c.kind, c.file, c.name); got != c.want {
			t.Errorf("NodeID(%s, %q, %q) = %q, want %q", c.kind, c.file, c.name, got, c.want)
		}
	}
}

func TestPosString(t *testing.T) {
	if got := (Pos{File: "a.tr", Line: 3, Col: 7}).String(); got != "a.tr:3:7" {
		t.Errorf("got %q", got)
	}
	if got := (Pos{File: "a.tr", Line: 3}).String(); got != "a.tr:3" {
		t.Errorf("a position without a column: %q", got)
	}
}
