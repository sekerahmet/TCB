package interp

import (
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// --- ATAMA ---

// atamaHedefi bir atama hedefini derler; dönen işlev sağ taraf değerini hedefe yazar
// (bileşik atamada önce mevcut değerle işlem yapar).
func (c *derleyici) atamaHedefi(k *derleyiciKapsam, target ast.Expression, kod uint8, line int) func(v *VM, env *Environment, rhs Value) error {
	bilesik := kod == opTopla || kod == opCikar || kod == opCarp || kod == opBol
	switch t := target.(type) {
	case *ast.Identifier:
		r := k.coz(t.Value)
		if r.tur == cozDinamik || r.tur == cozHenuzYok {
			return func(v *VM, env *Environment, rhs Value) error {
				return v.adlaAta(env, t, kod, rhs, line)
			}
		}
		i := r.yuva
		hizli := tamHizliIslev(kod)
		opMetni := atamaIslecMetni(kod)
		return func(v *VM, env *Environment, rhs Value) error {
			o := yuvaOrtami(r, env)
			if i < len(o.vals) {
				if cur := o.vals[i]; cur != nil && !o.sabitMi(i) {
					if !bilesik {
						o.vals[i] = rhs
						return nil
					}
					if li, ok := cur.(*IntVal); ok {
						if ri, ok := rhs.(*IntVal); ok {
							if x, ok := hizli(li.Val, ri.Val); ok {
								o.vals[i] = x
								return nil
							}
						}
					}
					yeni, err := v.evalInfixKod(kod, opMetni, cur, rhs, line)
					if err != nil {
						return err
					}
					o.vals[i] = yeni
					return nil
				}
			}
			return v.adlaAta(env, t, kod, rhs, line)
		}

	case *ast.IndexExpression:
		// A[i][j] = değer, A[i] okumasının sonucuna [j] ile atamadır (tensör satırı veriyi
		// paylaşan bir görünümdür; bkz. IndeksAtanabilir).
		sol := c.ifade(k, t.Left)
		indeks := c.ifade(k, t.Index)
		return func(v *VM, env *Environment, rhs Value) error {
			left, err := sol(v, env)
			if err != nil {
				return err
			}
			idx, err := indeks(v, env)
			if err != nil {
				return err
			}
			return v.indekseAta(left, idx, kod, rhs, line)
		}

	case *ast.MemberExpression:
		nesne := c.ifade(k, t.Object)
		prop := t.Property
		u := uyeCoz(prop)
		return func(v *VM, env *Environment, rhs Value) error {
			obj, err := nesne(v, env)
			if err != nil {
				return err
			}
			return v.uyeyeAta(obj, prop, u, kod, rhs, line)
		}
	}
	return func(v *VM, env *Environment, rhs Value) error {
		return fmt.Errorf("Satır %d: Geçersiz atama hedefi", line)
	}
}

// adlaAta bir ada (yuvası bilinmeden) atama yapar.
func (v *VM) adlaAta(env *Environment, t *ast.Identifier, kod uint8, rightVal Value, line int) error {
	var finalVal Value = rightVal
	if kod != opAta {
		cur, ok := env.Get(t.Value)
		if !ok {
			return fmt.Errorf("Satır %d, Sütun %d: '%s' adında bir değişken yok%s",
				line, t.Token.Column, t.Value, suggestName(t.Value, env))
		}
		var err error
		finalVal, err = v.atamaIslemi(kod, cur, rightVal, line)
		if err != nil {
			return err
		}
	}
	if err := env.Assign(t.Value, finalVal); err != nil {
		var tanimsiz *TanimsizDegiskenHatasi
		if errors.As(err, &tanimsiz) {
			return fmt.Errorf("Satır %d, Sütun %d: %v%s", line, t.Token.Column, err, suggestName(t.Value, env))
		}
		return fmt.Errorf("Satır %d: %v", line, err)
	}
	return nil
}
