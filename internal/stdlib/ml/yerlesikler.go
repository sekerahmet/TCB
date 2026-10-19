package ml

import (
	"fmt"
	"math"

	_ "github.com/sekerahmet/TCB/internal/compute/gpu" // GPU arka ucunu kaydeder (sürücü ilk kullanımda yüklenir)
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Kaydet tensör yerleşiklerini VM'ye kaydeder. Adlar kod listesindedir (internal/lang/codes); burada
// yalnızca kodlar geçer. Aynı adlı çekirdek ve yapay zekâ yerleşikleri (toplam, ortalama,
// min, maks, karekök) sarılır: argüman tensörse tensör yolu, değilse eski yerleşik çalışır.
func Kaydet(v *interp.VM) {
	// Türevsiz kip görev başınadır (gorev.go).
	v.AddTaskHook(gorevKancasi)

	olusturuculariKaydet(v)
	matematigiKaydet(v)
	matematigiGenisKaydet(v) // yerlesik_matematik.go

	// --- Sinir ağı ---
	sar(v, codes.T500, yeniYerlesik(gomme))
	sar(v, codes.T510, yeniYerlesik(caprazEntropi))
	sar(v, codes.T501, yeniYerlesik(katmanNormu))
	sar(v, codes.T502, yeniYerlesik(rmsNormu))
	sar(v, codes.T503, yeniYerlesik(dikkatYerlesik)) // model_araclari.go
	sar(v, codes.T530, yeniYerlesik(parametrelerYerlesik))
	sar(v, codes.T611, yeniYerlesik(orneklemeYerlesik))

	// --- Türev ---
	sar(v, codes.T300, yeniYerlesik(geriYayilim))
	sar(v, codes.T301, yeniYerlesik(turevsiz))
	ozelTurevKaydet(v) // özel_türev, türev_denetle (ozel_turev.go)

	// --- Veri türleri (veri_turu.go) ---
	sar(v, codes.T710, yeniYerlesik(karisikHassasiyet))

	// --- Cihazlar (cihaz.go) ---
	sar(v, codes.T720, yeniYerlesik(varsayilanCihazYerlesik))
	sar(v, codes.T721, yeniYerlesik(cihazlarYerlesik))

	// --- İyileştiriciler ---
	sar(v, codes.T400, yeniYerlesik(adamWKur))
	sar(v, codes.T401, yeniYerlesik(sgdKur))
	sar(v, codes.T402, yeniYerlesik(gradyanKirp))

	// --- İndeks, kaydetme ve yükleme ---
	sar(v, codes.T230, yeniYerlesik(maksIndeks))
	sar(v, codes.T700, yeniYerlesik(tensorKaydet))
	sar(v, codes.T701, yeniYerlesik(tensorYukle))
	sar(v, codes.T703, yeniYerlesik(parametreleriYukleYerlesik)) // parametreler.go
}

// olusturuculariKaydet tensör oluşturucularını (matris ve görünüm dahil) kaydeder.
func olusturuculariKaydet(v *interp.VM) {
	// --- Oluşturucular ---
	sar(v, codes.T100, yeniYerlesik(tensorYerlesik))
	sar(v, codes.T101, yeniYerlesik(sekilliOlusturucu(codes.T101, func(sh []int, _ map[string]interp.Value) (*tensor.Tensor, error) {
		return tensor.Zeros(sh...), nil
	})))
	sar(v, codes.T102, yeniYerlesik(sekilliOlusturucu(codes.T102, func(sh []int, _ map[string]interp.Value) (*tensor.Tensor, error) {
		return tensor.Ones(sh...), nil
	})))
	sar(v, codes.T103, yeniYerlesik(sekilliOlusturucu(codes.T103, rastgeleNormal)))

	// --- Matris (2 boyutlu tensör) oluşturucusu (matris.go) ---
	sar(v, codes.T111, yeniYerlesik(birimMatris))

	// --- Görünüm ---
	sar(v, codes.T121, yeniYerlesik(birlestirYerlesik)) // model_araclari.go
}

// matematigiKaydet eleman bazlı fonksiyonları, softmax'ı ve indirgemeleri kaydeder.
func matematigiKaydet(v *interp.VM) {
	// --- Eleman bazlı matematik ---
	birli(v, codes.T200, (*tensor.Tensor).Relu, relu64)
	birli(v, codes.T201, (*tensor.Tensor).Sigmoid, sigmoid64)
	birli(v, codes.T202, (*tensor.Tensor).Tanh, math.Tanh)
	birli(v, codes.T203, (*tensor.Tensor).Gelu, gelu64)
	birli(v, codes.G404, (*tensor.Tensor).Sqrt, nil) // karekök
	birli(v, codes.T220, (*tensor.Tensor).Exp, math.Exp)
	birli(v, codes.T221, (*tensor.Tensor).Log, math.Log)
	birli(v, codes.T222, (*tensor.Tensor).Sin, math.Sin)
	birli(v, codes.T223, (*tensor.Tensor).Cos, math.Cos)

	// --- Softmax ---
	softmaxKaydet(v, codes.T210, false)
	softmaxKaydet(v, codes.T211, true)

	// --- İndirgemeler (toplam, ortalama, maks, min) ---
	indirgeme(v, codes.G400, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.SumAll()
		}
		return t.Sum(e, koru)
	})
	indirgeme(v, codes.G410, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.MeanAll()
		}
		return t.Mean(e, koru)
	})
	indirgeme(v, codes.G402, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.Reshape(-1).Max(0, false)
		}
		return t.Max(e, koru)
	})
	indirgeme(v, codes.G401, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return enKucukTum(t)
		}
		return t.Neg().Max(e, koru).Neg()
	})
}

// enKucukTum tüm elemanların en küçüğünü 0 boyutlu tensör olarak verir. Çekirdekte Min
// işlemi yoktur; türev gerekiyorsa -maks(-x) ile (gradyan doğru elemana gider) hesaplanır.
// Türev gerekmiyorsa ara tensör ayırmadan konakta tek geçişle bulunur.
func enKucukTum(t *tensor.Tensor) *tensor.Tensor {
	if t.RequiresGrad() && tensor.GradEnabled() {
		return t.Neg().Reshape(-1).Max(0, false).Neg()
	}
	veri := t.Data()
	m := veri[0]
	for _, x := range veri[1:] {
		if x < m || x != x { // NaN yayılır (maks ile tutarlı)
			m = x
			if x != x {
				break
			}
		}
	}
	return tensor.Scalar(m).To(t.Device())
}

// sar kodun bütün yazımlarına (ad ve sözlüğe göre Türkçe karaktersiz biçimi) aynı yeni
// yerleşiği bağlar. Eski yerleşik (varsa) bu koddan okunur ve yeni fonksiyona verilir.
func sar(v *interp.VM, kd codes.Code, kur func(eski interp.BuiltinFunc) interp.BuiltinFunc) {
	var eski interp.BuiltinFunc
	if b := v.Builtin(kd); b != nil {
		eski = b.Fn
	}
	v.DefineBuiltin(kd, &interp.BuiltinVal{Name: codes.Name(kd), Fn: kur(eski)})
}

// eskiCagir eski yerleşiği çağırır; yoksa argümanın tensör olmadığını bildirir.
func eskiCagir(ad string, eski interp.BuiltinFunc, m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if eski != nil {
		return eski(m, args, named)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("%s() en az 1 parametre bekler", ad)
	}
	return nil, fmt.Errorf("%s() bir tensör bekler, %s verildi", ad, tipAdi(args[0]))
}

// yeniYerlesik eski karşılığı olmayan bir yerleşik kurar.
func yeniYerlesik(fn interp.BuiltinFunc) func(interp.BuiltinFunc) interp.BuiltinFunc {
	return func(interp.BuiltinFunc) interp.BuiltinFunc { return fn }
}

func tensorMu(args []interp.Value) bool {
	for _, a := range args {
		if _, ok := a.(*Deger); ok {
			return true
		}
	}
	return false
}
