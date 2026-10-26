package interp

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// fakeValue kancaları sınamak için kendi eşitliğini, işleçlerini ve indekslemesini sağlar.
type fakeValue struct{ n int }

func (s *fakeValue) Type() ValueType { return "Sahte" }
func (s *fakeValue) Inspect() string { return "sahte" } // hepsi aynı görünür
func (s *fakeValue) IsTruthy() bool  { return true }
func (s *fakeValue) Equal(d Value) bool {
	o, ok := d.(*fakeValue)
	return ok && o.n == s.n
}
func (s *fakeValue) Operator(op string, d Value, onRight bool) (Value, bool, error) {
	if op == "%" {
		return nil, true, errors.New("kalan desteklenmiyor")
	}
	if op != "-" || !onRight {
		return nil, false, nil
	}
	return &StringVal{Val: d.Inspect() + " eksi sahte"}, true, nil
}
func (s *fakeValue) Negate() (Value, error)       { return NewInt(int64(-s.n)), nil }
func (s *fakeValue) Index(i Value) (Value, error) { return NewInt(int64(s.n) * i.(*IntVal).Val), nil }

func parseExpr(t *testing.T, codeVal string) ast.Expression {
	t.Helper()
	p := parser.New(source.TextLexer(codeVal))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 || len(prog.Statements) != 1 {
		t.Fatalf("ifade ayrıştırılamadı: %q %v", codeVal, p.Errors())
	}
	return prog.Statements[0].(*ast.ExpressionStatement).Expression
}

func TestValueHooks(t *testing.T) {
	m := New()
	m.GlobalEnv.Define("a", &fakeValue{n: 1}, false)
	m.GlobalEnv.Define("b", &fakeValue{n: 2}, false)
	m.GlobalEnv.Define("c", &fakeValue{n: 1}, false)
	expression := func(codeVal string) (string, error) {
		val, err := m.EvalExpression(parseExpr(t, codeVal), m.GlobalEnv)
		if err != nil {
			return "", err
		}
		return val.Inspect(), nil
	}
	for codeVal, want := range map[string]string{
		"a == b": "yanlış", "a == c": "doğru", "a != b": "doğru", "[a] == [b]": "yanlış",
		"5 - a": "5 eksi sahte", "-b": "-2", "b[3]": "6",
	} {
		if got, err := expression(codeVal); err != nil || got != want {
			t.Errorf("%s: beklenen %s, alınan %s (%v)", codeVal, want, got, err)
		}
	}
	if _, err := expression("a % 2"); !atLine(err, 1, "kalan desteklenmiyor") {
		t.Errorf("kanca hatası satır önekli olmalı: %v", err)
	}
	if _, err := expression("a * 2"); err == nil || !strings.Contains(err.Error(), "işlemi desteklenmiyor") {
		t.Errorf("desteklenmeyen işleç genel hatayı vermeli: %v", err)
	}
}

// Kaplar eleman eleman karşılaştırılır; sözlüklerde sıra önemsizdir.
func TestContainerEquality(t *testing.T) {
	_, val, err := runTROut(t, `
		yapı N { x }
		sonuç [[1, 2] == [1.0, 2], [0.1 + 0.2] == [0.3], {"a": 1, "b": 2} == {"b": 2, "a": 1},
			{1: "x"} == {"1": "x"}, N(1) == N(1.0), N(1) == N(2), ["1"] == [1]]
	`)
	if err != nil || val.Inspect() != "[doğru, yanlış, doğru, yanlış, doğru, yanlış, yanlış]" {
		t.Fatalf("beklenmeyen: %v %v", val, err)
	}
}

// Kendini içeren kaplar yazdırılırken ve karşılaştırılırken süreç çökmemeli.
func TestSelfContainingContainers(t *testing.T) {
	out, val, err := runTROut(t, `
		değişken l = [1, "a"]
		l.ekle(l)
		yazdır(l)
		değişken d = {"x": 1}
		d["ben"] = d
		d["l"] = [d]
		yazdır(d)
		yapı D { sonraki }
		değişken n = D()
		n.sonraki = n
		yazdır(n)
		değişken m = [l, l]
		yazdır(m)
		değişken l2 = [1, "a"]
		l2.ekle(l2)
		sonuç [l == l, d == d, l == l2, metin(l).uzunluk]
	`)
	want := "[1, \"a\", [...]]\n{\"x\": 1, \"ben\": {...}, \"l\": [{...}]}\nD{sonraki: D{...}}\n[[1, \"a\", [...]], [1, \"a\", [...]]]\n"
	if err != nil || out != want || val.Inspect() != "[doğru, doğru, yanlış, 15]" {
		t.Fatalf("beklenmeyen:\n%s\n%v %v", out, val, err)
	}
}

// Tam sayı kenarları: en küçük değerin '/ -1' ve tekli eksisi taşma hatası verir; iki Tam
// doğrudan karşılaştırılır.
func TestIntEdges(t *testing.T) {
	_, val, err := runTROut(t, `
		değişken en_kucuk = -9223372036854775807 - 1
		değişken b = -1
		değişken sonuclar = []
		dene { sonuclar.ekle(en_kucuk / b) } yakala h { sonuclar.ekle(h) }
		dene { sonuclar.ekle(-en_kucuk) } yakala h { sonuclar.ekle(h) }
		dene { değişken x = en_kucuk
			x /= -1 } yakala h { sonuclar.ekle("bileşik taşma") }
		sonuclar.ekle(9007199254740993 == 9007199254740992)
		sonuclar.ekle(9007199254740993 != 9007199254740992)
		sonuclar.ekle([9007199254740993] == [9007199254740992])
		sonuclar.ekle(1 == 1.0)
		sonuclar.ekle(en_kucuk % b)
		sonuç sonuclar
	`)
	want := `["Satır 5: Tam sayı taşması (-9223372036854775808 / -1); ondalık sonuç için örn. -9223372036854775808.0 / -1 yazın", ` +
		`"Satır 6: Tam sayı taşması (-(-9223372036854775808)); ondalık sonuç için örn. -(-9223372036854775808.0) yazın", ` +
		`"bileşik taşma", yanlış, doğru, yanlış, doğru, 0]`
	if err != nil || val.Inspect() != want {
		t.Fatalf("beklenmeyen:\n%v\n%v", val, err)
	}
}

// Yerleşiğin geri çağırdığı fonksiyonun hatası zaten satırlıysa ikinci kez sarılmaz.
func TestInternalErrorLineOnce(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	_, val, err := runTROut(t, `
		değişken h = []
		dene { [1].filtrele((x) => x / 0) } yakala e { h.ekle(e) }
		dene { [1].filtrele(
			(x) => x / 0) } yakala e { h.ekle(e) }
		dene { [1].haritala(1, 2) } yakala e { h.ekle(e) }
		dene { aralık("a") } yakala e { h.ekle(e) }
		sonuç h
	`)
	want := `["Satır 3: Sıfıra bölme hatası", "Satır 5: Sıfıra bölme hatası", ` +
		`"Satır 6: liste.haritala() 1 argüman bekler, 2 verildi (kullanım: liste.haritala(fonksiyon))", "Satır 7: aralık() 1. argüman olarak Tam bekler, Metin (\"a\") verildi"]`
	if err != nil || val.Inspect() != want {
		t.Fatalf("beklenmeyen:\n%v\n%v", val, err)
	}
	var sh *LineError
	e := lineSuffix(4, errors.New("iç"))
	if !errors.As(e, &sh) || sh.Line != 4 || lineSuffix(9, e) != e {
		t.Fatalf("tipli satır hatası: %v", e)
	}
}

// Sonsuz ve tanımsız Ondalık değerler dilin değer adlarıyla (TR_val_inf, TR_val_nan; tensörle aynı) yazılır;
// bilimsel yazım ayrıştırılır.
func TestFloatPrintingAndScientific(t *testing.T) {
	for f, want := range map[float64]string{math.Inf(1): "sonsuz", math.Inf(-1): "-sonsuz", math.NaN(): "NaN", 1e21: "1e+21", 2: "2.0", 0.5: "0.5"} {
		if got := (&FloatVal{Val: f}).Inspect(); got != want {
			t.Errorf("%v: beklenen %s, alınan %s", f, want, got)
		}
	}
	_, val, err := runTROut(t, `sonuç [1e-5, 3.2E+4, 2e3 == 2000, 1e-5 * 1e5]`)
	if err != nil || val.Inspect() != "[1e-05, 32000.0, doğru, 1.0]" {
		t.Fatalf("bilimsel yazım: %v %v", val, err)
	}
}

// assignableValue indeksli atama ve dış biçime dönüşüm kancalarını sınar: a[i] = d, i
// konumuna d yazar; a[i] okuması yazılmış değeri (yoksa 0) verir.
type assignableValue struct{ cell map[int64]Value }

func (a *assignableValue) Type() ValueType { return "Atanabilir" }
func (a *assignableValue) Inspect() string { return "atanabilir" }
func (a *assignableValue) IsTruthy() bool  { return true }
func (a *assignableValue) Index(i Value) (Value, error) {
	if v, ok := a.cell[i.(*IntVal).Val]; ok {
		return v, nil
	}
	return NewInt(0), nil
}
func (a *assignableValue) SetIndex(i, d Value) error {
	n := i.(*IntVal).Val
	if n < 0 {
		return errors.New("negatif indeks")
	}
	a.cell[n] = d
	return nil
}
func (a *assignableValue) PlainValue() Value {
	return &ListVal{Elements: []Value{a.cell[0], a.cell[1]}}
}

func TestIndexAssignAndPlainValueHooks(t *testing.T) {
	a := &assignableValue{cell: map[int64]Value{}}
	vmx := New()
	vmx.GlobalEnv.Define("a", a, false)
	vmx.GlobalEnv.Define("l", &ListVal{Elements: []Value{a}}, false)
	prog := mustParseProgram(t, "a[0] = 5\na[1] += 2\na[1] *= 4\nl[0][0] -= 1\na[-1] = 3")
	_, err := vmx.Run(prog)
	if !atLine(err, 5, "negatif indeks") {
		t.Fatalf("kanca hatası satır önekli olmalı: %v", err)
	}
	if got := a.PlainValue().Inspect(); got != "[4, 8]" {
		t.Fatalf("atamalar ya da düz değer: %v", got)
	}
}

// iteratedValue n öğeli (öğe i: i*10) bir dış değerdir; n < 0 ise döngü kurulamaz.
type iteratedValue struct{ n int }

func (y *iteratedValue) Type() ValueType { return "Yinelenen" }
func (y *iteratedValue) Inspect() string { return "yinelenen" }
func (y *iteratedValue) IsTruthy() bool  { return true }
func (y *iteratedValue) IterLen() (int, error) {
	if y.n < 0 {
		return 0, errors.New("üzerinde gezilemez")
	}
	return y.n, nil
}
func (y *iteratedValue) IterItem(i int) (Value, error) { return NewInt(int64(i * 10)), nil }

// Yinelenebilir kancası 'döngü x içinde', 'döngü i, x içinde' ve liste üretecinde
// kullanılır; kanca hatası satır bilgisi taşır.
func TestIterableHook(t *testing.T) {
	runCode := func(codeVal string) (*VM, error) {
		vmx := New()
		vmx.GlobalEnv.Define("y", &iteratedValue{n: 4}, false)
		vmx.GlobalEnv.Define("bozuk", &iteratedValue{n: -1}, false)
		prog := mustParseProgram(t, codeVal)
		_, err := vmx.Run(prog)
		return vmx, err
	}
	vmx, err := runCode(`
değişken toplamı = 0
döngü x içinde y { toplamı = toplamı + x }
değişken çiftler = []
döngü i, x içinde y {
    eğer i == 3 { çık }
    çiftler.ekle([i, x])
}
değişken üretilen = [x + 1 döngü x içinde y eğer x > 0]
`)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"toplamı": "60", "çiftler": "[[0, 0], [1, 10], [2, 20]]", "üretilen": "[11, 21, 31]",
	} {
		if got, _ := vmx.GlobalEnv.Get(name); got == nil || got.Inspect() != want {
			t.Errorf("%s: beklenen %s, alınan %v", name, want, got)
		}
	}
	if _, err := runCode("\ndöngü x içinde bozuk { }"); !atLine(err, 2, "üzerinde gezilemez") {
		t.Errorf("döngü kanca hatası satır önekli olmalı: %v", err)
	}
	if _, err := runCode("\n\nyazdır([x döngü x içinde bozuk])"); !atLine(err, 3, "üzerinde gezilemez") {
		t.Errorf("üreteç kanca hatası satır önekli olmalı: %v", err)
	}
}

// streamValue 0, 10, 20 ... değerlerini n tane üreten tembel bir kaynaktır (yurut.Akis);
// n < 0 ise ilk istekte hata verir. kac kaç öğe istendiğini sayar (tembellik sınaması).
type streamValue struct{ i, n, howMany int }

func (a *streamValue) Type() ValueType { return "Akış" }
func (a *streamValue) Inspect() string { return "akış" }
func (a *streamValue) IsTruthy() bool  { return true }
func (a *streamValue) Next(v *VM) (Value, bool, error) {
	if a.n < 0 {
		return nil, false, errors.New("akış okunamadı")
	}
	if a.i >= a.n {
		return nil, false, nil
	}
	a.howMany++
	a.i++
	return NewInt(int64((a.i - 1) * 10)), true, nil
}

// Akis kancası 'döngü x içinde', 'döngü i, x içinde' ve liste üretecinde kullanılır; öğeler
// istendikçe alınır (çık'tan sonra fazlası istenmez), döngü kaldığı yerden tüketir; kanca
// hatası satır bilgisi taşır.
func TestStreamHook(t *testing.T) {
	a := &streamValue{n: 5}
	runCode := func(codeVal string) (*VM, error) {
		vmx := New()
		vmx.GlobalEnv.Define("a", a, false)
		vmx.GlobalEnv.Define("bozuk", &streamValue{n: -1}, false)
		prog := mustParseProgram(t, codeVal)
		_, err := vmx.Run(prog)
		return vmx, err
	}
	vmx, err := runCode(`
değişken çiftler = []
döngü i, x içinde a {
    çiftler.ekle([i, x])
    eğer i == 1 { çık }
}
değişken kalan = [x + 1 döngü x içinde a eğer x > 20]
değişken boş = 0
döngü x içinde a { boş += 1 }
`)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"çiftler": "[[0, 0], [1, 10]]", "kalan": "[31, 41]", "boş": "0",
	} {
		if got, _ := vmx.GlobalEnv.Get(name); got == nil || got.Inspect() != want {
			t.Errorf("%s: beklenen %s, alınan %v", name, want, got)
		}
	}
	if a.howMany != 5 {
		t.Errorf("akıştan %d öğe istendi, 5 bekleniyordu", a.howMany)
	}
	if _, err := runCode("\ndöngü x içinde bozuk { }"); !atLine(err, 2, "akış okunamadı") {
		t.Errorf("döngü kanca hatası satır önekli olmalı: %v", err)
	}
	if _, err := runCode("\n\nyazdır([x döngü x içinde bozuk])"); !atLine(err, 3, "akış okunamadı") {
		t.Errorf("üreteç kanca hatası satır önekli olmalı: %v", err)
	}
}

// codeAt reports whether err carries message code c at source line line (0: any line). Tests
// check the code and the structural position, not the message text.
func codeAt(err error, c messages.Code, line int) bool {
	if !messages.Has(err, c) {
		return false
	}
	if line == 0 {
		return true
	}
	l, _, ok := messages.Pos(err)
	return ok && l == line
}

// atLine reports whether err is a positioned error at line whose own message is text.
func atLine(err error, line int, text string) bool {
	var pe *messages.PosError
	return errors.As(err, &pe) && pe.Line == line && pe.Err.Error() == text
}
