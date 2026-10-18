package vm

import (
	"strings"
	"testing"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
)

// Kapanış derleyicisinin kapsam kurallarını ağaç yorumlayıcısının anlamıyla karşılaştıran testler.

func TestDerleyiciKapsamlari(t *testing.T) {
	out, _, err := runTROut(t, `
		değişken x = "genel"
		fonksiyon f() {
			yazdır(x)
			değişken x = "yerel"
			yazdır(x)
		}
		f()
		yazdır(x)

		// her tur kendi değişkenini yakalar
		değişken fs = []
		döngü i içinde 0..3 {
			değişken j = i * 10
			fs.ekle(() => i + j)
		}
		döngü g içinde fs { yazdır(g()) }

		// ortamı yakalamayan döngü gövdesi yeniden kullanılsa da önceki turun tanımı görünmez
		değişken k = 0
		değişken y = "dış"
		iken k < 2 {
			yazdır(y)
			değişken y = "tur"
			k += 1
		}

		eğer doğru { değişken gizli = 1 }
		dene { yazdır(gizli) } yakala h { yazdır("yok") }
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
func TestDerleyiciDinamikDegerlendirme(t *testing.T) {
	machine := New()
	env := NewEnvironment(machine.GlobalEnv)
	env.Define("a", NewInt(4), false)
	prog := parser.New(lexer.New("değişken b = a * 2\nb += 1")).ParseProgram()
	if _, err := machine.ExecStatements(prog.Statements, env); err != nil {
		t.Fatal(err)
	}
	expr := parser.New(lexer.New("a + b")).ParseProgram().Statements[0].(*ast.ExpressionStatement).Expression
	val, err := machine.EvalExpression(expr, env)
	if err != nil || val.Inspect() != "13" {
		t.Fatalf("13 bekleniyordu: %v %v", val, err)
	}
	var adlar []string
	env.EachLocal(func(ad string, _ Value) { adlar = append(adlar, ad) })
	if strings.Join(adlar, ",") != "a,b" {
		t.Fatalf("yerel adlar: %v", adlar)
	}
	if _, err := machine.EvalExpression(expr, machine.GlobalEnv); err == nil || !strings.Contains(err.Error(), "'a' adında") {
		t.Fatalf("genel kapsamda 'a' bulunmamalı: %v", err)
	}
}

// zıplayanAyiklayici belirli bir satıra ilk gelişte başka bir satıra atlatır ve gördüğü
// ortamda adla değişken okur.
type zıplayanAyiklayici struct {
	kaynak, hedef int
	atladi        bool
	gorulen       []string
}

func (d *zıplayanAyiklayici) BeforeStatement(v *VM, stmt ast.Statement, env *Environment) int {
	if val, ok := env.Get("s"); ok {
		d.gorulen = append(d.gorulen, val.Inspect())
	}
	if stmt.Line() == d.kaynak && !d.atladi {
		d.atladi = true
		return d.hedef
	}
	return 0
}

func (d *zıplayanAyiklayici) KesmeNoktasi(string) {}

// Hata ayıklayıcı bir satır döndürdüğünde yürütme o satıra (fonksiyon gövdesinde de) atlamalı.
func TestDerleyiciZipla(t *testing.T) {
	kod := "fonksiyon f() {\n" + // 1
		"    değişken s = 0\n" + // 2
		"    s += 1\n" + // 3
		"    s += 10\n" + // 4
		"    sonuç s\n" + // 5
		"}\n" + // 6
		"yazdır(f())\n" // 7
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	var out strings.Builder
	machine := New()
	machine.Out = &out
	d := &zıplayanAyiklayici{kaynak: 5, hedef: 3}
	machine.Debugger = d
	if _, err := machine.Run(prog); err != nil {
		t.Fatal(err)
	}
	if out.String() != "22\n" {
		t.Fatalf("geri zıplama sonrası 22 bekleniyordu, alınan %q", out.String())
	}
	if !d.atladi || len(d.gorulen) == 0 {
		t.Fatalf("hata ayıklayıcı yerel 's' değişkenini görmeliydi: %v", d.gorulen)
	}
}

// Hızlı sıralama yolu, kararlı genel sıralamayla aynı sonucu vermeli (eşitlerde özgün sıra).
func TestSiralaDegerlerKararli(t *testing.T) {
	a, b := &FloatVal{Val: 2}, NewInt(2)
	d := []Value{NewInt(3), a, NewInt(-1), b, &FloatVal{Val: 0.5}}
	siralaDegerler(d)
	if d[0].Inspect() != "-1" || d[1].Inspect() != "0.5" || d[2] != a || d[3] != b || d[4].Inspect() != "3" {
		t.Fatalf("beklenmeyen sıra: %v", d)
	}
	karisik := []Value{&StringVal{Val: "b"}, NewInt(1), &StringVal{Val: "a"}}
	siralaDegerler(karisik)
	if (&ListVal{Elements: karisik}).Inspect() != `[1, "a", "b"]` {
		t.Fatalf("karışık sıralama: %v", (&ListVal{Elements: karisik}).Inspect())
	}
}

// Havuzdan alınan çağrı ortamları özyinelemede ve hata sonrasında karışmamalı.
func TestCagriOrtamHavuzu(t *testing.T) {
	_, val, err := runTROut(t, `
		fonksiyon fib(n) {
			eğer n < 2 { sonuç n }
			sonuç fib(n - 1) + fib(n - 2)
		}
		fonksiyon bol(a, b) { sonuç a / b }
		dene { bol(1, 0) } yakala h { }
		fonksiyon iki(x, y = 10) {
			değişken z = x * y
			sonuç z
		}
		sonuç [fib(15), iki(fib(5)), iki(2, 3), bol(9, 3)]
	`)
	if err != nil || val.Inspect() != "[610, 50, 6, 3]" {
		t.Fatalf("[610, 50, 6, 3] bekleniyordu: %v %v", val, err)
	}
}

// Yerleşik adı gölgeleyen genel değişken sabit sayılmamalı; kullanıcı sabiti korunmalı.
func TestDerleyiciGolgelemeVeSabit(t *testing.T) {
	_, val, err := runTROut(t, `
		değişken toplam = 0
		döngü i içinde 1..=4 { toplam += i }
		sonuç toplam
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
