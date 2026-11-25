package ml

import (
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Bağımsızlık ölçüsü: streamTests ardışık tohum, adım d, akışın bir konumu. Ölçütler: art arda aynı
// onda bir (σ birimiyle), (u[s], u[s+d]) çiftlerinin 10×10 hücreye ve ikinci farkın on parçaya
// dağılımının ki-karesi (z). Son ikisi komşu akışların sabit farklı olmasını da yakalar.
const streamTests = 20_000

// independence bir konumun çekilişlerinin ölçüleridir; u[s] tohum s'ninki, [0, 1).
func independence(d int, u []float64) (same, pairs, second float64) {
	decile := func(v float64) int { return min(int(v*10), 9) }
	var equal float64
	var cells [100]float64
	var parts [10]float64
	for s := range streamTests {
		a, b := decile(u[s]), decile(u[s+d])
		if a == b {
			equal++
		}
		cells[10*a+b]++
		x := u[s] - 2*u[s+d] + u[s+2*d]
		parts[decile(x-math.Floor(x))]++
	}
	z := func(counts []float64) float64 {
		want, chi := float64(streamTests)/float64(len(counts)), 0.0
		for _, c := range counts {
			chi += (c - want) * (c - want) / want
		}
		free := float64(len(counts) - 1)
		return (chi - free) / math.Sqrt(2*free)
	}
	return (equal - streamTests/10) / math.Sqrt(streamTests*0.1*0.9), z(cells[:]), z(parts[:])
}

// positions sınanan konumlardır: akışın 1., 2. ve 10. çekilişi.
var positions = []int{0, 1, 9}

// firstDraws n tohumun akışlarından her konumun düzgün çekilişleridir: konum başına bir dizi.
func firstDraws(n int, stream func(s int) *rand.Rand) [][]float64 {
	u := make([][]float64, len(positions))
	for s := range n {
		r := stream(s)
		for drawn, k := 0, 0; k < len(positions); drawn++ {
			if v := r.Float64(); drawn == positions[k] {
				u[k] = append(u[k], v)
				k++
			}
		}
	}
	return u
}

// Kural (15.2, 15.26): ayrı tohumların çekilişleri birbirinden istatistikçe bağımsızdır, ardışık
// tohumların da. Ölçü önce bağımlı olduğu bilinen Go kaynağında denenir: onu yakalamalı.
func TestSeededStreamsAreIndependent(t *testing.T) {
	steps := []int{1, 2, 100, 1000}
	draws := func(stream func(s int) *rand.Rand) [][]float64 { return firstDraws(streamTests+2*1000, stream) }
	for k, u := range draws(func(s int) *rand.Rand { return rand.New(rand.NewSource(int64(s))) }) {
		for _, d := range steps {
			if _, pairs, second := independence(d, u); max(pairs, second) < 100 {
				t.Errorf("ölçü bağımlılığı görmüyor: NewSource, adım %d, konum %d: çift z %.1f, ikinci fark z %.1f", d, positions[k], pairs, second)
			}
		}
	}
	for _, label := range []uint64{randnLabel, sampleLabel} {
		for k, u := range draws(func(s int) *rand.Rand { return seededRandom(int64(s), label) }) {
			for _, d := range steps {
				same, pairs, second := independence(d, u)
				if math.Abs(same) > 5 || pairs > 6 || second > 6 {
					t.Errorf("etiket %d, adım %d, konum %d: aynı onda bir %.2fσ, çift z %.2f, ikinci fark z %.2f", label, d, positions[k], same, pairs, second)
				}
			}
		}
	}
}

// Kural (15.26a): türetilen tohumların çekilişleri birbirinden ve tohumun kendi çekilişlerinden
// istatistikçe bağımsızdır. Tohum değişirken, i değişirken ve kökle türetilen yan yana.
func TestFoldedStreamsAreIndependent(t *testing.T) {
	fold := func(s, i int) int64 { return int64(mixSeed(foldLabel, uint64(s), uint64(i))) }
	for name, seed := range map[string]func(s int) int64{
		"i değişir":      func(s int) int64 { return fold(42, s) },
		"tohum değişir":  func(s int) int64 { return fold(s, 42) },
		"kök ve türemiş": func(s int) int64 { return [2]int64{int64(s / 2), fold(s/2, 0)}[s%2] },
	} {
		for k, u := range firstDraws(streamTests+2, func(s int) *rand.Rand { return seededRandom(seed(s), sampleLabel) }) {
			same, pairs, second := independence(1, u)
			if math.Abs(same) > 5 || pairs > 6 || second > 6 {
				t.Errorf("%s, konum %d: aynı onda bir %.2fσ, çift z %.2f, ikinci fark z %.2f", name, positions[k], same, pairs, second)
			}
		}
	}
}

// Kural (15.2): tohum bütün 64 bitiyle alınır. Go kaynağının aynı akışa indirgediği tohumlar
// (2³¹−1 fark; 0 ile 89482311) ayrıdır; tek biti değişen tohum da.
func TestSeedUsesAllBits(t *testing.T) {
	first := func(seed int64) uint64 { return seededRandom(seed, randnLabel).Uint64() }
	for _, p := range [][2]int64{{7, 7 + math.MaxInt32}, {0, 89482311}} {
		if rand.New(rand.NewSource(p[0])).Uint64() != rand.New(rand.NewSource(p[1])).Uint64() {
			t.Errorf("%v Go kaynağında aynı akış olmalıydı: sınama hiçbir şey sınamıyor", p)
		}
		if first(p[0]) == first(p[1]) {
			t.Errorf("%v aynı akış", p)
		}
	}
	seen := map[uint64]int64{}
	for _, seed := range []int64{0, 7, -1, math.MaxInt64, math.MinInt64} {
		for bit := range 64 {
			s := seed ^ int64(uint64(1)<<bit)
			if old, ok := seen[first(s)]; ok && old != s {
				t.Errorf("tohum %d ile %d aynı akış", old, s)
			}
			seen[first(s)] = s
		}
	}
}

// Kural (15.2): aynı tohumla TR_randn ve TR_sample'ın çekilişleri birbirinden bağımsızdır.
func TestSampleAndRandnStreamsDiffer(t *testing.T) {
	equal := func(a, b uint64) (n int) {
		for s := range int64(streamTests) {
			if int(seededRandom(s, a).Float64()*10) == int(seededRandom(s, b).Float64()*10) {
				n++
			}
		}
		return n
	}
	if n := equal(randnLabel, randnLabel); n != streamTests {
		t.Fatalf("aynı etiket %d/%d: sınama hiçbir şey sınamıyor", n, streamTests)
	}
	for _, p := range [][2]uint64{{randnLabel, sampleLabel}, {randnLabel, foldLabel}, {sampleLabel, foldLabel}} {
		if z := (float64(equal(p[0], p[1])) - streamTests/10) / math.Sqrt(streamTests*0.1*0.9); math.Abs(z) > 5 {
			t.Errorf("etiketler %v: aynı onda bir %.2fσ", p, z)
		}
	}
}

// Kural (15.26a): TR_fold_in'in değerleri aynı ikilide sabittir, her TR_Int'i alır, sıra önemlidir,
// türetilenler çakışmaz.
func TestFoldInValues(t *testing.T) {
	fold := func(seed, i int64) int64 {
		v, err := foldIn(nil, []interp.Value{interp.NewInt(seed), interp.NewInt(i)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return v.(*interp.IntVal).Val
	}
	for _, c := range []struct{ seed, i, want int64 }{
		{0, 0, -2082265775130654761}, {1, 2, -7692566387965527368}, {2, 1, -4591327595990705484},
		{42, 0, -2231403545515160292}, {-1, 0, 1790891247437588095}, {5, -1, -3390812972170164145},
		{math.MaxInt64, math.MinInt64, -1442377049919101988}, {math.MinInt64, math.MaxInt64, -6939500346091817302},
	} {
		if got := fold(c.seed, c.i); got != c.want {
			t.Errorf("TR_fold_in(%d, %d) = %d, beklenen %d", c.seed, c.i, got, c.want)
		}
	}
	seen := make(map[int64]bool, 1_000_000)
	for seed := range int64(1000) {
		for i := range int64(1000) {
			seen[fold(seed-500, i-500)] = true
		}
	}
	if len(seen) != 1_000_000 {
		t.Errorf("1000 tohum × 1000 i: %d ayrı değer", len(seen))
	}
}
