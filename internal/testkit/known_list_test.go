package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Kural: bilinen kalanlar listesi yalnızca küçülür: listede olmayan kalan durum da, artık geçen
// liste satırı da testi düşürür; -update yalnız artık geçen satırları siler, yeni kalanı yazmaz
// (yeni kalan elle ve görünür eklenir).
func TestCheckKnownListOnlyShrinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testdata", "known.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# eski başlık\nb\na\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &recorder{TB: t}
	CheckKnownList(r, path, "", []string{"a", "b"}, false)
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "d") {
		t.Errorf("a stale line must fail: %v", r.errors)
	}
	r.errors = nil
	CheckKnownList(r, path, "", []string{"a", "c"}, false)
	if len(r.errors) != 2 || !strings.Contains(r.errors[0], "c") || !strings.Contains(r.errors[1], "b") {
		t.Errorf("an unlisted failing case and a stale line must both fail: %v", r.errors)
	}

	r.errors = nil
	CheckKnownList(r, path, "# header\n", []string{"b", "a", "a"}, true)
	if len(r.errors) != 0 {
		t.Errorf("update with known cases only must pass: %v", r.errors)
	}
	if data, _ := os.ReadFile(path); string(data) != "# header\na\nb\n" {
		t.Fatalf("update must write the sorted known cases that still fail under the header: %q", data)
	}

	CheckKnownList(r, path, "# header\n", []string{"a", "c"}, true)
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "c") {
		t.Errorf("update must fail on a failing case the list does not have: %v", r.errors)
	}
	if data, _ := os.ReadFile(path); string(data) != "# header\na\nb\n" {
		t.Errorf("update must not write the file when a new failing case appears: %q", data)
	}
}
