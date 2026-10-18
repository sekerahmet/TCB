package vm

import (
	"math"
	"os"
	"path/filepath"
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

// Sayılar tam ve kararlı sıralanır (eşitlerde özgün sıra); büyük Tam'lar Ondalığa
// yuvarlanmaz. Sayı ile metin karışımı karşılaştırılamaz: girdinin sırasına bağlı bir
// sonuç yerine hata verir.
func TestSiralaDegerlerKararli(t *testing.T) {
	a, b := &FloatVal{Val: 2}, NewInt(2)
	d := []Value{NewInt(3), a, NewInt(-1), b, &FloatVal{Val: 0.5}, &FloatVal{Val: math.NaN()}, NewInt(9007199254740993), &FloatVal{Val: 9007199254740992}}
	if err := siralaDegerler("sırala()", d); err != nil {
		t.Fatal(err)
	}
	if (&ListVal{Elements: d}).Inspect() != "[-1, 0.5, 2.0, 2, 3, 9.007199254740992e+15, 9007199254740993, NaN]" || d[2] != a || d[3] != b {
		t.Fatalf("beklenmeyen sıra: %v", (&ListVal{Elements: d}).Inspect())
	}
	tamlar := []Value{NewInt(9007199254740993), NewInt(9007199254740992)}
	_ = siralaDegerler("sırala()", tamlar)
	if tamlar[0].(*IntVal).Val != 9007199254740992 {
		t.Fatalf("büyük Tam'lar tam karşılaştırılmalı: %v", tamlar)
	}
	for _, karisik := range [][]Value{
		{&StringVal{Val: "b"}, NewInt(1), &StringVal{Val: "a"}},
		{NewInt(2), &StringVal{Val: "10"}, NewInt(10)},
		{TRUE, FALSE},
	} {
		if err := siralaDegerler("sırala()", karisik); err == nil || !strings.Contains(err.Error(), "yalnızca sayılardan ya da yalnızca metinlerden") {
			t.Fatalf("karışık liste hata vermeli: %v", err)
		}
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

// Doğrudan metot çağrıları bağlı yerleşik yoluyla aynı sonucu vermeli.
func TestDogrudanMetotCagrilari(t *testing.T) {
	_, val, err := runTROut(t, `
		değişken l = [1]
		l.ekle(uzunluk(l), uzunluk(l))
		l.ekle(9)
		değişken e = l.ekle
		e(7)
		değişken d = {"içerir": 5, "a": 1}
		sonuç [l, d.içerir("a"), d.a, "Kar".büyük_harf(), [3, 1, 2].sırala(), metin(12), uzunluk("çağ"),
			[1, 2, 3].filtrele { o > 1 }.haritala { o * 10 }]
	`)
	if err != nil || val.Inspect() != `[[1, 1, 1, 9, 7], doğru, 1, "KAR", [1, 2, 3], "12", 3, [20, 30]]` {
		t.Fatalf("beklenmeyen: %v %v", val, err)
	}
	_, _, err = runTROut(t, "değişken l = [1]\nl.haritala(1, 2)\n")
	if err == nil || err.Error() != "Satır 2: liste.haritala(fonksiyon) 1 parametre bekler" {
		t.Fatalf("metot hatası: %v", err)
	}
}

// Yapı alanı varsayılanları yapının tanımlandığı kapsamda değerlendirilir (modül, fonksiyon).
func TestYapiVarsayilaniTanimKapsaminda(t *testing.T) {
	dir := t.TempDir()
	modul := filepath.Join(dir, "modk.tr")
	if err := os.WriteFile(modul, []byte("sabit VARS = 5\nyapı K { a: Tam = VARS }\nfonksiyon K_yap() { sonuç K() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, val, err := runTROut(t, `
		aktar "`+filepath.ToSlash(modul)+`" olarak m
		fonksiyon yerel_yapi() {
			değişken taban = 7
			yapı Y { b: Tam = taban * 2 }
			sonuç Y()
		}
		yapı G { c: Tam = 1 }
		sonuç [m.K_yap().a, m.K().a, yerel_yapi().b, G().c]
	`)
	if err != nil || val.Inspect() != "[5, 5, 14, 1]" {
		t.Fatalf("[5, 5, 14, 1] bekleniyordu: %v %v", val, err)
	}
}

// Hızlı yollar (ASCII metin indeksi, Tam anahtarlı sözlük, doğrudan yapı metodu) eski
// anlamı korumalı.
func TestHizliYollarAnlami(t *testing.T) {
	_, val, err := runTROut(t, `
		değişken s = "abç"
		değişken a = "xyz"
		değişken d = {1: "bir", "1": "metin bir", 1.5: "ondalık"}
		d[1] = "BİR"
		d[2] = "iki"
		yapı P {
			x: Tam = 3
			f
			fonksiyon m(k = 1) { sonuç kendi.x * k }
			fonksiyon f() { sonuç "metot" }
		}
		değişken p = P()
		p.f = () => "alan"
		değişken bagli = p.m
		sonuç [s[2], s[-1], s[5], a[0], a[-1], a[3], a[0..2], s[1..3], a[-4],
			d[1], d["1"], d[1.5], d[2], d.anahtarlar(), d.içerir(2), d.içerir(3),
			p.m(), p.m(2), p.f(), bagli(10)]
	`)
	want := `["ç", "ç", yok, "x", "z", yok, "xy", "bç", yok, "BİR", "metin bir", "ondalık", "iki", [1, "1", 1.5, 2], doğru, yanlış, 3, 6, "alan", 30]`
	if err != nil || val.Inspect() != want {
		t.Fatalf("beklenmeyen:\n%v\n%v", val, err)
	}
	_, _, err = runTROut(t, "yapı Q { fonksiyon m(a) { sonuç a } }\ndeğişken q = Q()\nq.m()\n")
	if err == nil || !strings.Contains(err.Error(), "'m' fonksiyonu 1 argüman bekler") {
		t.Fatalf("metot argüman denetimi: %v", err)
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
