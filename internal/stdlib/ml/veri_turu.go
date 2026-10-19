package ml

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bu dosya tensör veri türlerini (float32, bf16, fp16) ve karışık_hassasiyet bloğunu TR'ye
// bağlar.
//
// Tensör başına tür: tensör(veri, tür: "bf16"), sıfırlar/birler/rastgele_normal(..., tür:),
// .tür (metin) ve .türe("bf16") (türevlenebilir dönüşüm). Hesaplar çekirdekte en az float32
// biriktirmeyle yapılır; sonuç girdilerin türüne (farklıysa float32'ye) bir kez yuvarlanır.
//
// karışık_hassasiyet("bf16") { ... } bloğu içinde matris çarpımları ('@') girdilerini
// bf16'ya çevirip float32 biriktirmeyle float32 sonuç verir; normalizasyon, softmax, kayıp ve
// indirgemeler 16 bitlik girdileri float32'ye açarak hesaplanır. Ağırlıkların kendisi
// değişmez (dönüşüm kopyadır, türevlenebilir), bu yüzden geri yayılım ve iyileştirici
// float32 ağırlıklarla çalışır.
//
// Blok durumu sürecin değil görevin (VM görünümünün, bkz. vm.(*VM).GorevDegeri) durumudur:
// iç içe bloklar yığın gibi önceki türe döner; blok GIL'i bırakan bir çağrıdayken (bekle(),
// dosya/ağ beklemesi) çalışan başka bir eşzamanlı görev ya da web isteği bloktan etkilenmez.
// Blok içinde başlatılan bir 'eşzamanlı' görev de kendi (boş) durumuyla başlar.

// Dildeki tür adları; sıra .tür'ün döndürdüğü adlarla aynıdır.
var turAdlari = []string{"float32", "bf16", "fp16"}

// turAdOnerileri başka kütüphanelerden alışılmış tür adlarının TR'deki karşılığıdır. Bu
// adlar KABUL EDİLMEZ; yalnızca hata mesajında doğru ad önerilir.
var turAdOnerileri = map[string]string{
	"bfloat16": "bf16", "bfloat": "bf16", "bf": "bf16",
	"float16": "fp16", "half": "fp16", "f16": "fp16", "yarım": "fp16",
	"fp32": "float32", "f32": "float32", "float": "float32", "single": "float32",
}

// turAdi çekirdek türünün dildeki adı.
func turAdi(dt tensor.DType) string {
	switch dt {
	case tensor.Float32:
		return "float32"
	case tensor.BFloat16:
		return "bf16"
	case tensor.Float16:
		return "fp16"
	}
	return dt.String()
}

// turAdiCoz dildeki tür adını çekirdek türüne çevirir; bilinmeyen adda en yakın adı önerir.
func turAdiCoz(fn, ad string) (tensor.DType, error) {
	switch ad {
	case "float32":
		return tensor.Float32, nil
	case "bf16":
		return tensor.BFloat16, nil
	case "fp16":
		return tensor.Float16, nil
	}
	kucuk := strings.ToLower(strings.TrimSpace(ad))
	oneri := turAdOnerileri[kucuk]
	if oneri == "" {
		oneri = turkish.Closest(kucuk, turAdlari)
	}
	if oneri == "" {
		// Yazım hatalı ama türü belli adlar: "bf61" → bf16, "fp61" → fp16.
		switch {
		case strings.HasPrefix(kucuk, "bf"):
			oneri = "bf16"
		case strings.HasPrefix(kucuk, "fp") || strings.HasPrefix(kucuk, "half"):
			oneri = "fp16"
		case strings.HasPrefix(kucuk, "float"):
			oneri = "float32"
		}
	}
	ek := ""
	if oneri != "" {
		ek = fmt.Sprintf(". Bunu mu demek istediniz: \"%s\"?", oneri)
	}
	return 0, fmt.Errorf("%s: bilinmeyen veri türü %q; geçerli türler: \"float32\", \"bf16\", \"fp16\"%s", fn, ad, ek)
}

// turDegeriCoz metin olması gereken tür argümanını çözer.
func turDegeriCoz(fn string, v interp.Value) (tensor.DType, error) {
	s, ok := v.(*interp.StringVal)
	if !ok {
		return 0, fmt.Errorf("%s: tür metin olmalıdır (\"float32\", \"bf16\" ya da \"fp16\"), %s verildi", fn, tipAdi(v))
	}
	return turAdiCoz(fn, s.Val)
}

// adliTur tür: adlı argümanını okur; verilmemişse varsayilan döner.
func adliTur(fn string, named map[string]interp.Value, varsayilan tensor.DType) (tensor.DType, error) {
	v, ok := bind.GetNamed(named, adTur)
	if !ok {
		return varsayilan, nil
	}
	return turDegeriCoz(fn, v)
}

// tureCevir t'yi dt türüne çevirir (aynıysa kendisi). Türev izlenen t'de dönüşüm çizgeye
// kaydedilir.
func tureCevir(t *tensor.Tensor, dt tensor.DType) (*tensor.Tensor, error) {
	if t.DType() == dt {
		return t, nil
	}
	return guvenli(func() *tensor.Tensor { return t.Cast(dt) })
}

// ture .türe("bf16") metodudur: tensörün verilen türe dönüştürülmüş kopyası. Türevlenebilir:
// gradyan kaynak tensöre (kendi türünde) geri akar. Tür zaten aynıysa tensörün kendisi döner.
func (d *Deger) ture(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(\"bf16\") 1 parametre (tür adı) bekler, %d verildi", codes.Name(uyeTure), len(args))
	}
	if err := bind.CheckNamed(codes.Name(uyeTure)+"()", named); err != nil {
		return nil, err
	}
	dt, err := turDegeriCoz(codes.Name(uyeTure), args[0])
	if err != nil {
		return nil, err
	}
	if dt == d.t.DType() {
		return d, nil
	}
	r, err := tureCevir(d.t, dt)
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// ----------------------------------------------------------------------------
// karışık_hassasiyet bloğu
// ----------------------------------------------------------------------------

// karisikAnahtari görev durumunda (yurut.GorevDegeri) karışık hassasiyet türünün anahtarı.
type karisikAnahtari struct{}

// karisikTuru görevin etkin karışık hassasiyet türü; blok dışında (ya da
// karışık_hassasiyet("float32") içinde) Float32.
func karisikTuru(m *interp.VM) tensor.DType {
	if m == nil {
		return tensor.Float32
	}
	if dt, ok := m.TaskValue(karisikAnahtari{}).(tensor.DType); ok {
		return dt
	}
	return tensor.Float32
}

// karisikHassasiyet karışık_hassasiyet("bf16") { ... } yerleşiğidir: bloğu verilen hesap
// türüyle çalıştırır ve bloğun son ifadesinin değerini döndürür. "float32" iç içe bir blokta
// karışık hassasiyeti kapatır. Blok bitince (hata ile bitse de) önceki tür geri gelir.
func karisikHassasiyet(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T710)
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		if len(args) == 1 {
			if s, metin := args[0].(*interp.StringVal); metin {
				return nil, fmt.Errorf("%s(%q) bir blok bekler: %s(%q) { ... }", fn, s.Val, fn, s.Val)
			}
		}
		return nil, fmt.Errorf("%s(\"bf16\") { ... } bir tür adı ve bir blok bekler, %d parametre verildi", fn, len(args))
	}
	dt, err := turDegeriCoz(fn, args[0])
	if err != nil {
		return nil, err
	}
	switch args[1].(type) {
	case *interp.FuncVal, *interp.BuiltinVal:
	default:
		return nil, fmt.Errorf("%s(\"bf16\") { ... }: ikinci parametre bir blok olmalıdır, %s verildi", fn, tipAdi(args[1]))
	}
	onceki := m.TaskValue(karisikAnahtari{})
	m.SetTaskValue(karisikAnahtari{}, dt)
	defer m.SetTaskValue(karisikAnahtari{}, onceki)
	sonuc, err := m.ApplyFunction(args[1], nil, nil, 0)
	if err != nil {
		return nil, err
	}
	if sonuc == nil {
		sonuc = interp.NULL
	}
	return sonuc, nil
}

// yuksekHassasiyet karışık hassasiyet bloğu içinde 16 bitlik bir tensörü (normalizasyon,
// softmax, kayıp ve indirgemelerden önce) float32'ye açar; blok dışında ya da tensör zaten
// float32 ise t'nin kendisi döner.
func yuksekHassasiyet(m *interp.VM, t *tensor.Tensor) (*tensor.Tensor, error) {
	if t == nil || t.DType() == tensor.Float32 || karisikTuru(m) == tensor.Float32 {
		return t, nil
	}
	return tureCevir(t, tensor.Float32)
}
