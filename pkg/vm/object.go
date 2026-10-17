package vm

import (
	"fmt"
	"strings"
	"tr_dili/pkg/ast"
)

type ValueType string

const (
	VAL_INT      ValueType = "Tam"
	VAL_FLOAT    ValueType = "Ondalık"
	VAL_STRING   ValueType = "Metin"
	VAL_BOOL     ValueType = "Mantık"
	VAL_NULL     ValueType = "Yok"
	VAL_LIST     ValueType = "Liste"
	VAL_DICT     ValueType = "Sözlük"
	VAL_RANGE    ValueType = "Aralık"
	VAL_FUNC     ValueType = "Fonksiyon"
	VAL_BUILTIN  ValueType = "YerleşikFonksiyon"
	VAL_STRUCT   ValueType = "YapıTanımı"
	VAL_INSTANCE ValueType = "Nesne"
	VAL_MATRIX   ValueType = "Matris"
	VAL_TENSOR   ValueType = "Tensör"
)

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
func (i *IntVal) Inspect() string { return fmt.Sprintf("%d", i.Val) }
func (i *IntVal) IsTruthy() bool  { return i.Val != 0 }

type FloatVal struct {
	Val float64
}

func (f *FloatVal) Type() ValueType { return VAL_FLOAT }
func (f *FloatVal) Inspect() string {
	s := fmt.Sprintf("%g", f.Val)
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
		return "doğru"
	}
	return "yanlış"
}
func (b *BoolVal) IsTruthy() bool { return b.Val }

type NullVal struct{}

func (n *NullVal) Type() ValueType { return VAL_NULL }
func (n *NullVal) Inspect() string { return "yok" }
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
func (l *ListVal) Inspect() string {
	var parts []string
	for _, el := range l.Elements {
		if el.Type() == VAL_STRING {
			parts = append(parts, fmt.Sprintf("%q", el.Inspect()))
		} else {
			parts = append(parts, el.Inspect())
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
func (l *ListVal) IsTruthy() bool { return len(l.Elements) > 0 }

// DictVal sıralı bir sözlüktür. Metin anahtarlar Pairs/Order içinde olduğu gibi
// saklanır (Go tarafındaki d.Pairs["ad"] erişimleri bu sayede çalışmaya devam eder).
// Metin olmayan anahtarlar (Tam, Ondalık, Mantık, ...) DictKey ile türüne göre
// kodlanır; böylece d[1] ile d["1"] farklı girdilerdir. Özgün anahtar değeri
// KeyVals içinde tutulur ve döngü/anahtarlar()/yazdırma sırasında geri verilir.
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

// DictKey bir değeri sözlük içindeki türe duyarlı anahtarına dönüştürür.
func DictKey(v Value) string {
	if s, ok := v.(*StringVal); ok {
		return s.Val
	}
	return nonStringKeyPrefix + string(v.Type()) + ":" + v.Inspect()
}

func (d *DictVal) Set(key string, val Value) {
	if _, exists := d.Pairs[key]; !exists {
		d.Order = append(d.Order, key)
	}
	d.Pairs[key] = val
}

// SetValue anahtarı bir TR değeri olarak alır (d[1] = ..., {1: ...}).
func (d *DictVal) SetValue(key Value, val Value) {
	k := DictKey(key)
	if _, isStr := key.(*StringVal); !isStr {
		if d.KeyVals == nil {
			d.KeyVals = make(map[string]Value)
		}
		d.KeyVals[k] = key
	}
	d.Set(k, val)
}

// GetValue anahtarı bir TR değeri olarak arar.
func (d *DictVal) GetValue(key Value) (Value, bool) {
	val, ok := d.Pairs[DictKey(key)]
	return val, ok
}

// KeyValue iç anahtarın özgün TR değerini döndürür.
func (d *DictVal) KeyValue(k string) Value {
	if kv, ok := d.KeyVals[k]; ok {
		return kv
	}
	return &StringVal{Val: k}
}

// DisplayKey iç anahtarın kullanıcıya gösterilecek metin hâlini döndürür.
func (d *DictVal) DisplayKey(k string) string {
	if kv, ok := d.KeyVals[k]; ok {
		return kv.Inspect()
	}
	return k
}

func (d *DictVal) Type() ValueType { return VAL_DICT }
func (d *DictVal) Inspect() string {
	var parts []string
	for _, k := range d.Order {
		v := d.Pairs[k]
		valStr := v.Inspect()
		if v.Type() == VAL_STRING {
			valStr = fmt.Sprintf("%q", valStr)
		}
		keyStr := fmt.Sprintf("%q", k)
		if kv, ok := d.KeyVals[k]; ok {
			keyStr = kv.Inspect()
		}
		parts = append(parts, fmt.Sprintf("%s: %s", keyStr, valStr))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
func (d *DictVal) IsTruthy() bool { return len(d.Pairs) > 0 }

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
}

func (f *FuncVal) Type() ValueType { return VAL_FUNC }
func (f *FuncVal) Inspect() string {
	if f.Name != "" {
		return fmt.Sprintf("<fonksiyon %s>", f.Name)
	}
	return "<ok fonksiyonu>"
}
func (f *FuncVal) IsTruthy() bool { return true }

type BuiltinFunc func(vm *VM, args []Value, named map[string]Value) (Value, error)

type BuiltinVal struct {
	Name string
	Fn   BuiltinFunc
}

func (b *BuiltinVal) Type() ValueType { return VAL_BUILTIN }
func (b *BuiltinVal) Inspect() string { return fmt.Sprintf("<yerleşik %s>", b.Name) }
func (b *BuiltinVal) IsTruthy() bool  { return true }

type StructDefVal struct {
	Name    string
	Fields  []ast.StructField
	Methods map[string]*FuncVal
}

func (s *StructDefVal) Type() ValueType { return VAL_STRUCT }
func (s *StructDefVal) Inspect() string { return fmt.Sprintf("<yapı %s>", s.Name) }
func (s *StructDefVal) IsTruthy() bool  { return true }

type StructInstanceVal struct {
	Def    *StructDefVal
	Fields map[string]Value
}

func (si *StructInstanceVal) Type() ValueType { return ValueType(si.Def.Name) }
func (si *StructInstanceVal) Inspect() string {
	var parts []string
	for _, f := range si.Def.Fields {
		if v, ok := si.Fields[f.Name]; ok {
			parts = append(parts, fmt.Sprintf("%s: %s", f.Name, v.Inspect()))
		}
	}
	return fmt.Sprintf("%s{%s}", si.Def.Name, strings.Join(parts, ", "))
}
func (si *StructInstanceVal) IsTruthy() bool { return true }

// --- KÜÇÜK TAMSAYI ÖNBELLEĞİ ---

// Döngü sayaçları, indeksler ve küçük sonuçlar için her seferinde yeni nesne ayırmamak
// adına -128..1023 aralığındaki tamsayılar önceden oluşturulur. IntVal değişmezdir (hiçbir
// yerde Val yerinde değiştirilmez); bu yüzden aynı nesne güvenle paylaşılabilir.
const (
	kucukIntEnAz  = -128
	kucukIntEnCok = 1023
)

var kucukIntler = func() [kucukIntEnCok - kucukIntEnAz + 1]*IntVal {
	var t [kucukIntEnCok - kucukIntEnAz + 1]*IntVal
	for i := range t {
		t[i] = &IntVal{Val: int64(i + kucukIntEnAz)}
	}
	return t
}()

// NewInt bir tamsayı değeri döndürür; küçük değerler önbellekten gelir.
func NewInt(n int64) *IntVal {
	if n >= kucukIntEnAz && n <= kucukIntEnCok {
		return kucukIntler[n-kucukIntEnAz]
	}
	return &IntVal{Val: n}
}

// --- ÇALIŞMA ORTAMI (ENVIRONMENT) ---

// Environment bir kapsamdaki değişkenleri tutar. Kilidi yoktur: TR kodu yalnızca yorumlayıcı
// kilidini (GIL, bkz. vmShared.gil) tutan görev tarafından çalıştırılır ve hata ayıklayıcının
// istekleri de duraklatılmış görevin kendi goroutine'inde işlenir. Bu yüzden bir ortama aynı
// anda iki goroutine erişmez. GIL'i tutmayan kod (Blocking içindeki işlemler, sunucu
// goroutine'leri) ortamlara dokunmamalıdır; dokunması gerekiyorsa RunTask kullanılır.
//
// Haritalar ilk tanımda oluşturulur: değişken tanımlamayan bloklar ve döngü turları için
// boş harita ayırmaya gerek kalmaz (nil haritadan okuma Go'da güvenlidir).
type Environment struct {
	store      map[string]Value
	consts     map[string]bool
	outer      *Environment
	IsFuncRoot bool
	defers     []func()
}

func NewEnvironment(outer *Environment) *Environment {
	return &Environment{
		outer:      outer,
		IsFuncRoot: outer == nil,
	}
}

// temizle, turlar arasında yeniden kullanılan bir döngü ortamını yeni açılmış gibi boşaltır.
func (e *Environment) temizle() {
	clear(e.store)
	clear(e.consts)
}

func (e *Environment) AddDefer(fn func()) {
	target := e
	for target.outer != nil && !target.IsFuncRoot {
		target = target.outer
	}
	target.defers = append(target.defers, fn)
}

func (e *Environment) RunDefers() {
	if len(e.defers) == 0 {
		return
	}
	list := e.defers
	e.defers = nil
	for i := len(list) - 1; i >= 0; i-- {
		list[i]()
	}
}

func (e *Environment) Get(name string) (Value, bool) {
	for cur := e; cur != nil; cur = cur.outer {
		if val, ok := cur.store[name]; ok {
			return val, true
		}
	}
	return nil, false
}

func (e *Environment) Define(name string, val Value, isConst bool) {
	if e.store == nil {
		e.store = make(map[string]Value)
	}
	// Yerleşik bir fonksiyonun adıyla (ör. 'toplam', 'uzunluk') değişken tanımlamak onu
	// gölgeler; yeni değişken sabit değildir. Kullanıcının 'sabit' tanımları korunur.
	if _, wasBuiltin := e.store[name].(*BuiltinVal); wasBuiltin && !isConst {
		delete(e.consts, name)
	}
	e.store[name] = val
	if isConst {
		if e.consts == nil {
			e.consts = make(map[string]bool)
		}
		e.consts[name] = true
	}
}

func (e *Environment) Assign(name string, val Value) error {
	for cur := e; cur != nil; cur = cur.outer {
		if _, exists := cur.store[name]; exists {
			if cur.consts[name] {
				return fmt.Errorf("%q bir sabittir ('sabit') ve değeri değiştirilemez", name)
			}
			cur.store[name] = val
			return nil
		}
	}
	// Hiçbir kapsamda yoksa sessizce genel değişken oluşturmak yazım hatalarını
	// gizler (ör. fonksiyon içinde 'toplm = 1'); bu yüzden açık tanım istenir.
	return &TanimsizDegiskenHatasi{Ad: name}
}

// TanimsizDegiskenHatasi tanımlanmamış bir değişkene atama yapıldığında döner; VM bu
// türü tanıyarak mesaja "Bunu mu demek istediniz?" önerisi ekler.
type TanimsizDegiskenHatasi struct{ Ad string }

func (e *TanimsizDegiskenHatasi) Error() string {
	return fmt.Sprintf("tanımsız değişken: '%s' (önce 'değişken' ile tanımlayın)", e.Ad)
}

// Names bu kapsamdan ve dış kapsamlardan görünen tüm adları döndürür (öneriler için).
func (e *Environment) Names() []string {
	seen := map[string]bool{}
	var names []string
	for cur := e; cur != nil; cur = cur.outer {
		for k := range cur.store {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		}
	}
	return names
}
