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
// iç içe bloklar yığın gibi önceki türe döner; blok GIL'i bırakan bir çağrıdayken (uyu(),
// dosya/ağ beklemesi) çalışan başka bir eşzamanlı görev ya da web isteği bloktan etkilenmez.
// Blok içinde başlatılan görev açıldığı yerin türünü alır (10.4, taskFork); sonra kendi durumudur.

// Dildeki tür adları; sıra .tür'ün döndürdüğü adlarla aynıdır.
var dtypeNames = []string{"float32", "bf16", "fp16"}

// dtypeSuggestions başka kütüphanelerden alışılmış tür adlarının TR'deki karşılığıdır. Bu
// adlar KABUL EDİLMEZ; yalnızca hata mesajında doğru ad önerilir.
var dtypeSuggestions = map[string]string{
	"bfloat16": "bf16", "bfloat": "bf16", "bf": "bf16",
	"float16": "fp16", "half": "fp16", "f16": "fp16",
	"fp32": "float32", "f32": "float32", "float": "float32", "single": "float32",
}

// parseDTypeName dildeki tür adını çekirdek türüne çevirir; bilinmeyen adda en yakın adı önerir.
func parseDTypeName(fn any, ad string) (tensor.DType, error) {
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
func parseDTypeValue(fn any, v interp.Value) (tensor.DType, error) {
	s, ok := v.(*interp.StringVal)
	if !ok {
		return 0, messages.New(messages.H6803, fn, typeName(v))
	}
	return parseDTypeName(fn, s.Val)
}

// castTo t'yi dt türüne çevirir (aynıysa kendisi). Türev izlenen t'de dönüşüm çizgeye
// kaydedilir.
func castTo(t *tensor.Tensor, dt tensor.DType) (*tensor.Tensor, error) {
	if t.DType() == dt {
		return t, nil
	}
	return safe(func() *tensor.Tensor { return t.Cast(dt) })
}

// toDType .TR_to_dtype("bf16") metodudur (15.3): cinsin içinde hassasiyeti değiştirilmiş kopya.
// Hesap ondalıkları arasında türevlenebilir: gradyan kaynak tensöre (kendi türünde) geri akar.
// Tam sayıda dar hassasiyete geçişte değer sığmalıdır. Hassasiyet zaten aynıysa tensörün kendisi
// döner. Başka cinsin hassasiyeti hatadır: cins dönüşümü tip çağrısıdır.
func (d *TensorValue) toDType(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, messages.New(messages.H6806, codes.NameRef(memberToDType), len(args))
	}
	if err := bind.CheckNamed(codes.NameRef(memberToDType).Called(), named); err != nil {
		return nil, err
	}
	fn := codes.NameRef(memberToDType)
	dt, name, err := anyDType(fn, args[0])
	if err != nil {
		return nil, err
	}
	if err := kindChangeError(fn, d.t.DType(), dt, name); err != nil {
		return nil, err
	}
	if dt == d.t.DType() {
		return d, nil
	}
	if dt.IsQuantized() {
		return nil, messages.New(messages.H6691, fn, name)
	}
	r, err := convertKind(fn, d.t, dt)
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
	fn := codes.NameRef(codes.FnMixedPrecision)
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
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
	result, err := applyFunction(m, args[1], nil, nil)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = interp.Null
	}
	return result, nil
}

// computeDType hesap_türü() yerleşiğidir: etkin karışık_hassasiyet bloğunun hesap türünün adı;
// blok dışında "float32". PyTorch'ta torch.get_autocast_dtype, Flax'ta modülün dtype'ı: bir
// sonucu doğrudan çarpımın okuyacağı türde yazmak için (normların tür: argümanı, 15.4).
func computeDType(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnComputeDtype)
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.FnComputeDtype, 0, len(args))
	}
	return &interp.StringVal{Val: mixedDType(m).Name()}, nil
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

// kindDTypeNames hesap ondalıkları dışındaki hassasiyetlerin adları (15.1); kurucuların TR_dtype:'ı
// içindir. "int8" yalnız TR_Int'indir.
var kindDTypeNames = map[string]tensor.DType{
	"float64": tensor.Float64,
	"int64":   tensor.Int64, "int32": tensor.Int32, "int16": tensor.Int16, "int8": tensor.Int8,
	"uint32": tensor.UInt32, "uint16": tensor.UInt16, "uint8": tensor.UInt8,
	"bool": tensor.Bool,
	"q8":   tensor.Q8, "q4_k": tensor.Q4K, "q6_k": tensor.Q6K,
}

// kindOf hassasiyetin öğe cinsi (TR_Float, TR_Int ya da TR_Bool'un kodu).
func kindOf(dt tensor.DType) codes.Code {
	switch dt.Elem() {
	case tensor.ElemInt:
		return codes.TypeInt
	case tensor.ElemBool:
		return codes.TypeBool
	}
	return codes.TypeFloat
}

// constructorDType kurucunun TR_dtype: adlı argümanını okur (15.2): her cinsin hassasiyet adı
// geçer; cinsi kind olmayan hassasiyet hatadır, ileti tip argümanını önerir. Verilmemişse
// kind'ın varsayılan hassasiyeti döner.
func constructorDType(fn codes.Ref, named map[string]interp.Value, kind codes.Code) (tensor.DType, error) {
	v, given := bind.GetNamed(named, nameDType)
	if !given {
		return kindDTypes[kind], nil
	}
	dt, name, err := anyDType(fn, v)
	if err != nil {
		return 0, err
	}
	if dt.IsQuantized() && fn.Code != codes.FnZeros {
		// Yalnız TR_zeros float bellek ayırmadan sıfır kurar (15.2); değerden nicemleme yoktur.
		return 0, messages.New(messages.H6691, fn, name)
	}
	if k := kindOf(dt); k != kind {
		return 0, messages.New(messages.H6976, fn, name, codes.NameRef(k), codes.NameRef(kind),
			kindHint(fn.Code, kind, k), codes.Spelling(fn.Code), codes.Spelling(k))
	}
	return dt, nil
}

// kindHint H6976'nın önerisini seçer: rastgele_normal yalnız ondalık kurar ("float_only",
// şablonun other dalı); TR_Tensor'ün verisi tip argümanıyla yalnız tam sayıdan ondalığa geçer,
// öbür cins değişimi verinin kendisini ister ("data"); öbür kurucularda tip argümanı yeter.
func kindHint(fn, from, to codes.Code) string {
	switch {
	case fn == codes.FnRandn:
		return "float_only"
	case fn == codes.TypeTensor && !(from == codes.TypeInt && to == codes.TypeFloat):
		return "data"
	}
	return "type_arg"
}

// anyDType her cinsin hassasiyet adını (metin) çözer; bilinmeyen adda en yakın adı önerir.
func anyDType(fn any, v interp.Value) (tensor.DType, string, error) {
	s, isString := v.(*interp.StringVal)
	if !isString {
		return 0, "", messages.New(messages.H6803, fn, typeName(v))
	}
	if dt, ok := kindDTypeNames[s.Val]; ok {
		return dt, s.Val, nil
	}
	dt, err := parseDTypeName(fn, s.Val)
	return dt, s.Val, err
}

// kindChangeError cins dönüşümünün hatası (15.3): mantık ile sayı arasında dönüşüm yoktur (H6978);
// öbür cinsler arası dönüşüm tip çağrısıdır (H6977, .TR_to_dtype'ta). Aynı cinste nil.
func kindChangeError(fn any, from, to tensor.DType, name string) error {
	fk, tk := kindOf(from), kindOf(to)
	switch {
	case fk == tk:
		return nil
	case fk == codes.TypeBool || tk == codes.TypeBool:
		return messages.New(messages.H6978, fn, codes.NameRef(fk), codes.NameRef(tk))
	}
	return messages.New(messages.H6977, fn, name, codes.NameRef(tk), codes.NameRef(fk), codes.Spelling(tk))
}

// convertKind t'yi dt'ye çevirir (aynıysa kendisi): ondalıklar arasında yuvarlar (türevlenebilir),
// cinsler arasında kesin çevirir; sığmayan ya da NaN öğe hatadır (15.2, 15.3).
func convertKind(fn any, t *tensor.Tensor, dt tensor.DType) (*tensor.Tensor, error) {
	if t.DType() == dt {
		return t, nil
	}
	if t.DType().IsFloat() && dt.IsFloat() {
		return castTo(t, dt)
	}
	dev := t.Device()
	onHost := dev != tensor.CPU && dt.Elem() == tensor.ElemInt
	var r *tensor.Tensor
	var f *tensor.ElemFault
	if _, err := safe(func() *tensor.Tensor {
		src := t
		if onHost {
			// Tam sayıya dönüşüm GPU'da sonucu bekleyen işlemdir (15.6: sığma ve NaN denetimi);
			// bu dilimde denetim ve dönüşüm konakta yapılır, sonuç cihaza döner.
			src = t.To(tensor.CPU)
		}
		r, f = tensor.Convert(src, dt)
		if onHost {
			if f == nil {
				host := r
				r = host.To(dev)
				host.Drop()
			}
			src.Drop()
		}
		return r
	}); err != nil {
		return nil, err
	}
	if f != nil {
		return nil, faultError(fn, "", f)
	}
	return r, nil
}

// gradKindError türev izlenen tensörün ondalık olmamasının hatasıdır (15.2: türev yalnız TR_Float'ta).
func gradKindError(fn any, named map[string]interp.Value, dt tensor.DType) error {
	if dt.Elem() == tensor.ElemFloat {
		return nil
	}
	if grad, given := bind.GetNamed(named, nameRequiresGrad); given && grad.IsTruthy() {
		return messages.New(messages.H6973, fn, codes.NameRef(codes.ArgRequiresGrad), codes.NameRef(codes.KwTrue), dt.Name())
	}
	return nil
}

// withoutNamed named'ın a'sız kopyası.
func withoutNamed(named map[string]interp.Value, a bind.Named) map[string]interp.Value {
	out := make(map[string]interp.Value, len(named))
	for k, v := range named {
		if _, is := bind.GetNamed(map[string]interp.Value{k: v}, a); !is {
			out[k] = v
		}
	}
	return out
}
