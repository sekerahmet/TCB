package main

import (
	"bytes"
	"strings"
	"testing"
)

const hooksSource = `package p

// OperatorHook ve TaskOperatorHook: yorumlayıcı önce ikincisini dener (TensorValue.Operator olayı).
type OperatorHook interface{ Operator(a int) int }
type TaskOperatorHook interface{ TaskOperator(a int) int }

type T struct{}

func (T) Operator(a int) int     { return a }
func (T) TaskOperator(a int) int { return a }

var (
	_ OperatorHook     = T{}
	_ TaskOperatorHook = T{}
)

func apply(v any) int {
	if k, ok := v.(TaskOperatorHook); ok {
		return k.TaskOperator(1)
	}
	if k, ok := v.(OperatorHook); ok {
		return k.Operator(1)
	}
	return 0
}

// Run apply'ı çağırır.
func Run() int { return apply(T{}) }

// U yalnızca Pinger denetimiyle tutulur; Pinger.Ping hiçbir yerde çağrılmaz.
type U struct{}

func (U) Ping() {}

type Pinger interface{ Ping() }

var _ Pinger = U{}

// V'nin metodunu kimse çağırmaz.
type V struct{}

func (V) Orphan() {}

// W'nin metodu yalnızca testten çağrılır.
type W struct{}

func (W) OnlyTest() {}

// X canlı: doğrudan çağrılır. Y canlı: arayüz üzerinden çağrılır.
type X struct{}

func (X) Live() {}

type Runner interface{ Go() }
type Y struct{}

func (Y) Go() {}

var _ Runner = Y{}

// Z yalnızca kapalı arayüz işaretçisidir: çağrılmaması beklenir, listelenmez.
type sealed interface{ mark() }
type Z struct{}

func (Z) mark() {}

var _ sealed = Z{}

func Use(r Runner) { X{}.Live(); r.Go() }
`

// Arayüz denetimiyle tutulan ölü yol: TensorValue.Operator olayının küçük modeli. Arayüz üzerinden
// çağrılan, doğrudan çağrılan ve dış sözleşme adlı metotlar listelenmez.
func TestDeadMethodsFindsInterfaceKeptMethods(t *testing.T) {
	root := auditModule(t, map[string]string{
		"p.go":      hooksSource,
		"p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestW(t *testing.T) { W{}.OnlyTest() }\n",
	})
	res := auditJSON(t, "dead-methods", root)
	got := map[string]string{}
	for _, v := range res["methods"].([]any) {
		m := v.(map[string]any)
		got[m["method"].(string)] = m["kind"].(string)
	}
	want := map[string]string{
		"audit.T.Operator": "shadowed", "audit.U.Ping": "assertion-only", "audit.V.Orphan": "unreferenced", "audit.W.OnlyTest": "tests-only",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	for _, live := range []string{"audit.T.TaskOperator", "audit.X.Live", "audit.Y.Go", "audit.Z.mark"} {
		if _, bad := got[live]; bad {
			t.Errorf("%s is reached and must not be listed", live)
		}
	}
	var out, errb bytes.Buffer
	run([]string{"dead-methods", "--root", root}, &out, &errb)
	if !strings.Contains(out.String(), "shadowed (1)") || !strings.Contains(out.String(), "returns first") {
		t.Errorf("text output names the shadowing check:\n%s", out.String())
	}
}

// deadKinds dead-methods'un JSON çıktısını yöntem -> sınıf eşlemine çevirir.
func deadKinds(t *testing.T, root string) map[string]string {
	t.Helper()
	got := map[string]string{}
	list, _ := auditJSON(t, "dead-methods", root)["methods"].([]any) // aday yoksa null
	for _, v := range list {
		m := v.(map[string]any)
		got[m["method"].(string)] = m["kind"].(string)
	}
	return got
}

// conversionsSource her dönüşüm yeri için bir tiptir (K4): her tip math/rand'in dış arayüzü rand.Source'u sağlar
// (Int63, Seed) ve bir de arayüzde olmayan dışa açık Extra metodu taşır. any'ye, error'a ve boş tanımlayıcıya verilen
// tipler dönüşüm değildir.
const conversionsSource = `package p

import "math/rand"

type cArg struct{}
type cVar struct{}
type cAssign struct{}
type cReturn struct{}
type cField struct{}
type cSlice struct{}
type cMapKey struct{}
type cMapValue struct{}
type cConv struct{}
type cChan struct{}
type cAppend struct{}
type cAny struct{}
type cBlank struct{}
type cErr struct{}

func (cArg) Int63() int64 { return 0 }
func (cArg) Seed(int64)   {}
func (cArg) Extra()       {}
func (cVar) Int63() int64 { return 0 }
func (cVar) Seed(int64)   {}
func (cVar) Extra()       {}
func (cAssign) Int63() int64 { return 0 }
func (cAssign) Seed(int64)   {}
func (cAssign) Extra()       {}
func (cReturn) Int63() int64 { return 0 }
func (cReturn) Seed(int64)   {}
func (cReturn) Extra()       {}
func (cField) Int63() int64 { return 0 }
func (cField) Seed(int64)   {}
func (cField) Extra()       {}
func (cSlice) Int63() int64 { return 0 }
func (cSlice) Seed(int64)   {}
func (cSlice) Extra()       {}
func (cMapKey) Int63() int64 { return 0 }
func (cMapKey) Seed(int64)   {}
func (cMapKey) Extra()       {}
func (cMapValue) Int63() int64 { return 0 }
func (cMapValue) Seed(int64)   {}
func (cMapValue) Extra()       {}
func (*cConv) Int63() int64 { return 0 }
func (*cConv) Seed(int64)   {}
func (*cConv) Extra()       {}
func (cChan) Int63() int64 { return 0 }
func (cChan) Seed(int64)   {}
func (cChan) Extra()       {}
func (cAppend) Int63() int64 { return 0 }
func (cAppend) Seed(int64)   {}
func (cAppend) Extra()       {}
func (cAny) Extra()          {}
func (cBlank) Int63() int64  { return 0 }
func (cBlank) Seed(int64)    {}
func (cBlank) Extra()        {}
func (cErr) Error() string   { return "" }
func (cErr) Extra()          {}

type holder struct{ src rand.Source }

var _ rand.Source = cBlank{}

func Use(c chan rand.Source, l []rand.Source) (rand.Source, holder, []rand.Source, map[rand.Source]int, map[string]rand.Source, any, error) {
	_ = rand.New(cArg{})
	var v rand.Source = cVar{}
	var w rand.Source
	w = cAssign{}
	_, _ = v, w
	_ = rand.Source(&cConv{})
	c <- cChan{}
	l = append(l, cAppend{})
	return cReturn{}, holder{src: cField{}}, []rand.Source{cSlice{}}, map[rand.Source]int{cMapKey{}: 1}, map[string]rand.Source{"a": cMapValue{}}, cAny{}, cErr{}
}
`

// Kural (K4): modülün somut tipinin değeri modül dışındaki adlı, boş olmayan bir arayüze dönüşüyorsa (çağrı argümanı,
// adlı var ve atama, return, alan, dilim öğesi, eşlem anahtarı ve değeri, açık dönüşüm, kanal gönderimi, append) o tipin
// dışa açık bütün metotları canlıdır, arayüzde olmayanlar da (math/rand Uint64'e Source64 iddiasıyla ulaşır). any'ye,
// error'a ve boş tanımlayıcıya (`var _ rand.Source = T{}`, derleme denetimi) verilen tipin metodu aday kalır.
func TestDeadMethodsCountsAnExternalInterfaceConversion(t *testing.T) {
	got := deadKinds(t, auditModule(t, map[string]string{"p.go": conversionsSource}))
	for _, ty := range []string{"cArg", "cVar", "cAssign", "cReturn", "cField", "cSlice", "cMapKey", "cMapValue", "cConv", "cChan", "cAppend"} {
		for _, m := range []string{"Int63", "Seed", "Extra"} {
			if k, listed := got["audit."+ty+"."+m]; listed {
				t.Errorf("audit.%s.%s is given to rand.Source and listed as %q", ty, m, k)
			}
		}
	}
	for _, m := range []string{"audit.cAny.Extra", "audit.cErr.Extra", "audit.cBlank.Int63", "audit.cBlank.Seed", "audit.cBlank.Extra"} {
		if got[m] != "unreferenced" {
			t.Errorf("%s: %q, want unreferenced (all: %v)", m, got[m], got)
		}
	}
}

// shadowSource gölgenin kendi deyiminde kaldığı iki durumdur (K4): aynı case satırındaki iki arayüz birbirini gölgelemez;
// bir işlevde gölgelenen metot başka bir işlevde arayüzden düz çağrılıyorsa canlıdır.
const shadowSource = `package p

type AHook interface{ A() int }
type BHook interface{ B() int }
type PHook interface{ P() int }
type QHook interface{ Q() int }

type S struct{}

func (S) A() int { return 1 }
func (S) B() int { return 2 }

type R struct{}

func (R) P() int { return 1 }
func (R) Q() int { return 2 }

var (
	_ AHook = S{}
	_ BHook = S{}
	_ PHook = R{}
	_ QHook = R{}
)

func pick(v any) int {
	switch k := v.(type) {
	case AHook, BHook:
		if b, ok := k.(BHook); ok {
			return b.B()
		}
	}
	return 0
}

func apply(v any) int {
	if k, ok := v.(PHook); ok {
		return k.P()
	}
	if k, ok := v.(QHook); ok {
		return k.Q()
	}
	return 0
}

func direct(q QHook) int { return q.Q() }

func Run() int { return pick(S{}) + apply(R{}) + direct(R{}) + S{}.A() }
`

func TestDeadMethodsShadowStaysInItsStatement(t *testing.T) {
	got := deadKinds(t, auditModule(t, map[string]string{"p.go": shadowSource}))
	for _, live := range []string{"audit.S.B", "audit.R.Q"} {
		if k, listed := got[live]; listed {
			t.Errorf("%s is listed as %q: a check in the same case, or a plain call elsewhere, keeps it live (all: %v)", live, k, got)
		}
	}
	got = deadKinds(t, auditModule(t, map[string]string{"p.go": hooksSource}))
	if got["audit.T.Operator"] != "shadowed" {
		t.Errorf("audit.T.Operator: %q, want shadowed (its only call is in the shadowed check)", got["audit.T.Operator"])
	}
}

// markersSource işaret arayüzünün küçük modelidir (interp.AssignCopier, ml.TensorValue.CopiesAssigned): apply bazı
// arayüzlerin metodunu çağırmadan yalnız tipin onları sağlayıp sağlamadığına bakar.
const markersSource = `package p

type Copier interface{ Copies() }    // değeri atılan iddia: işaret
type Kind interface{ KindMark() }    // değişkensiz tip seçimi: işaret
type Unbound interface{ FreeMark() } // değişkeni o kolda kullanılmayan tip seçimi: işaret
type Filler interface{ Fill() }      // değeri bağlanan iddia: işaret değil
type Bound interface{ BoundMark() }  // değişkeni kolda kullanılan tip seçimi: işaret değil
type Tested interface{ TestMark() }  // yalnız testteki iddia: işaret değil

type A struct{}
type B struct{}
type C struct{}
type D struct{}
type E struct{}
type F struct{}

func (A) Copies()    {}
func (B) KindMark()  {}
func (C) FreeMark()  {}
func (D) Fill()      {}
func (E) BoundMark() {}
func (F) TestMark()  {}

var (
	_ Copier  = A{}
	_ Kind    = B{}
	_ Unbound = C{}
	_ Filler  = D{}
	_ Bound   = E{}
	_ Tested  = F{}
)

func apply(v any) int {
	if _, copies := v.(Copier); !copies {
		return 1
	}
	if f, ok := v.(Filler); ok {
		_ = f
		return 2
	}
	switch v.(type) {
	case Kind:
		return 3
	}
	switch w := v.(type) {
	case Unbound:
		return 4
	case Bound:
		_ = w
		return 5
	}
	return 0
}

// Run apply'ı çağırır.
func Run() int { return apply(A{}) + apply(B{}) + apply(C{}) + apply(D{}) + apply(E{}) + apply(F{}) }
`

// Kural (dilim 5, Karar 3): değeri atılan tip iddiasında ya da bağladığı değişkeni o kolda kullanılmayan tip seçimi
// kolunda geçen arayüz bir işarettir; onu sağlayan metot hiç çağrılmasa da assertion-only listelenmez. Değeri
// bağlanan iddia ve testteki iddia işaret değildir: onların metodu aday kalır.
func TestDeadMethodsKeepsMarkerInterfaceMethods(t *testing.T) {
	root := auditModule(t, map[string]string{
		"p.go":      markersSource,
		"p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif _, ok := any(F{}).(Tested); !ok {\n\t\tt.Fatal()\n\t}\n}\n",
	})
	got := deadKinds(t, root)
	for _, live := range []string{"audit.A.Copies", "audit.B.KindMark", "audit.C.FreeMark"} {
		if k, bad := got[live]; bad {
			t.Errorf("%s satisfies a marker interface and must not be listed (listed as %s)", live, k)
		}
	}
	for _, dead := range []string{"audit.D.Fill", "audit.E.BoundMark", "audit.F.TestMark"} {
		if got[dead] != "assertion-only" {
			t.Errorf("%s: %q, want assertion-only (all: %v)", dead, got[dead], got)
		}
	}
}
