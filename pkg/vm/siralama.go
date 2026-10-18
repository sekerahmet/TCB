package vm

import (
	"math"
	"slices"
	"sort"
)

// siralaDegerler değerleri sırala() kuralıyla kararlı (eşitlerin sırası korunarak) sıralar:
// sayılar sayısal, diğerleri görünen metinleriyle karşılaştırılır.
//
// Hepsi sayı (ve NaN olmayan) ise anahtarlar bir kez çıkarılır ve (anahtar, özgün sıra)
// çiftleri üzerinde sıralanır; bu, kararlı sıralamayla aynı sonucu çok daha hızlı verir.
// Diğer durumlarda genel karşılaştırıcıyla kararlı sıralama yapılır.
func siralaDegerler(d []Value) {
	type anahtarli struct {
		k float64
		i int
		v Value
	}
	cift := make([]anahtarli, len(d))
	hizli := true
	for i, x := range d {
		f, ok := ToFloat(x)
		if !ok || math.IsNaN(f) {
			hizli = false
			break
		}
		cift[i] = anahtarli{k: f, i: i, v: x}
	}
	if hizli {
		slices.SortFunc(cift, func(a, b anahtarli) int {
			switch {
			case a.k < b.k:
				return -1
			case a.k > b.k:
				return 1
			}
			return a.i - b.i
		})
		for i := range cift {
			d[i] = cift[i].v
		}
		return
	}
	sort.SliceStable(d, func(i, j int) bool {
		fi, ok1 := ToFloat(d[i])
		fj, ok2 := ToFloat(d[j])
		if ok1 && ok2 {
			return fi < fj
		}
		return d[i].Inspect() < d[j].Inspect()
	})
}
