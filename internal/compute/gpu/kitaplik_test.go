package gpu

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestCarpimKitapligiKendiCekirdekleUyusur gerçek GPU'da kitaplık bulunduysa onun sonucunu
// kendi çekirdeğimizinkiyle karşılaştırır (devrikler, toplu, ekleme ve üzerine yazma).
func TestCarpimKitapligiKendiCekirdekleUyusur(t *testing.T) {
	g := gpuArkaUc()
	if g.lt == nil {
		t.Skip("kitaplık yok: " + g.carpimYolu())
	}
	rng := rand.New(rand.NewSource(31))
	for _, d := range [][4]int{{1, 1, 1, 1}, {1, 130, 70, 33}, {3, 65, 129, 31}, {2, 512, 512, 512}, {1, 1000, 7, 1024}} {
		b, M, K, N := d[0], d[1], d[2], d[3]
		av, bv, cv := rastgele(rng, b*M*K, 1), rastgele(rng, b*K*N, 1), rastgele(rng, b*M*N, 1)
		v := func(s tensor.Storage, n int) tensor.View {
			return tensor.View{S: s, Shape: []int{n}, Strides: []int{1}}
		}
		for _, ta := range []bool{false, true} {
			for _, tb := range []bool{false, true} {
				for _, uz := range []bool{false, true} {
					var sonuc [2][]float32
					for yol := 0; yol < 2; yol++ {
						a := g.FromHost(av, tensor.Float32)
						bb := g.FromHost(bv, tensor.Float32)
						c := g.FromHost(cv, tensor.Float32)
						lt := g.lt
						if yol == 1 {
							g.lt = nil
						}
						if uz {
							g.GemmSet(b, M, N, K, v(a, b*M*K), ta, v(bb, b*K*N), tb, c)
						} else {
							g.Gemm(b, M, N, K, v(a, b*M*K), ta, v(bb, b*K*N), tb, c)
						}
						g.lt = lt
						sonuc[yol] = g.ToHost(v(c, b*M*N))
					}
					farkDenetle(t, fmt.Sprintf("%v devrik %v/%v üzerine %v", d, ta, tb, uz), sonuc[0], sonuc[1], 1e-4)
				}
			}
		}
	}
}

// TestCarpimKitapligiYoksaKendiCekirdegi benzetimde kitaplık yokken matris çarpımının kendi
// çekirdeğimizle yapıldığını ve yolun teşhiste göründüğünü denetler.
func TestCarpimKitapligiYoksaKendiCekirdegi(t *testing.T) {
	if !benzetimde() {
		t.Skip("gerçek GPU: kitaplık sınaması TestCarpimKitapligiKendiCekirdekleUyusur")
	}
	g := yeniBenzetimArkaUcu(1 << 24)
	g.ltKur()
	if g.lt != nil {
		t.Skip("bu makinede kitaplık bulundu")
	}
	if y := g.carpimYolu(); !strings.HasPrefix(y, "kendi çekirdeği") {
		t.Fatalf("çarpım yolu: %q", y)
	}
}

// TestCarpimKitaplikAramaYolu arama yolunun yalnızca mutlak yerlerden oluştuğunu ve göreli
// bir TR_GPU_CUBLASLT'nin reddedildiğini denetler (kitaplık kaçırma).
func TestCarpimKitaplikAramaYolu(t *testing.T) {
	for _, k := range carpimKitaplikKlasorleri() {
		if !strings.HasPrefix(k, "/") && !(len(k) > 2 && k[1] == ':') {
			t.Fatalf("göreli arama klasörü: %q", k)
		}
	}
	t.Setenv("TR_GPU_CUBLASLT", "libcublasLt.so.12")
	if _, _, err := carpimKitaplikAdaylari(); err == nil || !strings.Contains(err.Error(), "tam yol") {
		t.Fatalf("göreli yol reddedilmedi: %v", err)
	}
	if aramaYoluGuvenli("/usr/lib::/opt") || aramaYoluGuvenli("lib") || aramaYoluGuvenli("/a;.") ||
		!aramaYoluGuvenli("/a:/b") {
		t.Fatal("LD_LIBRARY_PATH denetimi yanlış")
	}
}

// TestCarpimKitaplikSurumVeHiza mimariye göre en düşük kitaplık sürümünü ve işaretçi hizası
// hesabını denetler.
func TestCarpimKitaplikSurumVeHiza(t *testing.T) {
	for cc, enAz := range map[int]int{120: 120800, 100: 120800, 90: 110800, 89: 110800, 86: 110100, 80: 110000, 52: 110000} {
		if v := ltEnAzSurumu(cc); v != enAz {
			t.Fatalf("sm_%d için en az sürüm %d, %d bekleniyordu", cc, v, enAz)
		}
	}
	for adr, h := range map[uint64]uint32{0: 256, 256: 256, 1 << 40: 256, 4: 4, 24: 8, 48: 16, 1<<20 + 96: 32, 1<<20 + 128: 128} {
		if v := hiza(adr); v != h {
			t.Fatalf("hiza(%d) = %d, %d bekleniyordu", adr, v, h)
		}
	}
}

// TestCarpimOfsetliGorunum bitişik ama ofsetli (256 bayta hizasız) görünümle çarpım: gerçek
// GPU'da kitaplığa hizanın bildirildiğini, benzetimde kendi çekirdeğimizi sınar.
func TestCarpimOfsetliGorunum(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for _, d := range [][3]int{{5, 6, 4}, {9, 70, 33}, {40, 128, 64}} {
		M, K, N := d[0], d[1], d[2]
		av, bv := rastgele(rng, M*K, 1), rastgele(rng, K*N, 1)
		karsilastir(t, fmt.Sprintf("çarpım ofsetli %v", d), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			a := ikiz(av, true, M, K)[c]
			b := ikiz(bv, true, K, N)[c]
			// Son satır (B=1 üretimde son konumun gizli durumu) ve birinci satırdan başlayan dilim.
			return tensor.Add(tensor.MatMul(a.Slice(0, M-1, M), b).SumAll(), tensor.MatMul(a.Slice(0, 1, M), b).SumAll()),
				[]*tensor.Tensor{a, b}
		})
	}
}
