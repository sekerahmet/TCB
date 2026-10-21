package gpu

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestAttentionAlias q, k, v'nin (ya da ikisinin) aynı tensör olduğu dikkat: gradyanlar aynı
// depolamada birikir; float32 ve 16 bitlik gradyan türleri.
func TestAttentionAlias(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	xv, qv := random(rng, 2*20*8, 1), random(rng, 2*20*8, 1)
	for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16, tensor.Float16} {
		old := tensor.GradDType()
		if dt != tensor.Float32 {
			tensor.SetGradDType(dt)
		}
		tol := 2e-2
		if dt == tensor.Float32 {
			tol = 5e-5
		}
		convert := func(x *tensor.Tensor) *tensor.Tensor {
			if dt == tensor.Float32 {
				return x
			}
			return x.Cast(dt)
		}
		compare(t, "dikkat q=k=v "+dt.String(), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(xv, true, 1, 2, 20, 8)[c]
			xb := convert(x)
			return tensor.Attention(xb, xb, xb, tensor.AttentionOptions{Causal: true}).Cast(tensor.Float32), []*tensor.Tensor{x}
		})
		compare(t, "dikkat k=v "+dt.String(), tol, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x, q := twin(xv, true, 1, 2, 20, 8)[c], twin(qv, true, 1, 2, 20, 8)[c]
			xb := convert(x)
			return tensor.Attention(convert(q), xb, xb, tensor.AttentionOptions{}).Cast(tensor.Float32), []*tensor.Tensor{q, x}
		})
		tensor.SetGradDType(old)
	}
}

// TestAttentionRealSizes LLM'e özgü bakış boyu (Dh = 64: ileri 64×32; geri 48 KB'ta 32×32,
// açılabilen paylaşımlı bellekte 64×64 devingen döşeme), GQA, çok döşeme, Tq > Tk nedensel,
// pencere ve dolgu maskesi. Benzetimde iki geri döşeme de sınanır.
func TestAttentionRealSizes(t *testing.T) {
	skipUnderRace(t)
	if a, _ := pickFlash(64, 64, false, false, 99<<10); a.Br != 64 || a.Bc != 32 {
		t.Fatalf("Dh=64 ileri döşemesi %d×%d, 64×32 bekleniyordu", a.Br, a.Bc)
	}
	if a, _ := pickFlash(64, 64, false, true, 48<<10); a.Br != 32 || a.Bc != 32 || a.isDynamic(true) {
		t.Fatalf("Dh=64 geri döşemesi (48 KB) %d×%d, statik 32×32 bekleniyordu", a.Br, a.Bc)
	}
	if a, _ := pickFlash(64, 64, false, true, 99<<10); a.Br != 64 || a.Bc != 64 || !a.isDynamic(true) {
		t.Fatalf("Dh=64 geri döşemesi (99 KB) %d×%d, devingen 64×64 bekleniyordu", a.Br, a.Bc)
	}
	if bz, ok := gpuBackend().sr.(*simDriver); ok {
		old := bz.sharedMem
		for _, limit := range []int{48 << 10, 99 << 10} {
			bz.sharedMem = limit
			t.Run(fmt.Sprintf("paylaşım %d KB", limit>>10), attnRealSizes)
		}
		bz.sharedMem = old
		return
	}
	attnRealSizes(t)
}

func attnRealSizes(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	cases := []struct {
		ad                        string
		B, H, Hkv, Tq, Tk, Dh, Dv int
		opt                       tensor.AttentionOptions
		padding                   int // > 0: son anahtarlar maskeli (yalnızca ilk dolgu anahtar görünür)
	}{
		{"dh64 gqa nedensel", 1, 4, 2, 70, 70, 64, 64, tensor.AttentionOptions{Causal: true}, 0},
		{"Tq>Tk nedensel", 1, 2, 1, 40, 9, 16, 16, tensor.AttentionOptions{Causal: true}, 0},
		{"dh64 pencere", 1, 2, 2, 100, 100, 64, 64, tensor.AttentionOptions{Causal: true, Window: 33}, 0},
		{"dolgu maskesi", 1, 2, 2, 70, 70, 16, 16, tensor.AttentionOptions{}, 40},
	}
	for _, d := range cases {
		qv := random(rng, d.B*d.H*d.Tq*d.Dh, 1)
		kv := random(rng, d.B*d.Hkv*d.Tk*d.Dh, 1)
		vv := random(rng, d.B*d.Hkv*d.Tk*d.Dv, 1)
		var mv []float32
		if d.padding > 0 {
			mv = make([]float32, d.Tq*d.Tk)
			for i := 0; i < d.Tq; i++ {
				for j := 0; j < d.padding; j++ {
					mv[i*d.Tk+j] = 1
				}
			}
		}
		compare(t, "dikkat "+d.ad, 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			q := twin(qv, true, d.B, d.H, d.Tq, d.Dh)[c]
			k := twin(kv, true, d.B, d.Hkv, d.Tk, d.Dh)[c]
			v := twin(vv, true, d.B, d.Hkv, d.Tk, d.Dv)[c]
			opt := d.opt
			if mv != nil {
				opt.Mask = twin(mv, false, d.Tq, d.Tk)[c]
			}
			return tensor.Attention(q, k, v, opt), []*tensor.Tensor{q, k, v}
		})
	}
}

// TestAttentionLongContextLimit birleşik yolda puan matrisinin (B·H·Tq·Tk) boyutunun sınırlamadığını,
// gerçek 32 bitlik dizin sınırlarının (q/çıktı ve k/v) ise denetlendiğini sınar; matris yolu
// puan matrisini de denetler.
func TestAttentionLongContextLimit(t *testing.T) {
	long := tensor.AttentionParams{Batch: 16, H: 16, Hkv: 16, Tq: 4096, Tk: 4096, Dh: 64, Dv: 64}
	if err := panicError(func() { checkAttnSize(long, true) }); err != nil {
		t.Fatalf("birleşik yol uzun bağlamı reddetti: %v", err)
	}
	if err := panicError(func() { checkAttnSize(long, false) }); !messages.Has(err, messages.H4016) {
		t.Fatalf("matris yolu puan matrisi sınırını denetlemedi: %v", err)
	}
	tooLarge := tensor.AttentionParams{Batch: 64, H: 32, Hkv: 8, Tq: 16384, Tk: 16384, Dh: 128, Dv: 128}
	if err := panicError(func() { checkAttnSize(tooLarge, true) }); !messages.Has(err, messages.H4016) {
		t.Fatalf("q dizin sınırı denetlenmedi: %v", err)
	}
	// Benzetimde küçük bir uzun bağlam: birden çok sorgu ve anahtar döşemesi, nedensel.
	rng := rand.New(rand.NewSource(7))
	T := 200
	qv, kv, vv := random(rng, T*8, 1), random(rng, T*8, 1), random(rng, T*8, 1)
	compare(t, "dikkat T=200", 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		q, k, v := twin(qv, true, 1, 1, T, 8)[c], twin(kv, true, 1, 1, T, 8)[c], twin(vv, true, 1, 1, T, 8)[c]
		return tensor.Attention(q, k, v, tensor.AttentionOptions{Causal: true}), []*tensor.Tensor{q, k, v}
	})
}

// TestAttentionPathForced TR_GPU_DIKKAT=carpim'in matris yolunu seçtiğini (sonuç aynı) sınar.
func TestAttentionPathForced(t *testing.T) {
	g := gpuBackend()
	old := g.attnPath
	defer func() { g.attnPath = old }()
	g.attnPath = "carpim"
	if _, ok := g.flashFits(tensor.AttentionParams{Batch: 1, H: 1, Hkv: 1, Tq: 4, Tk: 4, Dh: 8, Dv: 8}, false, false); ok {
		t.Fatal("matris yolu zorlanmadı")
	}
	rng := rand.New(rand.NewSource(8))
	qv, kv, vv := random(rng, 2*9*8, 1), random(rng, 2*9*8, 1), random(rng, 2*9*8, 1)
	compare(t, "dikkat matris yolu", 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		q, k, v := twin(qv, true, 1, 2, 9, 8)[c], twin(kv, true, 1, 2, 9, 8)[c], twin(vv, true, 1, 2, 9, 8)[c]
		return tensor.Attention(q, k, v, tensor.AttentionOptions{Causal: true}), []*tensor.Tensor{q, k, v}
	})
}
