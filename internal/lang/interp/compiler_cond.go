package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- EĞER / EŞLE ---

func (c *compiler) ifExpr(k *compilerScope, e *ast.IfExpression) statementCode {
	condition := c.expression(k, e.Condition)
	condLine := e.Condition.Line()
	mainSrc := c.blockStatement(k, e.Consequence)
	otherCond := make([]exprCode, len(e.ElseIfs))
	otherLine := make([]int, len(e.ElseIfs))
	otherBranch := make([]statementCode, len(e.ElseIfs))
	for i, ei := range e.ElseIfs {
		otherCond[i] = c.expression(k, ei.Condition)
		otherLine[i] = ei.Condition.Line()
		otherBranch[i] = c.blockStatement(k, ei.Consequence)
	}
	var coalesce statementCode
	if e.Alternative != nil {
		coalesce = c.blockStatement(k, e.Alternative)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		cond, err := condition(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if ok, err := truth(cond, condLine); err != nil {
			return nil, signalNone, err
		} else if ok {
			return mainSrc(v, env)
		}
		for i, dk := range otherCond {
			elifCond, err := dk(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if ok, err := truth(elifCond, otherLine[i]); err != nil {
				return nil, signalNone, err
			} else if ok {
				return otherBranch[i](v, env)
			}
		}
		if coalesce != nil {
			return coalesce(v, env)
		}
		return Null, signalNone, nil
	}
}
func (c *compiler) matchExpr(k *compilerScope, e *ast.MatchExpression) statementCode {
	type statusCode struct {
		defaultVal bool
		patterns   []exprCode
		bodyBlock  statementCode
		value      exprCode
	}
	topic := c.expression(k, e.Subject)
	line := e.Line()
	caseList := make([]statusCode, len(e.Cases))
	for i, mc := range e.Cases {
		d := statusCode{defaultVal: mc.IsDefault, patterns: c.expressions(k, mc.Patterns)}
		if mc.Body != nil {
			d.bodyBlock = c.blockStatement(k, mc.Body)
		}
		if mc.ValueExpr != nil {
			d.value = c.armValue(k, mc.ValueExpr)
		}
		caseList[i] = d
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		subj, err := topic(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		for _, d := range caseList {
			matched := d.defaultVal
			if !matched {
				for _, pat := range d.patterns {
					patVal, err := pat(v, env)
					if err != nil {
						return nil, signalNone, err
					}
					ok, err := patternMatches(subj, patVal)
					if err != nil {
						return nil, signalNone, messages.At(line, 0, err)
					}
					if ok {
						matched = true
						break
					}
				}
			}
			if matched {
				if d.bodyBlock != nil {
					return d.bodyBlock(v, env)
				}
				if d.value != nil {
					val, err := d.value(v, env)
					return val, signalNone, err
				}
				return Null, signalNone, nil
			}
		}
		// Eşleşme her durumu kapsar (5.6): hiçbir kol tutmazsa sessizce yokluk vermez.
		return nil, signalNone, messages.At(line, 0, messages.New(messages.H2702, TypeAndSummary(subj)))
	}
}

// patternMatches 'eşle' deseninin konuya uyup uymadığını söyler (aralık desenleri dahil); desen konuyla
// '==' ile karşılaştırılır (5.6), JSON ve fonksiyon karşılaştırmasının hatası da onunkidir.
func patternMatches(subj, patVal Value) (bool, error) {
	if rng, ok := patVal.(*RangeVal); ok {
		if iv, ok := subj.(*IntVal); ok {
			if rng.Inclusive && iv.Val >= rng.Start && iv.Val <= rng.End {
				return true, nil
			} else if !rng.Inclusive && iv.Val >= rng.Start && iv.Val < rng.End {
				return true, nil
			}
		}
		return false, nil
	}
	return EqualValues(subj, patVal)
}

// armValue tek satırlık kolun ('desen => ifade') değerini derler. Kol kendi kapsamını açar (5.12):
// ifade adlı bir fonksiyon tanımlıyorsa ad kolun ortamında kalır, çevre kapsama yazılmaz. Ad
// tanımlamayan kollar için ortam açılmaz.
func (c *compiler) armValue(k *compilerScope, e ast.Expression) exprCode {
	defines := false
	ast.ExprDefinedNames(e, func(string) { defines = true })
	if !defines {
		return c.expression(k, e)
	}
	sub := subScope(k)
	ast.ExprDefinedNames(e, sub.define)
	value := c.expression(sub, e)
	info := sub.info
	info.local = !c.blocks.exprCaptures(e)
	return func(v *VM, env *Environment) (Value, error) {
		armEnv := newEnv(env, info)
		val, err := value(v, armEnv)
		armEnv.closeEnv()
		return val, err
	}
}
