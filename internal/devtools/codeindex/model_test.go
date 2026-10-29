package codeindex

import "testing"

func TestNodeID(t *testing.T) {
	cases := []struct {
		kind       Kind
		file, name string
		want       string
	}{
		{KindFunction, "lib/layers.tr", "cevre", "function:lib/layers.tr#cevre"},
		{KindMethod, "lib/layers.tr", "Doğrusal.ileri", "method:lib/layers.tr#Doğrusal.ileri"},
		{KindModule, "lib/layers.tr", "", "module:lib/layers.tr"},
		{KindBuiltin, "", "TR_fn_time_ms", "builtin:TR_fn_time_ms"},
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
