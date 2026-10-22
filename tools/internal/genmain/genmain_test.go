package genmain

import "testing"

// Kural: üretilen dosya karşılaştırması satır sonu farkını (CRLF çalışma ağacı) saymaz ama başka
// her farkı sayar.
// Neden: dört üretici kontrolü farklı yazıyordu; ikisi CRLF'de sahte "güncel değil" verirdi.
func TestSameBytesIgnoresCRLFOnly(t *testing.T) {
	for _, c := range []struct {
		old, fresh string
		want       bool
	}{
		{"a\r\nb\r\n", "a\nb\n", true},
		{"a\nb\n", "a\nb\n", true},
		{"a\nb\n", "a\nc\n", false},
		{"a\n", "a", false},
	} {
		if got := sameBytes([]byte(c.old), []byte(c.fresh)); got != c.want {
			t.Errorf("sameBytes(%q, %q) = %v", c.old, c.fresh, got)
		}
	}
}
