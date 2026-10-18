package gpu

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"tr_dili/internal/hesap/tensor"
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
	if _, _, err := carpimKitapligiAc(); err == nil || !strings.Contains(err.Error(), "tam yol") {
		t.Fatalf("göreli yol reddedilmedi: %v", err)
	}
	if aramaYoluGuvenli("/usr/lib::/opt") || aramaYoluGuvenli("lib") || !aramaYoluGuvenli("/a:/b") {
		t.Fatal("LD_LIBRARY_PATH denetimi yanlış")
	}
}
