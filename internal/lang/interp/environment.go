package interp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// --- ÇALIŞMA ORTAMI (ENVIRONMENT) ---

// kapsamBilgisi bir kapsamın ad tablosudur: her ada bir yuva (slot) indeksi verir. Derleyici
// kapsamda tanımlanan adları derleme anında buraya ekler; derlenmiş kod değişkenlere adla
// değil indeksle erişir. Aynı kapsamdan açılan tüm ortamlar (ör. bir fonksiyonun her
// çağrısı) tabloyu paylaşır. Tablo yalnızca büyür: çalışma anında adla yapılan bir tanım
// (hata ayıklayıcı, takma adsız 'aktar') yeni bir ad ekleyebilir; böyle bir ad, o yuvası
// henüz olmayan diğer ortamlarda "tanımsız" sayılır.
type kapsamBilgisi struct {
	adlar  []string
	indeks map[string]int
}

// yuva adın indeksini döndürür; ad tabloda yoksa ekler.
func (k *kapsamBilgisi) yuva(ad string) int {
	if i, ok := k.indeks[ad]; ok {
		return i
	}
	if k.indeks == nil {
		k.indeks = make(map[string]int)
	}
	i := len(k.adlar)
	k.adlar = append(k.adlar, ad)
	k.indeks[ad] = i
	return i
}

// ortamEk ortamların nadiren gereken alanlarıdır (yer kazanmak için ayrı tutulur).
type ortamEk struct {
	defers []func() error
	consts []bool // yuva başına 'sabit' bayrağı
}

// Environment holds the variables of a scope. Values live in slots that match indexes in the
// scope's name table; a nil slot means "not (yet) defined in this environment" and the lookup
// continues in outer environments. Access by name (Get, Define, Assign, Names, EachLocal) is
// kept for the debugger, suggestions and modules.
//
// It has no lock: TR code runs only on the task holding the interpreter lock (GIL, see
// vmShared.gil), and debugger requests are also handled on the paused task's own goroutine.
// So no two goroutines touch an environment (or its name table) at once. Code that does not
// hold the GIL (work inside Blocking, engines' background goroutines) must not touch
// environments.
type Environment struct {
	vals       []Value
	kapsam     *kapsamBilgisi
	outer      *Environment
	ek         *ortamEk
	IsFuncRoot bool
	// ic küçük ortamların yuvaları için satır içi depodur (ayrı bir ayırma gerekmez).
	ic [4]Value
}

// NewEnvironment opens a new environment with its own (empty) name table.
func NewEnvironment(outer *Environment) *Environment {
	return &Environment{
		outer:      outer,
		kapsam:     &kapsamBilgisi{},
		IsFuncRoot: outer == nil,
	}
}

// yeniOrtam derlenmiş bir kapsam için, tablodaki adlar kadar yuvayla ortam açar.
func yeniOrtam(outer *Environment, k *kapsamBilgisi) *Environment {
	e := &Environment{outer: outer, kapsam: k}
	if n := len(k.adlar); n <= len(e.ic) {
		e.vals = e.ic[:n:n]
	} else {
		e.vals = make([]Value, n)
	}
	return e
}

// temizle, turlar arasında yeniden kullanılan bir döngü ortamını yeni açılmış gibi boşaltır.
func (e *Environment) temizle() {
	clear(e.vals)
	if e.ek != nil {
		clear(e.ek.consts)
	}
}

func (e *Environment) sabitMi(i int) bool {
	return e.ek != nil && i < len(e.ek.consts) && e.ek.consts[i]
}

// tanimla i. yuvaya bir tanım yazar (Define'ın indeksli hâli).
func (e *Environment) tanimla(i int, val Value, isConst bool) {
	for len(e.vals) <= i {
		e.vals = append(e.vals, nil)
	}
	// Yerleşik bir fonksiyonun adıyla (ör. 'toplam', 'uzunluk') değişken tanımlamak onu
	// gölgeler; yeni değişken sabit değildir. Kullanıcının 'sabit' tanımları korunur.
	if _, wasBuiltin := e.vals[i].(*BuiltinVal); wasBuiltin && !isConst && e.sabitMi(i) {
		e.ek.consts[i] = false
	}
	e.vals[i] = val
	if isConst {
		if e.ek == nil {
			e.ek = &ortamEk{}
		}
		for len(e.ek.consts) <= i {
			e.ek.consts = append(e.ek.consts, false)
		}
		e.ek.consts[i] = true
	}
}

func (e *Environment) AddDefer(fn func() error) {
	target := e
	for target.outer != nil && !target.IsFuncRoot {
		target = target.outer
	}
	if target.ek == nil {
		target.ek = &ortamEk{}
	}
	target.ek.defers = append(target.ek.defers, fn)
}

// RunDefers runs the recorded deferred work in reverse order. An error in one does not stop
// the rest; the errors are joined in run order and returned (see erteleHatasiEkle).
func (e *Environment) RunDefers() error {
	if e.ek == nil || len(e.ek.defers) == 0 {
		return nil
	}
	list := e.ek.defers
	e.ek.defers = nil
	var err error
	for i := len(list) - 1; i >= 0; i-- {
		err = erteleHatasiEkle(err, list[i]())
	}
	return err
}

// erteleHatasi, bir hataya (fonksiyonun ya da önceki ertelenen işin hatası) sonradan çalışan
// ertelenen işlerin hatalarının eklenmiş hâlidir. İlk hata korunur (Unwrap onu verir).
type erteleHatasi struct {
	ana   error
	ekler []error
}

func (h *erteleHatasi) Error() string {
	parcalar := make([]string, len(h.ekler))
	for i, e := range h.ekler {
		parcalar[i] = e.Error()
	}
	return h.ana.Error() + "; ertele içinde de hata: " + strings.Join(parcalar, "; ")
}

func (h *erteleHatasi) Unwrap() error { return h.ana }

// erteleHatasiEkle ertelenen işin hatasını (ek) mevcut hataya ekler: mevcut hata yoksa ek
// hatanın kendisi olur, varsa ilk hata korunur ve ek ona eklenir.
func erteleHatasiEkle(mevcut, ek error) error {
	switch {
	case ek == nil:
		return mevcut
	case mevcut == nil:
		return ek
	}
	if h, ok := mevcut.(*erteleHatasi); ok {
		return &erteleHatasi{ana: h.ana, ekler: append(slices.Clip(h.ekler), ek)}
	}
	return &erteleHatasi{ana: mevcut, ekler: []error{ek}}
}

// yerel adın bu ortamdaki değerini döndürür (tanımlı değilse nil).
func (e *Environment) yerel(name string) (int, Value) {
	if i, ok := e.kapsam.indeks[name]; ok && i < len(e.vals) {
		return i, e.vals[i]
	}
	return -1, nil
}

// yerelDeger adın yalnızca bu ortamdaki değerini döndürür (yoksa nil).
func (e *Environment) yerelDeger(name string) Value {
	_, val := e.yerel(name)
	return val
}

func (e *Environment) Get(name string) (Value, bool) {
	for cur := e; cur != nil; cur = cur.outer {
		if _, val := cur.yerel(name); val != nil {
			return val, true
		}
	}
	return nil, false
}

func (e *Environment) Define(name string, val Value, isConst bool) {
	e.tanimla(e.kapsam.yuva(name), val, isConst)
}

func (e *Environment) Assign(name string, val Value) error {
	for cur := e; cur != nil; cur = cur.outer {
		if i, old := cur.yerel(name); old != nil {
			if cur.sabitMi(i) {
				return fmt.Errorf("%q bir sabittir ('%s') ve değeri değiştirilemez", name, codes.Name(codes.A031))
			}
			cur.vals[i] = val
			return nil
		}
	}
	// Hiçbir kapsamda yoksa sessizce genel değişken oluşturmak yazım hatalarını
	// gizler (ör. fonksiyon içinde 'toplm = 1'); bu yüzden açık tanım istenir.
	return &UndefinedVarError{Name: name}
}

// UndefinedVarError is returned on assignment to an undefined variable; the VM recognizes
// this type and adds a "Bunu mu demek istediniz?" suggestion to the message.
type UndefinedVarError struct{ Name string }

func (e *UndefinedVarError) Error() string {
	return fmt.Sprintf("tanımsız değişken: '%s' (önce '%s' ile tanımlayın)", e.Name, codes.Name(codes.A030))
}

// Names returns every name visible from this scope and the outer scopes (for suggestions).
func (e *Environment) Names() []string {
	seen := map[string]bool{}
	var names []string
	for cur := e; cur != nil; cur = cur.outer {
		cur.EachLocal(func(k string, _ Value) {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		})
	}
	return names
}

// Outer returns the enclosing scope (nil for the global scope).
func (e *Environment) Outer() *Environment {
	return e.outer
}

// EachLocal passes only the names defined in this scope (in definition order) to fn.
func (e *Environment) EachLocal(fn func(name string, val Value)) {
	for i, ad := range e.kapsam.adlar {
		if i >= len(e.vals) {
			break
		}
		if val := e.vals[i]; val != nil {
			fn(ad, val)
		}
	}
}
