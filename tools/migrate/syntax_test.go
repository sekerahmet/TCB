package main

import (
	"strings"
	"testing"
)

// Kalkan anahtar kelimenin eski yazımı (removed.json) yeni karşılığına çevrilir: 'iken' döngüdür.
func TestRemovedKeywordIsConverted(t *testing.T) {
	out, problems := convertTRFile("eski.tr", "değişken x = 0\niken x < 3 {\n    x += 1\n}\n")
	if len(problems) > 0 {
		t.Fatalf("beklenmeyen sorun: %v", problems)
	}
	if !strings.Contains(out, "TR_for x < 3 {") {
		t.Errorf("'iken' TR_for olmalıydı:\n%s", out)
	}
}
