package interp

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Metin, Liste ve Sözlük üzerinde nokta (.) ile çağrılan yerleşik metotlar. Metot gövdeleri
// nesneyi argüman olarak alan sıradan fonksiyonlardır: derlenmiş kod 'x.metot(...)'
// çağrılarını bağlı bir yerleşik nesnesi ayırmadan doğrudan bunlara yönlendirir;
// 'x.metot' bir değer olarak okunursa (ör. değişkene atanırsa) bağlı yerleşik üretilir.

// methodBody bir yerleşik metodun gövdesidir; line, geri çağrılan fonksiyonların
// hata satırıdır.
type methodBody func(y *VM, inst Value, args []Value, line int) (Value, error)

type builtinMethod struct {
	name     string // bağlı yerleşiğin adı (kod listesindeki ad)
	fullName string // hata mesajlarındaki tipli adı (metin.içerir)
	f        methodBody
}

// coreMethod bir üye kodunun Metin, Liste ve Sözlük üzerindeki gövdeleridir (tipte yoksa
// nil). Derleyici x.ad'ı derleme anında koda ve bu yapıya çevirir; çalışma anında yalnızca
// nesnenin tipine göre alan seçilir, ad ya da kod araması yapılmaz.
type coreMethod struct {
	text, listVal, dictVal *builtinMethod
}

// coreMethods üye kodu → Metin/Liste/Sözlük metotları. Adlar kod listesindedir (internal/lang/codes, Y).
var coreMethods = map[codes.Code]*coreMethod{}

const (
	typeString = iota
	typeList
	typeDict
)

// argKind bir metot argümanının beklenen tipidir (hata mesajındaki tip kodu ve denetimi).
type argKind struct {
	typ  codes.Code
	warn func(Value) bool
}

var (
	textArg = &argKind{codes.A202, func(v Value) bool { _, ok := v.(*StringVal); return ok }}
	intArg  = &argKind{codes.A200, func(v Value) bool { _, ok := v.(*IntVal); return ok }}
	funcArg = &argKind{codes.A208, callable}
)

// signature bir metodun argüman sayısı ve tipleridir: en az enAz, en çok enCok (< 0: sınırsız)
// argüman; tipler[i] i. argümanın tipi (nil ya da eksikse her tip). parametreler kullanım
// metnindeki parametre adlarıdır ("alt", "eski, yeni").
type signature struct {
	minN, maxN int
	parameters string
	typesOf    []*argKind
}

// typeCodes metodun alıcı tipinin kodudur; hata mesajlarında metot tipiyle yazılır
// (metin.içerir(); bkz. kod.AliciAdi).
var typeCodes = [...]codes.Code{typeString: codes.A202, typeList: codes.A205, typeDict: codes.A206}

// addMethod metodu sözlük koduyla kaydeder; ad sözlükten gelir. Gövde çağrılmadan önce
// argümanlar imzaya göre denetlenir (yerleşik fonksiyonlarla aynı hata biçimi).
func addMethod(typ int, kd codes.Code, im signature, f methodBody) {
	if kd.Namespace() != codes.NamespaceMember {
		panic(fmt.Sprintf("metot kaydı: %s bir üye kodu (Y) değil", kd))
	}
	cm := coreMethods[kd]
	if cm == nil {
		cm = &coreMethod{}
		coreMethods[kd] = cm
	}
	fullName := codes.ReceiverName(typeCodes[typ]) + "." + codes.Name(kd)
	bodyBlock := f
	f = func(y *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < im.minN || (im.maxN >= 0 && len(args) > im.maxN) {
			expected := strconv.Itoa(im.minN)
			switch {
			case im.maxN < 0:
				expected = AtLeast(im.minN)
			case im.maxN != im.minN:
				expected = Counts(im.minN, im.maxN)
			}
			return nil, argCountErrorByName(fullName, expected, len(args), fullName+"("+im.parameters+")")
		}
		for i, a := range args {
			if i < len(im.typesOf) && im.typesOf[i] != nil && !im.typesOf[i].warn(a) {
				return nil, argTypeErrorByName(fullName, i+1, codes.Name(im.typesOf[i].typ), a)
			}
		}
		return bodyBlock(y, inst, args, line)
	}
	m := &builtinMethod{name: codes.Name(kd), fullName: fullName, f: f}
	switch typ {
	case typeString:
		cm.text = m
	case typeList:
		cm.listVal = m
	case typeDict:
		cm.dictVal = m
	}
}

// Metin ve Liste özellikleri (metot değil, değer döndürürler).
const (
	memberLength = codes.Y001 // uzunluk (Metin, Liste)
	addMember    = codes.Y004 // ekle (Liste)
	memberFirst  = codes.Y210 // ilk (Liste)
	memberLast   = codes.Y211 // son (Liste)
)

// memberInfo bir x.ad erişiminin derleme anında çözülmüş biçimidir.
type memberInfo struct {
	codeVal codes.Code // ad bir üye adı değilse kod.Yok
	// core Metin/Liste/Sözlük metotlarıdır; ad bunların metodu değilse nil.
	core *coreMethod
}

// resolveMember adı derleme anında çözer.
func resolveMember(name string) memberInfo {
	u := memberInfo{codeVal: codes.MemberCode(name)}
	if u.codeVal == codes.None {
		return u
	}
	u.core = coreMethods[u.codeVal]
	return u
}

// coreMember kod Metin, Liste ya da Sözlük üyesi mi (kaldırılmış ad ipuçlarının süzgeci)?
func coreMember(k codes.Code) bool {
	_, meth := coreMethods[k]
	return meth || k == memberLength || k == memberFirst || k == memberLast
}

// isDictMember ad bir sözlük üyesinin (metot ya da .uzunluk) adı mı?
func isDictMember(name string) bool {
	k := codes.MemberCode(name)
	if k == memberLength {
		return true
	}
	cm := coreMethods[k]
	return cm != nil && cm.dictVal != nil
}

// findMethod nesnenin tipine göre derleme anında çözülmüş metodu döndürür.
func findMethod(obj Value, cm *coreMethod) *builtinMethod {
	if cm == nil {
		return nil
	}
	switch obj.(type) {
	case *StringVal:
		return cm.text
	case *ListVal:
		return cm.listVal
	case *DictVal:
		return cm.dictVal
	}
	return nil
}

// Tablolar init içinde kurulur: gövdeler ApplyFunction'ı çağırdığı için değişken
// başlatıcısında kurulsalardı başlatma döngüsü oluşurdu.
func init() {
	setupStringMethods()
	setupListMethods()
	setupDictMethods()
}

// setupStringMethods Metin tipinin metotlarını kaydeder.
func setupStringMethods() {
	addMethod(typeString, codes.Y100, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: turkish.Upper(inst.(*StringVal).Val)}, nil
	})
	addMethod(typeString, codes.Y101, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: turkish.Lower(inst.(*StringVal).Val)}, nil
	})
	addMethod(typeString, codes.Y102, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.TrimSpace(inst.(*StringVal).Val)}, nil
	})
	addMethod(typeString, codes.Y103, signature{0, 1, "[ayırıcı]", []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		sep := " "
		if len(args) > 0 {
			sep = args[0].Inspect()
		}
		parts := strings.Split(inst.(*StringVal).Val, sep)
		elems := make([]Value, len(parts))
		for i, p := range parts {
			elems[i] = &StringVal{Val: p}
		}
		return &ListVal{Elements: elems}, nil
	})
	addMethod(typeString, codes.Y104, signature{2, 2, "eski, yeni", []*argKind{textArg, textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ReplaceAll(inst.(*StringVal).Val, args[0].Inspect(), args[1].Inspect())}, nil
	})
	addMethod(typeString, codes.Y002, signature{1, 1, "alt", []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.Contains(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	addMethod(typeString, codes.Y105, signature{1, 1, "önek", []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.HasPrefix(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	addMethod(typeString, codes.Y106, signature{1, 1, "sonek", []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.HasSuffix(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	addMethod(typeString, codes.Y013, signature{1, 1, "indeks", []*argKind{intArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		runes := []rune(inst.(*StringVal).Val)
		if pos, ok := optionalIndex(args[0], len(runes)); ok {
			return &StringVal{Val: string(runes[pos])}, nil
		}
		return Null, nil
	})
	addMethod(typeString, codes.Y003, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		runes := []rune(inst.(*StringVal).Val)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &StringVal{Val: string(runes)}, nil
	})
}

// setupListMethods Liste tipinin metotlarını kaydeder.
func setupListMethods() {
	addMethod(typeList, codes.Y200, signature{1, 1, codes.ReceiverName(codes.A208), []*argKind{funcArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		var result []Value
		for _, item := range inst.(*ListVal).Elements {
			keep, err := y.ApplyFunction(args[0], []Value{item}, nil, line)
			if err != nil {
				return nil, err
			}
			if keep.IsTruthy() {
				result = append(result, item)
			}
		}
		return &ListVal{Elements: result}, nil
	})
	addMethod(typeList, codes.Y201, signature{1, 1, codes.ReceiverName(codes.A208), []*argKind{funcArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		result := make([]Value, 0, len(elems))
		for _, item := range elems {
			mapped, err := y.ApplyFunction(args[0], []Value{item}, nil, line)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		}
		return &ListVal{Elements: result}, nil
	})
	addMethod(typeList, codes.Y202, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		copy(copied, elems)
		if err := sortValues(CallPrefix(codes.Y202), copied); err != nil {
			return nil, err
		}
		return &ListVal{Elements: copied}, nil
	})
	addMethod(typeList, codes.Y013, signature{1, 1, "indeks", []*argKind{intArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		if pos, ok := optionalIndex(args[0], len(elems)); ok {
			return elems[pos], nil
		}
		return Null, nil
	})
	addMethod(typeList, codes.Y003, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		for i, el := range elems {
			copied[len(elems)-1-i] = el
		}
		return &ListVal{Elements: copied}, nil
	})
	addMethod(typeList, codes.Y203, signature{0, 1, "[ayırıcı]", []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		sep := ""
		if len(args) > 0 {
			sep = args[0].Inspect()
		}
		elems := inst.(*ListVal).Elements
		parts := make([]string, len(elems))
		for i, el := range elems {
			parts[i] = el.Inspect()
		}
		return &StringVal{Val: strings.Join(parts, sep)}, nil
	})
	addMethod(typeList, codes.Y002, signature{1, 1, "değer", nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		for _, el := range inst.(*ListVal).Elements {
			if ValuesEqual(el, args[0]) {
				return True, nil
			}
		}
		return False, nil
	})
	addMethod(typeList, codes.Y204, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		var unique []Value
		for _, el := range inst.(*ListVal).Elements {
			exists := false
			for _, u := range unique {
				if ValuesEqual(el, u) {
					exists = true
					break
				}
			}
			if !exists {
				unique = append(unique, el)
			}
		}
		return &ListVal{Elements: unique}, nil
	})
	addMethod(typeList, codes.Y004, signature{1, -1, "eleman, ...", nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		l := inst.(*ListVal)
		l.Elements = append(l.Elements, args...)
		return l, nil
	})
	addMethod(typeList, codes.Y207, signature{1, 2, "[başlangıç], " + codes.ReceiverName(codes.A208), nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return listReduce(y, inst.(*ListVal).Elements, args, line)
	})
}

// setupDictMethods Sözlük tipinin metotlarını kaydeder.
func setupDictMethods() {
	addMethod(typeDict, codes.Y300, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.KeyValue(k)
		}
		return &ListVal{Elements: elems}, nil
	})
	addMethod(typeDict, codes.Y301, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.Pairs[k]
		}
		return &ListVal{Elements: elems}, nil
	})
	addMethod(typeDict, codes.Y002, signature{1, 1, "anahtar", nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		if err := DictKeyError(args[0]); err != nil {
			return nil, messages.At(line, 0, err)
		}
		_, ok := inst.(*DictVal).GetValue(args[0])
		return BoolToVal(ok), nil
	})
}

// evalBuiltinMember: Metin, Liste ve Sözlük üzerinde nokta (.) ile özellik okur ya da metodu
// bağlı bir yerleşik olarak döndürür.
func (v *VM) evalBuiltinMember(obj Value, prop string, u memberInfo, safe bool, line int) (Value, error) {
	switch inst := obj.(type) {
	case *StringVal:
		if u.codeVal == memberLength {
			return NewInt(int64(utf8.RuneCountInString(inst.Val))), nil
		}
	case *DictVal:
		if u.codeVal == memberLength {
			return NewInt(int64(len(inst.Pairs))), nil
		}
	case *ListVal:
		switch u.codeVal {
		case memberLength:
			return NewInt(int64(len(inst.Elements))), nil
		case memberFirst:
			if len(inst.Elements) == 0 {
				return Null, nil
			}
			return inst.Elements[0], nil
		case memberLast:
			if len(inst.Elements) == 0 {
				return Null, nil
			}
			return inst.Elements[len(inst.Elements)-1], nil
		}
	}
	if m := findMethod(obj, u.core); m != nil {
		return &BuiltinVal{Name: m.name, Fn: func(y *VM, args []Value, named map[string]Value) (Value, error) {
			if len(named) > 0 { // çekirdek metotlar adlı argüman almaz
				return nil, NoNamedArgsError(m.fullName, named)
			}
			return m.f(y, obj, args, line)
		}}, nil
	}
	if d, ok := obj.(*DictVal); ok {
		if val, ok := d.Pairs[prop]; ok {
			return val, nil
		}
		return Null, nil
	}

	if safe {
		return Null, nil
	}
	if h := movedMemberError(obj, prop); h != "" {
		return nil, messages.At(line, 0, messages.New(messages.H2441, h))
	}
	return nil, messages.At(line, 0, messages.New(messages.H2474, obj.Type(), prop, removedMemberHint(prop)))
}

// listReduce liste.indirge([başlangıç], fonksiyon) metodunun gövdesidir. Fonksiyon son
// argümandır (sondaki blokla da yazılabilsin diye: liste.indirge(0) { birikim + o }) ve
// (birikim, eleman) ile çağrılır; kullanıcı fonksiyonu iki parametreli olmalıdır.
// Başlangıç verilmezse ilk eleman başlangıçtır; boş listede başlangıç (verilmemişse yok)
// döner.
func listReduce(y *VM, elems []Value, args []Value, line int) (Value, error) {
	name, fun := CallName(codes.Y207), codes.ReceiverName(codes.A208)
	f := args[len(args)-1]
	if !callable(f) {
		if len(args) == 2 && callable(args[0]) {
			return nil, messages.New(messages.H2475, name, fun)
		}
		return nil, ArgTypeError(codes.Y207, len(args), f, codes.A208)
	}
	if fn, ok := f.(*FuncVal); ok && len(fn.Parameters) != 2 {
		b := codes.Name(codes.A105)
		return nil, messages.New(messages.H2476, name, fun, b, len(fn.Parameters), codes.Name(codes.A104))
	}
	var acc Value
	begin := 0
	switch {
	case len(args) == 2:
		acc = args[0]
	case len(elems) == 0:
		return Null, nil
	default:
		acc, begin = elems[0], 1
	}
	for _, el := range elems[begin:] {
		var err error
		acc, err = y.ApplyFunction(f, []Value{acc, el}, nil, line)
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

// NumberMean returns the arithmetic mean of numbers as Ondalık; empty input and non-number
// elements are errors (used by the ortalama(liste) builtin).
func NumberMean(name string, elems []Value) (Value, error) {
	if len(elems) == 0 {
		return nil, messages.New(messages.H2477, name)
	}
	s := 0.0
	for i, el := range elems {
		f, ok := ToFloat(el)
		if !ok {
			return nil, ListElementError(name, i, numberType, el)
		}
		s += f
	}
	return &FloatVal{Val: s / float64(len(elems))}, nil
}

// optionalIndex varsa_al'ın indeksini çözer (negatif indeks sondan sayar); öğe yoksa ok=false.
func optionalIndex(arg Value, length int) (pos int, ok bool) {
	iv, isInt := arg.(*IntVal)
	if !isInt {
		return 0, false
	}
	pos = int(iv.Val)
	if pos < 0 {
		pos += length
	}
	return pos, pos >= 0 && pos < length
}
