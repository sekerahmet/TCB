package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Demet (3.6a): sabit sayıda, her biri kendi tipinde değeri bir arada tutan değiştirilemez değer;
// değişmezi (a, b), birden çok değer döndüren fonksiyonun sonucu (TR_return a, b; 6.3). Öğeler t.0,
// t.1 ile okunur, atanamaz. Açılır (TR_var a, b = t; 5.3), tutulur, argüman olur, listeye konur.
// '==' öğe öğedir, öğeleri sıralanabilen demet sözlük sırasıyla karşılaştırılır (4.4, 13.10),
// öğeleri anahtar olabilen demet sözlük anahtarıdır (3.7). Değerin metni (1, "a"), tipinin metni
// (Int, String).

// TupleVal is a tuple (3.6a). Its elements never change.
type TupleVal struct {
	Elements []Value
	// own demetin sahiplik durumu: ifadede kurulan ya da bir işlemin ürettiği demet içindeki
	// değerlerin sahibidir, sahip liste gibi (ownership.go, 15.43).
	own Ownership
}

// Type is the tuple's type, its elements' types in parentheses: "(Int, String)". The text is built
// when asked; a tuple holds no type of its own.
func (t *TupleVal) Type() ValueType { return tupleTypeName(t.Elements) }
func (t *TupleVal) Inspect() string { return writeContainer(t, nil, nil) }
func (t *TupleVal) IsTruthy() bool  { return true }

var _ Owned = (*TupleVal)(nil)

// Ownership (Owned) demetin sahiplik durumu: sahip demet öğelerinin sahibidir.
func (t *TupleVal) Ownership() *Ownership { return &t.own }

// Release (Owned) sahip demetin öğelerindeki bağını kaldırır.
func (t *TupleVal) Release() {
	for _, e := range t.Elements {
		unbind(e)
	}
}

// tupleTypeName öğelerin tiplerinin parantez içindeki yazımıdır; iletide tip adları iletinin
// dilindedir (ValueType.MessageText, tupleTypeText).
func tupleTypeName(elems []Value) ValueType {
	var sb strings.Builder
	sb.WriteByte('(')
	for i, e := range elems {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(string(e.Type()))
	}
	sb.WriteByte(')')
	return ValueType(sb.String())
}

// tupleTypeText demet tipinin ("(Int, (Bool, String))") iletinin dilindeki yazımıdır: en dış
// parantezin içi en üst düzeydeki virgüllerden bölünür, her parça ayrı çevrilir.
func tupleTypeText(t ValueType, lang string) string {
	s := string(t)
	inner := s[1 : len(s)-1]
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, inner[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, inner[start:])
	for i, p := range parts {
		parts[i] = ValueType(strings.TrimSpace(p)).MessageText(lang)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// tuple demet değişmezinin ve 'TR_return a, b'nin (6.3) kodudur: öğeler soldan sağa hesaplanır;
// sahiplenilmiş bir öğe taşıyan demet öğelerinin sahibidir (ownContainer).
func (c *compiler) tuple(k *compilerScope, e *ast.TupleExpression) exprCode {
	parts := c.expressions(k, e.Elements)
	return func(v *VM, env *Environment) (Value, error) {
		vals := make([]Value, len(parts))
		for i, p := range parts {
			x, err := p(v, env)
			if err != nil {
				return nil, err
			}
			vals[i] = x
		}
		t := &TupleVal{Elements: vals}
		v.ownContainer(t, vals)
		return t, nil
	}
}

// element t.0 okumasının kodudur. İndeks yazımdadır; sınır dışı indeks çalışma hatasıdır (tipi
// bilinen yerde denetim hatası, 4a2). Ara sonuç olan demet (f().0) okumadan sonra bırakılır; okunan
// öğe deyim sınırına kadar yaşar.
func (c *compiler) element(k *compilerScope, e *ast.ElementExpression) exprCode {
	object := c.expression(k, e.Object)
	index, line, col := e.Index, e.Line(), e.IndexToken.Column
	return func(v *VM, env *Environment) (Value, error) {
		obj, err := object(v, env)
		if err != nil {
			return nil, err
		}
		t, ok := obj.(*TupleVal)
		if !ok {
			return nil, messages.At(line, col, notTupleError(obj, index))
		}
		if index >= len(t.Elements) {
			return nil, messages.At(line, col, messages.New(messages.H2774, index, len(t.Elements), len(t.Elements)-1))
		}
		res := t.Elements[index]
		v.releaseTemp(t, res)
		return res, nil
	}
}

// notTupleError demet olmayan değerde t.0 okumasının hatasıdır; listede ileti indeksi önerir.
func notTupleError(obj Value, index int) error {
	kind := "other"
	switch obj.(type) {
	case *ListVal, *JSONVal:
		kind = "list"
	}
	return messages.New(messages.H2775, index, obj.Type(), kind)
}
