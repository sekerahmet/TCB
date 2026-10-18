package vm

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/sozluk"
)

// ValueType bir değerin tip adıdır (tip() sonucu, hata mesajları). Adlar sözlüktedir
// (pkg/sozluk, A2xx); başlangıçta bir kez okunur.
type ValueType string

var (
	VAL_INT      = TipAdi(sozluk.A + 200) // Tam
	VAL_FLOAT    = TipAdi(sozluk.A + 201) // Ondalık
	VAL_STRING   = TipAdi(sozluk.A + 202) // Metin
	VAL_BOOL     = TipAdi(sozluk.A + 203) // Mantık
	VAL_NULL     = TipAdi(sozluk.A + 204) // Yok
	VAL_LIST     = TipAdi(sozluk.A + 205) // Liste
	VAL_DICT     = TipAdi(sozluk.A + 206) // Sözlük
	VAL_RANGE    = TipAdi(sozluk.A + 207) // Aralık
	VAL_FUNC     = TipAdi(sozluk.A + 208) // Fonksiyon
	VAL_BUILTIN  = TipAdi(sozluk.A + 209) // YerleşikFonksiyon
	VAL_STRUCT   = TipAdi(sozluk.A + 210) // YapıTanımı
	VAL_INSTANCE = TipAdi(sozluk.A + 211) // Nesne
	VAL_TENSOR   = TipAdi(sozluk.A + 212) // Tensör
)

// TipAdi sözlükteki bir tip adı kodunun (A2xx) ValueType'ıdır; motorlar kendi tiplerinin
// adlarını bununla alır.
func TipAdi(k sozluk.Kod) ValueType {
	if k.Alan() != sozluk.AlanTip {
		panic(fmt.Sprintf("TipAdi: %s bir tip adı kodu (A2xx) değil", k))
	}
	return ValueType(sozluk.Ad(k))
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
func (l *ListVal) Inspect() string { return kapYaz(l, nil) }
func (l *ListVal) IsTruthy() bool  { return len(l.Elements) > 0 }

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
	// Tek harita işlemi: anahtar yeniyse harita büyür ve sıraya eklenir
	n := len(d.Pairs)
	d.Pairs[key] = val
	if len(d.Pairs) != n {
		d.Order = append(d.Order, key)
	}
}

// tamAnahtar bir Tam anahtarın iç kodlamasını (DictKey ile aynı) buf'a yazar; böylece
// aramada ara metin ayrılmaz.
func tamAnahtar(buf []byte, n int64) []byte {
	buf = append(buf, nonStringKeyPrefix+string(VAL_INT)+":"...)
	return strconv.AppendInt(buf, n, 10)
}

// SetValue anahtarı bir TR değeri olarak alır (d[1] = ..., {1: ...}).
func (d *DictVal) SetValue(key Value, val Value) {
	if iv, ok := key.(*IntVal); ok {
		// Tam anahtar: tek ayırma ve tek harita işlemi; özgün anahtar yalnızca yeni girdide
		// saklanır (aynı sayının her Tam değeri eşdeğerdir)
		var b [32]byte
		k := string(tamAnahtar(b[:0], iv.Val))
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
		if d.KeyVals == nil {
			d.KeyVals = make(map[string]Value)
		}
		d.KeyVals[k] = key
	}
	d.Set(k, val)
}

// GetValue anahtarı bir TR değeri olarak arar.
func (d *DictVal) GetValue(key Value) (Value, bool) {
	if iv, ok := key.(*IntVal); ok {
		var b [32]byte
		val, ok := d.Pairs[string(tamAnahtar(b[:0], iv.Val))] // dönüşüm ayırmaz
		return val, ok
	}
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
func (d *DictVal) Inspect() string { return kapYaz(d, nil) }
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
	kod *fonkKodu
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
	// Tek (isteğe bağlı), yerleşik tek bir konumlu argümanla çağrıldığında Fn yerine
	// çalışır ve argüman dilimi ayırmaz; sonucu Fn(vm, []Value{a}, nil) ile aynı olmalıdır.
	Tek func(vm *VM, a Value) (Value, error)
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
	alanKod []ifadeKodu
	// alanOrtam varsayılanların değerlendirildiği ortamdır (yapının tanımlandığı ortam).
	alanOrtam *Environment
}

func (s *StructDefVal) Type() ValueType { return VAL_STRUCT }
func (s *StructDefVal) Inspect() string { return fmt.Sprintf("<yapı %s>", s.Name) }
func (s *StructDefVal) IsTruthy() bool  { return true }

type StructInstanceVal struct {
	Def    *StructDefVal
	Fields map[string]Value
}

func (si *StructInstanceVal) Type() ValueType { return ValueType(si.Def.Name) }
func (si *StructInstanceVal) Inspect() string { return kapYaz(si, nil) }
func (si *StructInstanceVal) IsTruthy() bool  { return true }

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
