package interp

import (
	"testing"
)

// Kural (5.7, 3.1): aralıkta gezen döngünün her turunun sayacı ayrı bir değerdir. Listeye, kapanışa,
// göreve (TR_async) ve ertelenen işe (TR_defer) kaçan sayaç sonraki turlarda değişmez. Sayaçlar
// parça parça ayrılır (intChunk); büyük sayılar da (önbellekte olmayan) kendi değerini korur.
func TestARangeCounterKeepsItsValueWhenItEscapes(t *testing.T) {
	const source = `TR_var l = []
TR_var fs = []
TR_var gs = []
TR_var ks = []
TR_var son = []
TR_func al(x: TR_Int) -> TR_Int {
    TR_sleep(0.001)
    TR_return x
}
TR_func ertele() {
    TR_for i TR_in 3000..3003 {
        TR_defer { son.TR_append(i) }
    }
}
TR_for k, i TR_in 5000..5130 {
    l.TR_append(i)
    TR_var f = (x) => x + i
    fs.TR_append(f)
    gs.TR_append(TR_async al(i))
    ks.TR_append(k)
}
ertele()
TR_var toplam = 0
TR_for f TR_in fs {
    toplam += f(0)
}
TR_print(l[0], l[63], l[64], l[129], TR_sum(l), toplam, TR_sum(TR_wait_all(gs)), ks[129], son)
`
	out, _, err := runTROut(t, source)
	// 5000..5129'un toplamı 130·5000 + 129·130/2 = 658385.
	const want = "5000 5063 5064 5129 658385 658385 658385 129 [3000, 3001, 3002]\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

// Hız (S1): aralıklı döngünün sayacı turda bir ayırma değildir; 20 000 turluk döngüde
// sayaçlar parçalarla ayrılır. Eşik: parçasız yolun yarısı (turda ~2 ayırma: sayaç ve t += i).
func TestARangeCounterIsNotOneAllocationPerTurn(t *testing.T) {
	prog := mustParseProgram(t, "TR_var t = 0\nTR_for i TR_in 0..20000 {\n    t += i\n}\n")
	allocs := testing.AllocsPerRun(5, func() {
		machine := New()
		if _, err := machine.Run(prog); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 30000 {
		t.Fatalf("20 000 turda %v ayırma; sayaç parçasız (turda bir) ayrılıyor olabilir", allocs)
	}
}
