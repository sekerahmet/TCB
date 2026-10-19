package gpu

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestGemmLibraryMatchesOwnKernel gerçek GPU'da kitaplık bulunduysa onun sonucunu
// kendi çekirdeğimizinkiyle karşılaştırır (devrikler, toplu, ekleme ve üzerine yazma).
func TestGemmLibraryMatchesOwnKernel(t *testing.T) {
	g := gpuBackend()
	if g.lt == nil {
		t.Skip("kitaplık yok: " + g.gemmPath())
	}
	rng := rand.New(rand.NewSource(31))
	for _, d := range [][4]int{{1, 1, 1, 1}, {1, 130, 70, 33}, {3, 65, 129, 31}, {2, 512, 512, 512}, {1, 1000, 7, 1024}} {
		b, M, K, N := d[0], d[1], d[2], d[3]
		av, bv, cv := random(rng, b*M*K, 1), random(rng, b*K*N, 1), random(rng, b*M*N, 1)
		v := func(s tensor.Storage, n int) tensor.View {
			return tensor.View{S: s, Shape: []int{n}, Strides: []int{1}}
		}
		for _, ta := range []bool{false, true} {
			for _, tb := range []bool{false, true} {
				for _, uz := range []bool{false, true} {
					var result [2][]float32
					for path := 0; path < 2; path++ {
						a := g.FromHost(av, tensor.Float32)
						bb := g.FromHost(bv, tensor.Float32)
						c := g.FromHost(cv, tensor.Float32)
						lt := g.lt
						if path == 1 {
							g.lt = nil
						}
						if uz {
							g.GemmSet(b, M, N, K, v(a, b*M*K), ta, v(bb, b*K*N), tb, c)
						} else {
							g.Gemm(b, M, N, K, v(a, b*M*K), ta, v(bb, b*K*N), tb, c)
						}
						g.lt = lt
						result[path] = g.ToHost(v(c, b*M*N))
					}
					checkDiff(t, fmt.Sprintf("%v devrik %v/%v üzerine %v", d, ta, tb, uz), result[0], result[1], 1e-4)
				}
			}
		}
	}
}

// TestGemmLibraryMissingUsesOwnKernel benzetimde kitaplık yokken matris çarpımının kendi
// çekirdeğimizle yapıldığını ve yolun teşhiste göründüğünü denetler.
func TestGemmLibraryMissingUsesOwnKernel(t *testing.T) {
	if !inSim() {
		t.Skip("gerçek GPU: kitaplık sınaması TestGemmLibraryMatchesOwnKernel")
	}
	g := newSimBackend(1 << 24)
	g.ltSetup()
	if g.lt != nil {
		t.Skip("bu makinede kitaplık bulundu")
	}
	if y := g.gemmPath(); !strings.HasPrefix(y, "kendi çekirdeği") {
		t.Fatalf("çarpım yolu: %q", y)
	}
}

// TestGemmLibrarySearchPath arama yolunun yalnızca mutlak yerlerden oluştuğunu ve göreli
// bir TR_GPU_CUBLASLT'nin reddedildiğini denetler (kitaplık kaçırma).
func TestGemmLibrarySearchPath(t *testing.T) {
	for _, k := range gemmLibDirs() {
		if !strings.HasPrefix(k, "/") && !(len(k) > 2 && k[1] == ':') {
			t.Fatalf("göreli arama klasörü: %q", k)
		}
	}
	t.Setenv("TR_GPU_CUBLASLT", "libcublasLt.so.12")
	if _, _, err := gemmLibCandidates(); err == nil || !strings.Contains(err.Error(), "tam yol") {
		t.Fatalf("göreli yol reddedilmedi: %v", err)
	}
	if searchPathSafe("/usr/lib::/opt") || searchPathSafe("lib") || searchPathSafe("/a;.") ||
		!searchPathSafe("/a:/b") {
		t.Fatal("LD_LIBRARY_PATH denetimi yanlış")
	}
}

// TestGemmLibraryVersionAndAlign mimariye göre en düşük kitaplık sürümünü ve işaretçi hizası
// hesabını denetler.
func TestGemmLibraryVersionAndAlign(t *testing.T) {
	for cc, minN := range map[int]int{120: 120800, 100: 120800, 90: 110800, 89: 110800, 86: 110100, 80: 110000, 52: 110000} {
		if v := ltMinVersion(cc); v != minN {
			t.Fatalf("sm_%d için en az sürüm %d, %d bekleniyordu", cc, v, minN)
		}
	}
	for addr, h := range map[uint64]uint32{0: 256, 256: 256, 1 << 40: 256, 4: 4, 24: 8, 48: 16, 1<<20 + 96: 32, 1<<20 + 128: 128} {
		if v := align(addr); v != h {
			t.Fatalf("hiza(%d) = %d, %d bekleniyordu", addr, v, h)
		}
	}
}

// TestGemmOffsetView bitişik ama ofsetli (256 bayta hizasız) görünümle çarpım: gerçek
// GPU'da kitaplığa hizanın bildirildiğini, benzetimde kendi çekirdeğimizi sınar.
func TestGemmOffsetView(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for _, d := range [][3]int{{5, 6, 4}, {9, 70, 33}, {40, 128, 64}} {
		M, K, N := d[0], d[1], d[2]
		av, bv := random(rng, M*K, 1), random(rng, K*N, 1)
		compare(t, fmt.Sprintf("çarpım ofsetli %v", d), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			a := twin(av, true, M, K)[c]
			b := twin(bv, true, K, N)[c]
			// Son satır (B=1 üretimde son konumun gizli durumu) ve birinci satırdan başlayan dilim.
			return tensor.Add(tensor.MatMul(a.Slice(0, M-1, M), b).SumAll(), tensor.MatMul(a.Slice(0, 1, M), b).SumAll()),
				[]*tensor.Tensor{a, b}
		})
	}
}
