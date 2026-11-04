package interp

import (
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (5.7, 13.4): başka bir görev dolaşılan sözlükten anahtar çıkarınca dolaşma H2704 verir;
// çıkarma turun başındaki güvenli noktada olsa da döngü silinen anahtarın değerine ulaşmaz.
func TestAnotherTaskRemovingFromADictUnderIteration(t *testing.T) {
	const code = `TR_var d: TR_Dict[TR_Int, TR_Int] = {}
TR_for i TR_in 0..64 { d[i] = i }
TR_var dur = TR_false
TR_async {
    TR_for TR_not dur {
        TR_if d.TR_length == 0 { dur = TR_true } TR_else { d.TR_remove(d.TR_keys()[0]) }
    }
}
TR_var toplam = 0
TR_for TR_not dur {
    TR_for k, v TR_in d { toplam += v + 1 }
}`
	_, _, err := runTRWithin(t, 30*time.Second, code)
	if err == nil {
		return // görev dolaşma turlarının arasına düştü: kural çiğnenmedi
	}
	if !messages.Has(err, messages.H2704) {
		t.Fatalf("hata = %v, want H2704", err)
	}
}

// Kural (13.4, 13.11): denetimden geçmeyen yolda da listede .TR_remove denetimdeki hatayı verir.
func TestRemoveOnAListOfUnknownType(t *testing.T) {
	_, err := New().Run(mustParseProgram(t, "[1, 2].TR_remove(0)\n"))
	if !messages.Has(err, messages.H2535) {
		t.Errorf("hata = %v, want H2535", err)
	}
}

// Kural (13.4): silinen anahtardan iz kalmaz: paylaşılan tablo silinenlerle büyümez.
func TestRemovedKeyLeavesNothing(t *testing.T) {
	d := &DictVal{Pairs: map[string]Value{}}
	d.SetValue(NewInt(1), NewInt(10))
	d.SetValue(True, NewInt(20))
	for _, k := range []Value{NewInt(1), True} {
		if _, ok := d.removeValue(k); !ok {
			t.Fatalf("%s silinmedi", k.Inspect())
		}
	}
	if len(d.Pairs) != 0 || len(d.Order) != 0 || len(d.KeyVals) != 0 {
		t.Errorf("Pairs %d, Order %d, KeyVals %d, want 0", len(d.Pairs), len(d.Order), len(d.KeyVals))
	}
}

// Kural (5.7): iletideki yeni boy çıkarmadan sonraki boydur; aynı turda eklenince de "3 öğeden 3
// öğeye" demez, çok çıkarmada sıfırın altına inmez.
func TestIterationMessageShowsTheSizeAfterRemoval(t *testing.T) {
	m := iterMark{size: 3, removed: 2}
	for _, c := range []struct{ size, removed, want int }{
		{2, 3, 2},
		{3, 3, 2},
		{3, 9, 0},
		{4, 2, 4},
	} {
		if got := m.after(c.size, c.removed); got != c.want {
			t.Errorf("after(%d, %d) = %d, want %d", c.size, c.removed, got, c.want)
		}
	}
}
