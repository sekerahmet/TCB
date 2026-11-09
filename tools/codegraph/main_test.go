package main

import (
	"strings"
	"testing"
)

// Kural: `help`'in changed satırı ucu çıkış yapılmış HEAD olan farkın çalışma ağacına kadar alındığını söyler (gaps,
// "`changed`'in `help` satırı çalışma ağacını söylemiyor": yalnız README söylüyordu, yan oturumlar `changed HEAD`'i
// istek olarak yazdı; kural changeHunks'ta).
func TestHelpSaysChangedHeadReadsTheWorkingTree(t *testing.T) {
	var out, errOut strings.Builder
	if code := run([]string{"help"}, &out, &errOut); code != exitOK {
		t.Fatalf("help: exit %d: %s", code, errOut.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "changed <") {
			if !strings.Contains(line, "`changed HEAD`, the diff runs to the working tree") {
				t.Errorf("the changed line does not say the working tree is included:\n%s", line)
			}
			return
		}
	}
	t.Fatalf("help has no changed line:\n%s", out.String())
}
