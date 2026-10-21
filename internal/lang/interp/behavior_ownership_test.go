package interp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Sahiplik kuralları (ownership.go, dil tanımı 15.44). Testler sahte bir kaynak değeriyle
// koşar: her kaynağın bir numarası vardır, bırakılınca numarası kayda yazılır ve bırakılmış
// kaynağın kullanılması hatadır. Her test iki şeyi denetler: hiçbir kaynak ulaşılabilirken
// bırakılmaz (program hatasız biter ve kaynakları okur) ve bırakılması gereken kaynak
// beklenen noktada bırakılmıştır (bırakılan() o andaki kaydı verir).

// fakeRes bir motorun cihaz belleği gibi kaynak tutan değeridir.
type fakeRes struct {
	id       int
	own      Ownership
	released bool
	w        *fakeWorld
}

var (
	_ Owned        = (*fakeRes)(nil)
	_ OperatorHook = (*fakeRes)(nil)
)

func (r *fakeRes) Type() ValueType       { return "Kaynak" }
func (r *fakeRes) Inspect() string       { return fmt.Sprintf("k%d", r.id) }
func (r *fakeRes) IsTruthy() bool        { return true }
func (r *fakeRes) Ownership() *Ownership { return &r.own }

func (r *fakeRes) Release() {
	if r.released {
		panic(fmt.Sprintf("k%d iki kez bırakıldı", r.id))
	}
	r.released = true
	r.w.log = append(r.w.log, r.id)
}

// Operator iki kaynaktan yeni bir kaynak üretir (işlenenler canlı olmalıdır).
func (r *fakeRes) Operator(op string, other Value, onRight bool) (Value, bool, error) {
	if err := r.w.check(r, other); err != nil {
		return nil, true, err
	}
	return r.w.fresh(), true, nil
}

// fakeWorld kaynakları üretir ve bırakılanların kaydını tutar.
type fakeWorld struct {
	next int
	log  []int
	kept []Value // sakla() ile Go tarafında tutulanlar
}

func (w *fakeWorld) fresh() *fakeRes {
	w.next++
	r := &fakeRes{id: w.next, w: w}
	MarkFresh(r)
	return r
}

// check değerlerin içindeki kaynakların (listelerde de) canlı olduğunu denetler.
func (w *fakeWorld) check(vals ...Value) error {
	for _, x := range vals {
		switch v := x.(type) {
		case *fakeRes:
			if v.released {
				return fmt.Errorf("bırakılmış k%d kullanıldı", v.id)
			}
		case *ListVal:
			if err := w.check(v.Elements...); err != nil {
				return err
			}
		}
	}
	return nil
}

// install sahte motorun yerleşiklerini VM'ye tanımlar:
//
//	yeni()       taze kaynak (saf)
//	oku(x)       kaynağın numarası; bırakılmışsa hata (saf)
//	birleşim(l)  listedeki kaynaklardan yeni kaynak (saf; tensörün birleştir'i gibi)
//	böl2(x)      x'ten iki taze kaynaklık sahip liste (tensörün .böl'ü gibi)
//	sakla(x)     x'i Go tarafında tutar (saf değil: argümanı kaçar)
//	bırakılan()  o ana kadar bırakılanların numaraları
func (w *fakeWorld) install(v *VM) {
	def := func(name string, pure bool, fn BuiltinFunc) {
		v.GlobalEnv.Define(name, &BuiltinVal{Name: name, Fn: fn, Pure: pure}, true)
	}
	def("yeni", true, func(_ *VM, _ []Value, _ map[string]Value) (Value, error) { return w.fresh(), nil })
	def("oku", true, func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
		if err := w.check(args...); err != nil {
			return nil, err
		}
		return NewInt(int64(args[0].(*fakeRes).id)), nil
	})
	def("birleşim", true, func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
		if err := w.check(args...); err != nil {
			return nil, err
		}
		return w.fresh(), nil
	})
	def("böl2", true, func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
		if err := w.check(args...); err != nil {
			return nil, err
		}
		l := &ListVal{Elements: []Value{w.fresh(), w.fresh()}}
		MarkFresh(l)
		return l, nil
	})
	def("sakla", false, func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
		w.kept = append(w.kept, args...)
		return Null, nil
	})
	def("bırakılan", false, func(_ *VM, _ []Value, _ map[string]Value) (Value, error) {
		el := make([]Value, len(w.log))
		for i, id := range w.log {
			el[i] = NewInt(int64(id))
		}
		return &ListVal{Elements: el}, nil
	})
}

// runOwned programı sahte motorla çalıştırır, yazdır() çıktısını ve dünyayı verir.
func runOwned(t *testing.T, code string) (string, *fakeWorld) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	var out strings.Builder
	v := New()
	v.Out = &out
	w := &fakeWorld{}
	w.install(v)
	if _, err := v.Run(prog); err != nil {
		t.Fatalf("program hata verdi (ulaşılabilir bir kaynak bırakıldı mı?): %v\nçıktı:\n%s", err, out.String())
	}
	v.WaitAsync()
	if err := w.check(w.kept...); err != nil {
		t.Fatalf("Go tarafında tutulan kaynak bırakıldı: %v", err)
	}
	return strings.TrimSpace(out.String()), w
}

func expectOut(t *testing.T, code, want string) *fakeWorld {
	t.Helper()
	got, w := runOwned(t, code)
	if got != strings.TrimSpace(want) {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", got, strings.TrimSpace(want))
	}
	return w
}

// Bir işlemin ara sonucu işlem biter bitmez bırakılır (Mojo'nun "ASAP" yok etmesi gibi).
func TestOwnershipOperandTemporaryReleasedAfterOperation(t *testing.T) {
	expectOut(t, `
değişken a = yeni()
değişken b = yeni()
değişken c = a + b + a
yazdır(bırakılan())
yazdır(oku(a), oku(b), oku(c))
`, `
[3]
1 2 4`)
}

// Değişkenin eski değeri, onu gösteren son değişken başka değer alınca bırakılır. Üst
// düzeyin değişkenleri geneldir (fonksiyonlar görür): onlara yazılan değer kaçar.
func TestOwnershipOverwrittenVariableReleased(t *testing.T) {
	expectOut(t, `
fonksiyon f() {
    değişken x = yeni()
    değişken y = x
    x = yeni()
    yazdır(bırakılan())
    y = yeni()
    yazdır(bırakılan())
    yazdır(oku(x), oku(y))
}
f()
değişken genel = yeni()
genel = yeni()
yazdır(bırakılan())
`, `
[]
[1]
2 3
[1, 3, 2]`)
}

// Fonksiyonun yerelleri dönüşte bırakılır, döndürdüğü çağırana geçer; argüman çağıranındır.
func TestOwnershipReturnMovesToCallerLocalsReleased(t *testing.T) {
	expectOut(t, `
fonksiyon f(p) {
    değişken t = yeni()
    değişken u = p + t
    sonuç u
}
değişken q = yeni()
değişken r = f(q)
yazdır(bırakılan())
yazdır(oku(q), oku(r))
`, `
[2]
1 3`)
}

// Örtük dönüş (gövdenin son değeri) ve iç içe çağrılar.
func TestOwnershipImplicitReturnAndNestedCalls(t *testing.T) {
	expectOut(t, `
fonksiyon iç(a) { a + yeni() }
fonksiyon dış(a) {
    değişken x = iç(a)
    iç(x)
}
değişken s = yeni()
değişken r = dış(s)
yazdır(oku(s), oku(r))
yazdır(bırakılan())
`, `
1 5
[2, 4, 3]`)
}

// Nesne alanına, Go'nun tuttuğu yere ve kapanışın gördüğü değişkene yazılan değer kaçar.
func TestOwnershipEscapesAreNeverReleased(t *testing.T) {
	expectOut(t, `
yapı K {
    alan
}
değişken k = K(yok)
k.alan = yeni()
sakla(yeni())
fonksiyon yapıcı() {
    değişken gizli = yeni()
    sonuç () => oku(gizli)
}
değişken g = yapıcı()
değişken s = {a: yeni()}
döngü i içinde 0..3 { değişken geçici = yeni() }
yazdır(oku(k.alan), g(), oku(s["a"]))
yazdır(bırakılan())
`, `
1 3 4
[5, 6, 7]`)
}

// Liste yazımı elemanlarının sahibidir: liste bırakılınca elemanları da bırakılır; saf bir
// yerleşiğin ara sonuç argümanı (liste) çağrıdan hemen sonra bırakılır.
func TestOwnershipListLiteralOwnsElements(t *testing.T) {
	expectOut(t, `
fonksiyon f() {
    değişken a = yeni()
    değişken l = [yeni(), a]
    değişken c = birleşim([a + a, yeni()])
    yazdır(bırakılan())
    l = yok
    yazdır(bırakılan())
    yazdır(oku(a), oku(c))
}
f()
`, `
[4, 3]
[4, 3, 2]
1 5`)
}

// Sonradan değişen ya da bir alana yazılan sahip liste elemanlarıyla birlikte kaçar.
func TestOwnershipMutatedOrStoredListEscapes(t *testing.T) {
	expectOut(t, `
yapı K {
    liste
}
değişken l = [yeni()]
l.ekle(yeni())
değişken m = [yeni(), yeni()]
değişken k = K(m)
l = yok
m = yok
yazdır(bırakılan())
yazdır(oku(k.liste[0]))
`, `
[]
3`)
}

// Bir işlemin döndürdüğü liste (.böl gibi) elemanlarının sahibidir; listeden alınıp bir
// değişkene bağlanan eleman yaşar, diğeri liste ile bırakılır.
func TestOwnershipEngineListOwnsElements(t *testing.T) {
	expectOut(t, `
değişken p = böl2(yeni())[0]
yazdır(bırakılan())
değişken a, b = böl2(p)
yazdır(oku(p), oku(a), oku(b))
yazdır(bırakılan())
`, `
[1, 3]
2 4 5
[1, 3]`)
}

// Döngü turunun değerleri bir sonraki tur başlarken, sonuncusu döngü bitince bırakılır;
// gezilen liste döngü sürerken korunur.
func TestOwnershipLoopTurnsReleaseTheirValues(t *testing.T) {
	expectOut(t, `
döngü x içinde [yeni(), yeni()] {
    değişken t = x + x
    yazdır(oku(x), oku(t), bırakılan())
}
yazdır(bırakılan())
`, `
1 3 []
2 4 [3]
[3, 2, 1, 4]`)
}

// Fonksiyonun içinden, döngünün ortasından dönen değer çağırana geçer.
func TestOwnershipReturnFromLoop(t *testing.T) {
	expectOut(t, `
fonksiyon bul() {
    döngü i içinde 0..5 {
        değişken t = yeni()
        eğer i == 2 { sonuç t }
    }
    sonuç yok
}
değişken r = bul()
yazdır(oku(r), bırakılan())
`, `
3 [1, 2]`)
}

// İfadenin içindeki 'eğer' bloğu deyim çalıştırır; dıştaki ifadenin ara sonucu o sırada
// bırakılmaz.
func TestOwnershipNestedIfInsideExpressionKeepsOuterTemporaries(t *testing.T) {
	expectOut(t, `
değişken a = yeni()
değişken r = (a + a) + eğer doğru {
    değişken w = yeni()
    oku(w)
    w + w
} değilse {
    yeni()
}
yazdır(oku(r))
`, `
5`)
}

// Liste üreteci sonuçlarının sahibidir.
func TestOwnershipComprehensionOwnsResults(t *testing.T) {
	expectOut(t, `
fonksiyon kat(x) { sonuç x + yeni() }
fonksiyon f() {
    değişken a = yeni()
    değişken m = [kat(a) döngü i içinde 0..2]
    yazdır(oku(m[0]), oku(m[1]))
    m = yok
    yazdır(bırakılan())
}
f()
`, `
3 5
[2, 4, 5, 3]`)
}

// Go'dan çağrılan fonksiyonun sonucu (liste metotlarının geri çağrıları) kaçar.
func TestOwnershipCallbackResultsEscape(t *testing.T) {
	expectOut(t, `
değişken a = yeni()
değişken l = [1, 2].haritala((i) => a + a)
yazdır(oku(l[0]), oku(l[1]))
yazdır(bırakılan())
`, `
2 3
[]`)
}

// 'ertele' ve 'eşzamanlı' ortamı yakalar: yakalanan değer onlar çalışana kadar yaşar.
func TestOwnershipDeferAndAsyncCaptureEscape(t *testing.T) {
	expectOut(t, `
fonksiyon f() {
    değişken t = yeni()
    ertele { yazdır(oku(t)) }
    yeni()
}
f()
değişken u = yeni()
eşzamanlı { yazdır(oku(u)) }
bekle_hepsi()
`, `
1
3`)
}

// Özyineleme: her çağrının kendi çerçevesi vardır.
func TestOwnershipRecursion(t *testing.T) {
	expectOut(t, `
fonksiyon zincir(n, x) {
    eğer n == 0 { sonuç x }
    değişken y = x + yeni()
    sonuç zincir(n - 1, y)
}
değişken r = zincir(3, yeni())
yazdır(oku(r), bırakılan().uzunluk)
`, `
7 6`)
}
