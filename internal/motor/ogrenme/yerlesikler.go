package ogrenme

import (
	"fmt"
	"math"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	_ "tr_dili/internal/hesap/gpu" // GPU arka ucunu kaydeder (sürücü ilk kullanımda yüklenir)
	"tr_dili/internal/hesap/tensor"
)

// Kaydet tensör yerleşiklerini VM'ye kaydeder. Adlar kod listesindedir (internal/dil/kod); burada
// yalnızca kodlar geçer. Aynı adlı çekirdek ve yapay zekâ yerleşikleri (toplam, ortalama,
// min, maks, karekök) sarılır: argüman tensörse tensör yolu, değilse eski yerleşik çalışır.
func Kaydet(v *yurut.VM) {
	// Türevsiz kip görev başınadır (gorev.go).
	v.GorevKancasiEkle(gorevKancasi)

	olusturuculariKaydet(v)
	matematigiKaydet(v)

	// --- Sinir ağı ---
	sar(v, kod.T500, yeniYerlesik(gomme))
	sar(v, kod.T510, yeniYerlesik(caprazEntropi))
	sar(v, kod.T501, yeniYerlesik(katmanNormu))
	sar(v, kod.T502, yeniYerlesik(rmsNormu))
	sar(v, kod.T503, yeniYerlesik(dikkatYerlesik)) // model_araclari.go
	sar(v, kod.T530, yeniYerlesik(parametrelerYerlesik))
	sar(v, kod.T611, yeniYerlesik(orneklemeYerlesik))

	// --- Türev ---
	sar(v, kod.T300, yeniYerlesik(geriYayilim))
	sar(v, kod.T301, yeniYerlesik(turevsiz))
	ozelTurevKaydet(v) // özel_türev, türev_denetle (ozel_turev.go)

	// --- Veri türleri (veri_turu.go) ---
	sar(v, kod.T710, yeniYerlesik(karisikHassasiyet))

	// --- Cihazlar (cihaz.go) ---
	sar(v, kod.T720, yeniYerlesik(varsayilanCihazYerlesik))
	sar(v, kod.T721, yeniYerlesik(cihazlarYerlesik))

	// --- İyileştiriciler ---
	sar(v, kod.T400, yeniYerlesik(adamWKur))
	sar(v, kod.T401, yeniYerlesik(sgdKur))
	sar(v, kod.T402, yeniYerlesik(gradyanKirp))

	// --- İndeks, kaydetme ve yükleme ---
	sar(v, kod.T230, yeniYerlesik(maksIndeks))
	sar(v, kod.T700, yeniYerlesik(tensorKaydet))
	sar(v, kod.T701, yeniYerlesik(tensorYukle))
	sar(v, kod.T703, yeniYerlesik(parametreleriYukleYerlesik)) // parametreler.go
}

// olusturuculariKaydet tensör oluşturucularını (matris ve görünüm dahil) kaydeder.
func olusturuculariKaydet(v *yurut.VM) {
	// --- Oluşturucular ---
	sar(v, kod.T100, yeniYerlesik(tensorYerlesik))
	sar(v, kod.T101, yeniYerlesik(sekilliOlusturucu(kod.T101, func(sh []int, _ map[string]yurut.Value) (*tensor.Tensor, error) {
		return tensor.Zeros(sh...), nil
	})))
	sar(v, kod.T102, yeniYerlesik(sekilliOlusturucu(kod.T102, func(sh []int, _ map[string]yurut.Value) (*tensor.Tensor, error) {
		return tensor.Ones(sh...), nil
	})))
	sar(v, kod.T103, yeniYerlesik(sekilliOlusturucu(kod.T103, rastgeleNormal)))

	// --- Matris (2 boyutlu tensör) oluşturucusu (matris.go) ---
	sar(v, kod.T111, yeniYerlesik(birimMatris))

	// --- Görünüm ---
	sar(v, kod.T121, yeniYerlesik(birlestirYerlesik)) // model_araclari.go
}

// matematigiKaydet eleman bazlı fonksiyonları, softmax'ı ve indirgemeleri kaydeder.
func matematigiKaydet(v *yurut.VM) {
	// --- Eleman bazlı matematik ---
	birli(v, kod.T200, (*tensor.Tensor).Relu, relu64)
	birli(v, kod.T201, (*tensor.Tensor).Sigmoid, sigmoid64)
	birli(v, kod.T202, (*tensor.Tensor).Tanh, math.Tanh)
	birli(v, kod.T203, (*tensor.Tensor).Gelu, gelu64)
	birli(v, kod.G404, (*tensor.Tensor).Sqrt, nil) // karekök
	birli(v, kod.T220, (*tensor.Tensor).Exp, math.Exp)
	birli(v, kod.T221, (*tensor.Tensor).Log, math.Log)
	birli(v, kod.T222, (*tensor.Tensor).Sin, math.Sin)
	birli(v, kod.T223, (*tensor.Tensor).Cos, math.Cos)

	// --- Softmax ---
	softmaxKaydet(v, kod.T210, false)
	softmaxKaydet(v, kod.T211, true)

	// --- İndirgemeler (toplam, ortalama, maks, min) ---
	indirgeme(v, kod.G400, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.SumAll()
		}
		return t.Sum(e, koru)
	})
	indirgeme(v, kod.G410, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.MeanAll()
		}
		return t.Mean(e, koru)
	})
	indirgeme(v, kod.G402, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.Reshape(-1).Max(0, false)
		}
		return t.Max(e, koru)
	})
	indirgeme(v, kod.G401, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
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
func sar(v *yurut.VM, kd kod.Kod, kur func(eski yurut.BuiltinFunc) yurut.BuiltinFunc) {
	var eski yurut.BuiltinFunc
	if b := v.Yerlesik(kd); b != nil {
		eski = b.Fn
	}
	v.YerlesikTanimla(kd, &yurut.BuiltinVal{Name: kod.Ad(kd), Fn: kur(eski)})
}

// eskiCagir eski yerleşiği çağırır; yoksa argümanın tensör olmadığını bildirir.
func eskiCagir(ad string, eski yurut.BuiltinFunc, m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	if eski != nil {
		return eski(m, args, named)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("%s() en az 1 parametre bekler", ad)
	}
	return nil, fmt.Errorf("%s() bir tensör bekler, %s verildi", ad, tipAdi(args[0]))
}

// yeniYerlesik eski karşılığı olmayan bir yerleşik kurar.
func yeniYerlesik(fn yurut.BuiltinFunc) func(yurut.BuiltinFunc) yurut.BuiltinFunc {
	return func(yurut.BuiltinFunc) yurut.BuiltinFunc { return fn }
}

func tensorMu(args []yurut.Value) bool {
	for _, a := range args {
		if _, ok := a.(*Deger); ok {
			return true
		}
	}
	return false
}
