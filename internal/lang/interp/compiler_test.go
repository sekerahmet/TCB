package interp

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Kapanış derleyicisinin kapsam kurallarını ağaç yorumlayıcısının anlamıyla karşılaştıran testler.

func TestCompilerScopes(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_var x = "genel"
		TR_func f() {
			TR_print(x)
			TR_var x = "yerel"
			TR_print(x)
		}
		f()
		TR_print(x)

		# her tur kendi değişkenini yakalar
		TR_var fs = []
		TR_for i TR_in 0..3 {
			TR_var j = i * 10
			fs.TR_append(() => i + j)
		}
		TR_for g TR_in fs { TR_print(g()) }

		# ortamı yakalamayan döngü gövdesi yeniden kullanılsa da önceki turun tanımı görünmez
		TR_var k = 0
		TR_var y = "dış"
		TR_for k < 2 {
			TR_print(y)
			TR_var y = "tur"
			k += 1
		}

		TR_if TR_true { TR_var gizli = 1 }
		TR_try { TR_print(gizli) } TR_catch h { TR_print("yok") }
	`)
	if err != nil {
		t.Fatal(err)
	}
	want := "genel\nyerel\ngenel\n0\n11\n22\ndış\ndış\nyok\n"
	if out != want {
		t.Fatalf("beklenen:\n%s\nalınan:\n%s", want, out)
	}
}

// Hata ayıklayıcının değerlendirdiği ifade ve deyimler, düzeni derleme anında bilinmeyen
// ortamlarda adla çalışmalı; tanımladıkları adlar sonradan derlenmiş koddan görünmeli.
func TestCompilerDynamicEval(t *testing.T) {
	machine := New()
	env := NewEnvironment(machine.GlobalEnv)
	env.Define("a", NewInt(4), false)
	prog := parser.New(source.TextLexer("TR_var b = a * 2\nb += 1")).ParseProgram()
	if _, err := machine.ExecStatements(prog.Statements, env); err != nil {
		t.Fatal(err)
	}
	expr := parser.New(source.TextLexer("a + b")).ParseProgram().Statements[0].(*ast.ExpressionStatement).Expression
	val, err := machine.EvalExpression(expr, env)
	if err != nil || val.Inspect() != "13" {
		t.Fatalf("13 bekleniyordu: %v %v", val, err)
	}
	var nameList []string
	env.EachLocal(func(name string, _ Value) { nameList = append(nameList, name) })
	if strings.Join(nameList, ",") != "a,b" {
		t.Fatalf("yerel adlar: %v", nameList)
	}
	if _, err := machine.EvalExpression(expr, machine.GlobalEnv); !messages.Has(err, messages.H1106) {
		t.Fatalf("genel kapsamda 'a' bulunmamalı: %v", err)
	}
}

// jumpingDebugger belirli bir satıra ilk gelişte başka bir satıra atlatır ve gördüğü
// ortamda adla değişken okur.
type jumpingDebugger struct {
	source, dest int
	skipped      bool
	seenSet      []string
}

func (d *jumpingDebugger) BeforeStatement(v *VM, stmt ast.Statement, env *Environment) int {
	if val, ok := env.Get("s"); ok {
		d.seenSet = append(d.seenSet, val.Inspect())
	}
	if stmt.Line() == d.source && !d.skipped {
		d.skipped = true
		return d.dest
	}
	return 0
}

func (d *jumpingDebugger) Breakpoint(string) {}

// Hata ayıklayıcı bir satır döndürdüğünde yürütme o satıra (fonksiyon gövdesinde de) atlamalı.
func TestCompilerJump(t *testing.T) {
	codeVal := "fonksiyon f() {\n" + // 1
		"    TR_var s = 0\n" + // 2
		"    s += 1\n" + // 3
		"    s += 10\n" + // 4
		"    sonuç s\n" + // 5
		"}\n" + // 6
		"TR_print(f())\n" // 7
	p := parser.New(source.TextLexer(codeVal))
	prog := p.ParseProgram()
	var out strings.Builder
	machine := New()
	machine.Out = &out
	d := &jumpingDebugger{source: 5, dest: 3}
	machine.Debugger = d
	if _, err := machine.Run(prog); err != nil {
		t.Fatal(err)
	}
	if out.String() != "22\n" {
		t.Fatalf("geri zıplama sonrası 22 bekleniyordu, alınan %q", out.String())
	}
	if !d.skipped || len(d.seenSet) == 0 {
		t.Fatalf("hata ayıklayıcı yerel 's' değişkenini görmeliydi: %v", d.seenSet)
	}
}

// Sayılar tam ve kararlı sıralanır (eşitlerde özgün sıra); büyük Tam'lar Ondalığa
// yuvarlanmaz. Sayı ile metin karışımı karşılaştırılamaz: girdinin sırasına bağlı bir
// sonuç yerine hata verir.
func TestSortValuesStable(t *testing.T) {
	a, b := &FloatVal{Val: 2}, NewInt(2)
	d := []Value{NewInt(3), a, NewInt(-1), b, &FloatVal{Val: 0.5}, &FloatVal{Val: math.NaN()}, NewInt(9007199254740993), &FloatVal{Val: 9007199254740992}}
	if err := sortValues("sırala()", d); err != nil {
		t.Fatal(err)
	}
	if (&ListVal{Elements: d}).Inspect() != "[-1, 0.5, 2.0, 2, 3, 9.007199254740992e+15, 9007199254740993, NaN]" || d[2] != a || d[3] != b {
		t.Fatalf("beklenmeyen sıra: %v", (&ListVal{Elements: d}).Inspect())
	}
	ints := []Value{NewInt(9007199254740993), NewInt(9007199254740992)}
	_ = sortValues("sırala()", ints)
	if ints[0].(*IntVal).Val != 9007199254740992 {
		t.Fatalf("büyük Tam'lar tam karşılaştırılmalı: %v", ints)
	}
	for _, mixed := range [][]Value{
		{&StringVal{Val: "b"}, NewInt(1), &StringVal{Val: "a"}},
		{NewInt(2), &StringVal{Val: "10"}, NewInt(10)},
		{True, False},
	} {
		if err := sortValues("sırala()", mixed); err == nil || !strings.Contains(err.Error(), "yalnızca sayılardan ya da yalnızca metinlerden") {
			t.Fatalf("karışık liste hata vermeli: %v", err)
		}
	}
}

// Havuzdan alınan çağrı ortamları özyinelemede ve hata sonrasında karışmamalı.
func TestCallEnvPool(t *testing.T) {
	_, val, err := runTROut(t, `
		TR_func fib(n) {
			TR_if n < 2 { TR_return n }
			TR_return fib(n - 1) + fib(n - 2)
		}
		TR_func bol(a, b) { TR_return a / b }
		TR_try { bol(1, 0) } TR_catch h { }
		TR_func iki(x, y = 10) {
			TR_var z = x * y
			TR_return z
		}
		TR_var result = [fib(15), iki(fib(5)), iki(2, 3), bol(9, 3)]
	`)
	if err != nil || val.Inspect() != "[610, 50, 6, 3]" {
		t.Fatalf("[610, 50, 6, 3] bekleniyordu: %v %v", val, err)
	}
}

// Doğrudan metot çağrıları bağlı yerleşik yoluyla aynı sonucu vermeli.
func TestDirectMethodCalls(t *testing.T) {
	_, val, err := runTROut(t, `
		TR_var l = [1]
		l.TR_append(l.TR_length, l.TR_length)
		l.TR_append(9)
		TR_var e = l.TR_append
		e(7)
		TR_var d = {"içerir": 5, "a": 1}
		TR_var result = [l, d.TR_contains("a"), d.a, "Kar".TR_upper(), [3, 1, 2].TR_sort(), TR_str(12), "çağ".TR_length,
			[1, 2, 3].TR_filter { TR_it > 1 }.TR_map { TR_it * 10 }]
	`)
	if err != nil || val.Inspect() != `[[1, 1, 1, 9, 7], doğru, 1, "KAR", [1, 2, 3], "12", 3, [20, 30]]` {
		t.Fatalf("beklenmeyen: %v %v", val, err)
	}
	_, _, err = runTROut(t, "TR_var l = [1]\nl.TR_map(1, 2)\n")
	if err == nil || err.Error() != "Satır 2: liste.haritala() 1 argüman bekler, 2 verildi (kullanım: liste.haritala(fonksiyon))" {
		t.Fatalf("metot hatası: %v", err)
	}
}

// Yapı alanı varsayılanları yapının tanımlandığı kapsamda değerlendirilir (modül, fonksiyon).
func TestStructDefaultInDefScope(t *testing.T) {
	dir := t.TempDir()
	module := filepath.Join(dir, "modk.tr")
	if err := os.WriteFile(module, []byte(source.Header + "\nTR_const VARS = 5\nTR_struct K { a: TR_Int = VARS }\nTR_func K_yap() { TR_return K() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, val, err := runTROut(t, `
		aktar "`+filepath.ToSlash(module)+`" olarak m
		fonksiyon yerel_yapi() {
			değişken taban = 7
			yapı Y { b: Tam = taban * 2 }
			sonuç Y()
		}
		yapı G { c: Tam = 1 }
		değişken result = [m.K_yap().a, m.K().a, yerel_yapi().b, G().c]
	`)
	if err != nil || val.Inspect() != "[5, 5, 14, 1]" {
		t.Fatalf("[5, 5, 14, 1] bekleniyordu: %v %v", val, err)
	}
}

// Hızlı yollar (ASCII metin indeksi, Tam anahtarlı sözlük, doğrudan yapı metodu) eski
// anlamı korumalı.
func TestFastPathSemantics(t *testing.T) {
	_, val, err := runTROut(t, `
		TR_var s = "abç"
		TR_var a = "xyz"
		TR_var d = {1: "bir", "1": "metin bir", 1.5: "ondalık"}
		d[1] = "BİR"
		d[2] = "iki"
		TR_struct P {
			x: TR_Int = 3
			f
			TR_func m(k = 1) { TR_return TR_self.x * k }
		}
		TR_var p = P()
		p.f = () => "alan"
		TR_var bagli = p.m
		TR_var result = [s[2], s[-1], s.TR_get(5), a[0], a[-1], a.TR_get(3), a[0..2], s[1..3], a.TR_get(-4),
			d[1], d["1"], d[1.5], d[2], d.TR_keys(), d.TR_contains(2), d.TR_contains(3),
			p.m(), p.m(2), p.f(), bagli(10)]
	`)
	want := `["ç", "ç", yok, "x", "z", yok, "xy", "bç", yok, "BİR", "metin bir", "ondalık", "iki", [1, "1", 1.5, 2], doğru, yanlış, 3, 6, "alan", 30]`
	if err != nil || val.Inspect() != want {
		t.Fatalf("beklenmeyen:\n%v\n%v", val, err)
	}
	_, _, err = runTROut(t, "TR_struct Q { TR_func m(a) { TR_return a } }\nTR_var q = Q()\nq.m()\n")
	if !messages.Has(err, messages.H2414) {
		t.Fatalf("metot argüman denetimi: %v", err)
	}
}

// Yerleşik adı gölgeleyen genel değişken sabit sayılmamalı; kullanıcı sabiti korunmalı.
func TestCompilerShadowingAndConst(t *testing.T) {
	_, val, err := runTROut(t, `
		TR_var toplam = 0
		TR_for i TR_in 1..=4 { toplam += i }
		TR_var result = toplam
	`)
	if err != nil || val.Inspect() != "10" {
		t.Fatalf("10 bekleniyordu: %v %v", val, err)
	}
	machine := New()
	machine.GlobalEnv.Define("LIM", NewInt(1), true)
	if err := machine.GlobalEnv.Assign("LIM", NewInt(2)); err == nil {
		t.Fatal("sabite atama hata vermeliydi")
	}
}
