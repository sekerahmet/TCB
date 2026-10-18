package vm

import "fmt"

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
	defers []func()
	consts []bool // yuva başına 'sabit' bayrağı
}

// Environment bir kapsamdaki değişkenleri tutar. Değerler kapsamın ad tablosundaki
// indekslere karşılık gelen yuvalardadır; nil yuva "bu ortamda (henüz) tanımlı değil"
// demektir ve arama dış ortamlarla sürer. Adla erişim (Get, Define, Assign, Names,
// EachLocal) hata ayıklayıcı, öneriler ve modüller için korunur.
//
// Kilidi yoktur: TR kodu yalnızca yorumlayıcı kilidini (GIL, bkz. vmShared.gil) tutan görev
// tarafından çalıştırılır ve hata ayıklayıcının istekleri de duraklatılmış görevin kendi
// goroutine'inde işlenir. Bu yüzden bir ortama (ve ad tablosuna) aynı anda iki goroutine
// erişmez. GIL'i tutmayan kod (Blocking içindeki işlemler, sunucu goroutine'leri) ortamlara
// dokunmamalıdır; dokunması gerekiyorsa RunTask kullanılır.
type Environment struct {
	vals       []Value
	kapsam     *kapsamBilgisi
	outer      *Environment
	ek         *ortamEk
	IsFuncRoot bool
	// ic küçük ortamların yuvaları için satır içi depodur (ayrı bir ayırma gerekmez).
	ic [4]Value
}

// NewEnvironment kendi (boş) ad tablosuyla yeni bir ortam açar.
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

func (e *Environment) AddDefer(fn func()) {
	target := e
	for target.outer != nil && !target.IsFuncRoot {
		target = target.outer
	}
	if target.ek == nil {
		target.ek = &ortamEk{}
	}
	target.ek.defers = append(target.ek.defers, fn)
}

func (e *Environment) RunDefers() {
	if e.ek == nil || len(e.ek.defers) == 0 {
		return
	}
	list := e.ek.defers
	e.ek.defers = nil
	for i := len(list) - 1; i >= 0; i-- {
		list[i]()
	}
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
				return fmt.Errorf("%q bir sabittir ('sabit') ve değeri değiştirilemez", name)
			}
			cur.vals[i] = val
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
		cur.EachLocal(func(k string, _ Value) {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		})
	}
	return names
}

// Outer bir dıştaki kapsamı döndürür (genel kapsamda nil).
func (e *Environment) Outer() *Environment {
	return e.outer
}

// EachLocal yalnızca bu kapsamda tanımlı adları (tanım sırasıyla) fn'e verir.
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
