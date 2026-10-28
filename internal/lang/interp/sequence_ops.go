package interp

import (
	"strings"
	"unicode/utf8"

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
		if !mulFits(int64(len(s.Val)), n) || int64(len(s.Val))*n > maxRepeat {
			return nil, true, messages.At(line, 0, messages.New(messages.H2610, utf8.RuneCountInString(s.Val), n))
		}
		return &StringVal{Val: strings.Repeat(s.Val, int(n))}, true, nil
	case *ListVal:
		if !mulFits(int64(len(s.Elements)), n) || int64(len(s.Elements))*n > maxRepeat {
			return nil, true, messages.At(line, 0, messages.New(messages.H2610, len(s.Elements), n))
		}
		out := make([]Value, 0, len(s.Elements)*int(n))
		for range n {
			out = append(out, s.Elements...)
		}
		return &ListVal{Elements: out}, true, nil
	}
	return nil, false, nil
}

// maxRepeat yinelemenin en çok öğe ya da bayt sayısıdır: daha büyüğü belleği bir anda tüketir.
const maxRepeat = 1 << 31
