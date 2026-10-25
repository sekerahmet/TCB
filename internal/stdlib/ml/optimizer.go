package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"math"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Optimizer AdamW ve SGD iyileştiricilerinin TR değeridir; ikisinin de tipi
// İyileştirici'dir (TR_type_Optimizer). Metotları .adım() (parametreleri günceller) ve .sıfırla()
// (gradyanları sıfırlar). ad oluşturucunun adıdır (AdamW, SGD; yazdırmada görünür).
type Optimizer struct {
	ad     codes.Ref
	opt    tensor.Optimizer
	adamw  *tensor.AdamW
	sgd    *tensor.SGD
	params []*tensor.Tensor
}

func (o *Optimizer) Type() interp.ValueType { return optimizerType }
func (o *Optimizer) IsTruthy() bool         { return true }

// Inspect is the value's display text, in the person's writing language (TR_DIL).
func (o *Optimizer) Inspect() string { return o.Text(codes.Display()) }

var _ interp.LanguageText = (*Optimizer)(nil)

// Text is the value's text in lang: str() and yazdır give it in the language of the file they
// are written in (docs/spec/03_tipler.md).
func (o *Optimizer) Text(lang *codes.Language) string {
	f := func(x float32) string { return strconv.FormatFloat(float64(x), 'g', -1, 32) }
	var name codes.Code
	var settings []string
	set := func(k codes.Code, value string) { settings = append(settings, lang.Name(k)+": "+value) }
	if a := o.adamw; a != nil {
		name = codes.FnAdamW
		set(nameLR.Code, f(a.LR))
		set(nameWeightDecay.Code, f(a.WeightDecay))
		set(nameBeta1.Code, f(a.Beta1))
		set(nameBeta2.Code, f(a.Beta2))
		set(nameEpsilon.Code, f(a.Eps))
		set(memberStep, strconv.Itoa(a.StepCount()))
	} else {
		s := o.sgd
		name = codes.FnSGD
		set(nameLR.Code, f(s.LR))
		set(nameMomentum.Code, f(s.Momentum))
		set(nameWeightDecay.Code, f(s.WeightDecay))
	}
	return messages.TextIn(lang.Code, messages.H6820, lang.Name(name), len(o.params), strings.Join(settings, ", "))
}

// İyileştirici üyeleri (adları kod listesinde, internal/lang/codes mem).
const (
	memberStep     = codes.MemStep
	memberZeroGrad = codes.MemZeroGrad
	memberLR       = codes.MemLr
)

var optimizerMembers = []codes.Code{memberStep, memberZeroGrad, memberState, memberLoadState, memberLR}

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
		return messages.New(messages.H6502, o.ad, ad, codes.NameRef(memberLR))
	}
	f, ok := interp.ToFloat(value)
	if !ok {
		return messages.New(messages.H6503, o.ad, codes.NameRef(memberLR), codes.NameRef(codes.TypeInt), codes.NameRef(codes.TypeFloat), typeName(value))
	}
	// 0 geçerlidir (kurucudaki gibi): oran planları eğitimin sonunda orana 0'a iner. Sıfır olmayan
	// ama 32 bitte 0'a yuvarlanan değer reddedilir (sessizce dondurmasın).
	if !(f >= 0) || math.IsInf(float64(float32(f)), 0) || (f != 0 && float32(f) == 0) {
		return messages.New(messages.H6504, o.ad, codes.NameRef(memberLR), interp.Shown(value))
	}
	*o.rateField() = float32(f)
	return nil
}

// optimizerType AdamW ve SGD değerlerinin tip adıdır (tip(op) → "İyileştirici").
var optimizerType = interp.ValueType(interp.TypeName(codes.TypeOptimizer))

func (o *Optimizer) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case memberStep:
		return &interp.BuiltinVal{Name: codes.Name(memberStep), NoNamedArgs: true, Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, messages.New(messages.H6506, codes.NameRef(memberStep))
			}
			if _, err := safe(func() bool { o.opt.Step(); return true }); err != nil {
				return nil, err
			}
			return interp.Null, nil
		}}, nil
	case memberZeroGrad:
		return &interp.BuiltinVal{Name: codes.Name(memberZeroGrad), NoNamedArgs: true, Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, messages.New(messages.H6506, codes.NameRef(memberZeroGrad))
			}
			o.opt.ZeroGrad()
			return interp.Null, nil
		}}, nil
	case memberLR:
		// Oran 32 bit saklanır; en kısa gösterimiyle verilir (0.003 → 0.003).
		return &interp.FloatVal{Val: shortFloat64(*o.rateField())}, nil
	}
	if v, ok, err := o.stateMember(kd); ok {
		return v, err // durum, durumu_yükle (iyilestirici_durum.go)
	}
	if s, ok := bind.ClosestMember(prop, optimizerMembers); ok {
		return nil, messages.New(messages.H6530, o.ad, prop, s)
	}
	return nil, messages.New(messages.H6529, o.ad, prop)
}

// parameters iyileştiriciye verilen tensör ya da tensör listesini denetler.
func parameters(fn any, v interp.Value) ([]*tensor.Tensor, error) {
	var list []interp.Value
	switch x := v.(type) {
	case *TensorValue:
		list = []interp.Value{x}
	case *interp.ListVal:
		list = x.Elements
	default:
		return nil, messages.New(messages.H6509, fn, typeName(v))
	}
	if len(list) == 0 {
		return nil, messages.New(messages.H6510, fn)
	}
	out := make([]*tensor.Tensor, 0, len(list))
	seen := map[*tensor.Tensor]bool{}
	for i, el := range list {
		d, ok := el.(*TensorValue)
		if !ok {
			return nil, messages.New(messages.H6512, fn, i+1, typeName(el))
		}
		t := d.t
		if !t.RequiresGrad() {
			return nil, messages.New(messages.H6513, fn, i+1, tensor.ShapeText(t.Shape()), gradExample("..."))
		}
		if d.intermediate || !t.IsLeaf() {
			return nil, messages.New(messages.H6514, fn, i+1, tensor.ShapeText(t.Shape()), gradExample("..."))
		}
		if seen[t] {
			return nil, messages.New(messages.H6515, fn, i+1)
		}
		seen[t] = true
		out = append(out, t)
	}
	return out, nil
}

// namedRate adlı sayısal bir argümanı okur.
func namedRate(fn any, named map[string]interp.Value, defaultVal float64, minVal float64, ad bind.Named) (float32, error) {
	v, ok := bind.GetNamed(named, ad)
	if !ok {
		return float32(defaultVal), nil
	}
	f, ok := interp.ToFloat(v)
	if !ok {
		return 0, messages.New(messages.H6517, fn, ad, typeName(v))
	}
	// NaN ve sonsuz (float32'ye sığmayan) değerler reddedilir: tek adımda bütün
	// parametreleri NaN yapardı (f < minVal denetimi NaN için yanlıştır).
	if math.IsNaN(f) || math.IsInf(float64(float32(f)), 0) {
		return 0, messages.New(messages.H6519, fn, ad, interp.Shown(v))
	}
	if f < minVal {
		return 0, messages.New(messages.H6528, fn, ad, fmt.Sprintf("%g", minVal), fmt.Sprintf("%g", f))
	}
	return float32(f), nil
}

// gradClip parametre gradyanlarının toplam L2 normunu en_fazla değerine indirir ve
// kırpma öncesi normu 0 boyutlu tensör olarak döndürür: gradyan_kırp(parametreler, en_fazla: 1.0).
// sonlu_değilse_hata: doğru normu kırpmadan önce okur (GPU'yu bekler) ve NaN ya da sonsuzsa
// gradyanlara dokunmadan hata verir.
func gradClip(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnClipGrad)
	if len(args) != 1 {
		return nil, messages.New(messages.H6521, fn, nameMax, len(args))
	}
	if err := bind.CheckNamed(fn.Called(), named, nameMax, nameErrNonfinite); err != nil {
		return nil, err
	}
	strict, err := bind.OptionalBool(fn, named, nameErrNonfinite, false)
	if err != nil {
		return nil, err
	}
	params, err := parameters(fn, args[0])
	if err != nil {
		return nil, err
	}
	maxVal, err := positiveFloat(fn, named, nameMax, 1.0)
	if err != nil {
		return nil, err
	}
	// Norm 0 boyutlu tensördür (kayıp gibi): GPU'da norm ve kırpma cihazda yapılır, program
	// ancak norm okununca (.değer, yazdırma) bekler.
	if strict {
		// Önce kırpmadan (sınır +∞: ölçek yok) norm alınır ve denetlenir; hata verilirse
		// gradyanlar değişmemiştir (PyTorch clip_grad_norm_ de error_if_nonfinite'te önce
		// denetler). Bu kip GPU'yu zaten beklediği için normun iki kez alınması ölçülmez.
		probe, err := safe(func() *tensor.Tensor { return tensor.ClipGradNormTensor(params, math.Inf(1)) })
		if err != nil {
			return nil, err
		}
		n := float64(probe.Data()[0])
		probe.Drop()
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, messages.New(messages.H6534, fn, n, codes.NameRef(codes.FnIsFinite))
		}
	}
	norm, err := safe(func() *tensor.Tensor { return tensor.ClipGradNormTensor(params, maxVal) })
	if err != nil {
		return nil, err
	}
	return New(norm), nil
}

func optimizerParams(fn any, args []interp.Value) ([]*tensor.Tensor, error) {
	if len(args) != 1 {
		return nil, messages.New(messages.H6525, fn, nameLR, len(args))
	}
	return parameters(fn, args[0])
}

func setupAdamW(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnAdamW) // AdamW
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
		return nil, messages.New(messages.H6526, fn)
	}
	eps, err := namedRate(fn, named, 1e-8, 0, nameEpsilon)
	if err != nil {
		return nil, err
	}
	if eps <= 0 {
		// Sıfır gradyanlı ilk adımda 0 / (√0 + 0) = NaN olurdu (float32'de 0'a düşen değer de).
		v, _ := bind.GetNamed(named, nameEpsilon)
		return nil, messages.New(messages.H6527, fn, nameEpsilon, interp.Shown(v))
	}
	masterCopy, err := bind.OptionalBool(fn, named, nameMasterCopy, true)
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
	fn := codes.NameRef(codes.FnSGD) // SGD
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
	masterCopy, err := bind.OptionalBool(fn, named, nameMasterCopy, true)
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
