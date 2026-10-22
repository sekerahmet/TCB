package gpu

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
		v := flatView
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

// TestGemmLibrary16Bit gerçek GPU'da 16 bitlik girdilerin (bf16, f16) float32'ye açılmadan
// kitaplığa verildiğini (tensör çekirdekleri) ve sonucun, aynı girdiler float32'ye açılarak
// yapılan çarpımla float32 birikim yuvarlaması içinde aynı olduğunu denetler. C float32 ya da
// girdilerle aynı 16 bitlik türdedir; 16 bitlik C de ara tampon olmadan doğrudan yazılır,
// üzerine yazılarak ya da eski içeriğine eklenerek.
func TestGemmLibrary16Bit(t *testing.T) {
	g := gpuBackend()
	if g.lt == nil {
		t.Skip("kitaplık yok: " + g.gemmPath())
	}
	rng := rand.New(rand.NewSource(37))
	v := flatView
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, d := range [][4]int{{1, 64, 64, 64}, {3, 130, 72, 40}, {1, 1024, 512, 2048}} {
			b, M, K, N := d[0], d[1], d[2], d[3]
			av, bv, cv := random(rng, b*M*K, 1), random(rng, b*K*N, 1), random(rng, b*M*N, 1)
			for _, ct := range []tensor.DType{tensor.Float32, dt} {
				// 16 bitlik C bir kez yuvarlanır: iki yolun farkı en çok bir son basamaktır.
				tol := 1e-3
				if ct != tensor.Float32 {
					tol = 2e-2
				}
				for _, tr := range [][2]bool{{false, false}, {false, true}, {true, false}} {
					for _, set := range []bool{true, false} {
						var result [2][]float32
						for path := 0; path < 2; path++ {
							a, bb, c := g.FromHost(av, dt), g.FromHost(bv, dt), g.FromHost(cv, ct)
							before := g.lt.fallthroughCase
							lt := g.lt
							if path == 1 {
								g.lt = nil // girdiler float32'ye açılıp kendi çekirdeğimizle çarpılır
							}
							if set {
								g.GemmSet(b, M, N, K, v(a, b*M*K), tr[0], v(bb, b*K*N), tr[1], c)
							} else {
								g.Gemm(b, M, N, K, v(a, b*M*K), tr[0], v(bb, b*K*N), tr[1], c)
							}
							g.lt = lt
							if path == 0 && g.lt.fallthroughCase != before {
								t.Fatalf("%v %v C %v: kitaplık 16 bitlik çarpımı yapmadı (%s)", dt, d, ct, g.ltLastError)
							}
							result[path] = g.ToHost(v(c, b*M*N))
						}
						checkDiff(t, fmt.Sprintf("%v %v C %v devrik %v üzerine %v", dt, d, ct, tr, set), result[0], result[1], tol)
					}
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
	if y := g.gemmPath(); !strings.HasPrefix(y, messages.Text(messages.H4036)) {
		t.Fatalf("çarpım yolu: %q", y)
	}
}

// TestGemmLibrarySearchPath arama yolunun yalnızca mutlak yerlerden oluştuğunu ve göreli
// bir TR_GPU_CUBLASLT'nin reddedildiğini, LD_LIBRARY_PATH güvensizken sürücünün de yalnızca
// tam yolla arandığını denetler (kitaplık kaçırma).
func TestGemmLibrarySearchPath(t *testing.T) {
	for _, k := range gemmLibDirs() {
		if !strings.HasPrefix(k, "/") && !(len(k) > 2 && k[1] == ':') {
			t.Fatalf("göreli arama klasörü: %q", k)
		}
	}
	t.Setenv("TR_GPU_CUBLASLT", "libcublasLt.so.12")
	if _, _, err := gemmLibCandidates(); !messages.Has(err, messages.H4026) {
		t.Fatalf("göreli yol reddedilmedi: %v", err)
	}
	if searchPathSafe("/usr/lib::/opt") || searchPathSafe("lib") || searchPathSafe("/a;.") ||
		!searchPathSafe("/a:/b") {
		t.Fatal("LD_LIBRARY_PATH denetimi yanlış")
	}
	// Sürücü kitaplığı: güvenli arama yolunda dosya adıyla, güvensizde yalnızca tam yolla.
	if c := driverLibCandidates("/usr/lib64-nvidia"); c[0] != "libcuda.so.1" {
		t.Fatalf("güvenli arama yolunda dosya adı bekleniyordu: %v", c)
	}
	for _, ld := range []string{"/usr/lib::/opt", "lib", ".:/usr/lib"} {
		for _, c := range driverLibCandidates(ld) {
			if !strings.HasPrefix(c, "/") {
				t.Fatalf("LD_LIBRARY_PATH=%q iken göreli aday: %q", ld, c)
			}
		}
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
