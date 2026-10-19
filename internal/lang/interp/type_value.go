package interp

import (
	"errors"
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// Tip değerleri (3.11): tip(x) değerin tipini bir tip değeri olarak verir; tip adları (Tam,
// Liste, Tensör ...) ifadede bu değerlerdir. Değer dilden bağımsızdır: tip kodunu (A2xx) ya da
// yapı nesnelerinde yapının tanımını taşır. Yazdırılınca adı çalıştırma dilinde görünür;
// karşılaştırma değerle yapılır (tip(x) == Tam, tip(p) == Nokta). Böylece kodlu saklanan bir
// program hangi dilde gösterilirse gösterilsin aynı sonucu verir.

// VAL_TYPE is the type of type values (A215 Tip).
var VAL_TYPE = TypeName(codes.A215)

// TypeVal is a type value.
type TypeVal struct {
	// Code is the code of builtin and engine types (A2xx); codes.None for the type of struct
	// instances.
	Code codes.Code
	// Struct is the type of struct instances (tip(p) → Nokta); nil for other types.
	Struct *StructDefVal
}

func (t *TypeVal) Type() ValueType { return VAL_TYPE }
func (t *TypeVal) IsTruthy() bool  { return true }
func (t *TypeVal) Inspect() string {
	if t.Struct != nil {
		return t.Struct.Name
	}
	return codes.Name(t.Code)
}

// Equal is the equality of type values: the same type code or the same struct. The type of a
// struct instance also equals the struct itself (tip(p) == Nokta).
func (t *TypeVal) Equal(b Value) bool {
	switch x := b.(type) {
	case *TypeVal:
		return t.Code == x.Code && t.Struct == x.Struct
	case *StructDefVal:
		return t.Struct == x
	}
	return false
}

// tipDegerleri canlı tip kodlarının (A2xx) paylaşılan tip değerleridir (kodun sayısına göre
// dizi; çalışma anında ayırma ve ad araması yok). tipDegeriAdla motor tiplerinin değer
// tipinden (ValueType: kodun adı) tip değerine eşlemesidir.
var (
	typeValues      [300]*TypeVal
	typeValueByName = map[ValueType]*TypeVal{}
)

func init() {
	for _, k := range codes.Codes() {
		if k.Namespace() == codes.NamespaceType {
			t := &TypeVal{Code: k}
			typeValues[k.Number()] = t
			typeValueByName[TypeName(k)] = t
		}
	}
}

// TypeValue returns the type value of a type code (A2xx).
func TypeValue(k codes.Code) *TypeVal {
	if k.Namespace() != codes.NamespaceType || typeValues[k.Number()] == nil {
		panic(fmt.Sprintf("TipDegeri: %s canlı bir tip kodu (A2xx) değil", k))
	}
	return typeValues[k.Number()]
}

// TypeOfValue is the result of tip(v): v's type value.
func TypeOfValue(v Value) *TypeVal {
	switch x := v.(type) {
	case *IntVal:
		return typeValues[codes.A200.Number()]
	case *FloatVal:
		return typeValues[codes.A201.Number()]
	case *StringVal:
		return typeValues[codes.A202.Number()]
	case *BoolVal:
		return typeValues[codes.A203.Number()]
	case *NullVal:
		return typeValues[codes.A204.Number()]
	case *ListVal:
		return typeValues[codes.A205.Number()]
	case *DictVal:
		return typeValues[codes.A206.Number()]
	case *StructInstanceVal:
		return x.Def.typeValue()
	}
	if t := typeValueByName[v.Type()]; t != nil {
		return t
	}
	return typeValues[codes.A213.Number()] // Herhangi: tipi kod listesinde olmayan değer
}

// typeValue yapının nesnelerinin tip değeridir; ilk kullanımda kurulur (yorumlayıcı kilidi
// altında).
func (s *StructDefVal) typeValue() *TypeVal {
	if s.typ == nil {
		s.typ = &TypeVal{Struct: s}
	}
	return s.typ
}

// registerTypes tip adlarını genel kapsama tip değerleri olarak bağlar (tip(x) == Tam).
func (v *VM) registerTypes() {
	for _, k := range codes.Codes() {
		if k.Namespace() == codes.NamespaceType {
			for _, y := range codes.Spellings(k) {
				v.GlobalEnv.Define(y, typeValues[k.Number()], true)
			}
		}
	}
}

// typeTextError tip değeri metinle karşılaştırılıyorsa yol gösteren hatayı verir (tip
// değeri hiçbir metne eşit değildir; sessizce yanlış dönmek yerine hata). Değilse nil.
func typeTextError(op string, a, b Value) error {
	t, ok := a.(*TypeVal)
	m, ok2 := b.(*StringVal)
	if !ok || !ok2 {
		t, ok = b.(*TypeVal)
		m, ok2 = a.(*StringVal)
		if !ok || !ok2 {
			return nil
		}
	}
	return errors.New(hint.TypeTextComparison(op, m.Val, t.Inspect()))
}

// Compile-time check: the interpreter finds Equaler by type assertion, so a renamed or changed
// method would otherwise drop it silently.
var _ Equaler = (*TypeVal)(nil)
