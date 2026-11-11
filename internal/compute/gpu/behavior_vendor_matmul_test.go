package gpu

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (15.53b): TR_vendor_matmul bloğundaki çarpımlar üreticinin kitaplığına gider, bloğun dışındakiler
// gitmez (15.22a); M = 1 blokta da kendi çekirdektir; geri yön ileri yönde kaydedilen durumu kullanır;
// kitaplık yoksa ya da bir çağrıyı yapamazsa kendi çekirdekle sürülür ve kodlu uyarı süreçte bir kez
// verilir; kurulum bir kez ve kaydın havuzunun dışında yapılır. Benzeticide kitaplık yoktur: giriş
// sayacı (vendorCalls) ve kancalar (vendorHook, ltSetupHook) kitaplığın yerine geçer.

// freshWarnings uyarıları bir tampona yönlendirir ve "verildi" kaydını sıfırlar (test bitince geri).
func freshWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	reset := func() {
		warnedCodes.Range(func(k, _ any) bool { warnedCodes.Delete(k); return true })
	}
	var buf bytes.Buffer
	old := warnOut
	warnOut = &buf
	reset()
	t.Cleanup(func() { warnOut = old; reset() })
	return &buf
}

// countText metinde iletinin kaç kez geçtiğidir (iletinin ilk 30 baytıyla).
func countText(s string, code messages.Code, args ...any) int {
	head := messages.Text(code, args...)
	if len(head) > 30 {
		head = head[:30]
	}
	return strings.Count(s, head)
}

// vendorCase küçük bir M > 1 çarpımının işlenenleridir (benzetici arka ucunda).
func vendorCase(be *backend, M, N, K int) (a, b tensor.View, c tensor.Storage) {
	rng := rand.New(rand.NewSource(int64(M*1000 + N)))
	as, bs := be.FromHost(random(rng, M*K, 1), tensor.Float32), be.FromHost(random(rng, K*N, 1), tensor.Float32)
	return tensor.View{S: as, Shape: []int{M, K}, Strides: []int{K, 1}}, tensor.View{S: bs, Shape: []int{K, N}, Strides: []int{N, 1}},
		be.Alloc(M*N, tensor.Float32)
}

func TestVendorMatmulBlockSelectsLibrary(t *testing.T) {
	if !inSim() {
		t.Skip("giriş sayacı benzeticide sınanır; L4'te bir kez (U5)")
	}
	freshWarnings(t)
	be := newSimBackend(1 << 24)
	a, b, c := vendorCase(be, 4, 6, 8)
	be.GemmSet(1, 4, 6, 8, a, false, b, false, c)
	if be.vendorCalls != 0 {
		t.Fatalf("blok dışındaki çarpım kitaplık girişine gitti (%d)", be.vendorCalls)
	}
	want := bitsOfStorage(be, c, 24)
	be.GemmVendor(1, 4, 6, 8, a, false, b, false, c, true)
	if be.vendorCalls != 1 {
		t.Fatalf("bloktaki çarpım kitaplık girişine gitmedi (%d)", be.vendorCalls)
	}
	sameBitsAt(t, "kitaplıksız blok, bitler", bitsOfStorage(be, c, 24), want)
}

// bitsOfStorage bitişik n öğenin bitleridir.
func bitsOfStorage(be *backend, s tensor.Storage, n int) []uint32 {
	return bitsOf(tensor.NewOn(tensor.CPU, be.ToHost(flatView(s, n)), n))
}

// vendorDelta f'nin kayıtlı GPU arka ucunda kitaplık girişine kaç çarpım gönderdiğidir.
func vendorDelta(f func()) int {
	g := gpuBackend()
	before := g.vendorCalls
	f()
	return g.vendorCalls - before
}

func TestVendorMatmulScope(t *testing.T) {
	if !inSim() {
		t.Skip("giriş sayacı benzeticide sınanır")
	}
	freshWarnings(t)
	g := gpuBackend()
	rng := rand.New(rand.NewSource(81))
	const M, K, N = 4, 32, 6
	x := tensor.NewOn(gpuDevice, random(rng, M*K, 1), M, K)
	x1 := tensor.NewOn(gpuDevice, random(rng, K, 1), 1, K)
	w := tensor.NewOn(gpuDevice, random(rng, K*N, 1), K, N)
	q := tensor.NewOn(gpuDevice, random(rng, 2*5*8, 1), 5, 2, 8)
	_, card := quantWeights(t, rng, tensor.Q8, N, K)
	defer func(p string) { g.attnPath = p }(g.attnPath)
	g.attnPath = "carpim"
	families := []struct {
		name          string
		inside, plain func()
	}{
		{"@", func() { tensor.MatMulVendor(x, w, tensor.Float32) }, func() { tensor.MatMul(x, w) }},
		{"TR_attention (çarpımlı yol)", func() { tensor.Attention(q, q, q, tensor.AttentionOptions{Vendor: true}) },
			func() { tensor.Attention(q, q, q, tensor.AttentionOptions{}) }},
		{"nicemli M > 1", func() { tensor.MatMulVendor(x, card.T(), tensor.Float32) }, func() { tensor.MatMul(x, card.T()) }},
		{"16 bitlik @", func() {
			tensor.MatMulVendor(x.Cast(tensor.BFloat16), w.Cast(tensor.BFloat16), tensor.BFloat16)
		}, func() { tensor.MatMul(x.Cast(tensor.BFloat16), w.Cast(tensor.BFloat16)) }},
		{"TR_linear_cross_entropy", func() { tensor.LinearCrossEntropyVendor(x, w, []int{0, 1, 2, 3}) },
			func() { tensor.LinearCrossEntropy(x, w, []int{0, 1, 2, 3}) }},
	}
	for _, f := range families {
		if n := vendorDelta(f.inside); n == 0 {
			t.Errorf("%s: blokta kitaplık girişine gitmedi", f.name)
		}
		if n := vendorDelta(f.plain); n != 0 {
			t.Errorf("%s: blok dışında kitaplık girişine %d çarpım gitti", f.name, n)
		}
	}
	// TR_linear_cross_entropy'nin geri çarpımları ileri çağrının içinde, türev istenince koşar: türevsiz
	// yalnız logitler (C = X·W, üzerine yazma), türevli üç çarpım (logitler, dX'in parçası üzerine
	// yazarak, dW eklenerek).
	if n := vendorDelta(func() { tensor.LinearCrossEntropyVendor(x, w, []int{0, 1, 2, 3}) }); n != 1 {
		t.Errorf("TR_linear_cross_entropy türevsiz: kitaplık girişine %d çarpım, 1 bekleniyordu", n)
	}
	xg, wg := x.Detach().SetRequiresGrad(true), w.Detach().SetRequiresGrad(true)
	if n := vendorDelta(func() { tensor.LinearCrossEntropyVendor(xg, wg, []int{0, 1, 2, 3}) }); n != 3 {
		t.Errorf("TR_linear_cross_entropy türevli: kitaplık girişine %d çarpım, 3 bekleniyordu", n)
	}
	// M = 1 blokta da kendi çekirdektir: gemv, gemv kapalıyken de (döşemeli çekirdek).
	if n := vendorDelta(func() { tensor.MatMulVendor(x1, w, tensor.Float32) }); n != 0 {
		t.Errorf("M = 1: kitaplık girişine %d çarpım gitti", n)
	}
	g.gemvOff = true
	n := vendorDelta(func() { tensor.MatMulVendor(x1, w, tensor.Float32) })
	g.gemvOff = false
	if n != 0 {
		t.Errorf("M = 1, gemv kapalı: kitaplık girişine %d çarpım gitti", n)
	}
	// İşlemcide blok etkisizdir: bitler blok dışındakiyle aynı.
	xc, wc := x.To(tensor.CPU), w.To(tensor.CPU)
	sameBitsAt(t, "işlemcide blok", bitsOf(tensor.MatMulVendor(xc, wc, tensor.Float32)), bitsOf(tensor.MatMul(xc, wc)))
}

func TestVendorMatmulBackwardUsesRecordedState(t *testing.T) {
	if !inSim() {
		t.Skip("giriş sayacı benzeticide sınanır")
	}
	freshWarnings(t)
	rng := rand.New(rand.NewSource(83))
	xv, wv := random(rng, 4*16, 1), random(rng, 16*5, 1)
	for _, inside := range []bool{true, false} {
		x := tensor.NewOn(gpuDevice, xv, 4, 16).SetRequiresGrad(true)
		w := tensor.NewOn(gpuDevice, wv, 16, 5).SetRequiresGrad(true)
		var y *tensor.Tensor
		if inside {
			y = tensor.MatMulVendor(x, w, tensor.Float32)
		} else {
			y = tensor.MatMul(x, w)
		}
		loss := y.SumAll()
		n := vendorDelta(loss.Backward) // geri yön blok dışında çağrılır
		if inside && n != 2 {
			t.Errorf("blokta kaydedilen düğümün geri yönü: kitaplık girişine %d çarpım, 2 bekleniyordu", n)
		}
		if !inside && n != 0 {
			t.Errorf("blok dışında kaydedilen düğümün geri yönü: kitaplık girişine %d çarpım", n)
		}
	}
}

func TestVendorMatmulFallbackWarnsOnce(t *testing.T) {
	if !inSim() {
		t.Skip("düşüş kancayla benzeticide sınanır")
	}
	buf := freshWarnings(t)
	be := newSimBackend(1 << 24)
	be.vendorHook = func(batch, M, N, K int) bool { return false } // kitaplık çağrıyı yapamadı
	a, b, c := vendorCase(be, 5, 7, 9)
	be.GemmSet(1, 5, 7, 9, a, false, b, false, c)
	want := bitsOfStorage(be, c, 35)
	for i := 0; i < 2; i++ {
		be.GemmVendor(1, 5, 7, 9, a, false, b, false, c, true)
		sameBitsAt(t, "düşen çarpımın bitleri", bitsOfStorage(be, c, 35), want)
	}
	if n := countText(buf.String(), messages.H4131, "[5, 9] @ [9, 7]", ""); n != 1 {
		t.Fatalf("H4131 %d kez verildi, bir kez bekleniyordu:\n%s", n, buf.String())
	}
}

func TestVendorMatmulMissingLibraryWarnsOnce(t *testing.T) {
	if !inSim() {
		t.Skip("benzeticide kitaplık yoktur")
	}
	buf := freshWarnings(t)
	be := newSimBackend(1 << 24)
	a, b, c := vendorCase(be, 3, 4, 5)
	be.GemmVendor(1, 3, 4, 5, a, false, b, false, c, true)
	be.GemmVendor(1, 3, 4, 5, a, false, b, false, c, false)
	out := buf.String()
	if n := strings.Count(out, "\n"); n != 1 || !strings.Contains(out, exampleFor(runtime.GOOS)) {
		t.Fatalf("H4130 bir kez ve işletim sisteminin örneğiyle bekleniyordu:\n%s", out)
	}
	if be.lt != nil || !strings.HasPrefix(be.gemmPath(), messages.Text(messages.H4036)) {
		t.Fatalf("kitaplıksız blokta yol: %q", be.gemmPath())
	}
	for goos, want := range map[string]string{
		"linux":   "export TR_GPU_CUBLASLT=/.../libcublasLt.so.12; trlang ...",
		"windows": `$env:TR_GPU_CUBLASLT="C:\...\cublasLt64_12.dll"; trlang ...`,
	} {
		if got := exampleFor(goos); got != want {
			t.Errorf("%s örneği %q, %q bekleniyordu", goos, got, want)
		}
	}
}

func TestVendorMatmulSetupOnceConcurrent(t *testing.T) {
	if !inSim() {
		t.Skip("kurulum sayacı benzeticide sınanır")
	}
	buf := freshWarnings(t)
	be := newSimBackend(1 << 24)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		a, b, c := vendorCase(be, 3, 4, 5)
		wg.Add(1)
		go func() {
			defer wg.Done()
			be.GemmVendor(1, 3, 4, 5, a, false, b, false, c, true)
		}()
	}
	wg.Wait()
	if be.ltSetups != 1 || strings.Count(buf.String(), "\n") != 1 {
		t.Fatalf("kurulum %d kez, uyarı:\n%s", be.ltSetups, buf.String())
	}
}

func TestVendorMatmulFirstUseInsideReplay(t *testing.T) {
	freshWarnings(t)
	be := newSimBackend(1 << 24)
	if !be.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	var space *storage
	be.ltSetupHook = func() { space = be.allocate(1024, tensor.Float32, false) } // kitaplığın çalışma alanı
	a, b, c := vendorCase(be, 6, 5, 7)
	be.GemmSet(1, 6, 5, 7, a, false, b, false, c)
	want := bitsOfStorage(be, c, 30)
	be.Fill(c, 0)
	be.BeginCapture()
	priv := be.capturing
	be.GemmVendor(1, 6, 5, 7, a, false, b, false, c, true)
	rec := be.EndCapture()
	defer rec.Release()
	if space == nil || space.priv == priv {
		t.Fatalf("kitaplığın kurulumu kayıtta yapılmadı ya da çalışma alanı kaydın havuzunda (%v)", space)
	}
	be.Fill(c, 0)
	rec.Replay()
	sameBitsAt(t, "kayıt içinde ilk kurulum, oynatma", bitsOfStorage(be, c, 30), want)
}

func TestMixedPrecisionWithoutBlockWarns(t *testing.T) {
	if !inSim() {
		t.Skip("kitaplığın yüklenmediği benzeticide sınanır")
	}
	g := gpuBackend()
	rng := rand.New(rand.NewSource(85))
	x := tensor.NewOn(gpuDevice, random(rng, 4*16, 1), 4, 16).Cast(tensor.BFloat16)
	w := tensor.NewOn(gpuDevice, random(rng, 16*3, 1), 16, 3).Cast(tensor.BFloat16)
	if _, ok := gemmLibPresent(); !ok {
		buf := freshWarnings(t)
		tensor.MatMul(x, w)
		if buf.Len() != 0 {
			t.Fatalf("aday dosya yokken uyarı verildi:\n%s", buf.String())
		}
	}
	fake := filepath.Join(t.TempDir(), "libcublasLt.so.12")
	if err := os.WriteFile(fake, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TR_GPU_CUBLASLT", fake)
	buf := freshWarnings(t)
	tensor.MatMul(x, w)
	tensor.MatMul(x, w)
	if n := countText(buf.String(), messages.H4132, tensor.BFloat16.Name()); n != 1 {
		t.Fatalf("H4132 %d kez verildi, bir kez bekleniyordu:\n%s", n, buf.String())
	}
	if g.lt != nil {
		t.Fatal("blok dışında uyarı verilirken kitaplık yüklendi")
	}
}
