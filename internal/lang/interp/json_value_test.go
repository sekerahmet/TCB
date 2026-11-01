package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.7a): TR_JSON(x) de en çok 1000 kabı iç içe kurar, içteki değer ne olursa olsun; kabın
// içine konan JSON değerinin kapları da sayılır.
func TestMakeJSONDepthCountsContainers(t *testing.T) {
	nest := func(n int, inner Value) Value {
		v := inner
		for range n {
			v = &ListVal{Elements: []Value{v}}
		}
		return v
	}
	empty := func() Value { return &ListVal{Elements: []Value{}} }
	for _, c := range []struct {
		name       string
		inner      func() Value
		n          int // inner'ın çevresindeki liste sayısı
		containers int // toplam kap
	}{
		{"boş içte", empty, maxJSONDepth - 1, maxJSONDepth},
		{"sayı içte", func() Value { return NewInt(1) }, maxJSONDepth, maxJSONDepth},
	} {
		if _, err := MakeJSON(nest(c.n, c.inner())); err != nil {
			t.Errorf("%s: %d kap geçmeli: %v", c.name, c.containers, err)
		}
		if _, err := MakeJSON(nest(c.n+1, c.inner())); !messages.Has(err, messages.H2500) {
			t.Errorf("%s: %d kap H2500 vermeli: %v", c.name, c.containers+1, err)
		}
	}
	j, err := MakeJSON(nest(maxJSONDepth, NewInt(1)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MakeJSON(&ListVal{Elements: []Value{j}}); !messages.Has(err, messages.H2500) {
		t.Errorf("kaba konan %d kaplı JSON H2500 vermeli: %v", maxJSONDepth, err)
	}
}
