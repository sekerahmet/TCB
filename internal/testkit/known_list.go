package testkit

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// CheckKnownList bir sınavın bugün kalan durumlarını (failing) dosyadaki bilinen kalanlarla
// karşılaştırır: listede olmayan kalan durum da, artık geçen liste satırı da testi düşürür (liste
// yalnızca küçülür). update doğruysa karşılaştırmaz, dosyayı header ve failing ile yeniden yazar.
// Dosyada '#' ile başlayan satırlar açıklamadır.
func CheckKnownList(t testing.TB, path, header string, failing []string, update bool) {
	t.Helper()
	failing = slices.Compact(slices.Sorted(slices.Values(failing)))
	if update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(header+strings.Join(failing, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			known[line] = true
		}
	}
	var unexpected []string
	for _, c := range failing {
		if !known[c] {
			unexpected = append(unexpected, c)
		}
		delete(known, c)
	}
	if len(unexpected) > 0 {
		t.Errorf("%d failing cases are not in %s:\n%s", len(unexpected), path, strings.Join(unexpected, "\n"))
	}
	if stale := slices.Sorted(maps.Keys(known)); len(stale) > 0 {
		t.Errorf("%d cases in %s now pass; remove them (-update):\n%s", len(stale), path, strings.Join(stale, "\n"))
	}
	t.Logf("%d cases in %s", len(failing), path)
}
