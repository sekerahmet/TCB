package interp

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// ValueType is the type name of a value (tip() result, error messages). The names are in the
// dictionary (internal/lang/codes, A2xx) and are read once at startup.
type ValueType string

var (
	VAL_INT      = TypeName(codes.A200) // Tam
	VAL_FLOAT    = TypeName(codes.A201) // Ondalık
	VAL_STRING   = TypeName(codes.A202) // Metin
	VAL_BOOL     = TypeName(codes.A203) // Mantık
	VAL_NULL     = TypeName(codes.A204) // Yok
	VAL_LIST     = TypeName(codes.A205) // Liste
	VAL_DICT     = TypeName(codes.A206) // Sözlük
	VAL_RANGE    = TypeName(codes.A207) // Aralık
	VAL_FUNC     = TypeName(codes.A208) // Fonksiyon
	VAL_BUILTIN  = TypeName(codes.A209) // YerleşikFonksiyon
	VAL_STRUCT   = TypeName(codes.A210) // YapıTanımı
	VAL_INSTANCE = TypeName(codes.A211) // Nesne
	VAL_TENSOR   = TypeName(codes.A212) // Tensör
)

// TypeName is the ValueType of a type-name code from the dictionary (A2xx); engines get the
// names of their own types with it.
func TypeName(k codes.Code) ValueType {
	if k.Namespace() != codes.NamespaceType {
		panic(fmt.Sprintf("TipAdi: %s bir tip adı kodu (A2xx) değil", k))
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

func (i *IntVal) Type() ValueType { return VAL_INT }
func (i *IntVal) Inspect() string { return strconv.FormatInt(i.Val, 10) }
func (i *IntVal) IsTruthy() bool  { return i.Val != 0 }

type FloatVal struct {
	Val float64
}

func (f *FloatVal) Type() ValueType { return VAL_FLOAT }
func (f *FloatVal) Inspect() string {
	// Sonsuz ve tanımsız değerler tensör yazdırmasıyla aynı biçimde yazılır
	switch {
	case math.IsNaN(f.Val):
		return "NaN"
	case math.IsInf(f.Val, 1):
		return "+Sonsuz"
	case math.IsInf(f.Val, -1):
		return "-Sonsuz"
	}
	s := strconv.FormatFloat(f.Val, 'g', -1, 64)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}
func (f *FloatVal) IsTruthy() bool { return f.Val != 0.0 }

type StringVal struct {
	Val string
}

func (s *StringVal) Type() ValueType { return VAL_STRING }
func (s *StringVal) Inspect() string { return s.Val }
func (s *StringVal) IsTruthy() bool  { return len(s.Val) > 0 }

type BoolVal struct {
	Val bool
}

func (b *BoolVal) Type() ValueType { return VAL_BOOL }
func (b *BoolVal) Inspect() string {
	if b.Val {
		return codes.Name(codes.A040)
	}
	return codes.Name(codes.A041)
}
func (b *BoolVal) IsTruthy() bool { return b.Val }

type NullVal struct{}

func (n *NullVal) Type() ValueType { return VAL_NULL }
func (n *NullVal) Inspect() string { return codes.Name(codes.A042) }
func (n *NullVal) IsTruthy() bool  { return false }

var (
	TRUE  = &BoolVal{Val: true}
	FALSE = &BoolVal{Val: false}
	NULL  = &NullVal{}
)

type ListVal struct {
	Elements []Value
}

func (l *ListVal) Type() ValueType { return VAL_LIST }
func (l *ListVal) Inspect() string { return writeContainer(l, nil) }
func (l *ListVal) IsTruthy() bool  { return len(l.Elements) > 0 }

// DictVal is an ordered dict. String keys are stored as is in Pairs/Order (so Go-side accesses
// like d.Pairs["ad"] keep working). Non-string keys (Tam, Ondalık, Mantık, ...) are encoded by
// type with DictKey, so d[1] and d["1"] are different entries. The original key value is kept
// in KeyVals and given back in loops, anahtarlar() and printing.
type DictVal struct {
	Order   []string
	Pairs   map[string]Value
	KeyVals map[string]Value // yalnızca metin olmayan anahtarlar için
}

func NewDictVal() *DictVal {
	return &DictVal{
		Order: []string{},
		Pairs: make(map[string]Value),
	}
}

// nonStringKeyPrefix metin olmayan anahtarların iç kodlamasını başlatır. NUL
// karakteri normal metinlerde bulunmadığından metin anahtarlarla çakışmaz.
const nonStringKeyPrefix = "\x00"

// DictKey converts a value to its type-aware key inside a dict. Number keys follow number
// equality (4.5: 1 == 1.0): an integral Ondalık is the same key as the Tam with that value.
func DictKey(v Value) string {
	if s, ok := v.(*StringVal); ok {
		return s.Val
	}
	if n, ok := intKeyValue(v); ok {
		var b [32]byte
		return string(intKey(b[:0], n))
	}
	return nonStringKeyPrefix + string(v.Type()) + ":" + v.Inspect()
}

// tamAnahtarDegeri bir sayının Tam anahtar olarak değerini verir: Tam'ın kendisi ya da tam
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

func (d *DictVal) Set(key string, val Value) {
	// Tek harita işlemi: anahtar yeniyse harita büyür ve sıraya eklenir
	n := len(d.Pairs)
	d.Pairs[key] = val
	if len(d.Pairs) != n {
		d.Order = append(d.Order, key)
	}
}

// tamAnahtar bir Tam anahtarın iç kodlamasını (DictKey ile aynı) buf'a yazar; böylece
// aramada ara metin ayrılmaz.
func intKey(buf []byte, n int64) []byte {
	buf = append(buf, nonStringKeyPrefix+string(VAL_INT)+":"...)
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
	if _, isStr := key.(*StringVal); !isStr {
		// Özgün anahtar yalnızca yeni girdide saklanır (d[1] varken d[1.0] = ... aynı girdiyi
		// günceller, anahtar 1 olarak kalır)
		if _, var_ := d.Pairs[k]; !var_ {
			if d.KeyVals == nil {
				d.KeyVals = make(map[string]Value)
			}
			d.KeyVals[k] = key
		}
	}
	d.Set(k, val)
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

func (d *DictVal) Type() ValueType { return VAL_DICT }
func (d *DictVal) Inspect() string { return writeContainer(d, nil) }
func (d *DictVal) IsTruthy() bool  { return len(d.Pairs) > 0 }

type RangeVal struct {
	Start     int64
	End       int64
	Inclusive bool
}

func (r *RangeVal) Type() ValueType { return VAL_RANGE }
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
	// kod fonksiyonun derlenmiş hâlidir (nil ise ilk çağrıda derlenir).
	codeVal *funcCode
}

func (f *FuncVal) Type() ValueType { return VAL_FUNC }
func (f *FuncVal) Inspect() string {
	if f.Name != "" {
		return fmt.Sprintf("<fonksiyon %s>", f.Name)
	}
	return "<ok fonksiyonu>"
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
}

func (b *BuiltinVal) Type() ValueType { return VAL_BUILTIN }
func (b *BuiltinVal) Inspect() string { return fmt.Sprintf("<yerleşik %s>", b.Name) }
func (b *BuiltinVal) IsTruthy() bool  { return true }

type StructDefVal struct {
	Name    string
	Fields  []ast.StructField
	Methods map[string]*FuncVal
	// alanKod alanların derlenmiş varsayılan değer ifadeleridir (Fields ile aynı sırada;
	// varsayılanı olmayan alan için nil). nil dilimse varsayılanlar ilk kullanımda derlenir.
	fieldCode []exprCode
	// alanOrtam varsayılanların değerlendirildiği ortamdır (yapının tanımlandığı ortam).
	fieldEnv *Environment
	// tip nesnelerinin tip değeridir (tip(p) → Nokta; bkz. tipDegeri).
	typ *TypeVal
}

func (s *StructDefVal) Type() ValueType { return VAL_STRUCT }
func (s *StructDefVal) Inspect() string { return fmt.Sprintf("<yapı %s>", s.Name) }
func (s *StructDefVal) IsTruthy() bool  { return true }

type StructInstanceVal struct {
	Def    *StructDefVal
	Fields map[string]Value
}

func (si *StructInstanceVal) Type() ValueType { return ValueType(si.Def.Name) }
func (si *StructInstanceVal) Inspect() string { return writeContainer(si, nil) }
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
