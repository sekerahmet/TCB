package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Değeri kullanılmayan '-x' deyimi uyarı verir (2.8): satır başındaki '-' önceki satırı
// sürdürmez, bu yüzden 'toplam = a' ardından gelen '- b' satırı sessizce etkisiz kalırdı (Kotlin
// de bu durumda uyarır). Kabukta (etkileşimli) ifadenin değeri gösterildiği için uyarı yoktur.
func TestUnusedNegationWarns(t *testing.T) {
	check := func(code string, interactive bool) []error {
		p := parser.New(lexer.New(code))
		prog := p.ParseProgram()
		c := New()
		if interactive {
			c.SetInteractive()
		}
		c.CheckProgram(prog)
		return c.Warnings()
	}
	w := check("değişken b = 2\ndeğişken a = 1\n    - b", false)
	if len(w) != 1 || !messages.Has(w[0], messages.H1011) {
		t.Fatalf("H1011 uyarısı bekleniyordu: %v", w)
	}
	if line, _, _ := messages.Pos(w[0]); line != 3 {
		t.Errorf("uyarı 3. satırda olmalı: %v", w[0])
	}
	if w := check("değişken b = 2\ndeğişken a = 1 -\n    b\ndeğişken c = -b", false); len(w) != 0 {
		t.Errorf("kullanılan değerde uyarı olmamalı: %v", w)
	}
	if w := check("değişken b = 2\n-b", true); len(w) != 0 {
		t.Errorf("kabukta uyarı olmamalı: %v", w)
	}
}
