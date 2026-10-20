package tensor

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestCheckLNEmptyOptional boş ama nil olmayan isteğe bağlı bir dilimin (ör. mean) "yok"
// sayılmadığını, kısa dilim gibi reddedildiğini denetler: AVX2 çekirdeği onu yok, Go eşi var
// sayardı.
func TestCheckLNEmptyOptional(t *testing.T) {
	x, a, rstd := make([]float32, 8), make([]float32, 8), make([]float32, 2)
	if R := checkLN(4, x, a, nil, nil, nil, rstd, nil, nil, nil); R != 2 {
		t.Fatalf("satır sayısı %d, 2 bekleniyordu", R)
	}
	for ad, f := range map[string]func(){
		"mean": func() { checkLN(4, x, a, nil, nil, []float32{}, rstd, nil, nil, nil) },
		"g":    func() { checkLN(4, x, a, []float32{}, nil, nil, rstd, nil, nil, nil) },
		"pg":   func() { checkLN(4, x, a, nil, nil, nil, rstd, nil, []float64{}, nil) },
	} {
		func() {
			defer func() {
				if r, _ := recover().(error); !messages.Has(r, messages.H9000) {
					t.Errorf("%s: boş dilim reddedilmedi (%v)", ad, r)
				}
			}()
			f()
		}()
	}
}
