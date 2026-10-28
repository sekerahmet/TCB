package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// loopScope bir döngünün her turunda gövdenin çalışacağı ortamı sağlar. Gövde ortamı
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

// turn yeni bir turun ortamını verir; önceki turun bağı kalmayan sahipli değerleri bırakılır
// (ownership.go).
func (d *loopScope) turn(v *VM) *Environment {
	e := d.fresh()
	v.boundary()
	return e
}

// close son turun ortamındaki bağları kaldırır ve gezilen kaynağın korumasını bırakır.
func (d *loopScope) close(iter Value) {
	if d.env != nil {
		d.env.closeEnv()
	}
	unpin(iter)
}

// inLoop 'döngü x içinde ...' deyimini derler. Döngü değişkenleri ve gövdenin
// tanımları her turun kendi ortamındadır.
func (c *compiler) inLoop(k *compilerScope, s *ast.ForInStatement) statementCode {
	source := c.expression(k, s.Iterable)
	sub := subScope(k)
	keySlot := -1
	if s.KeyVar != "" {
		keySlot = sub.info.slot(s.KeyVar)
		sub.recordShadow(s.KeyVar, s.Line(), messages.H2452)
	}
	valueSlot := sub.info.slot(s.ValueVar)
	sub.recordShadow(s.ValueVar, s.Line(), messages.H2452)
	var bodyStatements []ast.Statement
	again := true
	if s.Body != nil {
		bodyStatements = s.Body.Statements
		again = c.blockFlags(s.Body)&blockCaptures == 0
	}
	sub.predefine(bodyStatements)
	bodyBlock := c.statements(sub, bodyStatements)
	info := sub.info
	info.local = again
	line := s.Line()

	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		iterVal, err := source(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		kind := loopScope{outerEnv: env, info: info, again: again}
		pin(iterVal) // gövdenin deyim sınırları gezilen kaynağı bırakmasın
		defer kind.close(iterVal)
		var lastVal Value = Null
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
				v.loopTurn(&handoff)
				loopEnv := kind.turn(v)
				// Tam sayılar sahiplenilemez: yuvaya doğrudan yazılır (ownership.go).
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
				v.loopTurn(&handoff)
				loopEnv := kind.turn(v)
				if keySlot >= 0 {
					loopEnv.store(keySlot, NewInt(int64(idx)))
				}
				loopEnv.store(valueSlot, item)
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
				v.loopTurn(&handoff)
				loopEnv := kind.turn(v)
				if keySlot >= 0 {
					loopEnv.store(keySlot, it.KeyValue(key))
					loopEnv.store(valueSlot, it.Pairs[key])
				} else {
					loopEnv.store(valueSlot, it.KeyValue(key))
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
				v.loopTurn(&handoff)
				loopEnv := kind.turn(v)
				if keySlot >= 0 {
					loopEnv.store(keySlot, NewInt(int64(idx)))
				}
				loopEnv.store(valueSlot, &StringVal{Val: string(ch)})
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
				v.loopTurn(&handoff)
				item, err := it.IterItem(idx)
				if err != nil {
					return nil, signalNone, lineSuffix(line, err)
				}
				loopEnv := kind.turn(v)
				if keySlot >= 0 {
					loopEnv.store(keySlot, NewInt(int64(idx)))
				}
				loopEnv.store(valueSlot, item)
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
			return nil, signalNone, messages.At(line, 0, messages.New(messages.H2443, iterVal.Type()))
		}
		return lastVal, signalNone, nil
	}
}

// streamLoop tembel bir kaynağı (Akis) biter, 'çık' ya da 'sonuç' gelene kadar gezer.
// Öğe sayısı önceden bilinmediğinden her tur Sonraki ile istenir.
func streamLoop(v *VM, it Stream, kind *loopScope, keySlot, valueSlot int, bodyBlock *blockCode, line int) (Value, flowSignal, error) {
	var lastVal Value = Null
	var handoff uint32
	for idx := int64(0); ; idx++ {
		v.loopTurn(&handoff)
		item, cont, err := it.Next(v)
		if err != nil {
			return nil, signalNone, lineSuffix(line, err)
		}
		if !cont {
			return lastVal, signalNone, nil
		}
		loopEnv := kind.turn(v)
		if keySlot >= 0 {
			loopEnv.store(keySlot, NewInt(idx))
		}
		loopEnv.store(valueSlot, item)
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

// whileStmt 'iken koşul { ... }' deyimini derler. Gövde ad tanımlamıyorsa tur ortamı açılmaz.
func (c *compiler) whileStmt(k *compilerScope, s *ast.WhileStatement) statementCode {
	condition := c.expression(k, s.Condition)
	condLine := s.Condition.Line()
	var bodyStatements []ast.Statement
	if s.Body != nil {
		bodyStatements = s.Body.Statements
	}
	if s.Body == nil || c.blockFlags(s.Body)&blockDefines == 0 {
		bodyBlock := c.statements(k, bodyStatements)
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			var lastVal Value = Null
			var handoff uint32
			for {
				v.loopTurn(&handoff)
				v.boundary()
				cond, err := condition(v, env)
				if err != nil {
					return nil, signalNone, err
				}
				if ok, err := truth(cond, condLine); err != nil {
					return nil, signalNone, err
				} else if !ok {
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
	again := c.blockFlags(s.Body)&blockCaptures == 0
	info.local = again
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		var lastVal Value = Null
		kind := loopScope{outerEnv: env, info: info, again: again}
		defer kind.close(nil)
		var handoff uint32
		for {
			v.loopTurn(&handoff)
			cond, err := condition(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if ok, err := truth(cond, condLine); err != nil {
				return nil, signalNone, err
			} else if !ok {
				break
			}
			val, sig, err := bodyBlock.run(v, kind.turn(v))
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

// truth koşulun değeridir (3.10): koşul yalnız TR_Bool olur; başka tip çalışma hatasıdır (H2600),
// ileti açık sınamayı önerir. Doğruluk sınaması (boş liste yanlış) yoktur.
func truth(x Value, line int) (bool, error) {
	if b, ok := x.(*BoolVal); ok {
		return b.Val, nil
	}
	return false, messages.At(line, 0, messages.New(messages.H2600, x.Type()))
}

// operand TR_and, TR_or ve TR_not'un işlenenidir (4.6): yalnız TR_Bool (H2601).
func operand(x Value, line int) (bool, error) {
	if b, ok := x.(*BoolVal); ok {
		return b.Val, nil
	}
	return false, messages.At(line, 0, messages.New(messages.H2601, x.Type()))
}

// --- LİSTE ÜRETECİ ---

func (c *compiler) listComprehension(k *compilerScope, lc *ast.ListComprehension) exprCode {
	source := c.expression(k, lc.Iterable)
	sub := subScope(k)
	varSlot := sub.info.slot(lc.VarName)
	ast.ExprDefinedNames(lc.Condition, sub.define)
	ast.ExprDefinedNames(lc.Output, sub.define)
	var condition exprCode
	condLine := 0
	if lc.Condition != nil {
		condition = c.expression(sub, lc.Condition)
		condLine = lc.Condition.Line()
	}
	output := c.expression(sub, lc.Output)
	info := sub.info
	// Çıktı ya da koşul fonksiyon oluşturmuyorsa ortam yakalanamaz; tek ortam yeterlidir.
	again := !c.blocks.exprCaptures(lc.Output) && !c.blocks.exprCaptures(lc.Condition)
	info.local = again
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
		// Üretecin turları deyim sınırı değildir (bir ifadenin içindedir); sonuçlar liste
		// kurulunca ona bağlanır.
		pin(iterVal)
		defer kind.close(iterVal)
		var handoff uint32
		for _, item := range items {
			v.loopTurn(&handoff)
			compEnv := kind.fresh()
			compEnv.store(varSlot, item)
			if condition != nil {
				condVal, err := condition(v, compEnv)
				if err != nil {
					return nil, err
				}
				if ok, err := truth(condVal, condLine); err != nil {
					return nil, err
				} else if !ok {
					continue
				}
			}
			outVal, err := output(v, compEnv)
			if err != nil {
				return nil, err
			}
			pin(outVal) // liste kurulana kadar
			results = append(results, outVal)
		}
		l := &ListVal{Elements: results}
		v.ownList(l)
		for _, r := range results {
			unpin(r)
		}
		return l, nil
	}
}
