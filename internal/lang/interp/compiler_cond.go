package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// --- EĞER / EŞLE ---

func (c *compiler) ifExpr(k *compilerScope, e *ast.IfExpression) statementCode {
	condition := c.expression(k, e.Condition)
	mainSrc := c.blockStatement(k, e.Consequence)
	otherCond := make([]exprCode, len(e.ElseIfs))
	otherBranch := make([]statementCode, len(e.ElseIfs))
	for i, ei := range e.ElseIfs {
		otherCond[i] = c.expression(k, ei.Condition)
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
		if isTrue(cond) {
			return mainSrc(v, env)
		}
		for i, dk := range otherCond {
			elifCond, err := dk(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if isTrue(elifCond) {
				return otherBranch[i](v, env)
			}
		}
		if coalesce != nil {
			return coalesce(v, env)
		}
		return NULL, signalNone, nil
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
	caseList := make([]statusCode, len(e.Cases))
	for i, mc := range e.Cases {
		d := statusCode{defaultVal: mc.IsDefault, patterns: c.expressions(k, mc.Patterns)}
		if mc.Body != nil {
			d.bodyBlock = c.blockStatement(k, mc.Body)
		}
		if mc.ValueExpr != nil {
			d.value = c.expression(k, mc.ValueExpr)
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
					if patternMatches(subj, patVal) {
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
				return NULL, signalNone, nil
			}
		}
		return NULL, signalNone, nil
	}
}

// patternMatches 'eşle' deseninin konuya uyup uymadığını söyler (aralık desenleri dahil).
func patternMatches(subj, patVal Value) bool {
	if rng, ok := patVal.(*RangeVal); ok {
		if iv, ok := subj.(*IntVal); ok {
			if rng.Inclusive && iv.Val >= rng.Start && iv.Val <= rng.End {
				return true
			} else if !rng.Inclusive && iv.Val >= rng.Start && iv.Val < rng.End {
				return true
			}
		}
		return false
	}
	return ValuesEqual(subj, patVal)
}
