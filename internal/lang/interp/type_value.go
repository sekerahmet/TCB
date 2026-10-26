package interp

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tip değerleri (3.11): tip(x) değerin tipini bir tip değeri olarak verir; tip adları (Tam,
// Liste, Tensör ...) ifadede bu değerlerdir. Değer dilden bağımsızdır: tip kodunu (type) ya da
// yapı nesnelerinde yapının tanımını taşır. Yazdırılınca adı çalıştırma dilinde görünür;
// karşılaştırma değerle yapılır (tip(x) == Tam, tip(p) == Nokta). Böylece kodlu saklanan bir
// program hangi dilde gösterilirse gösterilsin aynı sonucu verir.

// ValType is the type of type values (TR_type_Type Tip).
var ValType = TypeName(codes.TypeType)

// TypeVal is a type value.
type TypeVal struct {
	// Code is the code of builtin and engine types (type); codes.None for the type of struct
	// instances.
	Code codes.Code
	// Struct is the type of struct instances (tip(p) → Nokta); nil for other types.
	Struct *StructDefVal
}

func (t *TypeVal) Type() ValueType { return ValType }
func (t *TypeVal) IsTruthy() bool  { return true }
func (t *TypeVal) Inspect() string { return t.Text(codes.Source()) }
func (t *TypeVal) Text(lang *codes.Language) string {
	if t.Struct != nil {
		return t.Struct.Name
	}
	return lang.Name(t.Code)
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

// typeValues canlı tip kodlarının paylaşılan tip değerleridir (koda göre dizi; çalışma anında
// ayırma ve ad araması yok). typeValueByName motor tiplerinin değer tipinden (ValueType: kodun
// adı) tip değerine eşlemesidir.
var (
	typeValues      []*TypeVal
	typeValueByName = map[ValueType]*TypeVal{}
)

func init() {
	all := codes.Codes()
	typeValues = make([]*TypeVal, int(slices.Max(all))+1)
	for _, k := range all {
		if k.Namespace() == codes.NamespaceType {
			t := &TypeVal{Code: k}
			typeValues[k] = t
			typeValueByName[TypeName(k)] = t
		}
	}
}

// TypeOfValue is the result of tip(v): v's type value.
func TypeOfValue(v Value) *TypeVal {
	switch x := v.(type) {
	case *IntVal:
		return typeValues[codes.TypeInt]
	case *FloatVal:
		return typeValues[codes.TypeFloat]
	case *StringVal:
		return typeValues[codes.TypeString]
	case *BoolVal:
		return typeValues[codes.TypeBool]
	case *NullVal:
		return typeValues[codes.TypeNone]
	case *ListVal:
		return typeValues[codes.TypeList]
	case *DictVal:
		return typeValues[codes.TypeDict]
	case *StructInstanceVal:
		return x.Def.typeValue()
	}
	if t := typeValueByName[v.Type()]; t != nil {
		return t
	}
	return typeValues[codes.TypeAny] // Herhangi: tipi kod listesinde olmayan değer
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
				v.GlobalEnv.Define(y, typeValues[k], true)
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
	return messages.New(messages.H2441, hint.TypeTextComparisonError(op, m.Val, t))
}

// Compile-time check: the interpreter finds Equaler by type assertion, so a renamed or changed
// method would otherwise drop it silently.
var _ Equaler = (*TypeVal)(nil)
