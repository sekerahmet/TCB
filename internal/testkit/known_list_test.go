package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Kural: bilinen kalanlar listesi yalnızca küçülür: listede olmayan kalan durum da, artık geçen
// liste satırı da testi düşürür; -update dosyayı bugünkü sonuçla yeniden yazar.
func TestCheckKnownListOnlyShrinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testdata", "known.txt")
	CheckKnownList(t, path, "# header\n", []string{"b", "a", "a"}, true)
	if data, _ := os.ReadFile(path); string(data) != "# header\na\nb\n" {
		t.Fatalf("update must write the sorted cases under the header: %q", data)
	}
	r := &recorder{TB: t}
	CheckKnownList(r, path, "", []string{"a", "b"}, false)
	if len(r.errors) != 0 {
		t.Errorf("the same cases must pass: %v", r.errors)
	}
	CheckKnownList(r, path, "", []string{"a", "c"}, false)
	if len(r.errors) != 2 || !strings.Contains(r.errors[0], "c") || !strings.Contains(r.errors[1], "b") {
		t.Errorf("an unlisted failing case and a stale line must both fail: %v", r.errors)
	}
}
