package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// İndirgemeler: eksen toplamı ve ortalaması, hepsinin toplamı, en büyük değer ve
// indeksi; boş eksen, tek eleman, uzun ve kısa eksenler (iki çekirdek biçimi de).

func TestSums(t *testing.T) {
	skipUnderRace(t)
	rng := rand.New(rand.NewSource(5))
	// {3000, 5}, {9000}, {2100, 300}: uzun indirgenen eksen, az çıktı (iki aşamalı indirgeme).
	shapes := [][]int{{1}, {7}, {3, 5}, {4, 300}, {300, 4}, {2, 3, 70}, {5, 0, 3}, {2, 1031}, {1031, 3}, {600, 260},
		{3000, 5}, {9000}, {2100, 300}}
	for _, sh := range shapes {
		v := random(rng, tensor.NumelOf(sh), 1)
		tol := 1e-5
		if tensor.NumelOf(sh) > 5000 {
			tol = 1e-4 // binlerce float32 terimin toplamı (CPU float64 biriktirir)
		}
		for ax := range sh {
			for _, avg := range []bool{false, true} {
				if avg && sh[ax] == 0 {
					continue // boş eksenin ortalaması tanımsız (açık hata)
				}
				compare(t, fmt.Sprintf("%v eksen %d TR_mean=%v", sh, ax, avg), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := twin(v, true, sh...)[c]
					if avg {
						return x.Mean(ax, true), []*tensor.Tensor{x}
					}
					return x.Sum(ax, false), []*tensor.Tensor{x}
				})
			}
		}
		compare(t, fmt.Sprintf("%v hepsi", sh), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, true, sh...)[c]
			return x.SumAll(), []*tensor.Tensor{x}
		})
		compare(t, fmt.Sprintf("%v adımlı hepsi", sh), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, true, sh...)[c]
			if len(sh) < 2 {
				return x.MeanAll(), []*tensor.Tensor{x}
			}
			return x.Transpose(0, 1).SumAll(), []*tensor.Tensor{x}
		})
	}
}

// TestBiasGrad yayınlanan toplamanın (sapma) gradyanını uzun satır ekseninde denetler
// (iki aşamalı sütun indirgemesi).
func TestBiasGrad(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	xv, bv := random(rng, 2500*40, 1), random(rng, 40, 1)
	compare(t, "sapma gradyanı", 1e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x, b := twin(xv, true, 2500, 40)[c], twin(bv, true, 40)[c]
		return tensor.Mul(tensor.Add(x, b), b), []*tensor.Tensor{x, b}
	})
}

// TestDeterministic aynı girdinin her çalıştırmada bit düzeyinde aynı sonucu verdiğini
// denetler (indirgemeler, matris çarpımı, gradyanlar; atomik toplama yok).
func TestDeterministic(t *testing.T) {
	skipUnderRace(t)
	rng := rand.New(rand.NewSource(17))
	xv, wv := random(rng, 300*70, 1), random(rng, 70*90, 1)
	idx := make([]int, 300)
	for i := range idx {
		idx[i] = rng.Intn(5)
	}
	var prev [][]float32
	for repeat := 0; repeat < 3; repeat++ {
		x := tensor.NewOn(gpuDevice, xv, 300, 70).SetRequiresGrad(true)
		w := tensor.NewOn(gpuDevice, wv, 70, 90).SetRequiresGrad(true)
		y := tensor.MatMul(x, w)
		k := tensor.Add(y.Softmax(1).Sum(0, false).SumAll(), tensor.Embedding(w, idx).SumAll())
		k.Backward()
		now := [][]float32{k.Data(), x.Grad().Data(), w.Grad().Data()}
		if prev != nil {
			for i := range now {
				checkDiff(t, fmt.Sprintf("tekrar %d, çıktı %d", repeat, i), now[i], prev[i], 0)
			}
		}
		prev = now
	}
}

func TestMax(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	nan := float32(math.NaN())
	for _, sh := range [][]int{{1}, {9}, {4, 6}, {3, 4, 5}, {2, 300}} {
		v := random(rng, tensor.NumelOf(sh), 1)
		// Eşit en büyükler: ilk indeks kazanmalı.
		v[0] = 5
		if len(v) > 2 {
			v[2] = 5
		}
		for ax := range sh {
			compare(t, fmt.Sprintf("en büyük %v eksen %d", sh, ax), 0, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x := twin(v, true, sh...)[c]
				return x.Max(ax, false), []*tensor.Tensor{x}
			})
			gi := tensor.NewOn(gpuDevice, v, sh...).ArgMax(ax)
			ci := tensor.NewOn(tensor.CPU, v, sh...).ArgMax(ax)
			if fmt.Sprint(gi) != fmt.Sprint(ci) {
				t.Fatalf("en büyük indeks %v eksen %d: GPU %v, CPU %v", sh, ax, gi, ci)
			}
		}
	}
	// NaN en büyük sayılır, ilk NaN kazanır.
	v := []float32{1, nan, 3, nan, 2, 7}
	gi := tensor.NewOn(gpuDevice, v, 2, 3).ArgMax(1)
	ci := tensor.NewOn(tensor.CPU, v, 2, 3).ArgMax(1)
	if fmt.Sprint(gi) != fmt.Sprint(ci) {
		t.Fatalf("NaN'lı en büyük indeks: GPU %v, CPU %v", gi, ci)
	}
	// −0.0 0.0'dan küçüktür (15.22b): sıradan bağımsız +0 seçilir; sıralamada −0 önce gelir.
	n0 := float32(math.Copysign(0, -1))
	z := tensor.NewOn(gpuDevice, []float32{n0, 0, 0, n0, n0, n0}, 3, 2)
	if got := fmt.Sprint(z.ArgMax(1)); got != "[1 0 0]" {
		t.Fatalf("GPU'da sıfırların en büyük indeksi %s, beklenen [1 0 0]", got)
	}
	for i, v := range z.Max(1, false).Data() {
		if want := []bool{false, false, true}[i]; math.Signbit(float64(v)) != want {
			t.Fatalf("GPU'da sıfırların en büyüğü %v (%d. satır)", z.Max(1, false).Data(), i)
		}
	}
	// En küçük, ml'nin TR_min'i gibi −max(−x) ile kurulur: her satırda −0.0 seçilir.
	for i, v := range z.Neg().Max(1, false).Neg().Data() {
		if v != 0 || !math.Signbit(float64(v)) {
			t.Fatalf("GPU'da sıfırların en küçüğü %v (%d. satır), beklenen -0", z.Neg().Max(1, false).Neg().Data(), i)
		}
	}
	s := tensor.NewOn(gpuDevice, []float32{0, n0, 0, n0}, 4)
	if got := fmt.Sprint(s.ArgSort(0).Data()); got != "[1 3 0 2]" {
		t.Fatalf("GPU'da sıfırların sıralaması %s, beklenen [1 3 0 2]", got)
	}
	if _, ti := s.TopK(2, 0); fmt.Sprint(ti.Data()) != "[0 2]" {
		t.Fatalf("GPU'da sıfırların en büyük 2'si %v, beklenen [0 2]", ti.Data())
	}
}

// argmaxRun x'in ax eksenindeki en büyük indeksini, değerinin bitlerini ve gradyanının
// bitlerini verir (geri yayılımın tohumu sabit rastgele ağırlıklar).
func argmaxRun(dev tensor.Device, v []float32, sh []int, ax int) (string, []uint32, []uint32) {
	x := tensor.NewOn(dev, v, sh...).SetRequiresGrad(true)
	idx := fmt.Sprint(x.ArgMax(ax))
	m := x.Max(ax, false)
	w := tensor.NewOn(dev, random(rand.New(rand.NewSource(99)), m.Numel(), 1), m.Shape()...)
	tensor.Mul(m, w).SumAll().Backward()
	bitsOf := func(f []float32) []uint32 {
		b := make([]uint32, len(f))
		for i, e := range f {
			b[i] = math.Float32bits(e)
		}
		return b
	}
	return idx, bitsOf(m.Data()), bitsOf(x.Grad().Data())
}

// TestArgMaxBlockMatchesCPU az çıktılı uzun eksenin blok yolunu (argmaxBlockKernel)
// CPU'yla ve aynı kartta eski yolla (çıktı başına iş parçacığı, argmaxBlockMaxOut = 0) bit bit
// karşılaştırır: en büyük indeks (TR_argmax), değer (TR_max, NaN'ın yükü dahil) ve gradyanı.
// K = 1 ve uzun eksen (Whisper'ın sözlüğü), büyük K ve kısa eksen, adımlı eksen; rastgele,
// eşitler, ayrı yüklü NaN'lar, ±0 ve hepsi −∞.
func TestArgMaxBlockMatchesCPU(t *testing.T) {
	skipUnderRace(t)
	rng := rand.New(rand.NewSource(21))
	nanA, nanB := math.Float32frombits(0x7FC00001), math.Float32frombits(0xFFC00002)
	n0, ninf := float32(math.Copysign(0, -1)), float32(math.Inf(-1))
	cases := []struct {
		ad   string
		sh   []int
		fill func(v []float32)
	}{
		{"rastgele", []int{3000}, nil},
		{"eşitler", []int{2, 700}, func(v []float32) {
			for i := range v {
				v[i] = float32(rng.Intn(3))
			}
		}},
		{"NaN'lar", []int{3, 500}, func(v []float32) { v[37], v[400], v[520], v[900], v[1499] = nanB, nanA, nanA, nanB, nanA }},
		{"sıfırlar", []int{4, 300}, func(v []float32) {
			for i := range v {
				v[i] = n0
			}
			v[350], v[351], v[999] = 0, 0, 0
		}},
		{"hepsi −∞", []int{2, 256}, func(v []float32) {
			for i := range v {
				v[i] = ninf
			}
		}},
		{"K = 1, uzun eksen", []int{51865}, nil},
		{"K büyük, kısa eksen", []int{1200, 80}, nil},
		{"adımlı eksen", []int{700, 3}, nil},
	}
	for _, c := range cases {
		v := random(rng, tensor.NumelOf(c.sh), 1)
		if c.fill != nil {
			c.fill(v)
		}
		for ax := range c.sh {
			ad := fmt.Sprintf("%s %v eksen %d", c.ad, c.sh, ax)
			ci, cv, cg := argmaxRun(tensor.CPU, v, c.sh, ax)
			gi, gv, gg := argmaxRun(gpuDevice, v, c.sh, ax)
			old := argmaxBlockMaxOut
			argmaxBlockMaxOut = 0
			oi, ov, og := argmaxRun(gpuDevice, v, c.sh, ax)
			argmaxBlockMaxOut = old
			if gi != ci || gi != oi {
				t.Fatalf("%s: en büyük indeks GPU %s, eski yol %s, CPU %s", ad, gi, oi, ci)
			}
			for _, s := range []struct {
				ad         string
				g, eski, c []uint32
			}{{"değer", gv, ov, cv}, {"gradyan", gg, og, cg}} {
				if fmt.Sprint(s.g) != fmt.Sprint(s.eski) {
					t.Fatalf("%s: %s bitleri blok yolunda %x, eski yolda %x", ad, s.ad, s.g, s.eski)
				}
				if fmt.Sprint(s.g) != fmt.Sprint(s.c) {
					t.Fatalf("%s: %s bitleri GPU'da %x, CPU'da %x", ad, s.ad, s.g, s.c)
				}
			}
		}
	}
	// Blok yolu üç kipte de gerçekten seçildi: çekirdekleri bu koşuda derlendi.
	built := map[int]bool{}
	ptxSeen.Lock()
	for key := range ptxSeen.m {
		var nk, mode2, B int
		if n, _ := fmt.Sscanf(key, "argmax_blok:%d:%d:%d", &nk, &mode2, &B); n == 3 {
			built[mode2] = true
		}
	}
	ptxSeen.Unlock()
	if !built[0] || !built[1] || !built[2] {
		t.Fatalf("blok yolu her kipte seçilmedi: derlenen kipler %v", built)
	}
}

// TestArgMaxBlockThreshold argmax'ın blok yolunun eşiğini (argmaxBlockMaxOut, R4 A2) ölçer:
// K × eksen ızgarasında ([K, L], son eksen) eski yol (çıktı başına iş parçacığı) ve blok yolu
// aynı koşuda, sırası tekrardan tekrara dönüşümlü; her nokta 5 tekrar, tekrar 20 çağrının
// ortalaması (microMeasure); ortanca ve yayılım (en küçük, en büyük). 1 GiB'ı aşan girdi
// atlanır ve yazılır. Eşik: her eksende blok yolunun eskiden yavaş olmadığı en büyük K; o K'da
// bir eksende eşitse bir alttaki K. Yalnızca gerçek kartta ve TR_GPU_ARGMAX_ESIK=1 ile koşar.
func TestArgMaxBlockThreshold(t *testing.T) {
	if os.Getenv("TR_GPU_ARGMAX_ESIK") != "1" || inSim() {
		t.Skip("yalnızca gerçek GPU'da ve TR_GPU_ARGMAX_ESIK=1 ile")
	}
	argmaxThreshold([]int{1, 4, 16, 64, 256, 1024, 4096, 16384, 65536}, []int{256, 2048, 16384, 51865})
}

// argmaxThreshold TestArgMaxBlockThreshold'un ölçüsü: Ks × Ls ızgarasında satırları ve eşiği
// yazar, eşiği verir.
func argmaxThreshold(Ks, Ls []int) int {
	g := gpuBackend()
	rng := rand.New(rand.NewSource(3))
	keep := argmaxBlockMaxOut
	defer func() { argmaxBlockMaxOut = keep }()
	verdict := map[int]int{} // K → 1 her eksende hızlı, 0 bir eksende eşit, -1 bir eksende yavaş
	for _, K := range Ks {
		verdict[K] = 1
		for _, L := range Ls {
			if K*L > 1<<28 {
				fmt.Printf("ESIK|%d|%d|atlandı: girdi %d MiB > 1 GiB\n", K, L, K*L*4>>20)
				continue
			}
			x := randomStorage(g, rng, K*L, 1)
			out := g.AllocUninit(K, tensor.Float32)
			xv := tensor.View{S: x, Shape: []int{K, L}, Strides: []int{L, 1}}
			var ms [2][]float64
			for r := 0; r < 5; r++ {
				for i := 0; i < 2; i++ {
					p := (i + r) % 2 // 0 eski, 1 blok; sıra dönüşümlü
					argmaxBlockMaxOut = []int{0, math.MaxInt}[p]
					ms[p] = append(ms[p], microMeasure(g, 20, func() { g.Max(out, xv, 1) }))
				}
			}
			for p := range ms {
				sort.Float64s(ms[p])
			}
			oldM, newM := ms[0][2], ms[1][2]
			fmt.Printf("ESIK|%d|%d|eski %.4f ms [%.4f, %.4f]|blok %.4f ms [%.4f, %.4f]|oran %.2f\n", K, L,
				oldM, ms[0][0], ms[0][4], newM, ms[1][0], ms[1][4], oldM/newM)
			switch {
			case newM > oldM:
				verdict[K] = -1
			case newM == oldM && verdict[K] == 1:
				verdict[K] = 0
			}
			g.Release(x)
			g.Release(out)
		}
	}
	threshold := 0
	for _, K := range Ks {
		if verdict[K] < 0 {
			break
		}
		if verdict[K] == 0 {
			break // sınırda eşit: eski yol
		}
		threshold = K
	}
	fmt.Printf("ESIK|sonuç|argmaxBlockMaxOut = %d\n", threshold)
	return threshold
}
