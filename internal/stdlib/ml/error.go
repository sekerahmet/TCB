package ml

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Çekirdek tensör paketi (github.com/sekerahmet/TCB/internal/compute/tensor) hataları panik ile bildirir. Bu dosyadaki
// yardımcılar her çekirdek çağrısını sarar ve paniği, TR'nin 'dene/yakala' ile
// yakalanabilen sıradan bir hataya çevirir. Yerleşik çağrılarında VM mesajın başına
// "Satır N:" ekler. Adlı argüman denetimi ve paniği yakalama internal/stdlib/common/bind'nın
// ortak yardımcılarıyla yapılır.

// Adlı argümanlar. Adları kod listesindedir (internal/lang/codes, arg). Başka dillerden alışılmış
// adlar (lr, axis...) kabul edilmez; bind.CheckNamed hata mesajında Türkçesini önerir.
var (
	nameRequiresGrad = bind.NamedArg(codes.ArgRequiresGrad)
	nameAxis         = bind.NamedArg(codes.ArgDim)
	nameSeed         = bind.NamedArg(codes.ArgSeed)
	nameMax          = bind.NamedArg(codes.ArgMax)
	nameErrNonfinite = bind.NamedArg(codes.ArgErrorIfNonfinite)
	nameMin          = bind.NamedArg(codes.ArgMin)
	nameEpsilon      = bind.NamedArg(codes.ArgEps)
	nameLR           = bind.NamedArg(codes.ArgLr)
	nameWeightDecay  = bind.NamedArg(codes.ArgWeightDecay)
	nameBeta1        = bind.NamedArg(codes.ArgBeta1)
	nameBeta2        = bind.NamedArg(codes.ArgBeta2)
	nameMomentum     = bind.NamedArg(codes.ArgMomentum)
	nameWeight       = bind.NamedArg(codes.ArgWeight)
	nameBias         = bind.NamedArg(codes.ArgBias)
	nameDType        = bind.NamedArg(codes.ArgDtype)
	nameMasterCopy   = bind.NamedArg(codes.ArgMasterWeights)
)

// gradExample hata mesajlarındaki türev izleyen tensör örneğidir:
// gradExample("...") → "tensör(..., türev_izle: doğru)"; adlar koddan kurulur.
func gradExample(data string) string {
	return codes.Source().Name(codes.FnTensor) + "(" + data + ", " + codes.Source().Name(nameRequiresGrad.Code) + ": " + codes.Source().Name(codes.KwTrue) + ")"
}

// safe fn'i çalıştırır; çekirdekten gelen paniği Türkçe bir hataya dönüştürür.
func safe[T any](fn func() T) (T, error) { return bind.Recover(fn, panicError) }

// panicError çekirdeğin panik değerini kullanıcıya gösterilecek hataya çevirir.
func panicError(r any) error {
	if e, ok := r.(error); ok {
		return messages.New(messages.H6006, e)
	}
	msg := fmt.Sprint(r)
	msg = strings.TrimPrefix(msg, "tensor: ")
	return messages.New(messages.H6006, msg)
}

// panicReason panicError iletisinin nedenini (çekirdeğin bildirdiği hata) verir; başka bir
// hatada hatanın kendisini verir. Kendi önekiyle yeniden yazan çağıranlar içindir.
func panicReason(err error) any {
	if m, ok := err.(*messages.Error); ok && m.Code == messages.H6006 {
		return m.Args[0]
	}
	return err
}

// intNumber bir TR değerini tam sayıya çevirir (tam değerli ondalıklar da kabul edilir).
func intNumber(v interp.Value) (int, bool) {
	n, ok := bind.Int(v)
	return int(n), ok
}

// shapeProduct bir şekli "2×3" biçiminde yazar (izleme masası ve yazdırma başlığı için).
func shapeProduct(sh []int) string { return joinInts(sh, "×") }

// joinInts sayıları ayraçla birleştirir.
func joinInts(sh []int, sep string) string {
	parts := make([]string, len(sh))
	for i, d := range sh {
		parts[i] = strconv.Itoa(d)
	}
	return strings.Join(parts, sep)
}

// parseAxis negatif ekseni (sondan sayma) pozitife çevirir ve sınırı denetler.
func parseAxis(fn any, axis int, sh []int) (int, error) {
	nd := len(sh)
	if nd == 0 {
		return 0, messages.New(messages.H6015, fn)
	}
	e := axis
	if e < 0 {
		e += nd
	}
	if e < 0 || e >= nd {
		return 0, messages.New(messages.H6016, fn, axis, tensor.ShapeText(sh), nd, -nd, nd-1)
	}
	return e, nil
}

// typeName hata mesajlarında değerin tipini adlandırır.
func typeName(v interp.Value) interp.ValueType { return v.Type() } // iletide dile göre (messages.Localized)

// argText argümanları virgülle ayrılmış biçimde yazar.
func argText(args []interp.Value) string {
	p := make([]string, len(args))
	for i, a := range args {
		p[i] = a.Inspect()
	}
	return strings.Join(p, ", ")
}

// shapeList [2, 3] gibi bir TR listesini şekle çevirir.
func shapeList(fn any, v interp.Value) ([]int, error) {
	l, ok := v.(*interp.ListVal)
	if !ok {
		return nil, messages.New(messages.H6021, fn, typeName(v))
	}
	sh := make([]int, len(l.Elements))
	for i, el := range l.Elements {
		d, ok := intNumber(el)
		if !ok || d < 0 {
			return nil, messages.New(messages.H6024, fn, i+1, interp.Shown(el))
		}
		sh[i] = d
	}
	if err := checkShapeLimit(fn, sh); err != nil {
		return nil, err
	}
	return sh, nil
}

// checkShapeLimit şeklin eleman sınırını aşmadığını denetler (tensorLimit; tensör_yükle ile
// aynı kural). Çarpım taşmadan denetlenir; -1 (çıkarsanacak eksen) atlanır.
func checkShapeLimit(fn any, sh []int) error {
	dims := make([]uint64, 0, len(sh))
	for _, d := range sh {
		dims = append(dims, uint64(max(d, 1)))
	}
	hasZero := false
	for _, d := range sh {
		hasZero = hasZero || d == 0
	}
	if i, bound := tensorLimit(dims, hasZero); i >= 0 {
		return messages.New(messages.H6028, fn, tensor.ShapeText(sh[:i+1]), bound)
	}
	return nil
}

// maxTensorElements veri taşıyan bir tensörün en çok eleman sayısıdır (dil tanımı 15.46). Liste
// sınırından (interp.MaxCollectionSize) ayrıdır: hazır modellerin tabloları onu aşar (Qwen3-0.6B
// gömmesi 155 milyon, 70B modellerinkiler 1 milyar eleman). Asıl koruma ayırmadaki fiziksel
// bellek denetimi (tensor paketi) ve dosyalarda bildirilen verinin dosyada bulunmasıdır; bu
// sınır yalnızca hiçbir zaman karşılanamayacak şekilleri erken durdurur.
const maxTensorElements = 1 << 32

// tensorLimit şeklin (eksenler, sıfır uzunluklular 1 yazılmış) sınırı aştığı ilk eksenin
// indeksini ve uygulanan sınırı verir; aşmıyorsa -1. Sıfır eksenli şekil veri tutmaz ama öbür
// eksenleri işlemlerin çıktısını ([4, 0] @ [0, N] → [4, N]) ve .değer'in liste sayısını belirler:
// verisiz dev şekil (126 baytlık dosyada [1048576, 0, 8192], güvenlik denetimi 2026-10-18)
// liste sınırına tabidir; veri taşıyan tensör maxTensorElements'e.
func tensorLimit(dims []uint64, hasZero bool) (int, int64) {
	bound := uint64(maxTensorElements)
	if hasZero {
		bound = uint64(interp.MaxCollectionSize)
	}
	limit := uint64(1)
	for i, d := range dims {
		d = max(d, 1)
		if d > bound || limit > bound/d {
			return i, int64(bound)
		}
		limit *= d
	}
	return -1, int64(bound)
}
