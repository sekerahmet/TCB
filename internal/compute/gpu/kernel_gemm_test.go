package gpu

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Büyük yollar: 16 MB üstü eşzamanlı yükleme, hazırlık halkasının dolup başa dönmesi,
// 65535'ten çok toplu iş (ızgara parçalaması) ve M, N ≥ 1024'te otomatik büyük döşeme.

func TestLargeLoadAndRing(t *testing.T) {
	g := gpuBackend()
	rng := rand.New(rand.NewSource(19))
	// Eşzamanlı yol: hazirlikEnCok'tan büyük.
	n := setupMax/4 + 1000
	v := random(rng, n, 1)
	s := g.FromHost(v, tensor.Float32)
	checkDiff(t, "16 MB üstü yükleme", g.ToHost(tensor.View{S: s, Shape: []int{n}, Strides: []int{1}}), v, 0)
	// Halka: 4 MB'lık 20 yükleme 64 MB'lık halkayı en az bir kez başa döndürür; eşzamansız
	// kopyalar bitmeden halkanın üzerine yazılsaydı içerik bozulurdu.
	m := 1 << 20
	var datas [][]float32
	var storages []tensor.Storage
	for i := 0; i < 20; i++ {
		datas = append(datas, random(rng, m, 1))
		storages = append(storages, g.FromHost(datas[i], tensor.Float32))
	}
	for i, d := range storages {
		checkDiff(t, fmt.Sprintf("halka yüklemesi %d", i), fromDevice(g, d), datas[i], 0)
	}
}

func TestMultiBatchGemm(t *testing.T) {
	g := gpuBackend()
	if inSim() {
		// Benzetim yavaştır: iş parçacığı başına bir elemanlık döşemeyle koşar.
		g.gemmTile(&gemmConfig{1, 1, 1, 1, 1})
		defer g.gemmTile(nil)
	}
	rng := rand.New(rand.NewSource(20))
	b := 65535 + 3
	av, bv := random(rng, b*2, 1), random(rng, b*2, 1)
	compare(t, fmt.Sprintf("%d toplu çarpım", b), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x, y := twin(av, true, b, 1, 2)[c], twin(bv, true, b, 2, 1)[c]
		return tensor.MatMul(x, y), []*tensor.Tensor{x, y}
	})
}

func TestLargeTileAuto(t *testing.T) {
	g := gpuBackend()
	// Büyük döşeme, blok sayısı SM sayısını doldurduğunda seçilir: boy SM sayısından
	// hesaplanır (188 SM'li bir kartta 1024×1024 yetmez).
	_, sm := g.sr.capability()
	M := 1024
	for int(blocks(M, largeGemm.BM))*int(blocks(M, largeGemm.BN)) < max(sm, 1) {
		M += 1024
	}
	if g.pickGemm(1, M, M) != largeGemm || g.pickGemm(1, 200, 4096) != defaultGemm {
		t.Fatalf("döşeme seçimi beklenen değil (%d SM, M=N=%d)", sm, M)
	}
	// Kitaplık bulunduysa çarpımı o yapardı: kendi büyük döşememiz sınanmak için kapatılır.
	lt := g.lt
	g.lt = nil
	defer func() { g.lt = lt }()
	rng := rand.New(rand.NewSource(21))
	K, N := 3, M+6
	av, bv := random(rng, M*K, 1), random(rng, K*N, 1)
	compare(t, "büyük döşemeli çarpım", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		return tensor.MatMul(twin(av, false, M, K)[c], twin(bv, false, K, N)[c]), nil
	})
}
