package sysmem

import (
	"strings"
	"testing"
)

func TestParseMemAvailable(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"MemTotal:       16314880 kB\nMemFree:          403528 kB\nMemAvailable:    9876543 kB\n", 9876543 * 1024, true},
		{"MemAvailable:          0 kB\n", 0, true},
		{"MemTotal:       16314880 kB\nMemFree:          403528 kB\n", 0, false},
		{"MemAvailable:    12 MB\n", 0, false},
		{"MemAvailable:    -5 kB\n", 0, false},
		{"MemAvailable:    99999999999999999999 kB\n", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseMemAvailable(strings.NewReader(c.in))
		if got != c.want || ok != c.ok {
			t.Errorf("parseMemAvailable(%q) = %d, %v; beklenen %d, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
