package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// sequenceOp metnin ve listenin işleçleridir: metinler kod noktası sırasıyla karşılaştırılır
// (4.4; UTF-8 bayt sırası kod noktası sırasıyla aynıdır); '+' yalnız iki metni ya da iki listeyi
// uç uca ekler, başka tiple hata verir (4.10, ileti yerleştirmeyi önerir); '*' metni ya da listeyi
// bir TR_Int kadar yineler, iki sırayla da (sıfır ya da eksi boş). İşleç burada ele alınmıyorsa
// ok=false.
func sequenceOp(codeVal uint8, left, right Value, line int) (Value, bool, error) {
	ls, lStr := left.(*StringVal)
	rs, rStr := right.(*StringVal)
	ll, lList := left.(*ListVal)
	rl, rList := right.(*ListVal)
	switch codeVal {
	case opLt, opGt, opLe, opGe:
		if !lStr || !rStr {
			return nil, false, nil
		}
		c := strings.Compare(ls.Val, rs.Val)
		switch codeVal {
		case opLt:
			return BoolToVal(c < 0), true, nil
		case opGt:
			return BoolToVal(c > 0), true, nil
		case opLe:
			return BoolToVal(c <= 0), true, nil
		}
		return BoolToVal(c >= 0), true, nil
	case opAdd:
		switch {
		case lList && rList:
			combined := make([]Value, 0, len(ll.Elements)+len(rl.Elements))
			combined = append(combined, ll.Elements...)
			combined = append(combined, rl.Elements...)
			return &ListVal{Elements: combined}, true, nil
		case (lList || rList) && !lStr && !rStr:
			// Listeye öğe eklemek isteyen için ayrı ileti: metin yerleştirmesi orada yol değildir.
			return nil, true, messages.At(line, 0, messages.New(messages.H2614, left.Type(), right.Type()))
		case lStr || rStr || lList || rList:
			return nil, true, messages.At(line, 0, messages.New(messages.H2609, left.Type(), right.Type()))
		}
	case opMul:
		seq, count := left, right
		if _, ok := left.(*IntVal); ok {
			seq, count = right, left
		}
		n, ok := count.(*IntVal)
		if !ok {
			return nil, false, nil
		}
		return repeat(seq, n.Val, line)
	}
	return nil, false, nil
}

// repeat metni ya da listeyi n kez yineler (4.10); n sıfır ya da eksiyse sonuç boştur. Liste
// öğeleri paylaşılır (Python gibi); iç içe listenin yinelenmesinin denetimi 4. katmandadır.
func repeat(seq Value, n int64, line int) (Value, bool, error) {
	if n < 0 {
		n = 0
	}
	switch s := seq.(type) {
	case *StringVal:
		if !mulFits(int64(len(s.Val)), n) || int64(len(s.Val))*n > maxRepeatBytes {
			return nil, true, messages.At(line, 0, messages.New(messages.H2610, n, maxRepeatBytes))
		}
		return &StringVal{Val: strings.Repeat(s.Val, int(n))}, true, nil
	case *ListVal:
		// Boş liste ne kadar yinelenirse yinelensin boştur; n kez boşuna dönülmez.
		if len(s.Elements) == 0 || n == 0 {
			return &ListVal{Elements: []Value{}}, true, nil
		}
		if !mulFits(int64(len(s.Elements)), n) || int64(len(s.Elements))*n > maxRepeatItems {
			return nil, true, messages.At(line, 0, messages.New(messages.H2615, n, maxRepeatItems))
		}
		out := make([]Value, 0, len(s.Elements)*int(n))
		for range n {
			out = append(out, s.Elements...)
		}
		return &ListVal{Elements: out}, true, nil
	}
	return nil, false, nil
}

// Yinelemenin sonucu en çok 2^31 bayt metin ya da 2^27 öğe liste olur (4.10): daha büyüğü belleği
// bir anda tüketir. Listede bir öğe bir Value arayüzüdür (16 bayt), iki sınır aynı belleği tutar.
const (
	maxRepeatBytes = 1 << 31
	maxRepeatItems = 1 << 27
)
