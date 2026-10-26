package interp

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Metin, Liste ve Sözlük üzerinde nokta (.) ile çağrılan yerleşik metotlar. Metot gövdeleri
// nesneyi argüman olarak alan sıradan fonksiyonlardır: derlenmiş kod 'x.metot(...)'
// çağrılarını bağlı bir yerleşik nesnesi ayırmadan doğrudan bunlara yönlendirir;
// 'x.metot' bir değer olarak okunursa (ör. değişkene atanırsa) bağlı yerleşik üretilir.

// methodBody bir yerleşik metodun gövdesidir; line, geri çağrılan fonksiyonların
// hata satırıdır.
type methodBody func(y *VM, inst Value, args []Value, line int) (Value, error)

type builtinMethod struct {
	name     string    // bağlı yerleşiğin adı (kod listesindeki ad)
	fullName codes.Ref // hata mesajlarındaki tipli adı (metin.içerir; iletinin dilinde)
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
	textArg = &argKind{codes.TypeString, func(v Value) bool { _, ok := v.(*StringVal); return ok }}
	intArg  = &argKind{codes.TypeInt, func(v Value) bool { _, ok := v.(*IntVal); return ok }}
	funcArg = &argKind{codes.TypeFunction, callable}
)

// signature bir metodun argüman sayısı ve tipleridir: en az enAz, en çok enCok (< 0: sınırsız)
// argüman; tipler[i] i. argümanın tipi (nil ya da eksikse her tip). parametreler kullanım
// metnindeki parametre adlarıdır ("alt", "eski, yeni").
type signature struct {
	minN, maxN int
	parameters any // kullanım biçimi: metin ya da ileti parçası (usage)
	typesOf    []*argKind
}

// typeCodes metodun alıcı tipinin kodudur; hata mesajlarında metot tipiyle yazılır
// (metin.içerir(); bkz. kod.AliciAdi).
var typeCodes = [...]codes.Code{typeString: codes.TypeString, typeList: codes.TypeList, typeDict: codes.TypeDict}

// addMethod metodu sözlük koduyla kaydeder; ad sözlükten gelir. Gövde çağrılmadan önce
// argümanlar imzaya göre denetlenir (yerleşik fonksiyonlarla aynı hata biçimi).
func addMethod(typ int, kd codes.Code, im signature, f methodBody) {
	if kd.Namespace() != codes.NamespaceMember {
		panic(fmt.Sprintf("method registry: %s is not a member code (Y)", kd))
	}
	cm := coreMethods[kd]
	if cm == nil {
		cm = &coreMethod{}
		coreMethods[kd] = cm
	}
	fullName := codes.MemberRef(typeCodes[typ], kd)
	bodyBlock := f
	f = func(y *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < im.minN || (im.maxN >= 0 && len(args) > im.maxN) {
			var expected any = im.minN
			switch {
			case im.maxN < 0:
				expected = AtLeast(im.minN)
			case im.maxN != im.minN:
				expected = Counts(im.minN, im.maxN)
			}
			return nil, messages.New(messages.H2011, fullName, expected, len(args), usageByName(fullName, []any{im.parameters}))
		}
		for i, a := range args {
			if i < len(im.typesOf) && im.typesOf[i] != nil && !im.typesOf[i].warn(a) {
				return nil, argTypeErrorByName(fullName, i+1, TypeNames(im.typesOf[i].typ), a)
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
	memberLength = codes.MemLength // uzunluk (Metin, Liste)
	addMember    = codes.MemAppend // ekle (Liste)
	memberFirst  = codes.MemFirst  // ilk (Liste)
	memberLast   = codes.MemLast   // son (Liste)
)

// memberInfo bir x.ad erişiminin derleme anında çözülmüş biçimidir.
type memberInfo struct {
	codeVal codes.Code // ad bir üye adı değilse kod.Yok
	// core Metin/Liste/Sözlük metotlarıdır; ad bunların metodu değilse nil.
	core *coreMethod
	// keys, üye bir kod taşıyorsa (codeVal) sözlükte aranacak anahtarlardır: kodun her yazım
	// dilindeki adı, önce erişimin yazıldığı dosyanın dilindeki (3.7, 17.3). nil ise anahtar
	// erişimin adının kendisidir.
	keys []string
}

// member adı derleme anında derlenen dosyanın diliyle çözer (resolveMember ve sözlük anahtarları).
func (c *compiler) member(name string) memberInfo {
	u := resolveMember(name)
	if u.codeVal != codes.None {
		u.keys = memberKeys(u.codeVal, c.textLang)
	}
	return u
}

// memberKeys bir üye kodunun gömülü yazım dillerindeki adlarıdır: önce lang'deki (nil: kanonik),
// sonra kanonik ad, sonra öbür diller; her ad bir kez.
func memberKeys(k codes.Code, lang *codes.Language) []string {
	var keys []string
	add := func(l *codes.Language) {
		if e, ok := l.Entry(k); ok && !slices.Contains(keys, e.Name) {
			keys = append(keys, e.Name)
		}
	}
	if lang != nil {
		add(lang)
	}
	add(codes.Source())
	for _, code := range codes.Languages() {
		if l, err := codes.LoadLanguage(code); err == nil {
			add(l)
		}
	}
	return keys
}

// dictKey sözlükte u erişiminin anahtarıdır: üye bir kod taşıyorsa kodun bir yazım dilindeki
// adıyla duran anahtar (önce dosyanın dilindeki); yoksa ok false ve eklenecek ad (dosyanın
// dilindeki ad). Böylece aynı sözlükte bir kodun iki dilde iki anahtarı oluşmaz.
func (u memberInfo) dictKey(d *DictVal, prop string) (key string, ok bool) {
	if u.keys == nil {
		_, ok = d.Pairs[prop]
		return prop, ok
	}
	for _, key := range u.keys {
		if _, ok := d.Pairs[key]; ok {
			return key, true
		}
	}
	return u.keys[0], false
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
	addMethod(typeString, codes.MemUpper, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToUpper(inst.(*StringVal).Val)}, nil // Unicode kuralı, dilden bağımsız (13.2)
	})
	addMethod(typeString, codes.MemLower, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToLower(inst.(*StringVal).Val)}, nil
	})
	addMethod(typeString, codes.MemStrip, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.TrimSpace(inst.(*StringVal).Val)}, nil
	})
	addMethod(typeString, codes.MemSplit, signature{0, 1, messages.Text(messages.H2331), []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		sep := " "
		if len(args) > 0 {
			sep = y.textOf(args[0])
		}
		parts := strings.Split(inst.(*StringVal).Val, sep)
		elems := make([]Value, len(parts))
		for i, p := range parts {
			elems[i] = &StringVal{Val: p}
		}
		return &ListVal{Elements: elems}, nil
	})
	addMethod(typeString, codes.MemReplace, signature{2, 2, "eski, yeni", []*argKind{textArg, textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ReplaceAll(inst.(*StringVal).Val, y.textOf(args[0]), y.textOf(args[1]))}, nil
	})
	addMethod(typeString, codes.MemContains, signature{1, 1, "alt", []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.Contains(inst.(*StringVal).Val, y.textOf(args[0]))), nil
	})
	addMethod(typeString, codes.MemStartsWith, signature{1, 1, messages.Text(messages.H2332), []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.HasPrefix(inst.(*StringVal).Val, y.textOf(args[0]))), nil
	})
	addMethod(typeString, codes.MemEndsWith, signature{1, 1, "sonek", []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.HasSuffix(inst.(*StringVal).Val, y.textOf(args[0]))), nil
	})
	addMethod(typeString, codes.MemGet, signature{1, 1, "indeks", []*argKind{intArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		runes := []rune(inst.(*StringVal).Val)
		if pos, ok := optionalIndex(args[0], len(runes)); ok {
			return &StringVal{Val: string(runes[pos])}, nil
		}
		return Null, nil
	})
	addMethod(typeString, codes.MemReverse, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		runes := []rune(inst.(*StringVal).Val)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &StringVal{Val: string(runes)}, nil
	})
}

// setupListMethods Liste tipinin metotlarını kaydeder.
func setupListMethods() {
	addMethod(typeList, codes.MemFilter, signature{1, 1, codes.ReceiverRef(codes.TypeFunction), []*argKind{funcArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
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
	addMethod(typeList, codes.MemMap, signature{1, 1, codes.ReceiverRef(codes.TypeFunction), []*argKind{funcArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
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
	addMethod(typeList, codes.MemSort, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		copy(copied, elems)
		if err := sortValues(CallPrefix(codes.MemSort), copied); err != nil {
			return nil, err
		}
		return &ListVal{Elements: copied}, nil
	})
	addMethod(typeList, codes.MemGet, signature{1, 1, "indeks", []*argKind{intArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		if pos, ok := optionalIndex(args[0], len(elems)); ok {
			return elems[pos], nil
		}
		return Null, nil
	})
	addMethod(typeList, codes.MemReverse, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		for i, el := range elems {
			copied[len(elems)-1-i] = el
		}
		return &ListVal{Elements: copied}, nil
	})
	addMethod(typeList, codes.MemJoin, signature{0, 1, messages.Text(messages.H2331), []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		sep := ""
		if len(args) > 0 {
			sep = y.textOf(args[0])
		}
		elems := inst.(*ListVal).Elements
		parts := make([]string, len(elems))
		lang := y.textLanguage()
		for i, el := range elems {
			parts[i] = valueText(el, lang)
		}
		return &StringVal{Val: strings.Join(parts, sep)}, nil
	})
	addMethod(typeList, codes.MemContains, signature{1, 1, messages.Text(messages.H2301), nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		for _, el := range inst.(*ListVal).Elements {
			if ValuesEqual(el, args[0]) {
				return True, nil
			}
		}
		return False, nil
	})
	addMethod(typeList, codes.MemUnique, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
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
	addMethod(typeList, codes.MemAppend, signature{1, -1, "eleman, ...", nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		l := inst.(*ListVal)
		l.Elements = append(l.Elements, args...)
		return l, nil
	})
	addMethod(typeList, codes.MemReduce, signature{1, 2, messages.Text(messages.H2333, codes.ReceiverRef(codes.TypeFunction)), nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return listReduce(y, inst.(*ListVal).Elements, args, line)
	})
}

// setupDictMethods Sözlük tipinin metotlarını kaydeder.
func setupDictMethods() {
	addMethod(typeDict, codes.MemKeys, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.KeyValue(k)
		}
		return &ListVal{Elements: elems}, nil
	})
	addMethod(typeDict, codes.MemValues, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.Pairs[k]
		}
		return &ListVal{Elements: elems}, nil
	})
	addMethod(typeDict, codes.MemContains, signature{1, 1, "anahtar", nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
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
		if key, ok := u.dictKey(d, prop); ok {
			return d.Pairs[key], nil
		}
		if err := d.privateNameError(prop); err != nil {
			return nil, messages.At(line, 0, err)
		}
		return Null, nil
	}

	if safe {
		return Null, nil
	}
	if prop == codes.Name(codes.MemSlice) {
		// Liste ve Metin'de dilim köşeli parantezle alınır (dilim Tensör'ün metodudur).
		return nil, messages.At(line, 0, messages.New(messages.H2526, obj.Type(), prop))
	}
	return nil, messages.At(line, 0, messages.New(messages.H2474, obj.Type(), prop, removedMemberHint(prop)))
}

// listReduce liste.indirge([başlangıç], fonksiyon) metodunun gövdesidir. Fonksiyon son
// argümandır (sondaki blokla da yazılabilsin diye: liste.indirge(0) { birikim + o }) ve
// (birikim, eleman) ile çağrılır; kullanıcı fonksiyonu iki parametreli olmalıdır.
// Başlangıç verilmezse ilk eleman başlangıçtır; boş listede başlangıç (verilmemişse yok)
// döner.
func listReduce(y *VM, elems []Value, args []Value, line int) (Value, error) {
	name, fun := CallName(codes.MemReduce), codes.ReceiverRef(codes.TypeFunction)
	f := args[len(args)-1]
	if !callable(f) {
		if len(args) == 2 && callable(args[0]) {
			return nil, messages.New(messages.H2475, name, fun)
		}
		return nil, ArgTypeError(codes.MemReduce, len(args), f, codes.TypeFunction)
	}
	if fn, ok := f.(*FuncVal); ok && len(fn.Parameters) != 2 {
		b := codes.NameRef(codes.CtxAcc)
		return nil, messages.New(messages.H2476, name, fun, b, len(fn.Parameters), codes.NameRef(codes.CtxIt))
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
func NumberMean(name any, elems []Value) (Value, error) {
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

// resolveIndex indeksi çözer: negatif indeks sondan sayar; aralık dışıysa ok=false.
func resolveIndex(idx int64, length int) (pos int, ok bool) {
	pos = int(idx)
	if pos < 0 {
		pos += length
	}
	return pos, pos >= 0 && pos < length
}

// optionalIndex varsa_al'ın indeksini çözer (negatif indeks sondan sayar); öğe yoksa ok=false.
func optionalIndex(arg Value, length int) (pos int, ok bool) {
	iv, isInt := arg.(*IntVal)
	if !isInt {
		return 0, false
	}
	return resolveIndex(iv.Val, length)
}
