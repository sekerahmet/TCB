package interp

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ValueType is the type name of a value (tip() result, error messages). The names are in the
// dictionary (internal/lang/codes, type) and are read once at startup.
type ValueType string

var (
	ValInt      = TypeName(codes.TypeInt)             // Tam
	ValFloat    = TypeName(codes.TypeFloat)           // Ondalık
	ValString   = TypeName(codes.TypeString)          // Metin
	ValBool     = TypeName(codes.TypeBool)            // Mantık
	ValNull     = TypeName(codes.TypeNone)            // Yok
	ValList     = TypeName(codes.TypeList)            // Liste
	ValDict     = TypeName(codes.TypeDict)            // Sözlük
	ValRange    = TypeName(codes.TypeRange)           // Aralık
	ValFunc     = TypeName(codes.TypeFunction)        // Fonksiyon
	ValBuiltin  = TypeName(codes.TypeBuiltinFunction) // YerleşikFonksiyon
	ValStruct   = TypeName(codes.TypeStructType)      // YapıTanımı
	ValInstance = TypeName(codes.TypeObject)          // Nesne
	ValTensor   = TypeName(codes.TypeTensor)          // Tensör
)

// MessageText is the type's name in a message written in lang (messages.Localized): the type
// names in messages follow the message's language (TR_DIL). A name that is not a type code's
// (a struct's own name) stays as it is.
func (t ValueType) MessageText(lang string) string {
	k, ok := codes.Find(codes.NamespaceType, string(t))
	if !ok {
		return string(t)
	}
	l, err := codes.LoadLanguage(lang)
	if err != nil {
		return string(t)
	}
	return l.Name(k)
}

// Shown is v as a message parameter: its text follows the message's language (TR_DIL), like
// the display (DisplayText); Inspect, the canonical text, is for data, not for messages.
func Shown(v Value) messages.Localized { return shown{v} }

type shown struct{ v Value }

func (s shown) MessageText(lang string) string {
	l, err := codes.LoadLanguage(lang)
	if err != nil {
		return s.v.Inspect()
	}
	return valueText(s.v, l)
}

// TypeName is the ValueType of a type-name code from the dictionary (type); engines get the
// names of their own types with it.
func TypeName(k codes.Code) ValueType {
	if k.Namespace() != codes.NamespaceType {
		panic(fmt.Sprintf("TypeName: %s is not a type name code (type)", k))
	}
	return ValueType(codes.Name(k))
}

type Value interface {
	Type() ValueType
	Inspect() string
	IsTruthy() bool
}

// --- TEMEL TİPLER ---

type IntVal struct {
	Val int64
}

func (i *IntVal) Type() ValueType { return ValInt }
func (i *IntVal) Inspect() string { return strconv.FormatInt(i.Val, 10) }
func (i *IntVal) IsTruthy() bool  { return i.Val != 0 }

type FloatVal struct {
	Val float64
}

func (f *FloatVal) Type() ValueType { return ValFloat }
func (f *FloatVal) Inspect() string { return f.Text(codes.Source()) }

// Text sayının lang dilindeki metnidir. Sonsuz ve tanımsız değerin adı dilin değer adıdır (TR_val_inf,
// TR_val_nan; eksi sonsuz başına '-' alır); tensör yazdırması aynı adları kullanır (NonFiniteText).
func (f *FloatVal) Text(lang *codes.Language) string {
	if s, ok := NonFiniteText(f.Val, lang); ok {
		return s
	}
	s := strconv.FormatFloat(f.Val, 'g', -1, 64)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}
func (f *FloatVal) IsTruthy() bool { return f.Val != 0.0 }

// NonFiniteText is the text of a non-finite number in lang (3.2): the value names TR_val_inf (sonsuz,
// inf) and TR_val_nan (NaN, nan), minus infinity with a leading '-'; ok is false for a finite number.
// Numbers and tensors write them the same way; data formats (JSON, logs) do not use it.
func NonFiniteText(f float64, lang *codes.Language) (string, bool) {
	switch {
	case math.IsNaN(f):
		return lang.Name(codes.ValNan), true
	case math.IsInf(f, 1):
		return lang.Name(codes.ValInf), true
	case math.IsInf(f, -1):
		return "-" + lang.Name(codes.ValInf), true
	}
	return "", false
}

type StringVal struct {
	Val string
}

func (s *StringVal) Type() ValueType { return ValString }
func (s *StringVal) Inspect() string { return s.Val }
func (s *StringVal) IsTruthy() bool  { return len(s.Val) > 0 }

type BoolVal struct {
	Val bool
}

func (b *BoolVal) Type() ValueType { return ValBool }
func (b *BoolVal) Inspect() string { return b.Text(codes.Source()) }
func (b *BoolVal) Text(lang *codes.Language) string {
	if b.Val {
		return lang.Name(codes.KwTrue)
	}
	return lang.Name(codes.KwFalse)
}
func (b *BoolVal) IsTruthy() bool { return b.Val }

type NullVal struct{}

func (n *NullVal) Type() ValueType                  { return ValNull }
func (n *NullVal) Inspect() string                  { return n.Text(codes.Source()) }
func (n *NullVal) Text(lang *codes.Language) string { return lang.Name(codes.KwNone) }
func (n *NullVal) IsTruthy() bool                   { return false }

var (
	True  = &BoolVal{Val: true}
	False = &BoolVal{Val: false}
	Null  = &NullVal{}
)

type ListVal struct {
	Elements []Value
	// own listenin sahiplik durumu: liste yazımının, liste üretecinin ya da bir motor
	// işleminin kurduğu liste elemanlarının sahibidir (ownership.go).
	own Ownership
}

func (l *ListVal) Type() ValueType { return ValList }
func (l *ListVal) Inspect() string { return writeContainer(l, nil, nil) }
func (l *ListVal) IsTruthy() bool  { return len(l.Elements) > 0 }

// DictVal is an ordered dict. String keys are stored as is in Pairs/Order (so Go-side accesses
// like d.Pairs["ad"] keep working). Non-string keys (Tam, Ondalık, Mantık, ...) are encoded by
// type with DictKey, so d[1] and d["1"] are different entries. The original key value is kept
// in KeyVals and given back in loops, anahtarlar() and printing.
type DictVal struct {
	Order   []string
	Pairs   map[string]Value
	KeyVals map[string]Value // yalnızca metin olmayan anahtarlar için
	// module, 'aktar ... olarak ad' ile bağlanan modül sözlüğünün dışa aktarmadığı (başı _)
	// adlarını tutar (11.7); sıradan sözlükte nildir.
	module *moduleExports
}

// moduleExports bir modül sözlüğünün kaynağıdır: modülün gösterilen yolu ve modüle özel adları.
type moduleExports struct {
	path    string
	private map[string]bool
}

// privateNameError ad bu modül sözlüğünün dışa aktarmadığı bir adsa hatayı verir.
func (d *DictVal) privateNameError(name string) error {
	if d.module == nil || !d.module.private[name] {
		return nil
	}
	return messages.New(messages.H2487, name, d.module.path)
}

func NewDictVal() *DictVal {
	return &DictVal{
		Order: []string{},
		Pairs: make(map[string]Value),
	}
}

// nonStringKeyPrefix metin olmayan anahtarların iç kodlamasını başlatır. Bu önekle başlayan
// bir metin anahtar da kodlanır (DictKey), böylece hiçbir metin başka bir anahtarla çakışmaz.
const nonStringKeyPrefix = "\x00"

// DictKey converts a value to its type-aware key inside a dict. Keys follow equality (4.5):
// number keys follow number equality (1 == 1.0: an integral Ondalık is the same key as the Tam
// with that value); functions, builtins and struct definitions are keyed by identity. A string
// that starts like an encoded key is encoded too, so no string collides with a non-string key.
// Mutable values are not valid keys (see DictKeyError).
func DictKey(v Value) string {
	switch x := v.(type) {
	case *StringVal:
		if strings.HasPrefix(x.Val, nonStringKeyPrefix) {
			return nonStringKeyPrefix + string(ValString) + ":" + x.Val
		}
		return x.Val
	case *FuncVal:
		if x.codeVal == nil {
			return identityKey(x)
		}
		bound := "-"
		if x.BoundSelf != nil {
			bound = fmt.Sprintf("%p", x.BoundSelf)
		}
		return fmt.Sprintf("%s%s:%p:%p:%p:%s", nonStringKeyPrefix, x.Type(), x.codeVal, x.Env, x.Body, bound)
	case *TypeVal:
		if x.Struct != nil {
			return identityKey(x.Struct)
		}
	case *BoolVal, *NullVal, *FloatVal, *IntVal, *RangeVal:
	default:
		return identityKey(v)
	}
	if n, ok := intKeyValue(v); ok {
		var b [32]byte
		return string(intKey(b[:0], n))
	}
	return nonStringKeyPrefix + string(v.Type()) + ":" + v.Inspect()
}

// identityKey keys a value by identity: the same value is the same key, two values that only
// look alike are not.
func identityKey(v Value) string {
	return fmt.Sprintf("%s%s:%p", nonStringKeyPrefix, v.Type(), v)
}

// DictKeyError reports why a value cannot be a dict key, or nil. A value whose contents can
// change in place (a list, a dict, a struct instance, an engine object such as a tensor) would
// be lost from the dict once changed, so it is rejected.
func DictKeyError(v Value) error {
	mutable := false
	switch v.(type) {
	case *ListVal, *DictVal, *StructInstanceVal:
		mutable = true
	case IndexAssignable, MemberAssignable:
		mutable = true
	}
	if !mutable {
		return nil
	}
	return messages.New(messages.H2495, v.Type())
}

// intKeyValue bir sayının Tam anahtar olarak değerini verir: Tam'ın kendisi ya da tam
// değerli (kesirsiz, Tam aralığında) bir Ondalık.
func intKeyValue(v Value) (int64, bool) {
	switch x := v.(type) {
	case *IntVal:
		return x.Val, true
	case *FloatVal:
		if x.Val == math.Trunc(x.Val) && x.Val >= -intLimit && x.Val < intLimit {
			return int64(x.Val), true
		}
	}
	return 0, false
}

// Set stores val under the string key. A key that starts like an encoded non-string key is
// encoded (see DictKey), so text from outside (JSON, file names, user names) never collides
// with a number or another key.
func (d *DictVal) Set(key string, val Value) {
	if strings.HasPrefix(key, nonStringKeyPrefix) {
		d.SetValue(&StringVal{Val: key}, val)
		return
	}
	d.setRaw(key, val)
}

// setRaw stores val under an already encoded key.
func (d *DictVal) setRaw(key string, val Value) {
	// Tek harita işlemi: anahtar yeniyse harita büyür ve sıraya eklenir
	n := len(d.Pairs)
	d.Pairs[key] = val
	if len(d.Pairs) != n {
		d.Order = append(d.Order, key)
	}
}

// intKey bir Tam anahtarın iç kodlamasını (DictKey ile aynı) buf'a yazar; böylece
// aramada ara metin ayrılmaz.
func intKey(buf []byte, n int64) []byte {
	buf = append(buf, nonStringKeyPrefix+string(ValInt)+":"...)
	return strconv.AppendInt(buf, n, 10)
}

// SetValue takes the key as a TR value (d[1] = ..., {1: ...}).
func (d *DictVal) SetValue(key Value, val Value) {
	if iv, ok := key.(*IntVal); ok {
		// Tam anahtar: tek ayırma ve tek harita işlemi; özgün anahtar yalnızca yeni girdide
		// saklanır (aynı sayının her Tam değeri eşdeğerdir)
		var b [32]byte
		k := string(intKey(b[:0], iv.Val))
		n := len(d.Pairs)
		d.Pairs[k] = val
		if len(d.Pairs) != n {
			d.Order = append(d.Order, k)
			if d.KeyVals == nil {
				d.KeyVals = make(map[string]Value)
			}
			d.KeyVals[k] = key
		}
		return
	}
	k := DictKey(key)
	if s, isStr := key.(*StringVal); !isStr || k != s.Val {
		// Özgün anahtar yalnızca yeni girdide saklanır (d[1] varken d[1.0] = ... aynı girdiyi
		// günceller, anahtar 1 olarak kalır)
		if _, var_ := d.Pairs[k]; !var_ {
			if d.KeyVals == nil {
				d.KeyVals = make(map[string]Value)
			}
			d.KeyVals[k] = key
		}
	}
	d.setRaw(k, val)
}

// GetValue looks the key up as a TR value.
func (d *DictVal) GetValue(key Value) (Value, bool) {
	if iv, ok := key.(*IntVal); ok {
		var b [32]byte
		val, ok := d.Pairs[string(intKey(b[:0], iv.Val))] // dönüşüm ayırmaz
		return val, ok
	}
	val, ok := d.Pairs[DictKey(key)]
	return val, ok
}

// KeyValue returns the original TR value of an internal key.
func (d *DictVal) KeyValue(k string) Value {
	if kv, ok := d.KeyVals[k]; ok {
		return kv
	}
	return &StringVal{Val: k}
}

// DisplayKey returns the text of an internal key as shown to the user.
func (d *DictVal) DisplayKey(k string) string {
	if kv, ok := d.KeyVals[k]; ok {
		return kv.Inspect()
	}
	return k
}

func (d *DictVal) Type() ValueType { return ValDict }
func (d *DictVal) Inspect() string { return writeContainer(d, nil, nil) }
func (d *DictVal) IsTruthy() bool  { return len(d.Pairs) > 0 }

type RangeVal struct {
	Start     int64
	End       int64
	Inclusive bool
}

func (r *RangeVal) Type() ValueType { return ValRange }
func (r *RangeVal) Inspect() string {
	if r.Inclusive {
		return fmt.Sprintf("%d..=%d", r.Start, r.End)
	}
	return fmt.Sprintf("%d..%d", r.Start, r.End)
}
func (r *RangeVal) IsTruthy() bool { return true }

type FuncVal struct {
	Name       string
	Parameters []ast.Param
	Body       *ast.BlockStatement
	Env        *Environment
	BoundSelf  Value
	// codeVal fonksiyonun derlenmiş hâlidir (nil ise ilk çağrıda derlenir).
	codeVal *funcCode
}

func (f *FuncVal) Type() ValueType { return ValFunc }
func (f *FuncVal) Inspect() string {
	if f.Name != "" {
		return messages.Text(messages.H2496, f.Name)
	}
	return messages.Text(messages.H2442)
}
func (f *FuncVal) Text(lang *codes.Language) string {
	if f.Name != "" {
		return messages.TextIn(lang.Code, messages.H2496, f.Name)
	}
	return messages.TextIn(lang.Code, messages.H2442)
}
func (f *FuncVal) IsTruthy() bool { return true }

type BuiltinFunc func(y *VM, args []Value, named map[string]Value) (Value, error)

type BuiltinVal struct {
	Name string
	Fn   BuiltinFunc
	// Single (optional) runs instead of Fn when the builtin is called with exactly one
	// positional argument, without allocating an argument slice; its result must equal
	// Fn(y, []Value{a}, nil).
	Single func(y *VM, a Value) (Value, error)
	// NoNamedArgs, when true, means the builtin takes no named arguments: a call with named
	// arguments is rejected before the body runs with an "adlı argüman almaz" error (never
	// silently ignored).
	NoNamedArgs bool
	// Pure, when true, means the builtin neither keeps its arguments after it returns nor
	// returns them inside its result (tensor operations). The interpreter then releases
	// argument temporaries right after the call; arguments of other builtins escape
	// (ownership.go).
	Pure bool
}

func (b *BuiltinVal) Type() ValueType { return ValBuiltin }
func (b *BuiltinVal) Inspect() string { return messages.Text(messages.H2497, b.Name) }

// text yerleşiğin lang dilindeki metnidir: adı (kanonik ad, bir yerleşik ya da üye kodu) da o
// dildedir; koda bağlı olmayan ad olduğu gibi kalır.
func (b *BuiltinVal) Text(lang *codes.Language) string {
	name := b.Name
	for _, ns := range []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceMember} {
		if k, ok := codes.Find(ns, name); ok {
			name = lang.Name(k)
			break
		}
	}
	return messages.TextIn(lang.Code, messages.H2497, name)
}
func (b *BuiltinVal) IsTruthy() bool { return true }

type StructDefVal struct {
	Name    string
	Fields  []ast.StructField
	Methods map[string]*FuncVal
	// fieldCode alanların derlenmiş varsayılan değer ifadeleridir (Fields ile aynı sırada;
	// varsayılanı olmayan alan için nil). nil dilimse varsayılanlar ilk kullanımda derlenir.
	fieldCode []exprCode
	// fieldEnv varsayılanların değerlendirildiği ortamdır (yapının tanımlandığı ortam).
	fieldEnv *Environment
	// typ nesnelerinin tip değeridir (tip(p) → Nokta; bkz. tipDegeri).
	typ *TypeVal
}

func (s *StructDefVal) Type() ValueType { return ValStruct }
func (s *StructDefVal) Inspect() string { return messages.Text(messages.H2498, s.Name) }
func (s *StructDefVal) Text(lang *codes.Language) string {
	return messages.TextIn(lang.Code, messages.H2498, s.Name)
}
func (s *StructDefVal) IsTruthy() bool { return true }

type StructInstanceVal struct {
	Def    *StructDefVal
	Fields map[string]Value
}

func (si *StructInstanceVal) Type() ValueType { return ValueType(si.Def.Name) }
func (si *StructInstanceVal) Inspect() string { return writeContainer(si, nil, nil) }
func (si *StructInstanceVal) IsTruthy() bool  { return true }

// --- KÜÇÜK TAMSAYI ÖNBELLEĞİ ---

// Döngü sayaçları, indeksler ve küçük sonuçlar için her seferinde yeni nesne ayırmamak
// adına -128..1023 aralığındaki tamsayılar önceden oluşturulur. IntVal değişmezdir (hiçbir
// yerde Val yerinde değiştirilmez); bu yüzden aynı nesne güvenle paylaşılabilir.
const (
	smallIntMin = -128
	smallIntMax = 1023
)

var smallInts = func() [smallIntMax - smallIntMin + 1]*IntVal {
	var t [smallIntMax - smallIntMin + 1]*IntVal
	for i := range t {
		t[i] = &IntVal{Val: int64(i + smallIntMin)}
	}
	return t
}()

// NewInt returns an integer value; small values come from a cache.
func NewInt(n int64) *IntVal {
	if n >= smallIntMin && n <= smallIntMax {
		return smallInts[n-smallIntMin]
	}
	return &IntVal{Val: n}
}
