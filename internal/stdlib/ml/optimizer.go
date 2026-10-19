package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"math"
	"strconv"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Optimizer AdamW ve SGD iyileştiricilerinin TR değeridir; ikisinin de tipi
// İyileştirici'dir (A214). Metotları .adım() (parametreleri günceller) ve .sıfırla()
// (gradyanları sıfırlar). ad oluşturucunun adıdır (AdamW, SGD; yazdırmada görünür).
type Optimizer struct {
	ad     string
	opt    tensor.Optimizer
	adamw  *tensor.AdamW
	sgd    *tensor.SGD
	params []*tensor.Tensor
}

func (o *Optimizer) Type() interp.ValueType { return optimizerType }
func (o *Optimizer) IsTruthy() bool         { return true }

func (o *Optimizer) Inspect() string {
	f := func(x float32) string { return strconv.FormatFloat(float64(x), 'g', -1, 32) }
	if o.adamw != nil {
		a := o.adamw
		return fmt.Sprintf("%s(parametre: %d, %s: %s, %s: %s, %s: %s, %s: %s, %s: %s, %s: %d)",
			codes.Name(codes.T400), len(o.params), nameLR.TR, f(a.LR), nameWeightDecay.TR, f(a.WeightDecay),
			nameBeta1.TR, f(a.Beta1), nameBeta2.TR, f(a.Beta2), nameEpsilon.TR, f(a.Eps), codes.Name(memberStep), a.StepCount())
	}
	s := o.sgd
	return fmt.Sprintf("%s(parametre: %d, %s: %s, %s: %s, %s: %s)",
		codes.Name(codes.T401), len(o.params), nameLR.TR, f(s.LR), nameMomentum.TR, f(s.Momentum), nameWeightDecay.TR, f(s.WeightDecay))
}

// İyileştirici üyeleri (adları kod listesinde, internal/lang/codes Y50x).
const (
	memberStep     = codes.Y500
	memberZeroGrad = codes.Y501
	memberLR       = codes.Y504
)

var optimizerMembers = codes.Names(memberStep, memberZeroGrad, memberState, memberLoadState, memberLR)

// rateField iyileştiricinin öğrenme oranı alanıdır (AdamW ya da SGD); arka uç onu her adımda
// okur, yeni oran sonraki adımda kullanılır.
func (o *Optimizer) rateField() *float32 {
	if o.adamw != nil {
		return &o.adamw.LR
	}
	return &o.sgd.LR
}

// UyeyeAta op.öğrenme_oranı = değer atamasıdır (interp.MemberAssignable): yalnızca sonlu, negatif olmayan
// ve 32 bite sığan bir Tam ya da Ondalık kabul edilir.
func (o *Optimizer) SetMember(kd codes.Code, ad string, value interp.Value) error {
	if kd != memberLR {
		return fmt.Errorf("%s üzerinde %q üyesine atama yapılamaz; atanabilen üye: %s", o.ad, ad, codes.Name(memberLR))
	}
	var f float64
	switch x := value.(type) {
	case *interp.IntVal:
		f = float64(x.Val)
	case *interp.FloatVal:
		f = x.Val
	default:
		return fmt.Errorf("%s.%s bir %s ya da %s olmalıdır, %s verildi", o.ad, codes.Name(memberLR), codes.Name(codes.A200), codes.Name(codes.A201), typeName(value))
	}
	// 0 geçerlidir (kurucudaki gibi): oran planları eğitimin sonunda orana 0'a iner. Sıfır olmayan
	// ama 32 bitte 0'a yuvarlanan değer reddedilir (sessizce dondurmasın).
	if !(f >= 0) || math.IsInf(float64(float32(f)), 0) || (f != 0 && float32(f) == 0) {
		return fmt.Errorf("%s.%s sonlu ve negatif olmayan bir sayı olmalıdır, %s verildi", o.ad, codes.Name(memberLR), value.Inspect())
	}
	*o.rateField() = float32(f)
	return nil
}

// optimizerType AdamW ve SGD değerlerinin tip adıdır (tip(op) → "İyileştirici").
var optimizerType = interp.ValueType(interp.TypeName(codes.A214))

func (o *Optimizer) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case memberStep:
		return &interp.BuiltinVal{Name: codes.Name(memberStep), NoNamedArgs: true, Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() argüman almaz", codes.Name(memberStep))
			}
			if _, err := safe(func() bool { o.opt.Step(); return true }); err != nil {
				return nil, err
			}
			return interp.NULL, nil
		}}, nil
	case memberZeroGrad:
		return &interp.BuiltinVal{Name: codes.Name(memberZeroGrad), NoNamedArgs: true, Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() argüman almaz", codes.Name(memberZeroGrad))
			}
			o.opt.ZeroGrad()
			return interp.NULL, nil
		}}, nil
	case memberLR:
		// Oran 32 bit saklanır; en kısa gösterimiyle verilir (0.003 → 0.003).
		return &interp.FloatVal{Val: shortFloat64(*o.rateField())}, nil
	}
	if v, ok, err := o.stateMember(kd); ok {
		return v, err // durum, durumu_yükle (iyilestirici_durum.go)
	}
	suggestion := ""
	if s := turkish.Closest(prop, optimizerMembers); s != "" {
		suggestion = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", s)
	}
	return nil, fmt.Errorf("%s üzerinde %q metodu bulunamadı%s", o.ad, prop, suggestion)
}

// parameters iyileştiriciye verilen tensör ya da tensör listesini denetler.
func parameters(fn string, v interp.Value) ([]*tensor.Tensor, error) {
	var list []interp.Value
	switch x := v.(type) {
	case *TensorValue:
		list = []interp.Value{x}
	case *interp.ListVal:
		list = x.Elements
	default:
		return nil, fmt.Errorf("%s: parametreler bir tensör listesi olmalıdır, %s verildi", fn, typeName(v))
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s: parametre listesi boş", fn)
	}
	out := make([]*tensor.Tensor, 0, len(list))
	seen := map[*tensor.Tensor]bool{}
	for i, el := range list {
		d, ok := el.(*TensorValue)
		if !ok {
			return nil, fmt.Errorf("%s: %d. parametre bir tensör olmalıdır, %s verildi", fn, i+1, typeName(el))
		}
		t := d.t
		if !t.RequiresGrad() {
			return nil, fmt.Errorf("%s: %d. parametre (%s) türev izlemiyor; %s ile oluşturun", fn, i+1, shapeText(t.Shape()), gradExample("..."))
		}
		if d.intermediate || !t.IsLeaf() {
			return nil, fmt.Errorf("%s: %d. parametre (%s) bir işlemin sonucu (yaprak değil); parametreleri doğrudan %s ile oluşturun", fn, i+1, shapeText(t.Shape()), gradExample("..."))
		}
		if seen[t] {
			return nil, fmt.Errorf("%s: %d. parametre listede birden çok kez var", fn, i+1)
		}
		seen[t] = true
		out = append(out, t)
	}
	return out, nil
}

// namedRate adlı sayısal bir argümanı okur.
func namedRate(fn string, named map[string]interp.Value, defaultVal float64, minVal float64, ad bind.Named) (float32, error) {
	v, ok := bind.GetNamed(named, ad)
	if !ok {
		return float32(defaultVal), nil
	}
	f, ok := interp.ToFloat(v)
	if !ok {
		return 0, fmt.Errorf("%s: %s sayı olmalıdır, %s verildi", fn, ad.TR, typeName(v))
	}
	// NaN ve sonsuz (float32'ye sığmayan) değerler reddedilir: tek adımda bütün
	// parametreleri NaN yapardı (f < minVal denetimi NaN için yanlıştır).
	if math.IsNaN(f) || math.IsInf(float64(float32(f)), 0) {
		return 0, fmt.Errorf("%s: %s sonlu bir sayı olmalıdır, %s verildi", fn, ad.TR, v.Inspect())
	}
	if f < minVal {
		return 0, fmt.Errorf("%s: %s en az %g olmalıdır, %g verildi", fn, ad.TR, minVal, f)
	}
	return float32(f), nil
}

// gradClip parametre gradyanlarının toplam L2 normunu en_fazla değerine indirir ve
// kırpma öncesi normu 0 boyutlu tensör olarak döndürür: gradyan_kırp(parametreler, en_fazla: 1.0).
func gradClip(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T402)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parametreler, %s: 1.0) 1 konumlu parametre bekler, %d verildi", fn, nameMax.TR, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameMax); err != nil {
		return nil, err
	}
	params, err := parameters(fn, args[0])
	if err != nil {
		return nil, err
	}
	maxVal := 1.0
	if v, ok := bind.GetNamed(named, nameMax); ok {
		f, ok := interp.ToFloat(v)
		if !ok || !(f > 0) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("%s: %s pozitif bir sayı olmalıdır, %s verildi", fn, nameMax.TR, v.Inspect())
		}
		maxVal = f
	}
	// Norm 0 boyutlu tensördür (kayıp gibi): GPU'da norm ve kırpma cihazda yapılır, program
	// ancak norm okununca (.değer, yazdırma) bekler.
	norm, err := safe(func() *tensor.Tensor { return tensor.ClipGradNormTensor(params, maxVal) })
	if err != nil {
		return nil, err
	}
	return New(norm), nil
}

// masterCopyArg ana_kopya: doğru adlı argümanını okur (varsayılan doğru). Açıkken 16 bitlik
// (bf16/fp16) parametrelerin float32 ana kopyası tutulur: güncelleme ana kopyada yapılıp
// parametreye yuvarlanarak yazılır, böylece küçük adımlar 16 bitte kaybolmaz. float32
// parametreleri etkilemez.
func masterCopyArg(fn string, named map[string]interp.Value) (bool, error) {
	v, ok := bind.GetNamed(named, nameMasterCopy)
	if !ok {
		return true, nil
	}
	return bind.Bool(fn, nameMasterCopy.TR, v)
}

func optimizerParams(fn string, args []interp.Value) ([]*tensor.Tensor, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parametreler, %s: ...) 1 konumlu parametre bekler, %d verildi", fn, nameLR.TR, len(args))
	}
	return parameters(fn, args[0])
}

func setupAdamW(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T400) // AdamW
	if err := bind.CheckNamed(fn, named, nameLR, nameWeightDecay, nameBeta1, nameBeta2, nameEpsilon, nameMasterCopy); err != nil {
		return nil, err
	}
	params, err := optimizerParams(fn, args)
	if err != nil {
		return nil, err
	}
	lr, err := namedRate(fn, named, 0.001, 0, nameLR)
	if err != nil {
		return nil, err
	}
	wd, err := namedRate(fn, named, 0.01, 0, nameWeightDecay)
	if err != nil {
		return nil, err
	}
	b1, err := namedRate(fn, named, 0.9, 0, nameBeta1)
	if err != nil {
		return nil, err
	}
	b2, err := namedRate(fn, named, 0.999, 0, nameBeta2)
	if err != nil {
		return nil, err
	}
	if b1 >= 1 || b2 >= 1 {
		return nil, fmt.Errorf("%s: beta1 ve beta2 [0, 1) aralığında olmalıdır", fn)
	}
	eps, err := namedRate(fn, named, 1e-8, 0, nameEpsilon)
	if err != nil {
		return nil, err
	}
	if eps <= 0 {
		// Sıfır gradyanlı ilk adımda 0 / (√0 + 0) = NaN olurdu (float32'de 0'a düşen değer de).
		v, _ := bind.GetNamed(named, nameEpsilon)
		return nil, fmt.Errorf("%s: %s pozitif olmalıdır (float32'de 0'dan büyük), %s verildi", fn, nameEpsilon.TR, v.Inspect())
	}
	masterCopy, err := masterCopyArg(fn, named)
	if err != nil {
		return nil, err
	}
	a, err := safe(func() *tensor.AdamW { return tensor.NewAdamW(params, lr, wd) })
	if err != nil {
		return nil, err
	}
	a.Beta1, a.Beta2, a.Eps = b1, b2, eps
	a.SetMasterWeights(masterCopy)
	return &Optimizer{ad: fn, opt: a, adamw: a, params: params}, nil
}

func setupSGD(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T401) // SGD
	if err := bind.CheckNamed(fn, named, nameLR, nameMomentum, nameWeightDecay, nameMasterCopy); err != nil {
		return nil, err
	}
	params, err := optimizerParams(fn, args)
	if err != nil {
		return nil, err
	}
	lr, err := namedRate(fn, named, 0.01, 0, nameLR)
	if err != nil {
		return nil, err
	}
	mom, err := namedRate(fn, named, 0, 0, nameMomentum)
	if err != nil {
		return nil, err
	}
	wd, err := namedRate(fn, named, 0, 0, nameWeightDecay)
	if err != nil {
		return nil, err
	}
	masterCopy, err := masterCopyArg(fn, named)
	if err != nil {
		return nil, err
	}
	s, err := safe(func() *tensor.SGD { return tensor.NewSGD(params, lr, mom) })
	if err != nil {
		return nil, err
	}
	s.WeightDecay = wd
	s.SetMasterWeights(masterCopy)
	return &Optimizer{ad: fn, opt: s, sgd: s, params: params}, nil
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter   = (*Optimizer)(nil)
	_ interp.MemberAssignable = (*Optimizer)(nil)
)
