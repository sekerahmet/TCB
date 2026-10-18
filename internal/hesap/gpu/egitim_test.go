package gpu

import (
	"fmt"
	"math/rand"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// Konak gölgesi, iyileştiriciler ve küçük bir dönüştürücü modelin uçtan uca eğitimi.

// TestGolgeHerYazmadaGecersizlesir konaktan yüklenmiş (gölgeli) bir depolamaya her yazma
// yolundan sonra okunan değerin cihazdaki değer olduğunu denetler.
func TestGolgeHerYazmadaGecersizlesir(t *testing.T) {
	g := gpuArkaUc()
	rng := rand.New(rand.NewSource(13))
	n := 12
	yeni := func() tensor.Storage { return g.FromHost(rastgele(rng, n, 1), tensor.Float32) }
	duz := func(s tensor.Storage) tensor.View { return tensor.View{S: s, Shape: []int{n}, Strides: []int{1}} }
	yollar := map[string]func(s tensor.Storage){
		"AddTo":      func(s tensor.Storage) { g.AddTo(s, yeni(), 2) },
		"ScatterAdd": func(s tensor.Storage) { g.ScatterAdd(s, 0, []int{1}, yeni(), []int{n}) },
		"ExpandAdd": func(s tensor.Storage) {
			g.ExpandAdd(s, []int{n}, g.FromHost([]float32{3}, tensor.Float32), []int{1}, 1)
		},
		"Copy":        func(s tensor.Storage) { g.Copy(s, 0, duz(yeni())) },
		"CopyStrided": func(s tensor.Storage) { g.CopyStrided(duz(s), duz(yeni())) },
		"Fill":        func(s tensor.Storage) { g.Fill(s, 4) },
		"Binary":      func(s tensor.Storage) { g.Binary(tensor.OpAdd, s, []int{n}, duz(yeni()), duz(yeni())) },
		"BinaryGrad": func(s tensor.Storage) {
			g.BinaryGrad(tensor.GradMul, s, []int{n}, []int{n}, yeni(), duz(yeni()), yeni())
		},
		"Unary":     func(s tensor.Storage) { g.Unary(tensor.OpExp, 0, s, duz(yeni())) },
		"UnaryGrad": func(s tensor.Storage) { g.UnaryGrad(tensor.OpTanh, 0, s, yeni(), duz(yeni()), yeni()) },
		"Sum": func(s tensor.Storage) {
			g.Sum(s, tensor.View{S: yeni(), Shape: []int{1, n}, Strides: []int{n, 1}}, 0, 1)
		},
		"Softmax":    func(s tensor.Storage) { g.Softmax(s, duz(yeni()), 0, false) },
		"Gemm":       func(s tensor.Storage) { g.Gemm(1, 3, 4, 2, duz(yeni()), false, duz(yeni()), false, s) },
		"Embedding":  func(s tensor.Storage) { g.Embedding(s, duz(yeni()), []int{1, 0}, 6) },
		"EmbeddingG": func(s tensor.Storage) { g.EmbeddingGrad(s, yeni(), []int{1, 0}, 6) },
		"SGDStep":    func(s tensor.Storage) { g.SGDStep(s, yeni(), nil, 0.1, 0, 0.01) },
		"AdamWStep": func(s tensor.Storage) {
			g.AdamWStep(s, yeni(), yeni(), g.FromHost(make([]float32, n), tensor.Float32), 0.1, 0.9, 0.99, 1e-8, 0.1, 0.1, 0.01)
		},
		"Scale":   func(s tensor.Storage) { g.Scale(s, 3) },
		"RMSNorm": func(s tensor.Storage) { g.RMSNorm(s, duz(yeni()), nil, 6, 1e-5) },
	}
	for ad, yaz := range yollar {
		s := yeni()
		if g.depoOf(s).konak == nil {
			t.Fatalf("%s: küçük konak verisi gölgeli olmalıydı", ad)
		}
		yaz(s)
		farkDenetle(t, ad, g.ToHost(duz(s)), cihazdan(g, s), 0)
	}
	// Tensör katmanındaki yerinde değişiklikler (Set, CopyFrom, iyileştirici).
	x := tensor.NewOn(gpuCihaz, rastgele(rng, n, 1), n)
	x.Set(9, 3)
	x.CopyFrom(tensor.NewOn(gpuCihaz, []float32{5}))
	if d := x.Data(); d[3] != 5 || d[0] != 5 {
		t.Fatalf("CopyFrom sonrası okunan değer eski: %v", d)
	}
}

func TestIyilestiriciler(t *testing.T) {
	rng := rand.New(rand.NewSource(14))
	pv, hv := rastgele(rng, 300, 1), rastgele(rng, 300, 1)
	for _, ad := range []string{"adamw", "sgd", "sgd momentumlu"} {
		var sonuc [2][]float32
		var normlar [2]float64
		for c := 0; c < 2; c++ {
			p := ikiz(pv, true, 300)[c]
			h := ikiz(hv, false, 300)[c]
			adim := func() {}
			switch ad {
			case "adamw":
				o := tensor.NewAdamW([]*tensor.Tensor{p}, 0.01, 0.1)
				adim = o.Step
			case "sgd":
				o := tensor.NewSGD([]*tensor.Tensor{p}, 0.1, 0)
				adim = o.Step
			default:
				o := tensor.NewSGD([]*tensor.Tensor{p}, 0.1, 0.9)
				adim = o.Step
			}
			for i := 0; i < 3; i++ {
				p.ZeroGrad()
				tensor.Mul(p, h).Tanh().SumAll().Backward()
				normlar[c] = tensor.ClipGradNorm([]*tensor.Tensor{p}, 0.5)
				adim()
			}
			sonuc[c] = p.Data()
		}
		farkDenetle(t, ad, sonuc[1], sonuc[0], 1e-5)
		if d := normlar[1] - normlar[0]; d > 1e-5*normlar[0] || d < -1e-5*normlar[0] {
			t.Fatalf("%s: gradyan normu GPU %v, CPU %v", ad, normlar[1], normlar[0])
		}
	}
}

// kucukModel kıyas modelinin küçültülmüşü: gömme, RMS normu, nedensel çok başlı dikkat,
// SwiGLU, paylaşımlı çıkış, çapraz entropi.
type kucukModel struct {
	E, g1, wq, wk, wv, wo, wg, wu, w2, gs *tensor.Tensor
}

const (
	kmB, kmT, kmD, kmH, kmFF, kmV = 2, 6, 8, 2, 12, 20
)

func yeniKucukModel(dev tensor.Device, seed int64, dt tensor.DType) *kucukModel {
	rng := rand.New(rand.NewSource(seed))
	w := func(olcek float64, sh ...int) *tensor.Tensor {
		return yaprakTurde(tensor.NewOn(dev, rastgele(rng, numel(sh), olcek), sh...), dt)
	}
	bir := func(n int) *tensor.Tensor {
		v := make([]float32, n)
		for i := range v {
			v[i] = 1
		}
		return yaprakTurde(tensor.NewOn(dev, v, n), dt)
	}
	return &kucukModel{E: w(0.5, kmV, kmD), g1: bir(kmD), wq: w(0.3, kmD, kmD), wk: w(0.3, kmD, kmD), wv: w(0.3, kmD, kmD),
		wo: w(0.3, kmD, kmD), wg: w(0.3, kmD, kmFF), wu: w(0.3, kmD, kmFF), w2: w(0.3, kmFF, kmD), gs: bir(kmD)}
}

func (m *kucukModel) parametreler() []*tensor.Tensor {
	return []*tensor.Tensor{m.E, m.g1, m.wq, m.wk, m.wv, m.wo, m.wg, m.wu, m.w2, m.gs}
}

func (m *kucukModel) kayip(girdi, hedef []int) *tensor.Tensor {
	x := tensor.Embedding(m.E, girdi, kmB, kmT)
	h := tensor.RMSNorm(x, m.g1, 1e-5)
	bas := func(t *tensor.Tensor) *tensor.Tensor {
		return t.Reshape(kmB, kmT, kmH, kmD/kmH).Permute(0, 2, 1, 3)
	}
	y := tensor.Attention(bas(tensor.MatMul(h, m.wq)), bas(tensor.MatMul(h, m.wk)), bas(tensor.MatMul(h, m.wv)), tensor.AttentionOptions{Causal: true})
	s := tensor.Add(x, tensor.MatMul(y.Permute(0, 2, 1, 3).Reshape(kmB, kmT, kmD), m.wo))
	a := tensor.MatMul(s, m.wg)
	f := tensor.Mul(tensor.Mul(a, a.Sigmoid()), tensor.MatMul(s, m.wu))
	s = tensor.Add(s, tensor.MatMul(f, m.w2))
	cikti := tensor.MatMul(tensor.RMSNorm(s, m.gs, 1e-5), m.E.T())
	return tensor.CrossEntropy(cikti.Reshape(kmB*kmT, kmV), hedef)
}

// yaprakTurde tensörü dt türünde türev izleyen bir yaprağa çevirir.
func yaprakTurde(t *tensor.Tensor, dt tensor.DType) *tensor.Tensor {
	if dt != tensor.Float32 {
		t = t.Cast(dt).Detach()
	}
	return t.SetRequiresGrad(true)
}

// TestKucukModelEgitimi küçük dönüştürücüyü float32 ve bfloat16 ağırlıklarla (ana kopyalı
// AdamW) CPU ve GPU'da aynı veriyle eğitir ve kayıpları karşılaştırır. bf16 yolunda dikkatin
// geri yayılımı 16 bitlik çıktıyı açar (iç içe başlatma hatasının sınaması).
func TestKucukModelEgitimi(t *testing.T) {
	for _, d := range []struct {
		dt  tensor.DType
		tol float64
	}{{tensor.Float32, 1e-4}, {tensor.BFloat16, 3e-2}} {
		kucukModelEgit(t, d.dt, d.tol)
	}
}

func kucukModelEgit(t *testing.T, dt tensor.DType, tol float64) {
	rng := rand.New(rand.NewSource(15))
	adimlar := 4
	girdiler := make([][2][]int, adimlar)
	for i := range girdiler {
		for j := 0; j < 2; j++ {
			girdiler[i][j] = make([]int, kmB*kmT)
			for k := range girdiler[i][j] {
				girdiler[i][j][k] = rng.Intn(kmV)
			}
		}
	}
	var kayiplar [2][]float32
	var son [2][]float32
	for c, dev := range []tensor.Device{tensor.CPU, gpuCihaz} {
		m := yeniKucukModel(dev, 16, dt)
		o := tensor.NewAdamW(m.parametreler(), 0.01, 0.0)
		for i := 0; i < adimlar; i++ {
			o.ZeroGrad()
			l := m.kayip(girdiler[i][0], girdiler[i][1])
			l.Backward()
			tensor.ClipGradNorm(m.parametreler(), 1.0)
			o.Step()
			kayiplar[c] = append(kayiplar[c], l.Item())
		}
		son[c] = m.wq.Data()
	}
	farkDenetle(t, dt.String()+" kayıplar", kayiplar[1], kayiplar[0], tol)
	farkDenetle(t, dt.String()+" son ağırlıklar", son[1], son[0], tol)
	if kayiplar[1][adimlar-1] >= kayiplar[1][0] {
		t.Fatalf("%s: kayıp düşmedi: %v", dt, kayiplar[1])
	}
	t.Log(fmt.Sprintf("%s kayıplar GPU %v, CPU %v", dt, kayiplar[1], kayiplar[0]))
}
