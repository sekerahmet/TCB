package ast

import "github.com/sekerahmet/TCB/internal/lang/codes"

// WriteStepKind yerinde yazmanın hedef zincirindeki bir adımın türüdür (15.6d). Hedef adlardan,
// alanlardan, kap öğelerinden ve görünüm işlemlerinden kurulur; öbür her adım kopya verir ya da
// verebilir, yazma kopyada kalır.
type WriteStepKind uint8

const (
	StepName    WriteStepKind = iota // zincirin kökü olan ad
	StepField                        // alan (p.w, d.ad, kitaplık yapısının d.TR_bias'ı) ya da .TR_grad (15'te alan sayılır)
	StepElement                      // demetin öğesi (p.0)
	StepIndex                        // indeks: kap öğesi, tensörde tam sayı ya da aralık (görünüm) ya da tensörle indeks (kopya); tipe bağlı
	StepView                         // motordaki görünüm metodu: .TR_slice, .TR_transpose(), .TR_permute
	StepOp                           // işlemin sonucu: t * 2.0, -t (kopya)
	StepCall                         // çağrının sonucu: kurucu, tip çağrısı, fonksiyon (kopya)
	StepMethod                       // görünüm olmayan metodun sonucu: .TR_to_dtype, .TR_reshape (kopya)
	StepOther                        // hedefin öğesi olamayan ifade ya da başka bir dil özelliği (kopya)
)

// viewMethods motordaki görünüm metotlarıdır (15.6d'nin listesinden motorda olanlar). Motora gelen
// görünüm (.TR_select, .TR_unsqueeze, .TR_detach(), daraltma) buraya eklenir.
var viewMethods = map[codes.Code]bool{codes.MemSlice: true, codes.MemTranspose: true, codes.MemPermute: true}

// WriteStep zincirin bir adımıdır. Expr adımın sonucu olan ifadedir; StepIndex'te Left ve Index
// indeksin iki yanıdır.
type WriteStep struct {
	Kind        WriteStepKind
	Expr        Expression
	Left, Index Expression
}

// Copies says the step gives a copy whatever the types are (StepIndex depends on the types).
func (s WriteStep) Copies() bool { return s.Kind >= StepOp }

// WriteChain e'nin zincirini yazmaya en yakın adımdan (e'nin kendisi) köke doğru verir. Zincir ilk
// kopya adımında durur: onun altı yazmaya ulaşmaz, adım o adımın ifadesiyle anılır.
func WriteChain(e Expression) []WriteStep {
	var steps []WriteStep
	for e != nil {
		s := writeStep(e)
		steps = append(steps, s)
		if s.Copies() || s.Kind == StepName {
			break
		}
		e = stepBase(e)
	}
	return steps
}

// writeStep e'nin adım türüdür (WriteChain).
func writeStep(e Expression) WriteStep {
	switch x := e.(type) {
	case *Identifier:
		return WriteStep{Kind: StepName, Expr: e}
	case *IndexExpression:
		return WriteStep{Kind: StepIndex, Expr: e, Left: x.Left, Index: x.Index}
	case *ElementExpression:
		return WriteStep{Kind: StepElement, Expr: e}
	case *MemberExpression:
		if k := x.NameToken.Code; k == codes.None || k == codes.MemGrad || k.IsLibrary() {
			return WriteStep{Kind: StepField, Expr: e} // programın ya da kitaplık yapısının alanı (d.TR_bias), .TR_grad
		}
		return WriteStep{Kind: StepOther, Expr: e} // .TR_tolist gibi dilin özelliği
	case *CallExpression:
		switch f := x.Function.(type) {
		case *MemberExpression:
			if viewMethods[f.NameToken.Code] {
				return WriteStep{Kind: StepView, Expr: e}
			}
			return WriteStep{Kind: StepMethod, Expr: e}
		case *Identifier:
			return WriteStep{Kind: StepCall, Expr: e}
		}
		return WriteStep{Kind: StepCall, Expr: e}
	case *InfixExpression, *PrefixExpression:
		return WriteStep{Kind: StepOp, Expr: e}
	}
	return WriteStep{Kind: StepOther, Expr: e}
}

// stepBase adımın bir altındaki ifadedir (alanın, öğenin ve görünümün nesnesi, indeksin solu).
func stepBase(e Expression) Expression {
	switch x := e.(type) {
	case *IndexExpression:
		return x.Left
	case *ElementExpression:
		return x.Object
	case *MemberExpression:
		return x.Object
	case *CallExpression:
		if f, ok := x.Function.(*MemberExpression); ok {
			return f.Object
		}
	}
	return nil
}

// LiteralIndex says the index can not be a tensor whatever the types are: an integer, a range, a
// string, a negative integer. A write chain checks the other indexes by their types.
func LiteralIndex(e Expression) bool {
	switch x := e.(type) {
	case *IntegerLiteral, *RangeExpression, *StringLiteral:
		return true
	case *PrefixExpression:
		_, ok := x.Right.(*IntegerLiteral)
		return ok
	}
	return false
}

// NearestCopy zincirde yazmaya en yakın kopya adımının sırasıdır; copies StepIndex adımının
// tensörle indeks olduğunu söyler. clean, o adımın altında başka kopya adımı olmadığıdır. Kopya
// adımı yoksa -1.
func NearestCopy(chain []WriteStep, copies func(i int) bool) (j int, clean bool) {
	isCopy := func(i int) bool {
		return chain[i].Copies() || chain[i].Kind == StepIndex && copies(i)
	}
	for i := range chain {
		if !isCopy(i) {
			continue
		}
		for b := i + 1; b < len(chain); b++ {
			if isCopy(b) {
				return i, false
			}
		}
		return i, true
	}
	return -1, false
}
