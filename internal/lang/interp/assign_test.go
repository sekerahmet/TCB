package interp

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// assignInPlace sade değerde (plain) kancaya bakmadan döner; bu, hiçbir sade tipin
// InPlaceOperator olmamasına dayanır. Paket dışındaki bir tip interp.Value'yu kullanmak için
// interp'i aktarır, öyleyse plain'in tip listesine giremez (döngülü aktarma); paketin içinde
// InPlaceOp yöntemi tanımlı değildir. Bugün kancayı yalnız tensör (ml.TensorValue) uygular.
func TestNoPlainValueIsAnInPlaceOperator(t *testing.T) {
	for _, val := range []Value{NewInt(1), NewInt(5000), &FloatVal{Val: 1.5}, True, &StringVal{Val: "a"}} {
		if !plain(val) {
			t.Errorf("%T sade sayılmalı", val)
		}
		if _, ok := val.(InPlaceOperator); ok {
			t.Errorf("%T sade bir değer, InPlaceOperator olmamalı", val)
		}
	}
	dir := filepath.Join(reporoot.Root(), "internal", "lang", "interp")
	entries, err := reporoot.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Name.Name == "InPlaceOp" {
				t.Errorf("%s: interp'te InPlaceOp yöntemi; sade bir tipse assignInPlace'in erken dönüşü onu atlar", fset.Position(fn.Pos()))
			}
		}
	}
}

// taskAssignable SetIndexAt'i de uygulayan dış değer: her yazmanın görevini ve satırını kaydeder.
type taskAssignable struct {
	assignableValue
	lines []int
	vms   []*VM
}

func (a *taskAssignable) SetIndexAt(m *VM, line int, i, d Value) error {
	a.lines, a.vms = append(a.lines, line), append(a.vms, m)
	return a.SetIndex(i, d)
}

// Kural (A12, T1 dilim 2a; 15.45): indeksli atamada değer TaskIndexAssignable'ı uyguluyorsa VM onu
// görev ve atamanın satırıyla çağırır, SetIndex'in çağrıldığı her yerde (düz atama ve kancasız
// bileşik atamanın yazması); uygulamayan değerde SetIndex yolu aynen sürer; kancanın hatası satır
// öneklidir.
func TestIndexAssignTaskHookGetsTheLine(t *testing.T) {
	a := &taskAssignable{assignableValue: assignableValue{cell: map[int64]Value{}}}
	plain := &assignableValue{cell: map[int64]Value{}}
	vmx := New()
	vmx.GlobalEnv.Define("a", a, false)
	vmx.GlobalEnv.Define("p", plain, false)
	prog := mustParseProgram(t, "a[0] = 5\np[0] = 7\na[1] += 2\na[-1] = 3")
	_, err := vmx.Run(prog)
	if !atLine(err, 4, "negatif indeks") {
		t.Fatalf("kancanın hatası satır önekli olmalı: %v", err)
	}
	if fmt.Sprint(a.lines) != "[1 3 4]" || a.vms[0] == nil {
		t.Fatalf("kancanın satırları %v (görev %v), beklenen [1 3 4]", a.lines, a.vms)
	}
	if got := a.PlainValue().Inspect(); got != "[5, 2]" {
		t.Fatalf("kancalı atamalar: %v", got)
	}
	if got := plain.cell[0].Inspect(); got != "7" {
		t.Fatalf("kancasız değerde SetIndex yolu: %v", got)
	}
}

const unopenedPre = "TR_struct N {\n    x: TR_Int = 0\n}\nTR_struct K {\n    n: N? = TR_none\n    l: TR_List[TR_Int]? = TR_none\n}\n" +
	"TR_var d: TR_Dict[TR_String, TR_List[TR_Int]] = {\"a\": [1, 2]}\nTR_var k = K(N(4), [7, 8])\nTR_var z = 0\n"

// checkedErrs src'yi dosyasının yazımıyla denetler (öneriler dosyanın metninden kurulur).
func checkedErrs(t *testing.T, src string) []error {
	t.Helper()
	ch := checker.New()
	ch.SetFile(checker.ProgramFile, src)
	ch.CheckProgram(mustParseProgram(t, src))
	return ch.Errs()
}

// Kural (3.5, KB2): H1180'in önerisi kopyalanınca denetimden geçer, koşar ve kaynağa yazar. Kabın
// öğesini veren üyede H1181 kabı indeksler; öbür değerde H1182 değeri yerel ada alıp sınar: ifade
// deyimi, atama, tanım, dönüş, indekse ve alana yazma ve yerinde değiştirici bütünüyle yeniden
// yazılır. Koşulda öneri "..." sarar (kullanım tek başına deyim olamaz, 2.8).
func TestUnopenedIndexSuggestionsRun(t *testing.T) {
	for _, c := range []struct {
		wrong            string
		hint             messages.Code
		wrap, show, want string // wrap'te %s önerinin yeri; show sonra yazdırılan
	}{
		{"TR_print(d.TR_get(\"a\")[1])", messages.H1181, "%s", "", "2"},
		{"z = d.TR_get(\"a\")[0]", messages.H1181, "%s", "z", "1"},
		{"TR_var y = d.TR_get(\"a\").TR_length", messages.H1181, "%s\nTR_print(y)", "", "2"},
		{"TR_print(k.l[1])", messages.H1182, "%s", "", "8"},
		{"z = k.n.x", messages.H1182, "%s", "z", "4"},
		{"TR_var y = k.l[0]", messages.H1182, "%s", "y + 1", "8"}, // tanım: erken çıkış, ad sonra görünür
		{"TR_var y, w = k.l[0], 1", messages.H1182, "%s", "y + w", "8"},
		{"TR_func q(s: TR_String) -> K { TR_return k }\nTR_var y = q(\"a\").l[1]", messages.H1182,
			"TR_func q(s: TR_String) -> K { TR_return k }\n%s", "y", "8"}, // fırlatılan metinde tırnak kaçar
		{"k.l[0] = 9", messages.H1182, "%s", "k.l", "[9, 8]"},
		{"k.n.x = 3", messages.H1182, "%s", "k.n?.x", "3"},
		{"k.l.TR_append(1)", messages.H1182, "%s", "k.l", "[7, 8, 1]"},
		{"TR_func f() -> TR_Int { TR_return k.l[0] }", messages.H1182, "TR_func f() -> TR_Int { %s }", "f()", "7"},
	} {
		errs := checkedErrs(t, unopenedPre+c.wrong)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H1180) || !messages.Has(errs[0], c.hint) {
			t.Errorf("%s: tek %s ipuçlu H1180 bekleniyordu, gelen %v", c.wrong, c.hint, errs)
			continue
		}
		parts := strings.Split(messages.ErrorIn(errs[0], "en"), "`")
		sug := parts[len(parts)-2] // öneri iletinin son kod örneğidir
		code := unopenedPre + fmt.Sprintf(c.wrap, sug)
		if c.show != "" {
			code += "\nTR_print(" + c.show + ")"
		}
		if errs := checkedErrs(t, code); len(errs) > 0 {
			t.Errorf("%s: öneri %q denetimden geçmeli: %v", c.wrong, sug, errs)
		}
		want := c.want
		if want != "" {
			want += "\n"
		}
		if out, err := runSourced(t, code); err != nil || out != want {
			t.Errorf("%s: öneri %q: %q %v, beklenen %q", c.wrong, sug, out, err, want)
		}
	}
	// Tanımın erken çıkışı TR_none'da ifadenin metnini fırlatır: hata bugünkü gibi kalır, açık olur.
	errs := checkedErrs(t, unopenedPre+"TR_var y = k.l[0]")
	if len(errs) == 1 {
		parts := strings.Split(messages.ErrorIn(errs[0], "en"), "`")
		code := unopenedPre + "k.l = TR_none\nTR_try { " + parts[len(parts)-2] + " } TR_catch h { TR_print(h.TR_value) }"
		if out, err := runSourced(t, code); err != nil || out != "k.l\n" {
			t.Errorf("erken çıkış TR_none'da \"k.l\" fırlatmalı: %q %v", out, err)
		}
	}
	errs = checkedErrs(t, unopenedPre+"TR_if k.l[0] > 1 { TR_print(1) }")
	if len(errs) != 1 || !strings.Contains(messages.ErrorIn(errs[0], "en"), "`TR_var c = k.l; TR_if c != TR_none { ... }`") {
		t.Errorf("koşulda öneri sınamayı \"...\" ile yazmalı: %v", errs)
	}
}

// Kural (3.5, KB3): çalışırken TR_none indekslenince ya da indeksle yazılınca H2742, alanı, özelliği
// ya da metodu kullanılınca H2744 verilir. İleti TR_none'ı, tipini (TR_NoneType) ve üyeyi kimliğiyle
// (programın alanında adıyla) yazar; görünüm adı ('yok', YokTipi, uzunluk) sızmaz. Yazma kolu okuma
// önerisi ('??', '?.') vermez; yerinde değiştirici metot (in_place, .TR_append) da yazmadır, çünkü
// '(o ?? [5]).TR_append(1)' kopyaya yazar (H2740, DENETİM 755 N1). Önerilen yazımlar koşar.
func TestNoneIndexError(t *testing.T) {
	const pre = "TR_struct N {\n    x: TR_Int = 0\n}\nTR_var o = {\"a\": [1]}.TR_get(\"b\")\n" +
		"TR_struct K {\n    n: N? = TR_none\n}\nTR_var n = K().n\n"
	for _, c := range []struct {
		src  string
		code messages.Code
		has  string // İngilizce iletide
		read bool   // okuma kolu: '?.' ya da '??' önerir
	}{
		{"TR_print(o[0])", messages.H2742, "cannot be indexed", true},
		{"TR_print(o[0..1])", messages.H2742, "cannot be indexed", true},
		{"o[0] = 1", messages.H2742, "cannot be written by index", false},
		{"o[0] += 1", messages.H2742, "cannot be written by index", false},
		{"TR_print(n.x)", messages.H2744, "no member `x` to use", true},
		{"n.x = 1", messages.H2744, "no member `x` to write to", false},
		{"TR_print(o.TR_length)", messages.H2744, "no member `TR_length` to use", true},
		{"o.TR_append(1)", messages.H2744, "no member `TR_append` to change it with", false}, // yerinde değiştirici: '??' kopyaya yazardı (H2740)
	} {
		_, err := runSourced(t, pre+c.src)
		if !messages.Has(err, c.code) {
			t.Errorf("%s: %v bekleniyordu, gelen %v", c.src, c.code, err)
			continue
		}
		en, tr := messages.ErrorIn(err, "en"), messages.ErrorIn(err, "tr")
		if !strings.Contains(en, c.has) || !strings.Contains(en, "`TR_none`") || !strings.Contains(en, "`TR_NoneType`") {
			t.Errorf("%s: iletide %q, `TR_none` ve `TR_NoneType` olmalı: %s", c.src, c.has, en)
		}
		if strings.Contains(en, "??") != c.read || strings.Contains(tr, "??") != c.read {
			t.Errorf("%s: okuma önerisi yalnız okuma kolunda: %s", c.src, en)
		}
		for _, view := range []string{"YokTipi", "'yok'", "uzunluk", "ekle"} {
			if strings.Contains(tr, view) {
				t.Errorf("%s: iletide görünüm adı %q var: %s", c.src, view, tr)
			}
		}
	}
	for code, want := range map[string]string{ // önerilen yazımlar
		"TR_print(o?.TR_length)":                             "none",
		"TR_print((o ?? [5])[0])":                            "5",
		"TR_if o != TR_none { o[0] = 1 }\nTR_print(o)":       "none",
		"TR_if n != TR_none { n.x = 1 }\nTR_print(n?.x)":     "none",
		"TR_if o != TR_none { o.TR_append(1) }\nTR_print(o)": "none",
		"TR_print(o?.TR_get(0))":                             "none", // okuma kolunun '?.' önerisi metot çağrısında (4.8)
	} {
		if out, err := runSourced(t, pre+code); err != nil || out != want+"\n" {
			t.Errorf("%q: %q %v, beklenen %s", code, out, err, want)
		}
	}
	if _, err := runSourced(t, pre+"(o ?? [5]).TR_append(1)"); !messages.Has(err, messages.H2740) { // okuma önerisi burada yanlış olurdu
		t.Errorf("'??' ile yerinde değiştirme kopyaya yazar, H2740 bekleniyordu: %v", err)
	}
}
