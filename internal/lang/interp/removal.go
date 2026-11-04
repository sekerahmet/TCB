package interp

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- ÇIKARMA: LİSTEDEN VE SÖZLÜKTEN ÖĞE ÇIKARMA, DOLAŞIRKEN DEĞİŞME DENETİMİ (5.7, 13.3, 13.4) ---

// iterMark dolaşılan kabın dolaşma başındaki boyu ve çıkarma sayısıdır. Boy tek başına yetmez: aynı
// turda bir öğe çıkarıp bir öğe eklemek boyu değiştirmez ama öğe atlanır ya da yinelenir.
type iterMark struct{ size, removed int }

// markOf source'un bugünkü iterMark'ıdır; liste ve sözlük dışındaki değerde boştur.
func markOf(source Value) iterMark {
	switch x := source.(type) {
	case *ListVal:
		return iterMark{len(x.Elements), x.removed}
	case *DictVal:
		return iterMark{len(x.Order), x.removed}
	}
	return iterMark{}
}

// changed dolaşılan liste ya da sözlüğe dolaşma başladığından bu yana öğe eklendiyse ya da ondan
// öğe çıkarıldıysa çalışma hatasıdır (5.7): sonuç belirsiz olurdu. Var olan öğeye ve anahtara atama
// serbesttir.
func (m iterMark) changed(source Value, line int) error {
	switch x := source.(type) {
	case *ListVal:
		if len(x.Elements) != m.size || x.removed != m.removed {
			return messages.At(line, 0, messages.New(messages.H2703, m.size, m.after(len(x.Elements), x.removed)))
		}
	case *DictVal:
		if len(x.Order) != m.size || x.removed != m.removed {
			return messages.At(line, 0, messages.New(messages.H2704, m.size, m.after(len(x.Order), x.removed)))
		}
	}
	return nil
}

// after iletideki yeni boydur: çıkarılan kadar öğe eklendiyse boy aynı kalmıştır, ileti "3 öğeden 3
// öğeye" demesin diye eklenmeden önceki boyu verir.
func (m iterMark) after(size, removed int) int {
	if size != m.size {
		return size
	}
	return max(size-(removed-m.removed), 0)
}

// removeAt pos'taki öğeyi çıkarıp verir. Boşalan yer öğeyi tutmaz: tensör listeden çıkınca çöp
// toplayıcıya kalabilsin; yer nil değil yok olur, eski dilimi okuyan bir dolaşma boş değer görmesin.
func (l *ListVal) removeAt(pos int) Value {
	item, last := l.Elements[pos], len(l.Elements)-1
	copy(l.Elements[pos:], l.Elements[pos+1:])
	l.Elements[last] = Null
	l.Elements = l.Elements[:last]
	l.removed++
	return item
}

// removeValue key anahtarını siler ve değerini verir; anahtar yoksa ok=false.
func (d *DictVal) removeValue(key Value) (Value, bool) {
	k := DictKey(key)
	val, ok := d.Pairs[k]
	if !ok {
		return nil, false
	}
	delete(d.Pairs, k)
	delete(d.KeyVals, k)
	i := slices.Index(d.Order, k)
	d.Order = slices.Delete(d.Order, i, i+1)
	d.removed++
	return val, true
}

// setupRemovalMethods çıkaran metotları kaydeder.
func setupRemovalMethods() {
	addMethod(typeList, codes.MemPop, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		l := inst.(*ListVal)
		if len(l.Elements) == 0 {
			return nil, messages.At(line, 0, messages.New(messages.H2533, CallName(codes.MemPop)))
		}
		return l.removeAt(len(l.Elements) - 1), nil
	})
	addMethod(typeList, codes.MemRemoveAt, signature{1, 1, []*argKind{intArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		l, index := inst.(*ListVal), args[0].(*IntVal).Val
		pos, ok := resolveIndex(index, len(l.Elements))
		if !ok {
			return nil, messages.At(line, 0, messages.New(messages.H2534, CallName(codes.MemRemoveAt), index, len(l.Elements)))
		}
		return l.removeAt(pos), nil
	})
	// d.TR_remove(k): olmayan anahtarda yok (13.4), .TR_get gibi.
	addMethod(typeDict, codes.MemRemove, signature{1, 1, nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		if err := DictKeyError(args[0]); err != nil {
			return nil, messages.At(line, 0, err)
		}
		if val, ok := inst.(*DictVal).removeValue(args[0]); ok {
			return val, nil
		}
		return Null, nil
	})
}
