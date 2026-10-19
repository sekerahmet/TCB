package interp

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// donguOrtami bir döngünün her turunda gövdenin çalışacağı ortamı sağlar. Gövde ortamı
// yakalayamıyorsa (içinde fonksiyon, ertele, eşzamanlı, yapı yoksa) tek bir ortam turlar
// arasında temizlenip yeniden kullanılır; aksi hâlde her tur yeni ortam açılır.
type loopScope struct {
	outerEnv *Environment
	info     *scopeInfo
	env      *Environment
	again    bool
}

func (d *loopScope) fresh() *Environment {
	if d.env != nil && d.again {
		d.env.clear()
		return d.env
	}
	d.env = newEnv(d.outerEnv, d.info)
	return d.env
}

// donguIcinde 'döngü x içinde ...' deyimini derler. Döngü değişkenleri ve gövdenin
// tanımları her turun kendi ortamındadır.
func (c *compiler) inLoop(k *compilerScope, s *ast.ForInStatement) statementCode {
	source := c.expression(k, s.Iterable)
	sub := subScope(k)
	keySlot := -1
	if s.KeyVar != "" {
		keySlot = sub.info.slot(s.KeyVar)
		sub.recordShadow(s.KeyVar, s.Line(), "bir döngü değişkenine")
	}
	valueSlot := sub.info.slot(s.ValueVar)
	sub.recordShadow(s.ValueVar, s.Line(), "bir döngü değişkenine")
	var bodyStatements []ast.Statement
	again := true
	if s.Body != nil {
		bodyStatements = s.Body.Statements
		again = blockFlags(s.Body)&blockCaptures == 0
	}
	sub.predefine(bodyStatements)
	bodyBlock := c.statements(sub, bodyStatements)
	info := sub.info
	line := s.Line()

	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		iterVal, err := source(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		kind := loopScope{outerEnv: env, info: info, again: again}
		var lastVal Value = NULL
		var handoff uint32
		switch it := iterVal.(type) {
		case *RangeVal:
			first, last, step, empty := rangeSteps(it)
			if empty {
				break
			}
			idx := int64(0)
			// Son değer dahil gezilir ve döngü onda biter: int64 sınırında taşma olmaz.
			for i := first; ; i += step {
				if handoff++; handoff&255 == 0 {
					v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
				}
				loopEnv := kind.fresh()
				if keySlot >= 0 {
					loopEnv.vals[keySlot] = NewInt(idx)
				}
				loopEnv.vals[valueSlot] = NewInt(i)
				val, sig, err := bodyBlock.run(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
				if i == last {
					break
				}
				idx++
			}

		case *ListVal:
			for idx, item := range it.Elements {
				if handoff++; handoff&255 == 0 {
					v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
				}
				loopEnv := kind.fresh()
				if keySlot >= 0 {
					loopEnv.vals[keySlot] = NewInt(int64(idx))
				}
				loopEnv.vals[valueSlot] = item
				val, sig, err := bodyBlock.run(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}

		case *DictVal:
			for _, key := range it.Order {
				if handoff++; handoff&255 == 0 {
					v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
				}
				loopEnv := kind.fresh()
				if keySlot >= 0 {
					loopEnv.vals[keySlot] = it.KeyValue(key)
					loopEnv.vals[valueSlot] = it.Pairs[key]
				} else {
					loopEnv.vals[valueSlot] = it.KeyValue(key)
				}
				val, sig, err := bodyBlock.run(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}

		case *StringVal:
			for idx, ch := range []rune(it.Val) {
				if handoff++; handoff&255 == 0 {
					v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
				}
				loopEnv := kind.fresh()
				if keySlot >= 0 {
					loopEnv.vals[keySlot] = NewInt(int64(idx))
				}
				loopEnv.vals[valueSlot] = &StringVal{Val: string(ch)}
				val, sig, err := bodyBlock.run(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}

		case Iterable:
			// Dış değerler (ör. tensör: ilk eksen boyunca öğeler)
			n, err := it.IterLen()
			if err != nil {
				return nil, signalNone, lineSuffix(line, err)
			}
			for idx := 0; idx < n; idx++ {
				if handoff++; handoff&255 == 0 {
					v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
				}
				item, err := it.IterItem(idx)
				if err != nil {
					return nil, signalNone, lineSuffix(line, err)
				}
				loopEnv := kind.fresh()
				if keySlot >= 0 {
					loopEnv.vals[keySlot] = NewInt(int64(idx))
				}
				loopEnv.vals[valueSlot] = item
				val, sig, err := bodyBlock.run(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}

		case Stream:
			return streamLoop(v, it, &kind, keySlot, valueSlot, bodyBlock, line)

		default:
			return nil, signalNone, fmt.Errorf("Satır %d: %s tipi üzerinde döngü kurulamaz", line, iterVal.Type())
		}
		return lastVal, signalNone, nil
	}
}

// akisDongusu tembel bir kaynağı (Akis) biter, 'çık' ya da 'sonuç' gelene kadar gezer.
// Öğe sayısı önceden bilinmediğinden her tur Sonraki ile istenir.
func streamLoop(v *VM, it Stream, kind *loopScope, keySlot, valueSlot int, bodyBlock *blockCode, line int) (Value, flowSignal, error) {
	var lastVal Value = NULL
	var handoff uint32
	for idx := int64(0); ; idx++ {
		if handoff++; handoff&255 == 0 {
			v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
		}
		item, cont, err := it.Next(v)
		if err != nil {
			return nil, signalNone, lineSuffix(line, err)
		}
		if !cont {
			return lastVal, signalNone, nil
		}
		loopEnv := kind.fresh()
		if keySlot >= 0 {
			loopEnv.vals[keySlot] = NewInt(idx)
		}
		loopEnv.vals[valueSlot] = item
		val, sig, err := bodyBlock.run(v, loopEnv)
		if err != nil {
			return nil, signalNone, err
		}
		if sig == signalReturn {
			return val, signalReturn, nil
		}
		if sig == signalBreak {
			return lastVal, signalNone, nil
		}
		lastVal = val
	}
}

// iken 'iken koşul { ... }' deyimini derler. Gövde ad tanımlamıyorsa tur ortamı açılmaz.
func (c *compiler) whileStmt(k *compilerScope, s *ast.WhileStatement) statementCode {
	condition := c.expression(k, s.Condition)
	var bodyStatements []ast.Statement
	if s.Body != nil {
		bodyStatements = s.Body.Statements
	}
	if s.Body == nil || blockFlags(s.Body)&blockDefines == 0 {
		bodyBlock := c.statements(k, bodyStatements)
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			var lastVal Value = NULL
			var handoff uint32
			for {
				if handoff++; handoff&255 == 0 {
					v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
				}
				cond, err := condition(v, env)
				if err != nil {
					return nil, signalNone, err
				}
				if !isTrue(cond) {
					break
				}
				val, sig, err := bodyBlock.run(v, env)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}
			return lastVal, signalNone, nil
		}
	}
	sub := subScope(k)
	sub.predefine(bodyStatements)
	bodyBlock := c.statements(sub, bodyStatements)
	info := sub.info
	again := blockFlags(s.Body)&blockCaptures == 0
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		var lastVal Value = NULL
		kind := loopScope{outerEnv: env, info: info, again: again}
		var handoff uint32
		for {
			if handoff++; handoff&255 == 0 {
				v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
			}
			cond, err := condition(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if !isTrue(cond) {
				break
			}
			val, sig, err := bodyBlock.run(v, kind.fresh())
			if err != nil {
				return nil, signalNone, err
			}
			if sig == signalReturn {
				return val, signalReturn, nil
			}
			if sig == signalBreak {
				break
			}
			lastVal = val
		}
		return lastVal, signalNone, nil
	}
}

// dogruMu bir değerin doğruluğunu (Mantık değerleri için arayüz çağrısı yapmadan) verir.
func isTrue(x Value) bool {
	if b, ok := x.(*BoolVal); ok {
		return b.Val
	}
	return x.IsTruthy()
}

// --- LİSTE ÜRETECİ ---

func (c *compiler) listComprehension(k *compilerScope, lc *ast.ListComprehension) exprCode {
	source := c.expression(k, lc.Iterable)
	sub := subScope(k)
	varSlot := sub.info.slot(lc.VarName)
	exprNames(lc.Condition, sub.define)
	exprNames(lc.Output, sub.define)
	var condition exprCode
	if lc.Condition != nil {
		condition = c.expression(sub, lc.Condition)
	}
	output := c.expression(sub, lc.Output)
	info := sub.info
	// Çıktı ya da koşul fonksiyon oluşturmuyorsa ortam yakalanamaz; tek ortam yeterlidir.
	again := !exprCaptures(lc.Output) && !exprCaptures(lc.Condition)
	line := lc.Line()
	return func(v *VM, env *Environment) (Value, error) {
		iterVal, err := source(v, env)
		if err != nil {
			return nil, err
		}
		items, err := comprehensionItems(v, iterVal, line)
		if err != nil {
			return nil, err
		}
		var results []Value
		kind := loopScope{outerEnv: env, info: info, again: again}
		var handoff uint32
		for _, item := range items {
			if handoff++; handoff&255 == 0 {
				v.handOff() // her 256 turda bir: bekleyen göreve kilidi devret
			}
			compEnv := kind.fresh()
			compEnv.vals[varSlot] = item
			if condition != nil {
				condVal, err := condition(v, compEnv)
				if err != nil {
					return nil, err
				}
				if !condVal.IsTruthy() {
					continue
				}
			}
			outVal, err := output(v, compEnv)
			if err != nil {
				return nil, err
			}
			results = append(results, outVal)
		}
		return &ListVal{Elements: results}, nil
	}
}
