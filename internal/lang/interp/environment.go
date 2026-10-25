package interp

import (
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- ÇALIŞMA ORTAMI (ENVIRONMENT) ---

// scopeInfo bir kapsamın ad tablosudur: her ada bir yuva (slot) indeksi verir. Derleyici
// kapsamda tanımlanan adları derleme anında buraya ekler; derlenmiş kod değişkenlere adla
// değil indeksle erişir. Aynı kapsamdan açılan tüm ortamlar (ör. bir fonksiyonun her
// çağrısı) tabloyu paylaşır. Tablo yalnızca büyür: çalışma anında adla yapılan bir tanım
// (hata ayıklayıcı, takma adsız 'aktar') yeni bir ad ekleyebilir; böyle bir ad, o yuvası
// henüz olmayan diğer ortamlarda "tanımsız" sayılır.
type scopeInfo struct {
	nameList []string
	index    map[string]int
	// local: derleyici bu kapsamın ortamlarını hiçbir kapanışın göremeyeceğini bilir; yuvalara
	// yazılan sahipli değerler bağlanır, ortam kapanınca bağ kalkar (ownership.go). Diğer
	// ortamlara yazılan değer kaçar.
	local bool
}

// slot adın indeksini döndürür; ad tabloda yoksa ekler.
func (k *scopeInfo) slot(name string) int {
	if i, ok := k.index[name]; ok {
		return i
	}
	if k.index == nil {
		k.index = make(map[string]int)
	}
	i := len(k.nameList)
	k.nameList = append(k.nameList, name)
	k.index[name] = i
	return i
}

// envExtra ortamların nadiren gereken alanlarıdır (yer kazanmak için ayrı tutulur).
type envExtra struct {
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
	scope      *scopeInfo
	outer      *Environment
	extra      *envExtra
	IsFuncRoot bool
	// ic küçük ortamların yuvaları için satır içi depodur (ayrı bir ayırma gerekmez).
	ic [4]Value
}

// NewEnvironment opens a new environment with its own (empty) name table.
func NewEnvironment(outer *Environment) *Environment {
	return &Environment{
		outer:      outer,
		scope:      &scopeInfo{},
		IsFuncRoot: outer == nil,
	}
}

// newEnv derlenmiş bir kapsam için, tablodaki adlar kadar yuvayla ortam açar.
func newEnv(outer *Environment, k *scopeInfo) *Environment {
	e := &Environment{outer: outer, scope: k}
	if n := len(k.nameList); n <= len(e.ic) {
		e.vals = e.ic[:n:n]
	} else {
		e.vals = make([]Value, n)
	}
	return e
}

// clear, turlar arasında yeniden kullanılan bir döngü ortamını yeni açılmış gibi boşaltır
// (yuvalardaki bağlar kalkar).
func (e *Environment) clear() {
	e.closeEnv()
	clear(e.vals)
	if e.extra != nil {
		clear(e.extra.consts)
	}
}

func (e *Environment) isConstant(i int) bool {
	return e.extra != nil && i < len(e.extra.consts) && e.extra.consts[i]
}

// define i. yuvaya bir tanım yazar (Define'ın indeksli hâli).
func (e *Environment) define(i int, val Value, isConst bool) {
	for len(e.vals) <= i {
		e.vals = append(e.vals, nil)
	}
	// Yerleşik bir fonksiyonun adıyla (ör. 'toplam', 'uzunluk') değişken tanımlamak onu
	// gölgeler; yeni değişken sabit değildir. Kullanıcının 'sabit' tanımları korunur.
	if _, wasBuiltin := e.vals[i].(*BuiltinVal); wasBuiltin && !isConst && e.isConstant(i) {
		e.extra.consts[i] = false
	}
	e.store(i, val)
	if isConst {
		if e.extra == nil {
			e.extra = &envExtra{}
		}
		for len(e.extra.consts) <= i {
			e.extra.consts = append(e.extra.consts, false)
		}
		e.extra.consts[i] = true
	}
}

func (e *Environment) AddDefer(fn func() error) {
	target := e
	for target.outer != nil && !target.IsFuncRoot {
		target = target.outer
	}
	if target.extra == nil {
		target.extra = &envExtra{}
	}
	target.extra.defers = append(target.extra.defers, fn)
}

// RunDefers runs the recorded deferred work in reverse order. An error in one does not stop
// the rest; the errors are joined in run order and returned (see erteleHatasiEkle).
func (e *Environment) RunDefers() error {
	if e.extra == nil || len(e.extra.defers) == 0 {
		return nil
	}
	list := e.extra.defers
	e.extra.defers = nil
	var err error
	for i := len(list) - 1; i >= 0; i-- {
		err = addDeferError(err, list[i]())
	}
	return err
}

// deferError, bir hataya (fonksiyonun ya da önceki ertelenen işin hatası) sonradan çalışan
// ertelenen işlerin hatalarının eklenmiş hâlidir. İlk hata korunur (Unwrap onu verir).
type deferError struct {
	mainSrc error
	extras  []error
}

func (h *deferError) Error() string {
	pieces := make([]string, len(h.extras))
	for i, e := range h.extras {
		pieces[i] = e.Error()
	}
	return messages.Text(messages.H2519, h.mainSrc, codes.NameRef(codes.A081), strings.Join(pieces, "; "))
}

func (h *deferError) Unwrap() error { return h.mainSrc }

// addDeferError ertelenen işin hatasını (ek) mevcut hataya ekler: mevcut hata yoksa ek
// hatanın kendisi olur, varsa ilk hata korunur ve ek ona eklenir.
func addDeferError(current, extra error) error {
	switch {
	case extra == nil:
		return current
	case current == nil:
		return extra
	}
	if h, ok := current.(*deferError); ok {
		return &deferError{mainSrc: h.mainSrc, extras: append(slices.Clip(h.extras), extra)}
	}
	return &deferError{mainSrc: current, extras: []error{extra}}
}

// local adın bu ortamdaki değerini döndürür (tanımlı değilse nil).
func (e *Environment) local(name string) (int, Value) {
	if i, ok := e.scope.index[name]; ok && i < len(e.vals) {
		return i, e.vals[i]
	}
	return -1, nil
}

// localValue adın yalnızca bu ortamdaki değerini döndürür (yoksa nil).
func (e *Environment) localValue(name string) Value {
	_, val := e.local(name)
	return val
}

func (e *Environment) Get(name string) (Value, bool) {
	for cur := e; cur != nil; cur = cur.outer {
		if _, val := cur.local(name); val != nil {
			return val, true
		}
	}
	return nil, false
}

func (e *Environment) Define(name string, val Value, isConst bool) {
	e.define(e.scope.slot(name), val, isConst)
}

func (e *Environment) Assign(name string, val Value) error {
	for cur := e; cur != nil; cur = cur.outer {
		if i, old := cur.local(name); old != nil {
			if cur.isConstant(i) {
				return messages.New(messages.H2447, name, codes.NameRef(codes.A031))
			}
			cur.store(i, val)
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

func (e *UndefinedVarError) Error() string { return e.Unwrap().Error() }

// MessageText is the error's text in lang (messages.Localized; see ModuleError.MessageText).
func (e *UndefinedVarError) MessageText(lang string) string {
	return messages.ErrorIn(e.Unwrap(), lang)
}

// Unwrap returns the coded message, so messages.Has finds it in the chain.
func (e *UndefinedVarError) Unwrap() error {
	return messages.New(messages.H2448, e.Name, codes.NameRef(codes.A030))
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
	for i, name := range e.scope.nameList {
		if i >= len(e.vals) {
			break
		}
		if val := e.vals[i]; val != nil {
			fn(name, val)
		}
	}
}
