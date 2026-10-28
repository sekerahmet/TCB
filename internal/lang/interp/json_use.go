package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// JSON değerinin kullanımı (3.7a, 4.5, 5.7): JSON değeri yapısı önceden bilinmeyen veridir; asıl
// değere tip çağrısıyla geçilir (TR_Int(j)). Açılmadan kullanılan JSON değerinin hatası açan yazımı
// önerir: işleçte, JSON olmayan bir değerle karşılaştırmada ve yerleşiğin argümanında. Döngü dizinin
// öğelerinde ve nesnenin anahtarlarında döner.

// openable j'nin tip çağrısıyla açılan türüdür (Int, Float, String, Bool, dizi List); nesne ve
// JSON null açılmaz.
func (j *JSONVal) openable() bool {
	switch j.v.(type) {
	case *IntVal, *FloatVal, *StringVal, *BoolVal, *ListVal:
		return true
	}
	return false
}

// Operator, '==' ve '!=' dışındaki her işleçte JSON değerini reddeder: önce tipiyle açılır (4.2,
// 4.6, 4.10 JSON için tanımlı değildir).
func (j *JSONVal) Operator(op string, _ Value, _ bool) (Value, bool, error) {
	if j.openable() {
		return nil, true, messages.New(messages.H2662, op, j.kindName())
	}
	return nil, true, messages.New(messages.H2663, op, j.kindName())
}

// jsonCompareError JSON değeriyle JSON olmayan bir değerin '=='/'!='sinin hatasıdır (4.5): sonuç hep
// yanlış olurdu (j["a"] == 1 hiçbir zaman tutmaz). Yoklukla karşılaştırma serbesttir (3.7a: JSON
// null bir JSON değeridir, TR_none'a eşit değildir). İki yan da JSON değilse ya da ikisi de JSON'sa nil.
func jsonCompareError(op string, a, b Value) error {
	j, ok := a.(*JSONVal)
	other := b
	if !ok {
		j, ok = b.(*JSONVal)
		other = a
	}
	if !ok {
		return nil
	}
	switch other.(type) {
	case *JSONVal, *NullVal:
		return nil
	}
	if j.openable() {
		return messages.New(messages.H2660, op, j.kindName())
	}
	return messages.New(messages.H2661, op, j.kindName())
}

// jsonLoopSource döngünün JSON kaynağını açar (5.7): dizi öğelerini (JSON değerleri), nesne
// anahtarlarını (iki değişkenli döngüde anahtar ve JSON değeri) verir. Öbür türler hatadır. JSON
// olmayan kaynak olduğu gibi döner.
func jsonLoopSource(v Value) (Value, error) {
	j, ok := v.(*JSONVal)
	if !ok {
		return v, nil
	}
	switch x := j.v.(type) {
	case *ListVal:
		items := make([]Value, len(x.Elements))
		for i, e := range x.Elements {
			items[i] = &JSONVal{v: e}
		}
		return &ListVal{Elements: items}, nil
	case *DictVal:
		d := NewDictVal()
		for _, k := range x.Order {
			d.SetValue(x.KeyValue(k), &JSONVal{v: x.Pairs[k]})
		}
		return d, nil
	}
	return nil, messages.New(messages.H2664, j.kindName())
}

// jsonArgError, yerleşiğe beklenen tip yerine JSON değeri verildiğinde argüman hatasıdır; ileti
// değeri beklenen ilk tiple açmayı önerir (TR_Float(j)).
func jsonArgError(name any, order int, given *JSONVal, expected []codes.Code) error {
	return messages.New(messages.H2665, name, order, TypeNames(expected...), TypeAndSummary(given), codes.NameRef(expected[0]))
}

// Compile-time check: the interpreter finds OperatorHook by type assertion.
var _ OperatorHook = (*JSONVal)(nil)
