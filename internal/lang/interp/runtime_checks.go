package interp

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- TİPİ BİLİNMEYEN DEĞERİN TİPLİ YERDEKİ DENETİMİ (1.2) ---
//
// Denetleyici, tipini çıkaramadığı (ya da öğe tipini bilmediği kap olan) bir değer tipli bir yere
// girerken onu işaretler (ast.RuntimeCheck, checker/runtime_checks.go). Yorumlayıcı çalıştırdığı ve
// aktardığı her programın işaretlerini birleştirir; işaretli ifade derlenirken denetimle sarılır.
// Değer girişte, iç içe kaplarıyla, denetleyicinin kuralıyla denetlenir (checker.RuntimeFits); boş
// kap uyar. Uymazsa yerin denetimdeki koduyla çalışma hatasıdır; ileti değerin çalışırken bilinen
// tipini ve kapta uymayan ilk öğeyi (H1170) yazar.

// runtimeCheckSet VM'in çalıştırdığı ve aktardığı programların işaretleridir, işaretli düğüme göre.
// Derleme görevlerde de olur; okuma kilitlidir. any işaret yokken kilide gitmez.
type runtimeCheckSet struct {
	mu    sync.RWMutex
	any   atomic.Bool
	marks ast.RuntimeChecks
}

// addRuntimeChecks programın işaretlerini VM'in işaretlerine katar.
func (v *VM) addRuntimeChecks(prog *ast.Program) {
	if prog == nil || len(prog.RuntimeChecks) == 0 {
		return
	}
	s := &v.runtimeChecks
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.marks == nil {
		s.marks = ast.RuntimeChecks{}
	}
	for n, m := range prog.RuntimeChecks {
		s.marks[n] = m
	}
	s.any.Store(true)
}

// of n'nin işaretidir (yoksa nil).
func (s *runtimeCheckSet) of(n ast.Node) *ast.RuntimeCheck {
	if s == nil || !s.any.Load() {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.marks[n]
}

// runtimeChecked işaretli expr'in kodudur: değeri girişte m'ye göre denetler. İfadenin kendisi
// c.unchecked ile işaretsiz derlenir.
func (c *compiler) runtimeChecked(k *compilerScope, expr ast.Expression, m *ast.RuntimeCheck) exprCode {
	prev := c.unchecked
	c.unchecked = expr
	code := c.expression(k, expr)
	c.unchecked = prev
	return func(v *VM, env *Environment) (Value, error) {
		val, err := code(v, env)
		if err != nil {
			return nil, err
		}
		if err := checkEntry(m, val); err != nil {
			return nil, err
		}
		return val, nil
	}
}

// compoundChecked bileşik atamanın (x op= v) sağ yanının kodudur: atamanın işareti varsa giren
// değer sonuçtur; sonucun tipi hedefin tipinden ve sağ yanın çalışırken bilinen tipinden
// denetleyicinin kuralıyla çıkar (checker.RuntimeResultType). Listeye '+=' sağ yanı listenin
// tipine göre denetler (öğeler listeye girer).
func (c *compiler) compoundChecked(s *ast.AssignStatement, value exprCode) exprCode {
	m := c.checks.of(s)
	if m == nil {
		return value
	}
	return func(v *VM, env *Environment) (Value, error) {
		val, err := value(v, env)
		if err != nil {
			return nil, err
		}
		if _, list := val.(*ListVal); list && m.Op == "+" {
			if base, _, _, _ := checker.RuntimeShape(m.Expected); base == codes.Name(codes.TypeList) {
				return val, checkEntry(m, val)
			}
		}
		result := checker.RuntimeResultType(m.Op, m.Expected, runtimeType(val))
		if base, _, _, _ := checker.RuntimeShape(result); base == codes.Name(codes.TypeAny) ||
			checker.RuntimeFits(m.Expected, result, false, m.Structs) {
			return val, nil
		}
		return nil, entryError(m, result, nil)
	}
}

// checkEntry val'ın m'nin yerine girip giremeyeceğini denetler; giremiyorsa yerin koduyla hatadır.
func checkEntry(m *ast.RuntimeCheck, val Value) error {
	w := entryWalk{structs: m.Structs}
	if w.fits(m.Expected, val, false) {
		return nil
	}
	return entryError(m, runtimeType(val), w.bad)
}

// entryWalk değeri beklenen tipe göre iç içe gezer; bad uymayan ilk iç öğedir.
type entryWalk struct {
	structs map[string]bool
	bad     *badElement
}

// badElement kapta uymayan ilk öğedir: yolu (dıştan içe), tipi; key ise öğe bir sözlük
// anahtarıdır (yol anahtarın sözlüğüdür). spellable false ise yol programın yazımıyla
// yazılamaz (yazımı olmayan anahtar): ipucu yazılmaz.
type badElement struct {
	steps     []string
	key       Value
	typ       string
	spellable bool
}

// entryShape bir tip metninin çözülmüş parçalarıdır; leaf, metin isteğe bağlı olmayan, argümansız
// bir çekirdek tipse onun adıdır: çalışırken o tipteki değer uyar (kuralın tam aynılık hâli).
type entryShape struct {
	base        string
	args, elems []string
	leaf        ValueType
}

// entryShapes çözülmüş tip metinleridir; programın tip metinleriyle sınırlıdır, süreç boyunca
// paylaşılır: büyük kabın her öğesinde tip metni yeniden çözülmez.
var entryShapes sync.Map // tip metni → *entryShape

func shapeOfEntry(t string) *entryShape {
	if s, ok := entryShapes.Load(t); ok {
		return s.(*entryShape)
	}
	base, args, elems, optional := checker.RuntimeShape(t)
	s := &entryShape{base: base, args: args, elems: elems}
	if !optional && args == nil && elems == nil && checker.RuntimeCoreType(base) {
		s.leaf = ValueType(base)
	}
	entryShapes.Store(t, s)
	return s
}

// fits val expected'e uyuyor mu; inner bir kabın içidir.
func (w *entryWalk) fits(expected string, val Value, inner bool) bool {
	s := shapeOfEntry(expected)
	switch s.base {
	case codes.Name(codes.TypeAny), codes.Name(codes.TypeJSON):
		return true // TR_JSON yerin kendi kurmasındadır (3.7a); JSON'un null'u da vardır
	}
	if val == nil || val == Null {
		return checker.RuntimeFits(expected, codes.Name(codes.TypeNoneType), inner, w.structs)
	}
	if s.leaf != "" && val.Type() == s.leaf {
		return true
	}
	switch x := val.(type) {
	case *ListVal:
		if s.base == codes.Name(codes.TypeList) && len(s.args) == 1 {
			for i, e := range x.Elements {
				if !w.fits(s.args[0], e, true) {
					w.at(e, "["+strconv.Itoa(i)+"]", true)
					return false
				}
			}
			return true
		}
	case *DictVal:
		if s.base == codes.Name(codes.TypeDict) && len(s.args) == 2 {
			return w.dictFits(s.args[0], s.args[1], x)
		}
	case *TupleVal:
		if s.elems != nil && len(s.elems) == len(x.Elements) {
			for i, e := range x.Elements {
				if !w.fits(s.elems[i], e, true) {
					w.at(e, "."+strconv.Itoa(i), true)
					return false
				}
			}
			return true
		}
	case *FuncVal, *BuiltinVal:
		if checker.RuntimeIsFunctionType(expected) {
			return true // yalnız fonksiyon olduğuna bakılır (6.9)
		}
	}
	return checker.RuntimeFits(expected, runtimeType(val), inner, w.structs)
}

// dictFits sözlüğün anahtarlarını ve değerlerini sırayla denetler.
func (w *entryWalk) dictFits(keyType, valType string, d *DictVal) bool {
	stringKeys := shapeOfEntry(keyType).leaf == ValString
	for _, k := range d.Order {
		if _, other := d.KeyVals[k]; other || !stringKeys {
			if key := d.KeyValue(k); !w.fits(keyType, key, true) {
				if w.bad == nil {
					w.bad = &badElement{key: key, typ: runtimeType(key), spellable: true}
				}
				return false
			}
		}
		if v := d.Pairs[k]; !w.fits(valType, v, true) {
			step, spellable := keyStep(d.KeyValue(k))
			w.at(v, step, spellable)
			return false
		}
	}
	return true
}

// at uymayan bir öğenin adımını yola (başa) ekler; en içteki öğe bad'in kendisidir.
func (w *entryWalk) at(val Value, step string, spellable bool) {
	if w.bad == nil {
		w.bad = &badElement{typ: runtimeType(val), spellable: true}
	}
	w.bad.steps = append([]string{step}, w.bad.steps...)
	w.bad.spellable = w.bad.spellable && spellable
}

// keyStep sözlük anahtarının yoldaki adımıdır: metin anahtar TR'nin metin yazımıyla (2.6; ters
// tırnak yolun ters tırnağını kapatmasın diye \u{60}), tam sayı yazımıyla. Öbür anahtarların
// programda tek yazımı yoktur: spellable false.
func keyStep(key Value) (string, bool) {
	switch k := key.(type) {
	case *StringVal:
		return "[" + strings.ReplaceAll(QuoteText(k.Val), "`", `\u{60}`) + "]", true
	case *IntVal:
		return "[" + strconv.FormatInt(k.Val, 10) + "]", true
	}
	return "", false
}

// runtimeType değerin çalışırken bilinen tipidir, denetleyicinin tip metniyle: öğeleri tek tipte
// olan liste TR_List[T], anahtarları ve değerleri tek tipte olan sözlük TR_Dict[K, V]; boş ya da
// karışık kap asıl adıyla.
func runtimeType(val Value) string {
	switch x := val.(type) {
	case nil:
		return codes.Name(codes.TypeNoneType)
	case *ListVal:
		if t, ok := sameType(x.Elements); ok {
			return codes.Name(codes.TypeList) + "[" + t + "]"
		}
	case *DictVal:
		keys, vals := make([]Value, 0, len(x.Order)), make([]Value, 0, len(x.Order))
		for _, k := range x.Order {
			keys, vals = append(keys, x.KeyValue(k)), append(vals, x.Pairs[k])
		}
		kt, ok1 := sameType(keys)
		vt, ok2 := sameType(vals)
		if ok1 && ok2 {
			return codes.Name(codes.TypeDict) + "[" + kt + ", " + vt + "]"
		}
	}
	return string(val.Type())
}

// sameType öğelerin hepsinin çalışırken bilinen tipi aynıysa odur; boşsa yoktur.
func sameType(elems []Value) (string, bool) {
	if len(elems) == 0 {
		return "", false
	}
	t := runtimeType(elems[0])
	for _, e := range elems[1:] {
		if runtimeType(e) != t {
			return "", false
		}
	}
	return t, true
}

// entryError yerin koduyla çalışma hatasıdır: given değerin tipi, bad kapta uymayan ilk öğe (H1170).
func entryError(m *ast.RuntimeCheck, given string, bad *badElement) error {
	args := append([]any(nil), m.Args...)
	if m.Given >= 0 {
		args[m.Given] = checker.RuntimeTypeArg(given, m.Spelled)
	}
	if m.Hint >= 0 {
		args[m.Hint] = entryHint(m, bad)
	}
	err := messages.New(m.Code, args...)
	if r := ast.RangeOf(m.At); r.Line > 0 {
		return messages.AtRange(r, err)
	}
	return messages.At(m.At.Line(), 0, err)
}

// entryHint H1170'tir (anahtarda H1171): kapta uymayan ilk öğenin yolu programın yazımıyla ve tipi. Değerin yazımı
// bilinmiyorsa ya da yol yazılamıyorsa ipucu yoktur.
func entryHint(m *ast.RuntimeCheck, bad *badElement) any {
	if bad == nil || m.Text == "" || !bad.spellable || bad.key == nil && len(bad.steps) == 0 {
		return ""
	}
	path := m.Text + strings.Join(bad.steps, "")
	typ := checker.RuntimeTypeArg(bad.typ, m.Spelled)
	if bad.key != nil {
		return messages.New(messages.H1171, path, keyText(bad.key), typ)
	}
	return messages.New(messages.H1170, path, typ)
}

// keyText anahtarın iletideki metnidir ({key|q} onu tırnaklar).
func keyText(key Value) string {
	if s, ok := key.(*StringVal); ok {
		return s.Val
	}
	return key.Inspect()
}
