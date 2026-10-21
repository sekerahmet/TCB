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
// min, maks, karekök) sarılır: argüman tensörse tensör yolu, değilse eski yerleşik çalışır.
func Register(v *interp.VM) {
	// Türevsiz kip görev başınadır (gorev.go).
	v.AddTaskHook(taskHook)

	registerConstructors(v)
	registerMath(v)
	registerWideMath(v) // yerlesik_matematik.go

	// --- Sinir ağı ---
	wrap(v, codes.T500, newBuiltin(embedding))
	wrap(v, codes.T510, newBuiltin(crossEntropy))
	wrap(v, codes.T512, newBuiltin(linearCrossEntropy))
	wrap(v, codes.T501, newBuiltin(layerNorm))
	wrap(v, codes.T502, newBuiltin(rmsNorm))
	wrap(v, codes.T503, newBuiltin(attentionBuiltin)) // model_araclari.go
	wrap(v, codes.T530, newBuiltin(paramsBuiltin))
	wrap(v, codes.T611, newBuiltin(sampleBuiltin))

	// --- Türev ---
	wrap(v, codes.T300, newBuiltin(backpropagation))
	wrap(v, codes.T301, newBuiltin(noGrad))
	registerCustomGrad(v) // özel_türev, türev_denetle (ozel_turev.go)
	registerFuse(v)       // kaynaştır (builtin_fuse.go)

	// --- Veri türleri (veri_turu.go) ---
	wrap(v, codes.T710, newBuiltin(mixedPrecision))

	// --- Cihazlar (cihaz.go) ---
	wrap(v, codes.T720, newBuiltin(defaultDeviceBuiltin))
	wrap(v, codes.T721, newBuiltin(devicesBuiltin))

	// --- İyileştiriciler ---
	wrap(v, codes.T400, newBuiltin(setupAdamW))
	wrap(v, codes.T401, newBuiltin(setupSGD))
	wrap(v, codes.T402, newBuiltin(gradClip))

	// --- İndeks, kaydetme ve yükleme ---
	wrap(v, codes.T230, newBuiltin(argMax))
	wrap(v, codes.T700, newBuiltin(tensorSave))
	wrap(v, codes.T701, newBuiltin(tensorLoad))
	wrap(v, codes.T703, newBuiltin(loadParamsBuiltin)) // parametreler.go
}

// registerConstructors tensör oluşturucularını (matris ve görünüm dahil) kaydeder.
func registerConstructors(v *interp.VM) {
	// --- Oluşturucular ---
	wrap(v, codes.T100, newBuiltin(tensorBuiltin))
	wrap(v, codes.T101, newBuiltin(shapedConstructor(codes.T101, func(sh []int, _ map[string]interp.Value) (*tensor.Tensor, error) {
		return tensor.Zeros(sh...), nil
	})))
	wrap(v, codes.T102, newBuiltin(shapedConstructor(codes.T102, func(sh []int, _ map[string]interp.Value) (*tensor.Tensor, error) {
		return tensor.Ones(sh...), nil
	})))
	wrap(v, codes.T103, newBuiltin(shapedConstructor(codes.T103, randomNormal)))

	// --- Matris (2 boyutlu tensör) oluşturucusu (matris.go) ---
	wrap(v, codes.T111, newBuiltin(identity))

	// --- Görünüm ---
	wrap(v, codes.T121, newBuiltin(concatBuiltin)) // model_araclari.go
}

// registerMath eleman bazlı fonksiyonları, softmax'ı ve indirgemeleri kaydeder.
func registerMath(v *interp.VM) {
	// --- Eleman bazlı matematik ---
	unit(v, codes.T200, (*tensor.Tensor).Relu, relu64)
	unit(v, codes.T201, (*tensor.Tensor).Sigmoid, sigmoid64)
	unit(v, codes.T202, (*tensor.Tensor).Tanh, math.Tanh)
	registerGelu(v)
	unit(v, codes.G404, (*tensor.Tensor).Sqrt, nil) // karekök
	unit(v, codes.T220, (*tensor.Tensor).Exp, math.Exp)
	unit(v, codes.T221, (*tensor.Tensor).Log, math.Log)
	unit(v, codes.T222, (*tensor.Tensor).Sin, math.Sin)
	unit(v, codes.T223, (*tensor.Tensor).Cos, math.Cos)

	// --- Softmax ---
	registerSoftmax(v, codes.T210, false)
	registerSoftmax(v, codes.T211, true)

	// --- İndirgemeler (toplam, ortalama, maks, min) ---
	reduction(v, codes.G400, func(t *tensor.Tensor, e int, whole, keep bool) *tensor.Tensor {
		if whole {
			return t.SumAll()
		}
		return t.Sum(e, keep)
	})
	reduction(v, codes.G410, func(t *tensor.Tensor, e int, whole, keep bool) *tensor.Tensor {
		if whole {
			return t.MeanAll()
		}
		return t.Mean(e, keep)
	})
	reduction(v, codes.G402, func(t *tensor.Tensor, e int, whole, keep bool) *tensor.Tensor {
		if whole {
			return t.Reshape(-1).Max(0, false)
		}
		return t.Max(e, keep)
	})
	reduction(v, codes.G401, func(t *tensor.Tensor, e int, whole, keep bool) *tensor.Tensor {
		if whole {
			return minAll(t)
		}
		return t.Neg().Max(e, keep).Neg()
	})
}

// minAll tüm elemanların en küçüğünü 0 boyutlu tensör olarak verir. Çekirdekte Min
// işlemi yoktur; türev gerekiyorsa -maks(-x) ile (gradyan doğru elemana gider) hesaplanır.
// Türev gerekmiyorsa ara tensör ayırmadan konakta tek geçişle bulunur.
func minAll(t *tensor.Tensor) *tensor.Tensor {
	if t.RequiresGrad() && tensor.GradEnabled() {
		return t.Neg().Reshape(-1).Max(0, false).Neg()
	}
	data := t.Data()
	m := data[0]
	for _, x := range data[1:] {
		if x < m || x != x { // NaN yayılır (maks ile tutarlı)
			m = x
			if x != x {
				break
			}
		}
	}
	return tensor.Scalar(m).To(t.Device())
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
	codes.T300: true, // geri_yayılım: özel türevlerin geri çağrıları
	codes.T301: true, // türevsiz
	codes.T310: true, // özel_türev: ileri ve geri fonksiyonlarını saklar
	codes.T311: true, // türev_denetle
	codes.T312: true, // kaynaştır: fonksiyonu saklar
	codes.T400: true, // AdamW: parametre listesini saklar
	codes.T401: true, // SGD
	codes.T710: true, // karışık_hassasiyet: blok
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
func oldCall(ad string, old interp.BuiltinFunc, m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
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
