package interp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Sahiplik kuralları (ownership.go, dil tanımı 15.43). Testler sahte bir kaynak değeriyle
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

// check değerlerin içindeki kaynakların (listelerde ve demetlerde de) canlı olduğunu denetler.
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
		case *TupleVal:
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
//	birleşim(l)  listedeki kaynaklardan yeni kaynak (saf; tensörün bitiştir'i gibi)
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
	p := parser.New(bodyLexer(code))
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
TR_var a = yeni()
TR_var b = yeni()
TR_var c = a + b + a
TR_print(bırakılan())
TR_print(oku(a), oku(b), oku(c))
`, `
[3]
1 2 4`)
}

// Değişkenin eski değeri, onu gösteren son değişken başka değer alınca bırakılır. Üst
// düzeyin değişkenleri geneldir (fonksiyonlar görür): onlara yazılan değer kaçar.
func TestOwnershipOverwrittenVariableReleased(t *testing.T) {
	expectOut(t, `
TR_func f() {
    TR_var x = yeni()
    TR_var y = x
    x = yeni()
    TR_print(bırakılan())
    y = yeni()
    TR_print(bırakılan())
    TR_print(oku(x), oku(y))
}
f()
TR_var genel = yeni()
genel = yeni()
TR_print(bırakılan())
`, `
[]
[1]
2 3
[1, 3, 2]`)
}

// Fonksiyonun yerelleri dönüşte bırakılır, döndürdüğü çağırana geçer; argüman çağıranındır.
func TestOwnershipReturnMovesToCallerLocalsReleased(t *testing.T) {
	expectOut(t, `
TR_func f(p) {
    TR_var t = yeni()
    TR_var u = p + t
    TR_return u
}
TR_var q = yeni()
TR_var r = f(q)
TR_print(bırakılan())
TR_print(oku(q), oku(r))
`, `
[2]
1 3`)
}

// Dönüş değeri (çağırana geçer) ve iç içe çağrılar.
func TestOwnershipReturnAndNestedCalls(t *testing.T) {
	expectOut(t, `
TR_func iç(a) { TR_return a + yeni() }
TR_func dış(a) {
    TR_var x = iç(a)
    TR_return iç(x)
}
TR_var s = yeni()
TR_var r = dış(s)
TR_print(oku(s), oku(r))
TR_print(bırakılan())
`, `
1 5
[2, 4, 3]`)
}

// Nesne alanına, Go'nun tuttuğu yere ve kapanışın gördüğü değişkene yazılan değer kaçar.
func TestOwnershipEscapesAreNeverReleased(t *testing.T) {
	expectOut(t, `
TR_struct K {
    alan
}
TR_var k = K(TR_none)
k.alan = yeni()
sakla(yeni())
TR_func yapıcı() -> () -> TR_Int {
    TR_var gizli = yeni()
    TR_return () => oku(gizli)
}
TR_var g = yapıcı()
TR_var s = {"a": yeni()}
TR_for i TR_in 0..3 { TR_var geçici = yeni() }
TR_print(oku(k.alan), g(), oku(s["a"]))
TR_print(bırakılan())
`, `
1 3 4
[5, 6, 7]`)
}

// Liste yazımı elemanlarının sahibidir: liste bırakılınca elemanları da bırakılır; saf bir
// yerleşiğin ara sonuç argümanı (liste) çağrıdan hemen sonra bırakılır.
func TestOwnershipListLiteralOwnsElements(t *testing.T) {
	expectOut(t, `
TR_func f() {
    TR_var a = yeni()
    TR_var l = [yeni(), a]
    TR_var c = birleşim([a + a, yeni()])
    TR_print(bırakılan())
    l = TR_none
    TR_print(bırakılan())
    TR_print(oku(a), oku(c))
}
f()
`, `
[4, 3]
[4, 3, 2]
1 5`)
}

// Liste yazımı (15.43) demet yazımı gibi içindekilerin sahibidir: birleşim([a, b])'nin [a, b]'si
// çağrıdan sonra bırakılır. Listeye sonradan yazılan değer kaçar, liste ve öbür öğeleri kaçmaz:
// liste bırakılınca kendi öğesi (4) bırakılır, eklenen (5) bırakılmaz.
func TestOwnershipListLiteralOwnsElementsLikeATuple(t *testing.T) {
	expectOut(t, `
TR_func f() {
    TR_var a = yeni()
    TR_var b = yeni()
    TR_var c = birleşim([a, b])
    TR_var l = [yeni(), a]
    TR_print(bırakılan())
    l.TR_append(yeni())
    l = TR_none
    TR_print(bırakılan())
    TR_print(oku(a), oku(b), oku(c))
}
f()
TR_print(bırakılan())
`, `
[]
[4]
1 2 3
[4, 3, 2, 1]`)
}

// İndeksle atamada yazılan değer kaçar, yerine yazılan eski öğe (2) listeden düşünce bırakılır.
func TestOwnershipIndexAssignmentEscapesOnlyTheWrittenValue(t *testing.T) {
	expectOut(t, `
TR_func f() {
    TR_var a = yeni()
    TR_var l = [yeni(), a]
    l[0] = yeni()
    TR_print(bırakılan())
    l = TR_none
    TR_print(bırakılan())
    TR_print(oku(a))
}
f()
TR_print(bırakılan())
`, `
[2]
[2]
1
[2, 1]`)
}

// (a) Demet yazımı (3.6a, 15.43) öğelerinin sahibidir: demet bırakılınca öğeleri de bırakılır; saf
// bir yerleşiğin ara sonuç argümanı (demet) çağrıdan hemen sonra bırakılır.
func TestOwnershipTupleLiteralOwnsElements(t *testing.T) {
	expectOut(t, `
TR_func f() {
    TR_var a = yeni()
    TR_var t = (yeni(), a)
    TR_var c = birleşim((a + a, yeni()))
    TR_print(bırakılan())
    t = TR_none
    TR_print(bırakılan())
    TR_print(oku(a), oku(c))
}
f()
`, `
[4, 3]
[4, 3, 2]
1 5`)
}

// (b) Döndürülen demetin içindeki taze değerler çağırana taşınır: çerçeve kapanınca bırakılmaz.
func TestOwnershipReturnedTupleMovesItsElementsToTheCaller(t *testing.T) {
	expectOut(t, `
TR_func f(p) {
    TR_var t = yeni()
    TR_var u = p + t
    TR_return u, yeni()
}
TR_func g() {
    TR_var q = yeni()
    TR_var r = f(q)
    TR_print(bırakılan())
    TR_print(oku(q), oku(r.0), oku(r.1), oku(f(q).1))
    r = TR_none
    TR_print(bırakılan())
}
g()
`, `
[2]
1 3 4 7
[2, 5, 7, 6, 4, 3]`)
}

// (c) Listeye eklenen, sözlüğe, en dış değişkene ya da Go'ya konan demet öğeleriyle kaçar.
func TestOwnershipStoredTupleEscapes(t *testing.T) {
	expectOut(t, `
TR_func iki() { TR_return yeni(), yeni() }
TR_var dış = iki()
TR_func g() {
    TR_var l = [1]
    l.TR_append(iki())
    TR_var d = {"a": iki()}
    sakla(iki())
    TR_return l
}
TR_var l = g()
TR_print(bırakılan())
TR_print(oku(dış.0), oku(l[1].1))
`, `
[]
1 4`)
}

// (d) Üzerine yazılan demet değişkeni bırakılır; içindeki değer bir kez, son bağı da kalkınca.
func TestOwnershipOverwrittenTupleReleased(t *testing.T) {
	expectOut(t, `
TR_func f() {
    TR_var a = yeni()
    TR_var t = (a, yeni())
    t = (yeni(), a)
    TR_print(bırakılan())
    TR_print(oku(a), oku(t.0))
}
f()
TR_print(bırakılan())
`, `
[2]
1 3`+"\n[2, 1, 3]")
}

// (e) Açılan demetin öğeleri demetle paylaşılır: adlara bağlı kalır, sonra bir kez bırakılır.
func TestOwnershipUnpackedTupleSharesElements(t *testing.T) {
	expectOut(t, `
TR_func iki() { TR_return yeni(), yeni() }
TR_func f() {
    TR_var t = (yeni(), yeni())
    TR_var a, b = t
    t = TR_none
    TR_var x, y = iki()
    TR_print(bırakılan())
    TR_print(oku(a), oku(b), oku(x), oku(y))
}
f()
TR_print(bırakılan())
`, `
[]
1 2 3 4`+"\n[4, 3, 2, 1]")
}

// Sonradan değişen ya da bir alana yazılan sahip liste elemanlarıyla birlikte kaçar.
func TestOwnershipMutatedOrStoredListEscapes(t *testing.T) {
	expectOut(t, `
TR_struct K {
    liste
}
TR_var l = [yeni()]
l.TR_append(yeni())
TR_var m = [yeni(), yeni()]
TR_var k = K(m)
l = TR_none
m = TR_none
TR_print(bırakılan())
TR_print(oku(k.liste[0]))
`, `
[]
3`)
}

// Bir işlemin döndürdüğü liste (.böl gibi) elemanlarının sahibidir; listeden alınıp bir
// değişkene bağlanan eleman yaşar, diğeri liste ile bırakılır.
func TestOwnershipEngineListOwnsElements(t *testing.T) {
	expectOut(t, `
TR_var p = böl2(yeni())[0]
TR_print(bırakılan())
TR_var a, b = böl2(p)
TR_print(oku(p), oku(a), oku(b))
TR_print(bırakılan())
`, `
[1, 3]
2 4 5
[1, 3]`)
}

// Döngü turunun değerleri bir sonraki tur başlarken, sonuncusu döngü bitince bırakılır;
// gezilen liste döngü sürerken korunur.
func TestOwnershipLoopTurnsReleaseTheirValues(t *testing.T) {
	expectOut(t, `
TR_for x TR_in [yeni(), yeni()] {
    TR_var t = x + x
    TR_print(oku(x), oku(t), bırakılan())
}
TR_print(bırakılan())
`, `
1 3 []
2 4 [3]
[3, 2, 1, 4]`)
}

// Fonksiyonun içinden, döngünün ortasından dönen değer çağırana geçer.
func TestOwnershipReturnFromLoop(t *testing.T) {
	expectOut(t, `
TR_func bul() {
    TR_for i TR_in 0..5 {
        TR_var t = yeni()
        TR_if i == 2 { TR_return t }
    }
    TR_return TR_none
}
TR_var r = bul()
TR_print(oku(r), bırakılan())
`, `
3 [1, 2]`)
}

// İfadenin içindeki 'eğer' bloğu deyim çalıştırır; dıştaki ifadenin ara sonucu o sırada
// bırakılmaz.
func TestOwnershipNestedIfInsideExpressionKeepsOuterTemporaries(t *testing.T) {
	expectOut(t, `
TR_var a = yeni()
TR_var r = (a + a) + TR_if TR_true {
    TR_var w = yeni()
    oku(w)
    w + w
} TR_else {
    yeni()
}
TR_print(oku(r))
`, `
5`)
}

// Liste üreteci sonuçlarının sahibidir.
func TestOwnershipComprehensionOwnsResults(t *testing.T) {
	expectOut(t, `
TR_func kat(x) { TR_return x + yeni() }
TR_func f() {
    TR_var a = yeni()
    TR_var m = [kat(a) TR_for i TR_in 0..2]
    TR_print(oku(m[0]), oku(m[1]))
    m = TR_none
    TR_print(bırakılan())
}
f()
`, `
3 5
[2, 4, 5, 3]`)
}

// Go'dan çağrılan fonksiyonun sonucu (liste metotlarının geri çağrıları) kaçar.
func TestOwnershipCallbackResultsEscape(t *testing.T) {
	expectOut(t, `
TR_var a = yeni()
TR_var l = [1, 2].TR_map((i) => a + a)
TR_print(oku(l[0]), oku(l[1]))
TR_print(bırakılan())
`, `
2 3
[]`)
}

// 'ertele' ve 'eşzamanlı' ortamı yakalar: yakalanan değer onlar çalışana kadar yaşar.
func TestOwnershipDeferAndAsyncCaptureEscape(t *testing.T) {
	expectOut(t, `
TR_func f() {
    TR_var t = yeni()
    TR_defer { TR_print(oku(t)) }
    yeni()
}
f()
TR_var u = yeni()
TR_async { TR_print(oku(u)) }
TR_wait_all()
`, `
1
3`)
}

// Özyineleme: her çağrının kendi çerçevesi vardır.
func TestOwnershipRecursion(t *testing.T) {
	expectOut(t, `
TR_func zincir(n: TR_Int, x) {
    TR_if n == 0 { TR_return x }
    TR_var y = x + yeni()
    TR_return zincir(n - 1, y)
}
TR_var r = zincir(3, yeni())
TR_print(oku(r), bırakılan().TR_length)
`, `
7 6`)
}

// Kural: varsayılan parametre ifadesindeki kapanış fonksiyonun ortamını yakalar; ortam havuzlanmaz
// ve yakalanan kaynak çağrı dönünce bırakılmaz. Neden: yoksa kaynak erken bırakılır ve ortam
// havuza sıfırlanarak verilir, kapanışı çağırmak 'ad yok' hatası verir.
func TestDefaultParamClosureKeepsFunctionEnvironment(t *testing.T) {
	expectOut(t, `
TR_func kur(x = yeni(), g: () -> TR_Int = () => oku(x)) -> () -> TR_Int {
    TR_return g
}
TR_var f = kur()
TR_print(bırakılan())
TR_print(f())
`, `
[]
1`)
}

// Kural: hızlı çağrı yolunda sonraki bir argüman hata verirse önceki argümanların bağı çözülür
// ve kaynakları kapsam sonunda bırakılır. Neden: hata yakalansa bile bağ sayacı kalırsa
// kaynak hiç bırakılmaz.
func TestFailingCallArgumentReleasesEarlierArguments(t *testing.T) {
	expectOut(t, `
TR_func hedef(a, b) { TR_return a }
TR_func patla() { TR_throw "hata" }
TR_func sına() {
    TR_var x = yeni()
    TR_try {
        hedef(x, patla())
    } TR_catch {
    }
}
sına()
TR_print(bırakılan())
`, `
[1]`)
}
