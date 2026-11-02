package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// --- TİPLİ YERLER (3.7a, 3.13) ---
//
// Tipi açıkça yazılmış bir yer, bağladığı değeri yazılı tipe göre kurar. Yerler: değişken ve sabit
// bildirimi, parametre (argümanı ve varsayılanı), dönüş (-> T'nin TR_return'ü), yapı alanı ve
// bunlara sonradan atama. Yalnız AST'deki yazılı tip okunur; ilk değerinden çıkan tip
// (TR_var t = 0.0) okunmaz.
//   - TR_Float: tam sayı ondalık olur (3.13: r: TR_Float = 1 → 1.0). Çağrı yerinde çağrılan
//     bilinmediği için parametrede çeviri çağrılanın girişinde yapılır; TR_Int bir değişkenin
//     değeri de çevrilir (sayı aynı kalır; denetim hatası 3.13'ün, denetleyicinin işi).
//   - TR_JSON: liste ve sözlük JSON değeri olarak kurulur (3.7a "Kurmak"), içindeki değişmezlerle.
//     Denetleyici buraya yalnız değişmezi geçirir; tipi bilinmeyen bir kap TR_JSON(x) gibi
//     kopyalanır, JSON'a giremeyen öğesi çalışma hatasıdır.
//
// İsteğe bağlı tip (T?) de aynıdır; TR_none olduğu gibi kalır.

// place tipli bir yerin bağladığı değere yaptığıdır.
type place uint8

const (
	placeAny   place = iota // çeviri yok
	placeFloat              // TR_Float: tam sayı ondalık olur
	placeJSON               // TR_JSON: liste ve sözlük JSON değeri olur
)

// placeOf yazılı t tipinin yeridir; tip yazılmadıysa (nil) placeAny.
func placeOf(t *ast.TypeRef) place {
	if t == nil || t.Elems != nil || t.Return != nil {
		return placeAny
	}
	switch t.Token.Code {
	case codes.TypeFloat:
		return placeFloat
	case codes.TypeJSON:
		return placeJSON
	}
	return placeAny
}

// bind val'ı yerin tipine göre kurar.
func (p place) bind(val Value) (Value, error) {
	switch p {
	case placeFloat:
		if i, ok := val.(*IntVal); ok {
			return &FloatVal{Val: float64(i.Val)}, nil
		}
	case placeJSON:
		switch val.(type) {
		case *ListVal, *DictVal:
			return MakeJSON(val)
		}
	}
	return val, nil
}

// code e'nin değerini yerin tipine göre kuran koddur; çeviri yoksa e'nin kendisi. Kurmanın hatası
// (JSON'a giremeyen öğe) line satırındadır.
func (p place) code(e exprCode, line int) exprCode {
	if p == placeAny {
		return e
	}
	return func(v *VM, env *Environment) (Value, error) {
		val, err := e(v, env)
		if err != nil {
			return nil, err
		}
		val, err = p.bind(val)
		return val, lineSuffix(line, err)
	}
}

// decimalList 3.6'nın kuralıdır: öğelerinin hepsi sayı yazımı olan ve birinde ondalık bulunan liste
// değişmezinde tam sayı yazımları ondalıktır ([1, 2.5] bir TR_List[TR_Float]). Öğelerden biri
// değişkense ([1, x]) öğe tipi çıkarımdır, burada okunmaz.
func decimalList(elems []ast.Expression) bool {
	decimal := false
	for _, e := range elems {
		if p, ok := e.(*ast.PrefixExpression); ok && p.Operator == "-" {
			e = p.Right
		}
		switch e.(type) {
		case *ast.FloatLiteral:
			decimal = true
		case *ast.IntegerLiteral:
		default:
			return false
		}
	}
	return decimal
}

// recordPlace adın bu kapsamdaki yazılı tipinin yerini tutar (sonraki atama da ona göre kurulur).
func (k *compilerScope) recordPlace(name string, p place) {
	if k.info == nil || p == placeAny {
		return
	}
	if k.info.places == nil {
		k.info.places = map[string]place{}
	}
	k.info.places[name] = p
}

// placeOfName adın çözüldüğü tanımın yeridir (resolve gibi yürür); bilinmeyen ad placeAny.
func (k *compilerScope) placeOfName(name string) place {
	for c := k; c != nil; c = c.parent {
		if c.notYet[name] || c.info == nil {
			return placeAny
		}
		if _, ok := c.info.index[name]; ok {
			return c.info.places[name]
		}
		if c.dynamic {
			return placeAny
		}
	}
	return placeAny
}

// fieldPlace yapının name alanının yeridir.
func (s *StructDefVal) fieldPlace(name string) place {
	for i := range s.Fields {
		if s.Fields[i].Name == name {
			return placeOf(s.Fields[i].Type)
		}
	}
	return placeAny
}
