package ml

import (
	"math"

	_ "github.com/sekerahmet/TCB/internal/compute/gpu" // GPU arka ucunu kaydeder (sürücü ilk kullanımda yüklenir)
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Register tensör yerleşiklerini VM'ye kaydeder. Adlar kod listesindedir (internal/lang/codes); burada
// yalnızca kodlar geçer. Aynı adlı çekirdek ve yapay zekâ yerleşikleri (toplam, ortalama,
// min, max, karekök) sarılır: argüman tensörse tensör yolu, değilse eski yerleşik çalışır.
func Register(v *interp.VM) {
	// Türevsiz kip görev başınadır (gorev.go).
	v.AddTaskHook(taskHook)

	registerConstructors(v)
	registerMath(v)
	registerWideMath(v) // yerlesik_matematik.go

	// --- Sinir ağı ---
	wrap(v, codes.FnEmbedding, newBuiltin(embedding))
	wrap(v, codes.FnCrossEntropy, newBuiltin(crossEntropy))
	wrap(v, codes.FnLinearCrossEntropy, newBuiltin(linearCrossEntropy))
	wrap(v, codes.FnLayerNorm, newBuiltin(layerNorm))
	wrap(v, codes.FnRmsNorm, newBuiltin(rmsNorm))
	wrap(v, codes.FnAttention, newBuiltin(attentionBuiltin)) // model_araclari.go
	wrap(v, codes.FnParameters, newBuiltin(paramsBuiltin))
	wrap(v, codes.FnSample, newBuiltin(sampleBuiltin))

	// --- Türev ---
	wrap(v, codes.FnBackward, newBuiltin(backpropagation))
	wrap(v, codes.FnNoGrad, newBuiltin(noGrad))
	registerCustomGrad(v) // özel_türev, türev_denetle (ozel_turev.go)
	registerFuse(v)       // kaynaştır (builtin_fuse.go)
	registerReplay(v)     // tekrar_oynat, yerine_yaz (builtin_replay.go)

	// --- Veri türleri (veri_turu.go) ---
	wrap(v, codes.FnMixedPrecision, newBuiltin(mixedPrecision))
	wrap(v, codes.FnComputeDtype, newBuiltin(computeDType))

	// --- Cihazlar (cihaz.go) ---
	wrap(v, codes.FnDefaultDevice, newBuiltin(defaultDeviceBuiltin))
	wrap(v, codes.FnDevices, newBuiltin(devicesBuiltin))

	// --- İyileştiriciler ---
	wrap(v, codes.FnAdamW, newBuiltin(setupAdamW))
	wrap(v, codes.FnSGD, newBuiltin(setupSGD))
	wrap(v, codes.FnClipGrad, newBuiltin(gradClip))

	// --- İndeks, kaydetme ve yükleme ---
	wrap(v, codes.FnArgmax, newBuiltin(argMax))
	wrap(v, codes.FnSaveTensors, newBuiltin(tensorSave))
	wrap(v, codes.FnLoadTensors, newBuiltin(tensorLoad))
	wrap(v, codes.FnLoadParameters, newBuiltin(loadParamsBuiltin)) // parametreler.go
}

// registerConstructors tensör oluşturucularını (matris ve görünüm dahil) kaydeder.
func registerConstructors(v *interp.VM) {
	// --- Oluşturucular ---
	wrap(v, codes.FnTensor, newBuiltin(tensorBuiltin))
	wrap(v, codes.FnZeros, newBuiltin(shapedConstructor(codes.FnZeros, func(sh []int, _ map[string]interp.Value) (*tensor.Tensor, error) {
		return tensor.Zeros(sh...), nil
	})))
	wrap(v, codes.FnOnes, newBuiltin(shapedConstructor(codes.FnOnes, func(sh []int, _ map[string]interp.Value) (*tensor.Tensor, error) {
		return tensor.Ones(sh...), nil
	})))
	wrap(v, codes.FnRandn, newBuiltin(shapedConstructor(codes.FnRandn, randomNormal)))

	// --- Matris (2 boyutlu tensör) oluşturucusu (matris.go) ---
	wrap(v, codes.FnEye, newBuiltin(identity))

	// --- Görünüm ---
	wrap(v, codes.FnConcat, newBuiltin(concatBuiltin)) // model_araclari.go
}

// registerMath eleman bazlı fonksiyonları, softmax'ı ve indirgemeleri kaydeder.
func registerMath(v *interp.VM) {
	// --- Eleman bazlı matematik ---
	unit(v, codes.FnRelu, (*tensor.Tensor).Relu, relu64)
	unit(v, codes.FnSigmoid, (*tensor.Tensor).Sigmoid, sigmoid64)
	unit(v, codes.FnTanh, (*tensor.Tensor).Tanh, math.Tanh)
	registerGelu(v)
	unit(v, codes.FnSqrt, (*tensor.Tensor).Sqrt, nil) // karekök
	unit(v, codes.FnExp, (*tensor.Tensor).Exp, math.Exp)
	unit(v, codes.FnLog, (*tensor.Tensor).Log, math.Log)
	unit(v, codes.FnSin, (*tensor.Tensor).Sin, math.Sin)
	unit(v, codes.FnCos, (*tensor.Tensor).Cos, math.Cos)

	// --- Softmax ---
	registerSoftmax(v, codes.FnSoftmax, false)
	registerSoftmax(v, codes.FnLogSoftmax, true)

	// --- İndirgemeler (toplam, ortalama, max, min) ---
	reduction(v, codes.FnSum, func(t *tensor.Tensor, e int, whole, keep bool) *tensor.Tensor {
		if whole {
			return t.SumAll()
		}
		return t.Sum(e, keep)
	})
	reduction(v, codes.FnMean, func(t *tensor.Tensor, e int, whole, keep bool) *tensor.Tensor {
		if whole {
			return t.MeanAll()
		}
		return t.Mean(e, keep)
	})
	reduction(v, codes.FnMax, func(t *tensor.Tensor, e int, whole, keep bool) *tensor.Tensor {
		if whole {
			flat := t.Reshape(-1)
			r := flat.Max(0, false)
			dropTemp(t, flat)
			return r
		}
		return t.Max(e, keep)
	})
	reduction(v, codes.FnMin, func(t *tensor.Tensor, e int, whole, keep bool) *tensor.Tensor {
		if whole {
			return minAll(t)
		}
		neg := t.Neg()
		mx := neg.Max(e, keep)
		r := mx.Neg()
		dropTemp(t, neg, mx)
		return r
	})
}

// dropTemp bir hesabın ara başlıklarını bırakır (sonuç başka başlıktır; keep çağıranın
// girdisidir, dokunulmaz). Türev izlenen ara başlık çizgenin ebeveynidir, bırakılmaz
// (bkz. tensor/lifetime.go).
func dropTemp(keep *tensor.Tensor, temps ...*tensor.Tensor) {
	for _, t := range temps {
		if t != keep && !(tensor.GradEnabled() && t.RequiresGrad()) {
			t.Drop()
		}
	}
}

// minAll tüm elemanların en küçüğünü 0 boyutlu tensör olarak verir. Çekirdekte Min
// işlemi yoktur; türev gerekiyorsa ya da tensör CPU'da değilse -max(-x) ile (gradyan doğru
// elemana gider; GPU'da konağa okumaz, tekrar_oynat kaydında da çalışır) hesaplanır. CPU'da
// türev gerekmiyorsa ara tensör ayırmadan tek geçişle bulunur.
func minAll(t *tensor.Tensor) *tensor.Tensor {
	if t.RequiresGrad() && tensor.GradEnabled() || t.Device() != tensor.CPU {
		return t.Neg().Reshape(-1).Max(0, false).Neg() // türev izlenir: ara başlıklar çizgededir
	}
	data := t.Data()
	m := data[0]
	for _, x := range data[1:] {
		if x < m || x != x { // NaN yayılır (max ile tutarlı)
			m = x
			if x != x {
				break
			}
		}
	}
	return tensor.Scalar(m).Cast(t.DType()).To(t.Device()) // girdinin türünde (15.4)
}

// wrap kodun bütün yazımlarına (ad ve sözlüğe göre Türkçe karaktersiz biçimi) aynı yeni
// yerleşiği bağlar. Eski yerleşik (varsa) bu koddan okunur ve yeni fonksiyona verilir.
// Yerleşik guarded ile sarılır: hesap çekirdeğinin paniği, yerleşik onu kendisi safe ile
// sarmayı unutsa da yakalanabilir bir TR hatası olur.
func wrap(v *interp.VM, kd codes.Code, setup func(old interp.BuiltinFunc) interp.BuiltinFunc) {
	var old interp.BuiltinFunc
	if b := v.Builtin(kd); b != nil {
		old = b.Fn
	}
	v.DefineBuiltin(kd, &interp.BuiltinVal{Name: codes.Name(kd), Fn: guarded(setup(old)), Pure: !retaining[kd]})
}

// retaining argümanlarını (ya da onlardan kurduğu değerleri) saklayan veya TR geri
// çağrıları yapan yerleşiklerdir; argümanları kaçar. Diğer tensör yerleşikleri saftır:
// argümanlarını saklamaz, sonucunda döndürmez (interp.BuiltinVal.Pure).
var retaining = map[codes.Code]bool{
	codes.FnBackward:       true, // geri_yayılım: özel türevlerin geri çağrıları
	codes.FnNoGrad:         true, // türevsiz
	codes.FnCustomGrad:     true, // özel_türev: ileri ve geri fonksiyonlarını saklar
	codes.FnGradCheck:      true, // türev_denetle
	codes.FnFuse:           true, // kaynaştır: fonksiyonu saklar
	codes.FnReplay:         true, // tekrar_oynat: fonksiyonu saklar, TR geri çağrısı yapar
	codes.FnAdamW:          true, // AdamW: parametre listesini saklar
	codes.FnSGD:            true, // SGD
	codes.FnMixedPrecision: true, // karışık_hassasiyet: blok
}

// guarded fn'i safe içinde çağırır: çekirdeğin paniği (ör. GPU belleği yetmedi) dene/yakala ile
// yakalanabilen bir hata olur ve kodunu korur; yorumlayıcının en üst kurtarmasına çıkıp iç hata
// sayılmaz. Kesme (Ctrl+C) paniği hata değildir, geçer (bind.Recover).
// Kapanışsız doğrudan defer: her çağrıda yığına ayırma olmasın (sayıyla çağrılan yerleşikler
// sıcak döngüde de kullanılır).
func guarded(fn interp.BuiltinFunc) interp.BuiltinFunc {
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (result interp.Value, err error) {
		defer func() {
			if r := recover(); r != nil {
				if interp.IsInterrupt(r) {
					panic(r) // kesilme (Ctrl+C) hata değildir; yığın çözülmeye devam eder
				}
				result, err = nil, panicError(r)
			}
		}()
		start := tensor.Created()
		result, err = fn(m, args, named)
		if err == nil {
			markFresh(result, start)
		}
		return result, err
	}
}

// oldCall eski yerleşiği çağırır; yoksa argümanın tensör olmadığını bildirir.
func oldCall(ad any, old interp.BuiltinFunc, m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if old != nil {
		return old(m, args, named)
	}
	if len(args) == 0 {
		return nil, messages.New(messages.H6029, ad)
	}
	return nil, messages.New(messages.H6030, ad, typeName(args[0]))
}

// newBuiltin eski karşılığı olmayan bir yerleşik kurar.
func newBuiltin(fn interp.BuiltinFunc) func(interp.BuiltinFunc) interp.BuiltinFunc {
	return func(interp.BuiltinFunc) interp.BuiltinFunc { return fn }
}

func isTensor(args []interp.Value) bool {
	for _, a := range args {
		if _, ok := a.(*TensorValue); ok {
			return true
		}
	}
	return false
}
