package gpu

import (
	"fmt"
	"math/rand"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// Büyük yollar: 16 MB üstü eşzamanlı yükleme, hazırlık halkasının dolup başa dönmesi,
// 65535'ten çok toplu iş (ızgara parçalaması) ve M, N ≥ 1024'te otomatik büyük döşeme.

func TestBuyukYuklemeVeHalka(t *testing.T) {
	g := gpuArkaUc()
	rng := rand.New(rand.NewSource(19))
	// Eşzamanlı yol: hazirlikEnCok'tan büyük.
	n := hazirlikEnCok/4 + 1000
	v := rastgele(rng, n, 1)
	s := g.FromHost(v, tensor.Float32)
	farkDenetle(t, "16 MB üstü yükleme", g.ToHost(tensor.View{S: s, Shape: []int{n}, Strides: []int{1}}), v, 0)
	// Halka: 4 MB'lık 20 yükleme 64 MB'lık halkayı en az bir kez başa döndürür; eşzamansız
	// kopyalar bitmeden halkanın üzerine yazılsaydı içerik bozulurdu.
	m := 1 << 20
	var veriler [][]float32
	var depolar []tensor.Storage
	for i := 0; i < 20; i++ {
		veriler = append(veriler, rastgele(rng, m, 1))
		depolar = append(depolar, g.FromHost(veriler[i], tensor.Float32))
	}
	for i, d := range depolar {
		farkDenetle(t, fmt.Sprintf("halka yüklemesi %d", i), cihazdan(g, d), veriler[i], 0)
	}
}

func TestCokTopluCarpim(t *testing.T) {
	g := gpuArkaUc()
	if benzetimde() {
		// Benzetim yavaştır: iş parçacığı başına bir elemanlık döşemeyle koşar.
		g.carpimDosemesi(&carpimAyari{1, 1, 1, 1, 1})
		defer g.carpimDosemesi(nil)
	}
	rng := rand.New(rand.NewSource(20))
	b := 65535 + 3
	av, bv := rastgele(rng, b*2, 1), rastgele(rng, b*2, 1)
	karsilastir(t, fmt.Sprintf("%d toplu çarpım", b), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x, y := ikiz(av, true, b, 1, 2)[c], ikiz(bv, true, b, 2, 1)[c]
		return tensor.MatMul(x, y), []*tensor.Tensor{x, y}
	})
}

func TestBuyukDosemeOtomatik(t *testing.T) {
	g := gpuArkaUc()
	// Büyük döşeme, blok sayısı SM sayısını doldurduğunda seçilir: boy SM sayısından
	// hesaplanır (188 SM'li bir kartta 1024×1024 yetmez).
	_, sm := g.sr.yetenek()
	M := 1024
	for int(bloklar(M, buyukCarpim.BM))*int(bloklar(M, buyukCarpim.BN)) < max(sm, 1) {
		M += 1024
	}
	if g.carpimSec(1, M, M) != buyukCarpim || g.carpimSec(1, 200, 4096) != varsayilanCarpim {
		t.Fatalf("döşeme seçimi beklenen değil (%d SM, M=N=%d)", sm, M)
	}
	// Kitaplık bulunduysa çarpımı o yapardı: kendi büyük döşememiz sınanmak için kapatılır.
	lt := g.lt
	g.lt = nil
	defer func() { g.lt = lt }()
	rng := rand.New(rand.NewSource(21))
	K, N := 3, M+6
	av, bv := rastgele(rng, M*K, 1), rastgele(rng, K*N, 1)
	karsilastir(t, "büyük döşemeli çarpım", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		return tensor.MatMul(ikiz(av, false, M, K)[c], ikiz(bv, false, K, N)[c]), nil
	})
}
