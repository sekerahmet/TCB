package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// G2a hakem bulgularının kenar durumları (K1, K2, Y2, O3, O6, O7): her biri CPU ile
// karşılaştırılır.

// TestCaprazEntropiEksiSonsuz −∞ logitli satırlar (başta, ortada, tamamı): −∞ katkı vermez;
// tamamı −∞ olan satır CPU'daki gibi NaN verir. Tekli ve 4'lü vektör yolu.
func TestCaprazEntropiEksiSonsuz(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	eksi := float32(math.Inf(-1))
	for _, C := range []int{7, 300} {
		R := 4
		v := rastgele(rng, R*C, 1)
		v[0*C] = eksi                 // satır 0: ilk eleman (her iş parçacığının ilk gördüğü)
		v[1*C], v[1*C+1] = eksi, eksi // satır 1: ilk iki
		v[2*C+C/2] = eksi             // satır 2: ortada
		hedef := []int{1, 2, 3, 0}
		karsilastir(t, fmt.Sprintf("ce −∞ C=%d", C), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(v, true, R, C)[c]
			return tensor.CrossEntropy(x, hedef), []*tensor.Tensor{x}
		})
		tum := append([]float32(nil), v...)
		for j := 0; j < C; j++ {
			tum[3*C+j] = eksi // satır 3: tamamı −∞ (CPU: NaN)
		}
		karsilastir(t, fmt.Sprintf("ce tamamı −∞ satır C=%d", C), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(tum, true, R, C)[c]
			return tensor.CrossEntropy(x, hedef), []*tensor.Tensor{x}
		})
	}
}

// TestDikkatTakmaAd q, k, v'nin (ya da ikisinin) aynı tensör olduğu dikkat: gradyanlar aynı
// depolamada birikir; float32 ve 16 bitlik gradyan türleri.
func TestDikkatTakmaAd(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	xv, qv := rastgele(rng, 2*20*8, 1), rastgele(rng, 2*20*8, 1)
	for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16, tensor.Float16} {
		eski := tensor.GradDType()
		if dt != tensor.Float32 {
			tensor.SetGradDType(dt)
		}
		tol := 2e-2
		if dt == tensor.Float32 {
			tol = 5e-5
		}
		donustur := func(x *tensor.Tensor) *tensor.Tensor {
			if dt == tensor.Float32 {
				return x
			}
			return x.Cast(dt)
		}
		karsilastir(t, "dikkat q=k=v "+dt.String(), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := ikiz(xv, true, 1, 2, 20, 8)[c]
			xb := donustur(x)
			return tensor.Attention(xb, xb, xb, tensor.AttentionOptions{Causal: true}).Cast(tensor.Float32), []*tensor.Tensor{x}
		})
		karsilastir(t, "dikkat k=v "+dt.String(), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x, q := ikiz(xv, true, 1, 2, 20, 8)[c], ikiz(qv, true, 1, 2, 20, 8)[c]
			xb := donustur(x)
			return tensor.Attention(donustur(q), xb, xb, tensor.AttentionOptions{}).Cast(tensor.Float32), []*tensor.Tensor{q, x}
		})
		tensor.SetGradDType(eski)
	}
}

// TestDikkatGercekBoylar LLM'e özgü baş boyu (Dh = 64: ileri 64×32; geri 48 KB'ta 32×32,
// açılabilen paylaşımlı bellekte 64×64 devingen döşeme), GQA, çok döşeme, Tq > Tk nedensel,
// pencere ve dolgu maskesi. Benzetimde iki geri döşeme de sınanır.
func TestDikkatGercekBoylar(t *testing.T) {
	if a, _ := flashSec(64, 64, false, false, 99<<10); a.Br != 64 || a.Bc != 32 {
		t.Fatalf("Dh=64 ileri döşemesi %d×%d, 64×32 bekleniyordu", a.Br, a.Bc)
	}
	if a, _ := flashSec(64, 64, false, true, 48<<10); a.Br != 32 || a.Bc != 32 || a.dinamikMi(true) {
		t.Fatalf("Dh=64 geri döşemesi (48 KB) %d×%d, statik 32×32 bekleniyordu", a.Br, a.Bc)
	}
	if a, _ := flashSec(64, 64, false, true, 99<<10); a.Br != 64 || a.Bc != 64 || !a.dinamikMi(true) {
		t.Fatalf("Dh=64 geri döşemesi (99 KB) %d×%d, devingen 64×64 bekleniyordu", a.Br, a.Bc)
	}
	if bz, ok := gpuArkaUc().sr.(*benzetimSurucu); ok {
		eski := bz.paylasim
		for _, sinir := range []int{48 << 10, 99 << 10} {
			bz.paylasim = sinir
			t.Run(fmt.Sprintf("paylaşım %d KB", sinir>>10), dikkatGercekBoylar)
		}
		bz.paylasim = eski
		return
	}
	dikkatGercekBoylar(t)
}

func dikkatGercekBoylar(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	durumlar := []struct {
		ad                        string
		B, H, Hkv, Tq, Tk, Dh, Dv int
		opt                       tensor.AttentionOptions
		dolgu                     int // > 0: son anahtarlar maskeli (yalnızca ilk dolgu anahtar görünür)
	}{
		{"dh64 gqa nedensel", 1, 4, 2, 70, 70, 64, 64, tensor.AttentionOptions{Causal: true}, 0},
		{"Tq>Tk nedensel", 1, 2, 1, 40, 9, 16, 16, tensor.AttentionOptions{Causal: true}, 0},
		{"dh64 pencere", 1, 2, 2, 100, 100, 64, 64, tensor.AttentionOptions{Causal: true, Window: 33}, 0},
		{"dolgu maskesi", 1, 2, 2, 70, 70, 16, 16, tensor.AttentionOptions{}, 40},
	}
	for _, d := range durumlar {
		qv := rastgele(rng, d.B*d.H*d.Tq*d.Dh, 1)
		kv := rastgele(rng, d.B*d.Hkv*d.Tk*d.Dh, 1)
		vv := rastgele(rng, d.B*d.Hkv*d.Tk*d.Dv, 1)
		var mv []float32
		if d.dolgu > 0 {
			mv = make([]float32, d.Tq*d.Tk)
			for i := 0; i < d.Tq; i++ {
				for j := 0; j < d.dolgu; j++ {
					mv[i*d.Tk+j] = 1
				}
			}
		}
		karsilastir(t, "dikkat "+d.ad, 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			q := ikiz(qv, true, d.B, d.H, d.Tq, d.Dh)[c]
			k := ikiz(kv, true, d.B, d.Hkv, d.Tk, d.Dh)[c]
			v := ikiz(vv, true, d.B, d.Hkv, d.Tk, d.Dv)[c]
			opt := d.opt
			if mv != nil {
				opt.Mask = ikiz(mv, false, d.Tq, d.Tk)[c]
			}
			return tensor.Attention(q, k, v, opt), []*tensor.Tensor{q, k, v}
		})
	}
}

// TestDikkatUzunBaglamSiniri birleşik yolda puan matrisinin (B·H·Tq·Tk) boyutunun sınırlamadığını,
// gerçek 32 bitlik dizin sınırlarının (q/çıktı ve k/v) ise denetlendiğini sınar; matris yolu
// puan matrisini de denetler.
func TestDikkatUzunBaglamSiniri(t *testing.T) {
	uzun := tensor.AttentionParams{Batch: 16, H: 16, Hkv: 16, Tq: 4096, Tk: 4096, Dh: 64, Dv: 64}
	if m := panikMetni(func() { dikkatBoyDenetle(uzun, true) }); m != "" {
		t.Fatalf("birleşik yol uzun bağlamı reddetti: %s", m)
	}
	if m := panikMetni(func() { dikkatBoyDenetle(uzun, false) }); !strings.Contains(m, "2^31") {
		t.Fatalf("matris yolu puan matrisi sınırını denetlemedi: %q", m)
	}
	cokBuyuk := tensor.AttentionParams{Batch: 64, H: 32, Hkv: 8, Tq: 16384, Tk: 16384, Dh: 128, Dv: 128}
	if m := panikMetni(func() { dikkatBoyDenetle(cokBuyuk, true) }); !strings.Contains(m, "2^31") {
		t.Fatalf("q dizin sınırı denetlenmedi: %q", m)
	}
	// Benzetimde küçük bir uzun bağlam: birden çok sorgu ve anahtar döşemesi, nedensel.
	rng := rand.New(rand.NewSource(7))
	T := 200
	qv, kv, vv := rastgele(rng, T*8, 1), rastgele(rng, T*8, 1), rastgele(rng, T*8, 1)
	karsilastir(t, "dikkat T=200", 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		q, k, v := ikiz(qv, true, 1, 1, T, 8)[c], ikiz(kv, true, 1, 1, T, 8)[c], ikiz(vv, true, 1, 1, T, 8)[c]
		return tensor.Attention(q, k, v, tensor.AttentionOptions{Causal: true}), []*tensor.Tensor{q, k, v}
	})
}

// TestDikkatYoluZorlanir TR_GPU_DIKKAT=carpim'in matris yolunu seçtiğini (sonuç aynı) sınar.
func TestDikkatYoluZorlanir(t *testing.T) {
	g := gpuArkaUc()
	eski := g.dikkatYolu
	defer func() { g.dikkatYolu = eski }()
	g.dikkatYolu = "carpim"
	if _, ok := g.flashUygun(tensor.AttentionParams{Batch: 1, H: 1, Hkv: 1, Tq: 4, Tk: 4, Dh: 8, Dv: 8}, false, false); ok {
		t.Fatal("matris yolu zorlanmadı")
	}
	rng := rand.New(rand.NewSource(8))
	qv, kv, vv := rastgele(rng, 2*9*8, 1), rastgele(rng, 2*9*8, 1), rastgele(rng, 2*9*8, 1)
	karsilastir(t, "dikkat matris yolu", 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		q, k, v := ikiz(qv, true, 1, 2, 9, 8)[c], ikiz(kv, true, 1, 2, 9, 8)[c], ikiz(vv, true, 1, 2, 9, 8)[c]
		return tensor.Attention(q, k, v, tensor.AttentionOptions{Causal: true}), []*tensor.Tensor{q, k, v}
	})
}

// TestIyilestiriciTekrarliParametre aynı parametre listede iki kez: CPU'daki gibi iki ardışık
// güncelleme (tek başlatmada yarış olmaz); gradyan kırpma da aynı gradyanı iki kez ölçekler.
func TestIyilestiriciTekrarliParametre(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	pv, hv := rastgele(rng, 50, 1), rastgele(rng, 50, 1)
	for _, ad := range []string{"adamw", "sgd"} {
		var sonuc [2][]float32
		for c := 0; c < 2; c++ {
			p := ikiz(pv, true, 50)[c]
			ps := []*tensor.Tensor{p, p}
			adim := tensor.NewSGD(ps, 0.1, 0.9).Step
			if ad == "adamw" {
				adim = tensor.NewAdamW(ps, 0.01, 0.1).Step
			}
			for k := 0; k < 3; k++ {
				p.ZeroGrad()
				tensor.Mul(p, tensor.NewOn(p.Device(), hv, 50)).Tanh().SumAll().Backward()
				tensor.ClipGradNorm(ps, 0.5)
				adim()
			}
			sonuc[c] = p.Data()
		}
		farkDenetle(t, ad+" tekrarlı parametre", sonuc[1], sonuc[0], 1e-5)
	}
}

// TestCokTensorluParcali geçici kopya sınırı aşılınca çok tensörlü işin parçalara bölündüğü
// yolu (16 bitlik gradyanlar açılır) CPU ile karşılaştırır.
func TestCokTensorluParcali(t *testing.T) {
	eskiSinir, eskiTur := cokGeciciSinir, tensor.GradDType()
	cokGeciciSinir = 64 // her tensörde bir parça
	tensor.SetGradDType(tensor.BFloat16)
	defer func() { cokGeciciSinir = eskiSinir; tensor.SetGradDType(eskiTur) }()
	rng := rand.New(rand.NewSource(10))
	boylar := []int{30, 70, 9}
	var sonuc [2][][]float32
	for c := 0; c < 2; c++ {
		r := rand.New(rand.NewSource(11))
		var ps []*tensor.Tensor
		for _, n := range boylar {
			ps = append(ps, ikiz(rastgele(r, n, 1), true, n)[c])
		}
		o := tensor.NewAdamW(ps, 0.01, 0.1)
		for k := 0; k < 2; k++ {
			tensor.ZeroGrads(ps)
			var kayip *tensor.Tensor
			for i, p := range ps {
				y := tensor.Mul(p, tensor.NewOn(p.Device(), rastgele(rand.New(rand.NewSource(int64(i))), boylar[i], 1), boylar[i])).SumAll()
				if kayip == nil {
					kayip = y
				} else {
					kayip = tensor.Add(kayip, y)
				}
			}
			kayip.Backward()
			tensor.ClipGradNorm(ps, 0.1)
			o.Step()
		}
		for _, p := range ps {
			sonuc[c] = append(sonuc[c], p.Data())
		}
	}
	_ = rng
	for i := range boylar {
		farkDenetle(t, fmt.Sprintf("parçalı parametre %d", i), sonuc[1][i], sonuc[0][i], 2e-3)
	}
}
