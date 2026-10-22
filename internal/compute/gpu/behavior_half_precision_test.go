package gpu

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// 16 bitlik türler (bfloat16, float16) float32'ye açılmadan işlenir: çekirdekler 16 bitlik
// değerleri kendi türünde okur ve yazar, hesap float32'dedir. Bu dosyanın testleri bu
// davranışı birden çok çekirdek ailesinde (çapraz entropi, eleman işlemleri, indirgeme,
// normlar, softmax)
// CPU ile karşılaştırarak sınar; matris çarpımının 16 bitlik kitaplık yolu gemm_lib_test.go'da,
// tensör çekirdekli dikkat kernel_flash_mma*_test.go'dadır.

// TestCrossEntropy16Bit bfloat16 ve float16 logitlerle çapraz entropinin ileri ve geri yönünü
// denetler: çekirdek logitleri kendi türünde okur, gradyanı kendi türünde (float32 ya da
// logitlerle aynı 16 bitlik tür) yazar; üzerine ve iki kayıpta eskisine ekleyerek. bfloat16'da
// 4'lü vektör yolu, float16'da tekli yol.
func TestCrossEntropy16Bit(t *testing.T) {
	old := tensor.GradDType()
	defer tensor.SetGradDType(old)
	rng := rand.New(rand.NewSource(19))
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, gt := range []tensor.DType{tensor.Float32, dt} {
			tensor.SetGradDType(gt)
			for _, d := range [][2]int{{5, 7}, {9, 300}, {3, 2056}} {
				R, C := d[0], d[1]
				v := random(rng, R*C, 3)
				target, second := make([]int, R), make([]int, R)
				for i := range target {
					target[i] = rng.Intn(C)
					second[i] = (target[i] + 1) % C
				}
				target[1] = -1 // yok sayılan satır
				name := fmt.Sprintf("çapraz entropi %s (gradyan %s) %d×%d", dt, gt, R, C)
				compare(t, name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := twin(v, true, R, C)[c]
					return tensor.CrossEntropy(x.Cast(dt), target), []*tensor.Tensor{x}
				})
				compare(t, "iki "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := twin(v, true, R, C)[c]
					h := x.Cast(dt)
					return tensor.Add(tensor.CrossEntropy(h, target), tensor.CrossEntropy(h, second)), []*tensor.Tensor{x}
				})
			}
		}
	}
}

// TestElementwise16Bit 16 bitlik girdilerle tekli ve ikili işlemlerin ileri ve geri yönünü
// denetler: çekirdekler 16 bitlik değerleri kendi türünde okur ve yazar (float32'ye açmadan).
// İkili işlemlerde iki taraf 16 bit (sonuç 16 bit) ya da biri float32 (sonuç float32),
// yayınlı ve adımlı (devrik) görünümlerle; gradyan türü float32 ya da girdiyle aynıdır. Aynı
// tensörün iki kez kullanılması gradyanı eskisine ekler (biriktirme).
func TestElementwise16Bit(t *testing.T) {
	old := tensor.GradDType()
	defer tensor.SetGradDType(old)
	rng := rand.New(rand.NewSource(20))
	unary := []struct {
		ad string
		f  func(x *tensor.Tensor) *tensor.Tensor
	}{
		{"üstel", (*tensor.Tensor).Exp}, {"tanh", (*tensor.Tensor).Tanh}, {"gelu", (*tensor.Tensor).Gelu},
		{"sigmoid", (*tensor.Tensor).Sigmoid}, {"relu", (*tensor.Tensor).Relu}, {"kare", (*tensor.Tensor).Square},
		{"+s", func(x *tensor.Tensor) *tensor.Tensor { return x.AddScalar(1.5) }},
	}
	binary := []struct {
		ad string
		f  func(a, b *tensor.Tensor) *tensor.Tensor
	}{{"+", tensor.Add}, {"-", tensor.Sub}, {"*", tensor.Mul}, {"/", tensor.Div}}
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, gt := range []tensor.DType{tensor.Float32, dt} {
			tensor.SetGradDType(gt)
			name := fmt.Sprintf("%s (gradyan %s)", dt, gt)
			v := random(rng, 3*130, 1)
			for _, u := range unary {
				compare(t, u.ad+" "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := twin(v, true, 3, 130)[c]
					h := x.Cast(dt)
					return tensor.Add(u.f(h), h).Cast(tensor.Float32), []*tensor.Tensor{x}
				})
			}
			bv := random(rng, 130, 1)
			for i := range bv {
				bv[i] += 3 // bölmede sıfırdan uzak
			}
			for _, b := range binary {
				compare(t, "16+16 "+b.ad+" "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x, y := twin(v, true, 3, 130)[c], twin(bv, true, 130)[c]
					return b.f(x.Cast(dt), y.Cast(dt)).Cast(tensor.Float32), []*tensor.Tensor{x, y}
				})
				compare(t, "16+32 "+b.ad+" "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x, y := twin(v, true, 3, 130)[c], twin(bv, true, 130)[c]
					return b.f(x.Cast(dt), y), []*tensor.Tensor{x, y}
				})
				compare(t, "devrik 16 "+b.ad+" "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x, y := twin(v, true, 3, 130)[c], twin(bv, true, 130, 1)[c]
					return b.f(x.Cast(dt).Transpose(0, 1), y.Cast(dt)).Cast(tensor.Float32), []*tensor.Tensor{x, y}
				})
			}
		}
	}
}

// TestReduce16Bit 16 bitlik girdiyle eksen toplamını, hepsinin toplamını (tek ve iki aşamalı)
// ve gradyanlarını denetler; çıktı 16 bitlik türdedir.
func TestReduce16Bit(t *testing.T) {
	old := tensor.GradDType()
	defer tensor.SetGradDType(old)
	rng := rand.New(rand.NewSource(21))
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, gt := range []tensor.DType{tensor.Float32, dt} {
			tensor.SetGradDType(gt)
			name := fmt.Sprintf("%s (gradyan %s)", dt, gt)
			// 4096×3: indirgenen eksen uzun, çıktı az: iki aşamalı indirgeme.
			for _, sh := range [][2]int{{5, 130}, {4096, 3}} {
				v := random(rng, sh[0]*sh[1], 1)
				for axis := 0; axis < 2; axis++ {
					compare(t, fmt.Sprintf("toplam %v eksen %d %s", sh, axis, name), 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
						x := twin(v, true, sh[0], sh[1])[c]
						y := x.Cast(dt).Sum(axis, false)
						if y.DType() != dt {
							t.Fatalf("toplamın türü %s olmalıydı, %s", dt, y.DType())
						}
						return y.Cast(tensor.Float32), []*tensor.Tensor{x}
					})
				}
				compare(t, fmt.Sprintf("hepsinin toplamı %v %s", sh, name), 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x := twin(v, true, sh[0], sh[1])[c]
					return x.Cast(dt).SumAll().Cast(tensor.Float32), []*tensor.Tensor{x}
				})
			}
		}
	}
}

// TestNorms16Bit 16 bitlik x (ve γ, β) ile RMS normunu ve katman normunu, ileri ve geri
// yönde denetler: çıktı x'in türündedir.
func TestNorms16Bit(t *testing.T) {
	old := tensor.GradDType()
	defer tensor.SetGradDType(old)
	rng := rand.New(rand.NewSource(22))
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, gt := range []tensor.DType{tensor.Float32, dt} {
			tensor.SetGradDType(gt)
			// D = 32, 64, 256 ileri ve geri, 512 yalnızca geride warp çekirdeğidir.
			for _, d := range [][2]int{{3, 8}, {5, 129}, {2, 600}, {37, 64}, {9, 256}, {300, 512}, {3, 32}} {
				R, D := d[0], d[1]
				xv, gv, bv := random(rng, R*D, 2), random(rng, D, 1), random(rng, D, 1)
				name := fmt.Sprintf("%s (gradyan %s) %d×%d", dt, gt, R, D)
				compare(t, "rms normu "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x, gm := twin(xv, true, R, D)[c], twin(gv, true, D)[c]
					y := tensor.RMSNorm(x.Cast(dt), gm.Cast(dt), 1e-5)
					if y.DType() != dt {
						t.Fatalf("normun türü %s olmalıydı, %s", dt, y.DType())
					}
					return y.Cast(tensor.Float32), []*tensor.Tensor{x, gm}
				})
				compare(t, "katman normu (γ, β float32) "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					x, gm, bt := twin(xv, true, R, D)[c], twin(gv, true, D)[c], twin(bv, true, D)[c]
					return tensor.LayerNorm(x.Cast(dt), gm, bt, 1e-5).Cast(tensor.Float32), []*tensor.Tensor{x, gm, bt}
				})
			}
		}
	}
}

// TestNormsOutputDType float32 girdili normun çıktısını 16 bitte yazar (RMSNormAs, LayerNormAs;
// dil tanımı 15.13): değer float32 normun o türe dönüşümüyle bit düzeyinde aynıdır, ileri ve
// geri yön CPU ile karşılaştırılır.
func TestNormsOutputDType(t *testing.T) {
	rng := rand.New(rand.NewSource(24))
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, d := range [][2]int{{3, 8}, {5, 129}, {2, 600}, {37, 64}, {9, 256}} {
			R, D := d[0], d[1]
			xv, gv, bv := random(rng, R*D, 2), random(rng, D, 1), random(rng, D, 1)
			x, gm, bt := tensor.NewOn(gpuDevice, xv, R, D), tensor.NewOn(gpuDevice, gv, D), tensor.NewOn(gpuDevice, bv, D)
			name := fmt.Sprintf("%s %d×%d", dt, R, D)
			bitsEqual(t, "rms normu çıktısı "+name, tensor.RMSNormAs(x, gm, 1e-5, dt), tensor.RMSNorm(x, gm, 1e-5).Cast(dt))
			bitsEqual(t, "katman normu çıktısı "+name, tensor.LayerNormAs(x, gm, bt, 1e-5, dt), tensor.LayerNorm(x, gm, bt, 1e-5).Cast(dt))
			compare(t, "rms normu "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x, gm := twin(xv, true, R, D)[c], twin(gv, true, D)[c]
				y := tensor.RMSNormAs(x, gm, 1e-5, dt)
				if y.DType() != dt {
					t.Fatalf("normun türü %s olmalıydı, %s", dt, y.DType())
				}
				return y.Cast(tensor.Float32), []*tensor.Tensor{x, gm}
			})
			compare(t, "katman normu "+name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				x, gm, bt := twin(xv, true, R, D)[c], twin(gv, true, D)[c], twin(bv, true, D)[c]
				return tensor.LayerNormAs(x, gm, bt, 1e-5, dt).Cast(tensor.Float32), []*tensor.Tensor{x, gm, bt}
			})
		}
	}
}

// TestSoftmax16Bit 16 bitlik girdiyle softmax ve log_softmax'ı, son ve iç eksende, ileri ve
// geri yönde denetler.
func TestSoftmax16Bit(t *testing.T) {
	old := tensor.GradDType()
	defer tensor.SetGradDType(old)
	rng := rand.New(rand.NewSource(23))
	v := random(rng, 4*70*3, 2)
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, gt := range []tensor.DType{tensor.Float32, dt} {
			tensor.SetGradDType(gt)
			for _, logMode := range []bool{false, true} {
				for _, ax := range []int{2, 1} {
					name := fmt.Sprintf("softmax log=%v eksen %d %s (gradyan %s)", logMode, ax, dt, gt)
					compare(t, name, 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
						x := twin(v, true, 4, 70, 3)[c]
						h := x.Cast(dt)
						y := h.Softmax(ax)
						if logMode {
							y = h.LogSoftmax(ax)
						}
						return y.Cast(tensor.Float32), []*tensor.Tensor{x}
					})
				}
			}
		}
	}
}

// TestSharedGradStorage16Bit aynı tensör iki girdi olarak verildiğinde (atan2(x, x),
// katman_normu(x, w, w)) 16 bitlik gradyanın iki katkıyı da topladığını (biri ötekini
// ezmeden) CPU ile karşılaştırır.
func TestSharedGradStorage16Bit(t *testing.T) {
	old := tensor.GradDType()
	defer tensor.SetGradDType(old)
	rng := rand.New(rand.NewSource(25))
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		tensor.SetGradDType(dt)
		v, wv := random(rng, 3*8, 1), random(rng, 8, 1)
		compare(t, "atan2(x, x) "+dt.String(), 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, false, 3, 8)[c].Cast(dt).SetRequiresGrad(true)
			return tensor.Atan2(x, x).Cast(tensor.Float32), []*tensor.Tensor{x}
		})
		compare(t, "katman_normu(x, w, w) "+dt.String(), 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, true, 3, 8)[c]
			w := twin(wv, false, 8)[c].Cast(dt).SetRequiresGrad(true)
			return tensor.LayerNorm(x.Cast(dt), w, w, 1e-5).Cast(tensor.Float32), []*tensor.Tensor{x, w}
		})
	}
}

// Adımlı satırlar (K yarısı) 16 bitte warp çekirdeklerinde CPU ile aynıdır (boylar TestNorms16Bit'te).
func TestNormsWarp16Bit(t *testing.T) {
	old := tensor.GradDType()
	defer tensor.SetGradDType(old)
	rng := rand.New(rand.NewSource(23))
	for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
		for _, gt := range []tensor.DType{tensor.Float32, dt} {
			tensor.SetGradDType(gt)
			T, H, D := 11, 3, 64
			kv, gv := random(rng, T*2*H*D, 2), random(rng, D, 1)
			compare(t, fmt.Sprintf("adımlı %s (gradyan %s)", dt, gt), 2e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				base, gm := twin(kv, true, T, 2, H, D)[c], twin(gv, true, D)[c]
				return tensor.RMSNorm(base.Cast(dt).Slice(1, 0, 1), gm.Cast(dt), 1e-5).Cast(tensor.Float32), []*tensor.Tensor{base, gm}
			})
		}
	}
}
