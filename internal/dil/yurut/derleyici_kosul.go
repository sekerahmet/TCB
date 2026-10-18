package yurut

import (
	"tr_dili/internal/dil/agac"
)

// --- EĞER / EŞLE ---

func (c *derleyici) eger(k *derleyiciKapsam, e *agac.IfExpression) deyimKodu {
	kosul := c.ifade(k, e.Condition)
	ana := c.blokDeyimi(k, e.Consequence)
	digerKosul := make([]ifadeKodu, len(e.ElseIfs))
	digerDal := make([]deyimKodu, len(e.ElseIfs))
	for i, ei := range e.ElseIfs {
		digerKosul[i] = c.ifade(k, ei.Condition)
		digerDal[i] = c.blokDeyimi(k, ei.Consequence)
	}
	var yoksa deyimKodu
	if e.Alternative != nil {
		yoksa = c.blokDeyimi(k, e.Alternative)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		cond, err := kosul(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if dogruMu(cond) {
			return ana(v, env)
		}
		for i, dk := range digerKosul {
			elifCond, err := dk(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if dogruMu(elifCond) {
				return digerDal[i](v, env)
			}
		}
		if yoksa != nil {
			return yoksa(v, env)
		}
		return NULL, signalNone, nil
	}
}
func (c *derleyici) esle(k *derleyiciKapsam, e *agac.MatchExpression) deyimKodu {
	type durumKodu struct {
		varsayilan bool
		desenler   []ifadeKodu
		govde      deyimKodu
		deger      ifadeKodu
	}
	konu := c.ifade(k, e.Subject)
	durumlar := make([]durumKodu, len(e.Cases))
	for i, mc := range e.Cases {
		d := durumKodu{varsayilan: mc.IsDefault, desenler: c.ifadeler(k, mc.Patterns)}
		if mc.Body != nil {
			d.govde = c.blokDeyimi(k, mc.Body)
		}
		if mc.ValueExpr != nil {
			d.deger = c.ifade(k, mc.ValueExpr)
		}
		durumlar[i] = d
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		subj, err := konu(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		for _, d := range durumlar {
			matched := d.varsayilan
			if !matched {
				for _, pat := range d.desenler {
					patVal, err := pat(v, env)
					if err != nil {
						return nil, signalNone, err
					}
					if desenUyar(subj, patVal) {
						matched = true
						break
					}
				}
			}
			if matched {
				if d.govde != nil {
					return d.govde(v, env)
				}
				if d.deger != nil {
					val, err := d.deger(v, env)
					return val, signalNone, err
				}
				return NULL, signalNone, nil
			}
		}
		return NULL, signalNone, nil
	}
}

// desenUyar 'eşle' deseninin konuya uyup uymadığını söyler (aralık desenleri dahil).
func desenUyar(subj, patVal Value) bool {
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
