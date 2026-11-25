package gpu

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

func splitParams(d splitCase) tensor.AttentionParams {
	return tensor.AttentionParams{Batch: d.B, H: d.H, Hkv: d.Hkv, Tq: d.Tq, Tk: d.Tk, Dh: d.D, Dv: d.D, Causal: true, SeqMajor: true}
}

// TestAttentionSplitConfigIsFixed: bölünmüş dikkatin ayarları (R, W, KC, tek parça) yalnız şekilden gelir ve sabittir;
// değişirlerse bitler TR sürümleri arasında değişir (15.22a, sürüm notu).
func TestAttentionSplitConfigIsFixed(t *testing.T) {
	if splitMaxD != 128 || splitMaxRows != 256 || splitMinKeys != 16 || splitLongKeys != 256 || splitLongD != 64 ||
		splitTempMax != 64<<20 || splitFewRows != 4 {
		t.Fatal("bölünmüş dikkatin sabitleri değiştiyse sürüm notu gerekir (15.22a)")
	}
	for _, c := range []struct {
		d    splitCase
		want flashSplit
	}{
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 1, Tk: 512, D: 128}, flashSplit{R: 4, W: 16, KC: 32, D: 128, single: true}},
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 1, Tk: 513, D: 128}, flashSplit{R: 4, W: 16, KC: 64, D: 128, single: true}},
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 1, Tk: 1024, D: 128}, flashSplit{R: 4, W: 16, KC: 64, D: 128, single: true}},
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 1, Tk: 1025, D: 128}, flashSplit{R: 4, W: 16, KC: 32, D: 128}},
		{splitCase{B: 1, H: 4, Hkv: 4, Tq: 4, Tk: 100, D: 64}, flashSplit{R: 4, W: 16, KC: 32, D: 64, single: true}},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 5, Tk: 100, D: 64}, flashSplit{R: 8, W: 8, KC: 32, D: 64, single: true}},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 8, Tk: 256, D: 64}, flashSplit{R: 8, W: 8, KC: 32, D: 64, single: true}},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 8, Tk: 257, D: 64}, flashSplit{R: 8, W: 8, KC: 32, D: 64}},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 9, Tk: 256, D: 32}, flashSplit{R: 16, W: 4, KC: 64, D: 32, single: true}},
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 40, Tk: 4140, D: 128}, flashSplit{R: 16, W: 4, KC: 64, D: 128}},
	} {
		if got := pickSplit(splitParams(c.d)); got != c.want {
			t.Errorf("%v: ayar %+v, beklenen %+v", c.d, got, c.want)
		}
	}
}

// TestAttentionSplitSelection seçim kuralı (backend_attention_split.go): her sabitin iki yanı, boy koşulları, zorlanan
// yollar ve geçici belleğin sınırı.
func TestAttentionSplitSelection(t *testing.T) {
	g := gpuBackend()
	for _, c := range []struct {
		d    splitCase
		want bool
	}{
		// (a) Tq ≤ Br/4 (D 96 ve 128'de Br 32, 32 ve 64'te 64), Tk ≥ 16.
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 8, Tk: 16, D: 128}, true},
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 9, Tk: 16, D: 128}, false},
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 8, Tk: 15, D: 128}, false},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 8, Tk: 16, D: 96}, true},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 9, Tk: 16, D: 96}, false},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 16, Tk: 16, D: 64}, true},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 17, Tk: 16, D: 64}, false},
		{splitCase{B: 64, H: 8, Hkv: 8, Tq: 16, Tk: 16, D: 32}, true},
		{splitCase{B: 64, H: 8, Hkv: 8, Tq: 17, Tk: 16, D: 32}, false},
		// (b) D ≥ 64, Tk ≥ 256, ⌈Tq/Br⌉·H·toplu ≤ D/2.
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 64, Tk: 256, D: 128}, true},
		{splitCase{B: 1, H: 32, Hkv: 8, Tq: 64, Tk: 255, D: 128}, false},
		{splitCase{B: 1, H: 33, Hkv: 33, Tq: 64, Tk: 256, D: 128}, false},
		{splitCase{B: 2, H: 32, Hkv: 8, Tq: 32, Tk: 256, D: 128}, true},
		{splitCase{B: 2, H: 33, Hkv: 33, Tq: 32, Tk: 256, D: 128}, false},
		{splitCase{B: 1, H: 16, Hkv: 16, Tq: 64, Tk: 256, D: 64}, true},
		{splitCase{B: 1, H: 33, Hkv: 33, Tq: 64, Tk: 256, D: 64}, false},
		{splitCase{B: 1, H: 24, Hkv: 24, Tq: 64, Tk: 256, D: 96}, true},
		{splitCase{B: 1, H: 25, Hkv: 25, Tq: 64, Tk: 256, D: 96}, false},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 64, Tk: 4140, D: 32}, false},
		// Satır sınırı (Tq·G ≤ 256): iki kola da uygulanır.
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 256, Tk: 256, D: 64}, true},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 257, Tk: 256, D: 64}, false},
		{splitCase{B: 1, H: 32, Hkv: 1, Tq: 8, Tk: 256, D: 128}, true},
		{splitCase{B: 1, H: 64, Hkv: 1, Tq: 5, Tk: 16, D: 128}, false},
		// Geçici bellek: 4·toplu·H·Tq·parça·(D + 2) ≤ 64 MiB (parça 17: Tk 4 140, R = 8).
		{splitCase{B: 118, H: 8, Hkv: 8, Tq: 8, Tk: 4140, D: 128}, true},
		{splitCase{B: 119, H: 8, Hkv: 8, Tq: 8, Tk: 4140, D: 128}, false},
		// Boy: D 32'nin katı ve ≤ 128.
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 1, Tk: 100, D: 256}, false},
		{splitCase{B: 1, H: 8, Hkv: 8, Tq: 1, Tk: 100, D: 48}, false},
	} {
		if _, got := g.splitFits(false, splitParams(c.d)); got != c.want {
			t.Errorf("%v: bölünmüş yol %v, beklenen %v", c.d, got, c.want)
		}
	}
	one := splitParams(splitCase{B: 1, H: 8, Hkv: 8, Tq: 1, Tk: 100, D: 64})
	if _, ok := g.splitFits(true, one); ok {
		t.Error("ek maskeli tanım bölünmüş yolda olmamalı")
	}
	two := one
	two.Dv = 32
	if _, ok := g.splitFits(false, two); ok {
		t.Error("Dh ≠ Dv bölünmüş yolda olmamalı")
	}
	for path, want := range map[string]bool{"birlesik": false, "carpim": false, "tensor": false, "bolme": true} {
		withAttnPath(path, func() {
			if _, ok := g.splitFits(false, one); ok != want {
				t.Errorf("yol %q: bölünmüş yol %v", path, ok)
			}
		})
	}
	withAttnPath("bolme", func() {
		if _, ok := g.splitFits(false, splitParams(splitCase{B: 1, H: 8, Hkv: 8, Tq: 400, Tk: 20, D: 64})); !ok {
			t.Error("\"bolme\" seçim kuralını geçer")
		}
		if _, ok := g.splitFits(false, splitParams(splitCase{B: 119, H: 8, Hkv: 8, Tq: 8, Tk: 4140, D: 128})); ok {
			t.Error("\"bolme\" geçici belleğin sınırını geçmez")
		}
	})
}

// splitSpeedCases TestAttentionSplitSpeed'in ızgarası (tasarım, B4): G = 4 ve D 128'in derinlikleri; D 32–128'de G = 1 (D 128'de 2 de)
// kuralın sınırında ve üstünde; az sorgunun (Tq = Br/4) kısa ve uzun anahtarı, toplular; pencere, çapraz dikkat,
// toplu üretim.
func splitSpeedCases() []splitCase {
	causal, cross := tensor.AttentionOptions{Causal: true}, tensor.AttentionOptions{}
	var cs []splitCase
	add := func(B, H, Hkv, Tq, Tk, D int, opt tensor.AttentionOptions) {
		if c := (splitCase{B, H, Hkv, Tq, Tk, D, opt}); Tk >= Tq && !slices.Contains(cs, c) {
			cs = append(cs, c)
		}
	}
	for _, tq := range []int{1, 2, 4, 8, 16, 32, 64, 128} {
		for _, tk := range []int{tq, 16, 64, 128, 256, 640, 1500, 4140} {
			add(1, 32, 8, tq, tk, 128, causal)
		}
	}
	for _, d := range []int{32, 64, 96, 128} {
		br := 64
		if d >= 96 {
			br = 32
		}
		for _, c := range [][3]int{{1, 8, 4}, {1, 12, 4}, {1, 16, 4}, {1, 24, 4}, {2, 12, 2}, {4, 8, 1}, {4, 12, 1}, {4, 16, 1}, {8, 8, 1}} { // toplu, H, Tq/Br
			for _, tk := range []int{256, 1024, 1500} {
				add(c[0], c[1], c[1], c[2]*br, tk, d, causal)
				if d == 128 {
					add(c[0], 2*c[1], c[1], c[2]*br/2, tk, d, causal)
				}
			}
		}
		for _, b := range []int{1, 8, 32} {
			for _, tk := range []int{16, 64, 128, 1024} {
				add(b, 12, 12, br/4, tk, d, causal)
				add(b, 12, 12, 1, tk, d, causal)
			}
		}
	}
	for _, c := range [][3]int{{1, 4140, 512}, {40, 4140, 512}, {1, 1500, 0}} {
		add(1, 32, 8, c[0], c[1], 128, tensor.AttentionOptions{Causal: true, Window: c[2]})
	}
	for _, b := range []int{1, 5} {
		add(b, 20, 20, 1, 1500, 64, cross)
		add(b, 6, 6, 4, 1500, 64, cross)
	}
	for _, b := range []int{8, 32} {
		add(b, 32, 8, 1, 1024, 128, causal)
	}
	add(16, 12, 12, 128, 128, 64, causal)
	add(8, 12, 12, 256, 256, 64, causal)
	add(1, 14, 2, 1, 1024, 64, causal)
	return cs
}

// TestAttentionSplitSpeed ızgaranın şekillerinde birleşik ve bölünmüş yolun süresini ve kuralın seçimini yazar (gerçek kartta; benzeticide
// atlanır). Ölçü değildir; kuralın kart sınıfına uyup uymadığını gösterir: seçilen bir şeklin oranı 1'i gürültünün üstünde geçmemelidir.
func TestAttentionSplitSpeed(t *testing.T) {
	if inSim() {
		t.Skip("benzetici")
	}
	rng := rand.New(rand.NewSource(37))
	g := gpuBackend()
	for _, d := range splitSpeedCases() {
		qv, kv, vv := splitInputs(rng, d, 1, 1)
		q, k, v := splitTensors(qv, kv, vv, false, d, 1)
		run := func(path string) time.Duration {
			best := time.Duration(1 << 62)
			withAttnPath(path, func() {
				tensor.Attention(q, k, v, d.opt).Drop()
				g.equalize()
				for range 3 {
					t0 := time.Now()
					for range 40 {
						tensor.Attention(q, k, v, d.opt).Drop()
					}
					g.equalize()
					best = min(best, time.Since(t0)/40)
				}
			})
			return best
		}
		p := splitParams(d)
		p.Causal, p.Window = d.opt.Causal, d.opt.Window
		_, sel := g.splitFits(false, p)
		fused, split := run("birlesik"), run("bolme")
		t.Logf("B %v: birleşik %v, bölünmüş %v, oran %.2f, seçilir %v", fmt.Sprint(d), fused, split, float64(split)/float64(fused), sel)
	}
}
