package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Döngünün adları (5.7): indeks ya da anahtar, öğe ya da öğenin yerindeki demet deseni
// (TR_for (a, b) TR_in l, TR_for i, (a, b) TR_in l). Desenli döngüde her tur öğeyi desenin ilk adının
// yuvasına yazar (inLoop'un hızlı yolları değişmez); gövdenin önüne konan açma adımı onu oradan alıp
// demetin öğelerini adlara dağıtır. Öğe demet değilse H2731, demetin öğe sayısı ad sayısını tutmazsa
// H2709: tipi bilinen öğede ikisi de denetimde yakalanır (checker/loops.go), burada tipi bilinmeyen öğe
// için aynı kodla.

// loopSlots döngünün adlarına yuva ayırır: indeks ya da anahtar keySlot (yoksa -1), öğe valueSlot.
// Desenli döngüde unpack gövdenin önüne konacak açma adımıdır; desen yoksa nil. pairs iki adlı
// biçimin demet denetimidir (tek adlı ve desenli döngüde nil).
func (c *compiler) loopSlots(sub *compilerScope, s *ast.ForInStatement) (keySlot, valueSlot int, unpack statementCode, pairs *tupleGuard) {
	if s.KeyVar != "" && s.Pattern == nil {
		pairs = &tupleGuard{s}
	}
	slotOf := func(name string) int {
		slot := sub.info.slot(name)
		sub.recordShadow(name, s.Line(), messages.H2452)
		return slot
	}
	names := s.Names()
	keySlot = -1
	if s.KeyVar != "" {
		keySlot = slotOf(names[0].Name)
		names = names[1:]
	}
	slots := make([]int, len(names))
	for i, n := range names {
		slots[i] = slotOf(n.Name)
	}
	if s.Pattern == nil {
		return keySlot, slots[0], nil, pairs
	}
	return keySlot, slots[0], patternUnpack(s, slots), nil
}

// tupleGuard iki adlı biçimin çalışma anı denetimidir (5.7, K7): sözlük dışındaki kaynakta öğe demetse
// H2730, ileti iki deseni önerir. Tipi bilinen öğede hata denetimdedir (checker/loops.go); burada
// tipi bilinmeyen öğe, demet çıktığı turda. Aralık, metin ve sözlük kollarında çağrılmaz: aralığın ve
// metnin öğesi demet olamaz, sözlükte ikinci ad değerdir.
type tupleGuard struct{ s *ast.ForInStatement }

// check öğe demetse H2730 verir; g nil ise (tek adlı ya da desenli döngü) hiçbir şey yapmaz.
func (g *tupleGuard) check(item Value) error {
	if g == nil {
		return nil
	}
	t, ok := item.(*TupleVal)
	if !ok {
		return nil
	}
	pattern, index := g.s.TupleSuggestion(len(t.Elements))
	return messages.At(g.s.Line(), 0, messages.New(messages.H2730, len(t.Elements), pattern, index, g.s.IterableToken.Literal))
}

// patternUnpack desenin açma adımıdır: öğeyi desenin ilk adının yuvasından alır, öğelerini
// adların yuvalarına yazar. Sondan başa yazılır: demetin bağı ilk yuvada en son kalkar, öğeleri o
// ana kadar yuvalarına bağlanmıştır (ownership.go).
func patternUnpack(s *ast.ForInStatement, slots []int) statementCode {
	line := s.Line()
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		item := env.vals[slots[0]]
		t, ok := item.(*TupleVal)
		if !ok {
			kind := "other"
			if _, list := item.(*ListVal); list {
				kind = "list"
			}
			return nil, signalNone, messages.At(line, 0, messages.New(messages.H2731,
				s.PatternSpelling(), item.Type(), kind, s.PatternHead(), s.IterableToken.Literal))
		}
		if len(t.Elements) != len(slots) {
			return nil, signalNone, unpackCountError(s, len(t.Elements), len(slots)) // compiler_stmt.go
		}
		for i := len(slots) - 1; i >= 0; i-- {
			env.store(slots[i], t.Elements[i])
		}
		return Null, signalNone, nil
	}
}

// prepend gövdenin önüne bir adım koyar (döngü deseninin açması); adımın deyimi yoktur, hata
// ayıklayıcı onu atlar (consultDebugger, jumpTarget). step nil ise blok değişmez.
func (b *blockCode) prepend(step statementCode) {
	if step == nil {
		return
	}
	b.codeList = append([]statementCode{step}, b.codeList...)
	b.statements = append([]ast.Statement{nil}, b.statements...)
}
