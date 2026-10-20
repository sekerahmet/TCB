package ml

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
// karışık_hassasiyet("bf16") { ... } bloğu içinde matris çarpımları ('@') ve dikkat girdilerini
// bf16'ya çevirip float32 biriktirmeyle bf16 sonuç verir; normalizasyon, softmax ve
// indirgemeler 16 bitlik girdileri float32'ye açarak hesaplanır, çapraz entropi 16 bitlik
// logitleri açmadan float32'de işler. Ağırlıkların kendisi
// değişmez (dönüşüm kopyadır, türevlenebilir), bu yüzden geri yayılım ve iyileştirici
// float32 ağırlıklarla çalışır.
//
// Blok durumu sürecin değil görevin (VM görünümünün, bkz. interp.(*VM).TaskValue) durumudur:
// iç içe bloklar yığın gibi önceki türe döner; blok GIL'i bırakan bir çağrıdayken (bekle(),
// dosya/ağ beklemesi) çalışan başka bir eşzamanlı görev ya da web isteği bloktan etkilenmez.
// Blok içinde başlatılan bir 'eşzamanlı' görev de kendi (boş) durumuyla başlar.

// Dildeki tür adları; sıra .tür'ün döndürdüğü adlarla aynıdır.
var dtypeNames = []string{"float32", "bf16", "fp16"}

// dtypeSuggestions başka kütüphanelerden alışılmış tür adlarının TR'deki karşılığıdır. Bu
// adlar KABUL EDİLMEZ; yalnızca hata mesajında doğru ad önerilir.
var dtypeSuggestions = map[string]string{
	"bfloat16": "bf16", "bfloat": "bf16", "bf": "bf16",
	"float16": "fp16", "half": "fp16", "f16": "fp16",
	"fp32": "float32", "f32": "float32", "float": "float32", "single": "float32",
}

// dtypeName çekirdek türünün dildeki adı.
func dtypeName(dt tensor.DType) string { return dt.Name() }

// parseDTypeName dildeki tür adını çekirdek türüne çevirir; bilinmeyen adda en yakın adı önerir.
func parseDTypeName(fn, ad string) (tensor.DType, error) {
	switch ad {
	case "float32":
		return tensor.Float32, nil
	case "bf16":
		return tensor.BFloat16, nil
	case "fp16":
		return tensor.Float16, nil
	}
	small := strings.ToLower(strings.TrimSpace(ad))
	suggestion := dtypeSuggestions[small]
	if suggestion == "" {
		suggestion = turkish.Closest(small, dtypeNames)
	}
	if suggestion == "" {
		// Yazım hatalı ama türü belli adlar: "bf61" → bf16, "fp61" → fp16.
		switch {
		case strings.HasPrefix(small, "bf"):
			suggestion = "bf16"
		case strings.HasPrefix(small, "fp") || strings.HasPrefix(small, "half"):
			suggestion = "fp16"
		case strings.HasPrefix(small, "float"):
			suggestion = "float32"
		}
	}
	if suggestion != "" {
		return 0, messages.New(messages.H6811, fn, ad, suggestion)
	}
	return 0, messages.New(messages.H6810, fn, ad)
}

// parseDTypeValue metin olması gereken tür argümanını çözer.
func parseDTypeValue(fn string, v interp.Value) (tensor.DType, error) {
	s, ok := v.(*interp.StringVal)
	if !ok {
		return 0, messages.New(messages.H6803, fn, typeName(v))
	}
	return parseDTypeName(fn, s.Val)
}

// namedDType tür: adlı argümanını okur; verilmemişse varsayilan döner.
func namedDType(fn string, named map[string]interp.Value, defaultVal tensor.DType) (tensor.DType, error) {
	v, ok := bind.GetNamed(named, nameDType)
	if !ok {
		return defaultVal, nil
	}
	return parseDTypeValue(fn, v)
}

// castTo t'yi dt türüne çevirir (aynıysa kendisi). Türev izlenen t'de dönüşüm çizgeye
// kaydedilir.
func castTo(t *tensor.Tensor, dt tensor.DType) (*tensor.Tensor, error) {
	if t.DType() == dt {
		return t, nil
	}
	return safe(func() *tensor.Tensor { return t.Cast(dt) })
}

// toDType .türe("bf16") metodudur: tensörün verilen türe dönüştürülmüş kopyası. Türevlenebilir:
// gradyan kaynak tensöre (kendi türünde) geri akar. Tür zaten aynıysa tensörün kendisi döner.
func (d *TensorValue) toDType(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, messages.New(messages.H6806, codes.Name(memberToDType), len(args))
	}
	if err := bind.CheckNamed(codes.Name(memberToDType)+"()", named); err != nil {
		return nil, err
	}
	dt, err := parseDTypeValue(codes.Name(memberToDType), args[0])
	if err != nil {
		return nil, err
	}
	if dt == d.t.DType() {
		return d, nil
	}
	r, err := castTo(d.t, dt)
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// ----------------------------------------------------------------------------
// karışık_hassasiyet bloğu
// ----------------------------------------------------------------------------

// mixedKey görev durumunda (interp.GorevDegeri) karışık hassasiyet türünün anahtarı.
type mixedKey struct{}

// mixedDType görevin etkin karışık hassasiyet türü; blok dışında (ya da
// karışık_hassasiyet("float32") içinde) Float32.
func mixedDType(m *interp.VM) tensor.DType {
	if m == nil {
		return tensor.Float32
	}
	if dt, ok := m.TaskValue(mixedKey{}).(tensor.DType); ok {
		return dt
	}
	return tensor.Float32
}

// mixedPrecision karışık_hassasiyet("bf16") { ... } yerleşiğidir: bloğu verilen hesap
// türüyle çalıştırır ve bloğun son ifadesinin değerini döndürür. "float32" iç içe bir blokta
// karışık hassasiyeti kapatır. Blok bitince (hata ile bitse de) önceki tür geri gelir.
func mixedPrecision(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T710)
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		if len(args) == 1 {
			if s, text := args[0].(*interp.StringVal); text {
				return nil, messages.New(messages.H6807, fn, s.Val)
			}
		}
		return nil, messages.New(messages.H6808, fn, len(args))
	}
	dt, err := parseDTypeValue(fn, args[0])
	if err != nil {
		return nil, err
	}
	switch args[1].(type) {
	case *interp.FuncVal, *interp.BuiltinVal:
	default:
		return nil, messages.New(messages.H6809, fn, typeName(args[1]))
	}
	prev := m.TaskValue(mixedKey{})
	m.SetTaskValue(mixedKey{}, dt)
	defer m.SetTaskValue(mixedKey{}, prev)
	result, err := m.ApplyFunction(args[1], nil, nil, 0)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = interp.Null
	}
	return result, nil
}

// highPrecision karışık hassasiyet bloğu içinde 16 bitlik bir tensörü (normalizasyon,
// softmax ve indirgemelerden önce) float32'ye açar; blok dışında ya da tensör zaten
// float32 ise t'nin kendisi döner. Bloğun kendi dönüşümleri kullanıcıya görünmediğinden
// paylaşılır (CastShared): aynı ara sonuç bir kez dönüştürülür.
func highPrecision(m *interp.VM, t *tensor.Tensor) (*tensor.Tensor, error) {
	if t == nil || t.DType() == tensor.Float32 || mixedDType(m) == tensor.Float32 {
		return t, nil
	}
	return safe(func() *tensor.Tensor { return t.CastShared(tensor.Float32) })
}
