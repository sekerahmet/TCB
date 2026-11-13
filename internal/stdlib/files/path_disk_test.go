package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func pathMember(t *testing.T, p string, kd codes.Code) (interp.Value, error) {
	t.Helper()
	return NewPath(p).TaskMember(nil, kd, "")
}

// TestPathModifiedNs .TR_modified_ns dosyanın değiştirilme zamanı, nanosaniye; yazmadan sonra
// artar ya da eşit kalır; klasörde H7736, olmayan dosyada okuma hatası (18.3).
func TestPathModifiedNs(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := pathMember(t, f, codes.MemModifiedNs)
	if err != nil {
		t.Fatal(err)
	}
	first := v.(*interp.IntVal).Val
	st, _ := os.Stat(f)
	if first != st.ModTime().UnixNano() {
		t.Fatalf("%d, Stat %d", first, st.ModTime().UnixNano())
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(f, later, later); err != nil {
		t.Fatal(err)
	}
	v, _ = pathMember(t, f, codes.MemModifiedNs)
	if v.(*interp.IntVal).Val <= first {
		t.Fatalf("değişme zamanı artmadı: %d → %d", first, v.(*interp.IntVal).Val)
	}
	if _, err := pathMember(t, dir, codes.MemModifiedNs); !messages.Has(err, messages.H7736) {
		t.Fatalf("klasörde H7736 bekleniyordu: %v", err)
	}
	if _, err := pathMember(t, filepath.Join(dir, "yok.txt"), codes.MemModifiedNs); err == nil {
		t.Fatalf("olmayan dosyada hata bekleniyordu")
	}
}

// TestPathDiskAvailableBytes .TR_disk_available_bytes dosyada ve klasörde pozitif; olmayan
// yolda H7737 üst klasörü önerir ve üst klasöre kendiliğinden çıkmaz (18.3).
func TestPathDiskAvailableBytes(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{dir, f} {
		v, err := pathMember(t, p, codes.MemDiskAvailableBytes)
		if err != nil {
			t.Fatal(err)
		}
		if v.(*interp.IntVal).Val <= 0 {
			t.Fatalf("%s: boş yer %v", p, v)
		}
	}
	missing := filepath.ToSlash(filepath.Join(dir, "yok", "model.bin"))
	_, err := pathMember(t, missing, codes.MemDiskAvailableBytes)
	e, ok := messages.As(err)
	if !ok || e.Code != messages.H7737 {
		t.Fatalf("H7737 bekleniyordu: %v", err)
	}
	if want := filepath.ToSlash(filepath.Join(dir, "yok")); len(e.Args) != 3 || e.Args[2] != want {
		t.Fatalf("üst klasör önerisi %v, beklenen %q", e.Args, want)
	}
}
