package e2e

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Program tek bir dosyadır: en dışındaki deyimler yukarıdan aşağı çalışır, main adlı bir
// fonksiyon giriş noktası değildir, kendiliğinden çağrılmaz (dil tanımı 1.1).
func TestProgramRunsTopToBottom(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "p.tr", `yazdır("bir")
fonksiyon main() {
    yazdır("main")
}
yazdır("iki")
`)
	s := tr(t, dir, "p.tr")
	testkit.Wait(t, s, 0, nil, nil)
	if s.Stdout != "bir\niki\n" {
		t.Errorf("çıktı %q, beklenen %q", s.Stdout, "bir\niki\n")
	}
}
