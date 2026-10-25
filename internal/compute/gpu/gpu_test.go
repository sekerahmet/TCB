package gpu

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// TestPickGCThreshold eşiğin kart belleğine göre seçildiğini ve ortam değişkeniyle
// değiştirilebildiğini denetler.
func TestPickGCThreshold(t *testing.T) {
	for _, d := range []struct {
		total     uint64
		env       string
		threshold uint64
	}{
		{96 << 30, "", 1536 << 20},   // 1/64
		{24 << 30, "", 384 << 20},    // 1/64
		{4 << 30, "", 256 << 20},     // en az 256 MB
		{1 << 40, "", 2 << 30},       // en çok 2 GB
		{96 << 30, "512", 512 << 20}, // ortam
		{96 << 30, "0", 0},           // kapalı
		{96 << 30, "x", 1536 << 20},  // geçersiz ortam yok sayılır
	} {
		if e := pickGCThreshold(d.total, d.env); e != d.threshold {
			t.Errorf("toplam %d, ortam %q: eşik %d, beklenen %d", d.total, d.env, e, d.threshold)
		}
	}
}

// TestGCThresholdStartsCollection eşiği aşan ayırmanın çöp toplamayı başlattığını ve
// ulaşılamaz depolamaların havuza döndüğünü denetler (benzetim: 1 MB eşik).
func TestGCThresholdStartsCollection(t *testing.T) {
	g := newBackend(tensor.GPU(7), newSim(64<<20))
	g.gcThreshold = 1 << 20
	before := g.pool.measurement().GCRuns
	for i := 0; i < 16; i++ {
		g.Alloc(128<<10, tensor.Float32) // 512 KB, hemen ulaşılamaz
	}
	o := g.pool.measurement()
	if o.GCRuns-before < 4 {
		t.Fatalf("16 × 512 KB ayırmada %d çöp toplama, en az 4 bekleniyordu", o.GCRuns-before)
	}
	if o.Reserved > 8<<20 {
		t.Fatalf("havuz şişti: %d bayt ayrılmış (ulaşılamaz depolamalar geri dönmeli)", o.Reserved)
	}
}

// Konak gölgesi, iyileştiriciler ve küçük bir dönüştürücü modelin uçtan uca eğitimi.

// TestShadowInvalidatedOnWrite konaktan yüklenmiş (gölgeli) bir depolamaya her yazma
// yolundan sonra okunan değerin cihazdaki değer olduğunu denetler.
func TestShadowInvalidatedOnWrite(t *testing.T) {
	g := gpuBackend()
	rng := rand.New(rand.NewSource(13))
	n := 12
	fresh2 := func() tensor.Storage { return g.FromHost(random(rng, n, 1), tensor.Float32) }
	plain := func(s tensor.Storage) tensor.View { return flatView(s, n) }
	paths := map[string]func(s tensor.Storage){
		"AddTo":      func(s tensor.Storage) { g.AddTo(s, fresh2(), 2) },
		"ScatterAdd": func(s tensor.Storage) { g.ScatterAdd(s, 0, []int{1}, fresh2(), []int{n}) },
		"ExpandAdd": func(s tensor.Storage) {
			g.ExpandAdd(s, []int{n}, g.FromHost([]float32{3}, tensor.Float32), []int{1}, 1)
		},
		"Copy":        func(s tensor.Storage) { g.Copy(s, 0, plain(fresh2())) },
		"CopyStrided": func(s tensor.Storage) { g.CopyStrided(plain(s), plain(fresh2())) },
		"Fill":        func(s tensor.Storage) { g.Fill(s, 4) },
		"Binary":      func(s tensor.Storage) { g.Binary(tensor.OpAdd, s, []int{n}, plain(fresh2()), plain(fresh2())) },
		"BinaryGrad": func(s tensor.Storage) {
			g.BinaryGrad(tensor.GradMul, s, []int{n}, []int{n}, fresh2(), plain(fresh2()), fresh2())
		},
		"Unary":     func(s tensor.Storage) { g.Unary(tensor.OpExp, 0, s, plain(fresh2())) },
		"UnaryGrad": func(s tensor.Storage) { g.UnaryGrad(tensor.OpTanh, 0, s, fresh2(), plain(fresh2()), fresh2()) },
		"Sum": func(s tensor.Storage) {
			g.Sum(s, tensor.View{S: fresh2(), Shape: []int{1, n}, Strides: []int{n, 1}}, 0, 1)
		},
		"Softmax":    func(s tensor.Storage) { g.Softmax(s, plain(fresh2()), 0, false) },
		"Gemm":       func(s tensor.Storage) { g.Gemm(1, 3, 4, 2, plain(fresh2()), false, plain(fresh2()), false, s) },
		"Embedding":  func(s tensor.Storage) { g.Embedding(s, plain(fresh2()), []int{1, 0}, 6) },
		"EmbeddingG": func(s tensor.Storage) { g.EmbeddingGrad(s, fresh2(), []int{1, 0}, 6) },
		"SGDStep":    func(s tensor.Storage) { g.SGDStep(s, fresh2(), nil, 0.1, 0, 0.01) },
		"AdamWStep": func(s tensor.Storage) {
			g.AdamWStep(s, fresh2(), fresh2(), g.FromHost(make([]float32, n), tensor.Float32), 0.1, 0.9, 0.99, 1e-8, 0.1, 0.1, 0.01)
		},
		"Scale":   func(s tensor.Storage) { g.Scale(s, 3) },
		"RMSNorm": func(s tensor.Storage) { g.RMSNorm(s, plain(fresh2()), nil, 6, 1e-5) },
	}
	for ad, write := range paths {
		s := fresh2()
		if g.storageOf(s).host == nil {
			t.Fatalf("%s: küçük konak verisi gölgeli olmalıydı", ad)
		}
		write(s)
		checkDiff(t, ad, g.ToHost(plain(s)), fromDevice(g, s), 0)
	}
	// Tensör katmanındaki yerinde değişiklikler (Set, CopyFrom, iyileştirici).
	x := tensor.NewOn(gpuDevice, random(rng, n, 1), n)
	x.Set(9, 3)
	x.CopyFrom(tensor.NewOn(gpuDevice, []float32{5}))
	if d := x.Data(); d[3] != 5 || d[0] != 5 {
		t.Fatalf("CopyFrom sonrası okunan değer eski: %v", d)
	}
}

func TestOptimizers(t *testing.T) {
	rng := rand.New(rand.NewSource(14))
	pv, hv := random(rng, 300, 1), random(rng, 300, 1)
	for _, ad := range []string{"adamw", "sgd", "sgd momentumlu"} {
		var result [2][]float32
		var norms [2]float64
		for c := 0; c < 2; c++ {
			p := twin(pv, true, 300)[c]
			h := twin(hv, false, 300)[c]
			var step func()
			switch ad {
			case "adamw":
				o := tensor.NewAdamW([]*tensor.Tensor{p}, 0.01, 0.1)
				step = o.Step
			case "sgd":
				o := tensor.NewSGD([]*tensor.Tensor{p}, 0.1, 0)
				step = o.Step
			default:
				o := tensor.NewSGD([]*tensor.Tensor{p}, 0.1, 0.9)
				step = o.Step
			}
			for i := 0; i < 3; i++ {
				p.ZeroGrad()
				tensor.Mul(p, h).Tanh().SumAll().Backward()
				norms[c] = tensor.ClipGradNorm([]*tensor.Tensor{p}, 0.5)
				step()
			}
			result[c] = p.Data()
		}
		checkDiff(t, ad, result[1], result[0], 1e-5)
		if d := norms[1] - norms[0]; d > 1e-5*norms[0] || d < -1e-5*norms[0] {
			t.Fatalf("%s: gradyan normu GPU %v, CPU %v", ad, norms[1], norms[0])
		}
	}
}

// smallModel kıyas modelinin küçültülmüşü: gömme, RMS normu, nedensel çok başlı dikkat,
// SwiGLU, paylaşımlı çıkış, çapraz entropi.
type smallModel struct {
	E, g1, wq, wk, wv, wo, wg, wu, w2, gs *tensor.Tensor
}

const (
	kmB, kmT, kmD, kmH, kmFF, kmV = 2, 6, 8, 2, 12, 20
)

func newSmallModel(dev tensor.Device, seed int64, dt tensor.DType) *smallModel {
	rng := rand.New(rand.NewSource(seed))
	w := func(scaleF float64, sh ...int) *tensor.Tensor {
		return inLeafDType(tensor.NewOn(dev, random(rng, tensor.NumelOf(sh), scaleF), sh...), dt)
	}
	one := func(n int) *tensor.Tensor {
		v := make([]float32, n)
		for i := range v {
			v[i] = 1
		}
		return inLeafDType(tensor.NewOn(dev, v, n), dt)
	}
	return &smallModel{E: w(0.5, kmV, kmD), g1: one(kmD), wq: w(0.3, kmD, kmD), wk: w(0.3, kmD, kmD), wv: w(0.3, kmD, kmD),
		wo: w(0.3, kmD, kmD), wg: w(0.3, kmD, kmFF), wu: w(0.3, kmD, kmFF), w2: w(0.3, kmFF, kmD), gs: one(kmD)}
}

func (m *smallModel) parameters() []*tensor.Tensor {
	return []*tensor.Tensor{m.E, m.g1, m.wq, m.wk, m.wv, m.wo, m.wg, m.wu, m.w2, m.gs}
}

func (m *smallModel) loss2(input, target []int) *tensor.Tensor {
	x := tensor.Embedding(m.E, input, kmB, kmT)
	h := tensor.RMSNorm(x, m.g1, 1e-5)
	begin := func(t *tensor.Tensor) *tensor.Tensor { return t.Reshape(kmB, kmT, kmH, kmD/kmH) } // [B, T, H, dh]
	y := tensor.Attention(begin(tensor.MatMul(h, m.wq)), begin(tensor.MatMul(h, m.wk)), begin(tensor.MatMul(h, m.wv)), tensor.AttentionOptions{Causal: true})
	s := tensor.Add(x, tensor.MatMul(y.Reshape(kmB, kmT, kmD), m.wo))
	a := tensor.MatMul(s, m.wg)
	f := tensor.Mul(tensor.Mul(a, a.Sigmoid()), tensor.MatMul(s, m.wu))
	s = tensor.Add(s, tensor.MatMul(f, m.w2))
	output := tensor.MatMul(tensor.RMSNorm(s, m.gs, 1e-5), m.E.T())
	return tensor.CrossEntropy(output.Reshape(kmB*kmT, kmV), target)
}

// inLeafDType tensörü dt türünde türev izleyen bir yaprağa çevirir.
func inLeafDType(t *tensor.Tensor, dt tensor.DType) *tensor.Tensor {
	if dt != tensor.Float32 {
		t = t.Cast(dt).Detach()
	}
	return t.SetRequiresGrad(true)
}

// TestSmallModelTraining küçük dönüştürücüyü float32 ve bfloat16 ağırlıklarla (ana kopyalı
// AdamW) CPU ve GPU'da aynı veriyle eğitir ve kayıpları karşılaştırır. bf16 yolunda dikkatin
// geri yayılımı 16 bitlik çıktıyı açar (iç içe başlatma hatasının sınaması).
func TestSmallModelTraining(t *testing.T) {
	for _, d := range []struct {
		dt  tensor.DType
		tol float64
	}{{tensor.Float32, 1e-4}, {tensor.BFloat16, 3e-2}} {
		trainSmallModel(t, d.dt, d.tol)
	}
}

func trainSmallModel(t *testing.T, dt tensor.DType, tol float64) {
	rng := rand.New(rand.NewSource(15))
	steps := 4
	inputs := make([][2][]int, steps)
	for i := range inputs {
		for j := 0; j < 2; j++ {
			inputs[i][j] = make([]int, kmB*kmT)
			for k := range inputs[i][j] {
				inputs[i][j][k] = rng.Intn(kmV)
			}
		}
	}
	var losses [2][]float32
	var last [2][]float32
	for c, dev := range []tensor.Device{tensor.CPU, gpuDevice} {
		m := newSmallModel(dev, 16, dt)
		o := tensor.NewAdamW(m.parameters(), 0.01, 0.0)
		for i := 0; i < steps; i++ {
			o.ZeroGrad()
			l := m.loss2(inputs[i][0], inputs[i][1])
			l.Backward()
			tensor.ClipGradNorm(m.parameters(), 1.0)
			o.Step()
			losses[c] = append(losses[c], l.Item())
		}
		last[c] = m.wq.Data()
	}
	checkDiff(t, dt.String()+" kayıplar", losses[1], losses[0], tol)
	checkDiff(t, dt.String()+" son ağırlıklar", last[1], last[0], tol)
	if losses[1][steps-1] >= losses[1][0] {
		t.Fatalf("%s: kayıp düşmedi: %v", dt, losses[1])
	}
	t.Logf("%s kayıplar GPU %v, CPU %v", dt, losses[1], losses[0])
}

// GPU iletilerindeki bellek miktarı tensör paketinin bellek iletileriyle aynı biçimdedir: 1024
// tabanlı, ondalık virgüllü (dil tanımı 15.41: "bu GPU'da 96,0 GB var"); 1 KB altı bayt sayısı
// iletinin dilindedir (H4020).
func TestMemoryTextIsTheTensorRule(t *testing.T) {
	g := newSimBackend(1 << 20)
	err := testkit.PanicError(func() {
		var hold []tensor.Storage
		for i := 0; i < 8; i++ {
			hold = append(hold, g.Alloc(64<<10, tensor.Float32)) // 256 KB
		}
		_ = hold
	})
	for _, want := range []string{tensor.ByteSize(256 << 10).MessageText("tr"), tensor.ByteSize(1 << 20).MessageText("tr")} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("GPU bellek iletisi miktarı tensör iletileriyle aynı yazmalı (%q): %v", want, err)
		}
	}
	// Tek kural (4E): sözcük ve ondalık ayırıcı iletinin dilinde, birim simgesi dilden bağımsız.
	for b, want := range map[float64][2]string{0: {"0 bayt", "0 bytes"}, 1: {"1 bayt", "1 byte"}, 1023: {"1023 bayt", "1023 bytes"},
		1536: {"1,5 KB", "1.5 KB"}, 1 << 30: {"1,0 GB", "1.0 GB"}, 1288490189: {"1,2 GB", "1.2 GB"}, 2 << 40: {"2,0 TB", "2.0 TB"}} {
		if tr, en := tensor.ByteSize(b).MessageText("tr"), tensor.ByteSize(b).MessageText("en"); tr != want[0] || en != want[1] {
			t.Errorf("ByteSize(%v) = %q / %q, beklenen %q / %q", b, tr, en, want[0], want[1])
		}
	}
}
