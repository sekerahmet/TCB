package checker

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Eşitlenebilirlik (13.10'un "eşitlenebilir" kısıtı; 4.5, 7.6–7.8, 15.5): bir tipin değerleri '==' ile
// TR_Bool veren bir karşılaştırmaya girer mi. Tensör, fonksiyon ve arayüz eşitlenemez; onları (iç içe de)
// tutan liste, sözlük, demet, yapı alanı ve durumlu tipin durum alanı da. T?, T eşitlenebiliyorsa
// eşitlenebilir. '==' ve '!=' (equality_check.go), '==' isteyen değer deseni (5.6) ve .TR_contains,
// .TR_index_of, .TR_unique bu tek tanımı kullanır. Tipi bilinmeyen değer (TR_Any, tip parametresi,
// başka modülün tipi) eşitlenebilir sayılır: yanlış alarm verilmez, çalışma anı aynı kuralı izler.

// Eşitlenemeyen içerik: iletinin "what" seçimi.
const (
	holdsTensor    = "tensor"
	holdsFunction  = "function"
	holdsInterface = "interface"
)

// noteTypeDefs programın üst düzeydeki yapı, durumlu tip ve arayüz tanımlarını kaydeder; tanımından önce
// yazılan bir fonksiyonun gövdesi de tipi tanır.
func (c *Checker) noteTypeDefs(prog *ast.Program) {
	for _, stmt := range prog.Statements {
		if s, ok := stmt.(*ast.StructStatement); ok {
			c.noteTypeDef(s)
		}
	}
}

func (c *Checker) noteTypeDef(s *ast.StructStatement) {
	if c.typeDefs == nil {
		c.typeDefs = map[string]*ast.StructStatement{}
	}
	c.typeDefs[s.Name] = s
}

// unequatable t'nin eşitlenemeyen içeriğidir (holdsTensor, holdsFunction, holdsInterface); eşitlenebilir
// tipte "". Kendi tipinde alanı olan yapı (Düğüm { s: Düğüm? }) bir kez gezilir.
func (c *Checker) unequatable(t string) string {
	return c.unequatableIn(t, map[string]bool{})
}

func (c *Checker) unequatableIn(t string, seen map[string]bool) string {
	s := shapeOf(t)
	if s.elems != nil {
		return c.firstUnequatable(s.elems, seen)
	}
	switch base := normalizeTypeName(s.base); base {
	case typeTensor:
		return holdsTensor
	case typeFunction, codes.Name(codes.TypeBuiltinFunction):
		return holdsFunction
	case typeList, typeDict:
		return c.firstUnequatable(s.args, seen)
	default:
		def := c.typeDefs[base]
		if def == nil || seen[base] {
			return ""
		}
		seen[base] = true
		if def.Kind == ast.KindInterface {
			return holdsInterface
		}
		for _, f := range def.Fields {
			if w := c.unequatableIn(normalizedType(f.Type), seen); w != "" {
				return w
			}
		}
		for _, v := range def.Variants {
			for _, f := range v.Fields {
				if w := c.unequatableIn(normalizedType(f.Type), seen); w != "" {
					return w
				}
			}
		}
	}
	return ""
}

func (c *Checker) firstUnequatable(types []string, seen map[string]bool) string {
	for _, t := range types {
		if w := c.unequatableIn(t, seen); w != "" {
			return w
		}
	}
	return ""
}

// elementwise t'nin '=='si tensörün öğe öğe karşılaştırması mı (15.5): isteğe bağlı olmayan tensör.
func elementwise(t string) bool {
	s := shapeOf(t)
	return !s.optional && s.elems == nil && normalizeTypeName(s.base) == typeTensor
}

// noneLiteral ifade TR_none değişmezi mi (3.5'in istisnası yalnız değişmezdedir; değişkenden gelen TR_none
// değil).
func noneLiteral(e ast.Expression) bool {
	_, ok := e.(*ast.NullLiteral)
	return ok
}

// equalityAdvice tensör tutan t'nin (isteğe bağlı değil) iletisinin "how" seçimi ve önerisidir: demette ve
// yapıda, işlenenler adsa, parça parça yazım ("fix"); durumlu tipte durumu açmak ("states"); öbüründe öğe öğe.
func (c *Checker) equalityAdvice(t string, l, r ast.Expression) (how, suggestion string) {
	s := shapeOf(t)
	if lt, lok := c.operandText(l); lok {
		if rt, rok := c.operandText(r); rok {
			if parts, ok := c.equalParts(t, lt, rt, map[string]bool{}); ok {
				return "fix", strings.Join(parts, " "+codes.Spelling(codes.KwAnd)+" ")
			}
		}
	}
	if s.elems == nil && c.typeDefs[normalizeTypeName(s.base)] != nil &&
		c.typeDefs[normalizeTypeName(s.base)].Kind == ast.KindEnum {
		return "states", ""
	}
	return "elements", ""
}

// operandText işlenenin dosyadaki yazımıdır, yalnız ad ve alan zinciri (a, k.t): önerinin a.0'ı onunla
// kurulur, başka ifade (değişmez, çağrı) iki kez hesaplanır ya da derlenmez. İşlenen yoksa (nil) yazım da
// yoktur.
func (c *Checker) operandText(e ast.Expression) (string, bool) {
	switch x := e.(type) {
	case *ast.Identifier:
	case *ast.MemberExpression:
		if x.Safe {
			return "", false
		}
		if _, ok := c.operandText(x.Object); !ok {
			return "", false
		}
	default:
		return "", false
	}
	return c.exprText(e)
}

// equalParts t tipindeki l ile r'nin öğe öğe karşılaştırmasıdır: tensör TR_equal ile, eşitlenebilir öğe
// '==' ile, demet ve yapı öğeleriyle. Başka eşitlenemeyen öğe (tensör listesi, isteğe bağlı tensör) ya da
// tipi yazılmamış alan varsa yazım yoktur.
func (c *Checker) equalParts(t, l, r string, seen map[string]bool) ([]string, bool) {
	if elementwise(t) {
		return []string{codes.Spelling(codes.FnEqual) + "(" + l + ", " + r + ")"}, true
	}
	if c.unequatable(t) == "" {
		return []string{l + " == " + r}, true
	}
	s := shapeOf(t)
	if s.optional {
		return nil, false
	}
	var parts []string
	add := func(et, name string) bool {
		p, ok := c.equalParts(et, l+"."+name, r+"."+name, seen)
		parts = append(parts, p...)
		return ok
	}
	if s.elems != nil {
		for i, et := range s.elems {
			if !add(et, fmt.Sprint(i)) {
				return nil, false
			}
		}
		return parts, true
	}
	def := c.typeDefs[normalizeTypeName(s.base)]
	if def == nil || def.Kind != ast.KindStruct || seen[def.Name] {
		return nil, false
	}
	seen[def.Name] = true
	for _, f := range def.Fields {
		if f.Type == nil || !add(normalizedType(f.Type), f.Name) {
			return nil, false
		}
	}
	return parts, true
}
