package ml

import (
	"fmt"
	"strconv"
	"strings"

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

// Adlı argümanlar. Adları kod listesindedir (internal/lang/codes, T9xx). Başka dillerden alışılmış
// adlar (lr, axis...) kabul edilmez; bind.CheckNamed hata mesajında Türkçesini önerir.
var (
	nameRequiresGrad = bind.NamedArg(codes.T900)
	nameAxis         = bind.NamedArg(codes.T902)
	nameSeed         = bind.NamedArg(codes.T903)
	nameMax          = bind.NamedArg(codes.T904)
	nameErrNonfinite = bind.NamedArg(codes.T986)
	nameMin          = bind.NamedArg(codes.T907)
	nameEpsilon      = bind.NamedArg(codes.T905)
	nameLR           = bind.NamedArg(codes.T910)
	nameWeightDecay  = bind.NamedArg(codes.T911)
	nameBeta1        = bind.NamedArg(codes.T912)
	nameBeta2        = bind.NamedArg(codes.T913)
	nameMomentum     = bind.NamedArg(codes.T914)
	nameWeight       = bind.NamedArg(codes.T920)
	nameBias         = bind.NamedArg(codes.T921)
	nameDType        = bind.NamedArg(codes.T901)
	nameMasterCopy   = bind.NamedArg(codes.T915)
)

// gradExample hata mesajlarındaki türev izleyen tensör örneğidir:
// gradExample("...") → "tensör(..., türev_izle: doğru)"; adlar koddan kurulur.
func gradExample(data string) string {
	return codes.Name(codes.T100) + "(" + data + ", " + nameRequiresGrad.TR + ": " + codes.Name(codes.A040) + ")"
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

// shapeText bir şekli "[2, 3]" biçiminde yazar.
func shapeText(sh []int) string { return "[" + joinInts(sh, ", ") + "]" }

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
func parseAxis(fn string, axis int, sh []int) (int, error) {
	nd := len(sh)
	if nd == 0 {
		return 0, messages.New(messages.H6015, fn)
	}
	e := axis
	if e < 0 {
		e += nd
	}
	if e < 0 || e >= nd {
		return 0, messages.New(messages.H6016, fn, axis, shapeText(sh), nd, -nd, nd-1)
	}
	return e, nil
}

// typeName hata mesajlarında değerin tipini adlandırır.
func typeName(v interp.Value) string { return string(v.Type()) }

// argText argümanları virgülle ayrılmış biçimde yazar.
func argText(args []interp.Value) string {
	p := make([]string, len(args))
	for i, a := range args {
		p[i] = a.Inspect()
	}
	return strings.Join(p, ", ")
}

// shapeList [2, 3] gibi bir TR listesini şekle çevirir.
func shapeList(fn string, v interp.Value) ([]int, error) {
	l, ok := v.(*interp.ListVal)
	if !ok {
		return nil, messages.New(messages.H6021, fn, typeName(v))
	}
	sh := make([]int, len(l.Elements))
	for i, el := range l.Elements {
		d, ok := intNumber(el)
		if !ok || d < 0 {
			return nil, messages.New(messages.H6024, fn, i+1, el.Inspect())
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
func checkShapeLimit(fn string, sh []int) error {
	dims := make([]uint64, 0, len(sh))
	for _, d := range sh {
		dims = append(dims, uint64(max(d, 1)))
	}
	hasZero := false
	for _, d := range sh {
		hasZero = hasZero || d == 0
	}
	if i, bound := tensorLimit(dims, hasZero); i >= 0 {
		return messages.New(messages.H6028, fn, shapeText(sh[:i+1]), bound)
	}
	return nil
}

// maxTensorElements veri taşıyan bir tensörün en çok eleman sayısıdır (dil tanımı 15.10). Liste
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
