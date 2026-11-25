package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (23.1, 23.3, 23.5, 23.7, 23.8, 15.41): TR_rfft, TR_stft, TR_irfft ve TR_istft kartta da çalışır; sonuç ve türev kartta
// kalır, işlemcininkiyle 15.41'in 3. basamağında aynıdır. Kartta çarpım kitaplık girişine hiç
// uğramaz (TR'nin kendi çekirdeği, ileri ve geri). Kartta uzunluk en çok 4096'dır: 4096 geçer,
// 4097 H4134 verir ve taşımanın yazımını söyler. Sınır girdileri: n = 1, tek çerçeve, yansıtmada
// sinyal n/2 + 1, kısa sinyal (sıfır dolgu), toplu girdi. Ağır boylar (Whisper'ın n'si ve log-Mel'i,
// n = 4096) yarış dedektöründe koşmaz; öbür kiplerde ve gerçek kartta koşar.

var fourierTrig = tensor.Trig{Sin: math.Sin, Cos: math.Cos}

var fourierCall = tensor.FourierCall{Fn: "fourier", Move: "x", Trig: fourierTrig}

func TestRfftOnTheCardMatchesCPU(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	type rfftCase struct {
		shape []int
		n     int
	}
	cases := []rfftCase{{[]int{1}, 1}, {[]int{5}, 8}, {[]int{2, 3, 7}, 12}}
	if !raceEnabled { // Whisper'ın n'si; ağır boylar yarış dedektöründe koşmaz (skipUnderRace'in gerekçesi)
		cases = append(cases, rfftCase{[]int{1, 400}, 400})
	}
	for _, c := range cases {
		n := 1
		for _, d := range c.shape {
			n *= d
		}
		data := random(rng, n, 1)
		for part := range 2 {
			compare(t, fmt.Sprintf("rfft %v n %d kısım %d", c.shape, c.n, part), 1e-4, func(dev int) (*tensor.Tensor, []*tensor.Tensor) {
				x := twin(data, true, c.shape...)[dev]
				re, im := tensor.Rfft(x, c.n, fourierCall)
				return []*tensor.Tensor{re, im}[part], []*tensor.Tensor{x}
			})
		}
	}
}

func TestStftOnTheCardMatchesCPU(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	type stftCase struct {
		rows, signal int
		o            tensor.StftOptions
	}
	cases := []stftCase{
		{2, 50, tensor.StftOptions{N: 8, Hop: 3, Center: true, Reflect: true}},
		{1, 50, tensor.StftOptions{N: 8, Hop: 3, Center: true}},
		{1, 20, tensor.StftOptions{N: 8, Hop: 5}},
		{1, 5, tensor.StftOptions{N: 8, Hop: 2, Center: true, Reflect: true}},
		{1, 6, tensor.StftOptions{N: 1, Hop: 1}},
		{1, 8, tensor.StftOptions{N: 8, Hop: 4}},
	}
	if !raceEnabled {
		// Whisper'ın log-Mel'i; ağır boylar yarış dedektöründe koşmaz (skipUnderRace'in gerekçesi).
		cases = append(cases, stftCase{1, 4000, tensor.StftOptions{N: 400, Hop: 160, Center: true, Reflect: true}})
	}
	for _, c := range cases {
		data, w := random(rng, c.rows*c.signal, 1), random(rng, c.o.N, 1)
		for part := range 2 {
			compare(t, fmt.Sprintf("stft %d×%d %+v kısım %d", c.rows, c.signal, c.o, part), 1e-4, func(dev int) (*tensor.Tensor, []*tensor.Tensor) {
				x, win := twin(data, true, c.rows, c.signal)[dev], twin(w, true, c.o.N)[dev]
				re, im := tensor.Stft(x, win, c.o, fourierCall)
				return []*tensor.Tensor{re, im}[part], []*tensor.Tensor{x, win}
			})
		}
	}
}

func TestInverseFourierOnTheCardMatchesCPU(t *testing.T) {
	rng := rand.New(rand.NewSource(14))
	type irfftCase struct {
		shape []int
		n     int
	}
	cases := []irfftCase{{[]int{1}, 1}, {[]int{2}, 2}, {[]int{2, 3, 7}, 13}, {[]int{2, 9}, 16}}
	if !inSim() { // PerTh'in n'si; benzeticide 2048'lik taban çarpımı ağırdır, gerçek kartta koşar
		cases = append(cases, irfftCase{[]int{2, 1025}, 2048})
	}
	for _, c := range cases {
		n := 1
		for _, d := range c.shape {
			n *= d
		}
		rd, id := random(rng, n, 1), random(rng, n, 1)
		compare(t, fmt.Sprintf("irfft %v n %d", c.shape, c.n), 1e-4, func(dev int) (*tensor.Tensor, []*tensor.Tensor) {
			re, im := twin(rd, true, c.shape...)[dev], twin(id, true, c.shape...)[dev]
			return tensor.Irfft(re, im, c.n, fourierCall), []*tensor.Tensor{re, im}
		})
	}
	type istftCase struct {
		rows, frames int
		o            tensor.IstftOptions
	}
	frames := []istftCase{
		{2, 6, tensor.IstftOptions{N: 16, Hop: 4, Center: true, Length: 20}},
		{1, 4, tensor.IstftOptions{N: 9, Hop: 3, Length: 18}},
		{1, 1, tensor.IstftOptions{N: 8, Hop: 5, Center: true, Length: 4}},
	}
	if !inSim() { // PerTh'in boyu
		frames = append(frames, istftCase{1, 11, tensor.IstftOptions{N: 2048, Hop: 320, Center: true, Length: 3200}})
	}
	for _, c := range frames {
		F := c.o.N/2 + 1
		rd, id, wd := random(rng, c.rows*F*c.frames, 1), random(rng, c.rows*F*c.frames, 1), random(rng, c.o.N, 0.25)
		for i := range wd {
			wd[i] += 0.75
		}
		compare(t, fmt.Sprintf("istft %d×%d %+v", c.rows, c.frames, c.o), 1e-4, func(dev int) (*tensor.Tensor, []*tensor.Tensor) {
			re, im, win := twin(rd, true, c.rows, F, c.frames)[dev], twin(id, true, c.rows, F, c.frames)[dev], twin(wd, true, c.o.N)[dev]
			return tensor.Istft(re, im, win, c.o, fourierCall), []*tensor.Tensor{re, im, win}
		})
	}
}

func TestFourierNeverCallsTheLibrary(t *testing.T) {
	if !inSim() {
		t.Skip("giriş sayacı benzeticide sınanır; gerçek kartta bitler ml'nin bloklu testinde")
	}
	sig, n := 300, 64
	if raceEnabled { // yarış dedektöründe küçük boy (skipUnderRace'in gerekçesi); satır sayısı ve yollar aynı
		sig, n = 40, 16
	}
	rng := rand.New(rand.NewSource(13))
	x := tensor.NewOn(gpuDevice, random(rng, 3*sig, 1), 3, sig).SetRequiresGrad(true)
	w := tensor.NewOn(gpuDevice, random(rng, n, 1), n).SetRequiresGrad(true)
	if d := vendorDelta(func() {
		re, im := tensor.Rfft(x, sig, fourierCall)
		re = tensor.Irfft(re, im, sig, fourierCall)
		sr, si := tensor.Stft(x, w, tensor.StftOptions{N: n, Hop: n / 4, Center: true, Reflect: true}, fourierCall)
		_, _, length := tensor.IstftLengths(sr.Shape()[2], tensor.IstftOptions{N: n, Hop: n / 4, Center: true})
		y := tensor.Istft(sr, si, w, tensor.IstftOptions{N: n, Hop: n / 4, Center: true, Length: length}, fourierCall)
		tensor.Add(tensor.Add(re.SumAll(), sr.SumAll()), tensor.Add(si.SumAll(), y.SumAll())).Backward()
	}); d != 0 {
		t.Fatalf("Fourier'in %d çarpımı kitaplık girişine gitti", d)
	}
	// Sayaç çağrı anında artar; türevler indirilir ki benzeticinin eşzamansız işi bu testin süresinde kalsın.
	x.Grad().Data()
	w.Grad().Data()
}

func TestFourierLengthLimitOnTheCard(t *testing.T) {
	x := tensor.NewOn(gpuDevice, make([]float32, 8), 1, 8)
	if !raceEnabled { // ağır boy (4096 × 4098'lik taban ve çarpımı) yarış dedektöründe koşmaz (skipUnderRace'in gerekçesi)
		// Sonuç indirilir: benzetici eşzamansızdır, indirilmeyen çarpım sonraki testin süresine yazılırdı.
		if m := panicText(func() { re, _ := tensor.Rfft(x, tensor.FourierCardLimit, fourierCall); re.Data() }); m != "" {
			t.Fatalf("n = %d kartta geçmeliydi: %s", tensor.FourierCardLimit, m)
		}
	}
	err := testkit.PanicError(func() { tensor.Rfft(x, tensor.FourierCardLimit+1, fourierCall) })
	if !messages.Has(err, messages.H4134) || !strings.Contains(err.Error(), fmt.Sprint(tensor.FourierCardLimit+1)) {
		t.Fatalf("n = %d: %v, beklenen H4134", tensor.FourierCardLimit+1, err)
	}
	part := tensor.NewOn(gpuDevice, make([]float32, 2049), 2049)
	if err := testkit.PanicError(func() { tensor.Irfft(part, part, 4097, fourierCall) }); !messages.Has(err, messages.H4134) {
		t.Fatalf("irfft n = 4097: %v, beklenen H4134", err)
	}
	w := tensor.NewOn(gpuDevice, make([]float32, 4097), 4097)
	sig := tensor.NewOn(gpuDevice, make([]float32, 5000), 1, 5000)
	if err := testkit.PanicError(func() { tensor.Stft(sig, w, tensor.StftOptions{N: 4097, Hop: 512}, fourierCall) }); !messages.Has(err, messages.H4134) {
		t.Fatalf("stft n = 4097: %v, beklenen H4134", err)
	}
	spec := tensor.NewOn(gpuDevice, make([]float32, 2049*2), 2049, 2)
	if err := testkit.PanicError(func() {
		tensor.Istft(spec, spec, w, tensor.IstftOptions{N: 4097, Hop: 512, Center: true, Length: 513}, fourierCall)
	}); !messages.Has(err, messages.H4134) {
		t.Fatalf("istft n = 4097: %v, beklenen H4134", err)
	}
}
