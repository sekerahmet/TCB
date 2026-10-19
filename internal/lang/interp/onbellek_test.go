package interp

import "testing"

func TestKucukIntOnbellegi(t *testing.T) {
	for _, n := range []int64{kucukIntEnAz, -1, 0, 1, kucukIntEnCok} {
		a, b := NewInt(n), NewInt(n)
		if a != b || a.Val != n {
			t.Errorf("NewInt(%d) önbellekten aynı nesneyi döndürmeliydi", n)
		}
	}
	for _, n := range []int64{kucukIntEnAz - 1, kucukIntEnCok + 1, 1 << 40} {
		if NewInt(n) == NewInt(n) || NewInt(n).Val != n {
			t.Errorf("NewInt(%d) önbellek dışında yeni nesne döndürmeliydi", n)
		}
	}
}

// Önbellekteki paylaşılan tamsayılar yerinde değişmemeli: bir değişkeni artırmak aynı
// değeri tutan başka bir değişkeni etkilememeli.
func TestPaylasilanTamsayiDegismez(t *testing.T) {
	sonuc, err := runTR(t, `
değişken a = 5
değişken b = a
a += 1
değişken l = [5, 5]
l[0] += 10
değişken s = "{a} {b} {l} {2 + 3}"
s`)
	if err != nil {
		t.Fatal(err)
	}
	if sonuc.Inspect() != "6 5 [15, 5] 5" {
		t.Fatalf("beklenen \"6 5 [15, 5] 5\", alınan %q", sonuc.Inspect())
	}
}
