package vm

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
)

// sahteDeger kancaları sınamak için kendi eşitliğini, işleçlerini ve indekslemesini sağlar.
type sahteDeger struct{ n int }

func (s *sahteDeger) Type() ValueType { return "Sahte" }
func (s *sahteDeger) Inspect() string { return "sahte" } // hepsi aynı görünür
func (s *sahteDeger) IsTruthy() bool  { return true }
func (s *sahteDeger) Esit(d Value) bool {
	o, ok := d.(*sahteDeger)
	return ok && o.n == s.n
}
func (s *sahteDeger) Islec(op string, d Value, sagda bool) (Value, bool, error) {
	if op == "%" {
		return nil, true, errors.New("kalan desteklenmiyor")
	}
	if op != "-" || !sagda {
		return nil, false, nil
	}
	return &StringVal{Val: d.Inspect() + " eksi sahte"}, true, nil
}
func (s *sahteDeger) Olumsuz() (Value, error)       { return NewInt(int64(-s.n)), nil }
func (s *sahteDeger) Indeks(i Value) (Value, error) { return NewInt(int64(s.n) * i.(*IntVal).Val), nil }

func ifadeAyristir(t *testing.T, kod string) ast.Expression {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 || len(prog.Statements) != 1 {
		t.Fatalf("ifade ayrıştırılamadı: %q %v", kod, p.Errors())
	}
	return prog.Statements[0].(*ast.ExpressionStatement).Expression
}

func TestDegerKancalari(t *testing.T) {
	m := New()
	m.GlobalEnv.Define("a", &sahteDeger{n: 1}, false)
	m.GlobalEnv.Define("b", &sahteDeger{n: 2}, false)
	m.GlobalEnv.Define("c", &sahteDeger{n: 1}, false)
	ifade := func(kod string) (string, error) {
		val, err := m.EvalExpression(ifadeAyristir(t, kod), m.GlobalEnv)
		if err != nil {
			return "", err
		}
		return val.Inspect(), nil
	}
	for kod, want := range map[string]string{
		"a == b": "yanlış", "a == c": "doğru", "a != b": "doğru", "[a] == [b]": "yanlış",
		"5 - a": "5 eksi sahte", "-b": "-2", "b[3]": "6",
	} {
		if got, err := ifade(kod); err != nil || got != want {
			t.Errorf("%s: beklenen %s, alınan %s (%v)", kod, want, got, err)
		}
	}
	if _, err := ifade("a % 2"); err == nil || err.Error() != "Satır 1: kalan desteklenmiyor" {
		t.Errorf("kanca hatası satır önekli olmalı: %v", err)
	}
	if _, err := ifade("a * 2"); err == nil || !strings.Contains(err.Error(), "işlemi desteklenmiyor") {
		t.Errorf("desteklenmeyen işleç genel hatayı vermeli: %v", err)
	}
}

// Kaplar eleman eleman karşılaştırılır; sözlüklerde sıra önemsizdir.
func TestKapEsitligi(t *testing.T) {
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
func TestKendiniIcerenKaplar(t *testing.T) {
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
func TestTamSayiKenarlari(t *testing.T) {
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
func TestIcHataSatiriTekKez(t *testing.T) {
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
	var sh *SatirliHata
	e := satirEki(4, errors.New("iç"))
	if !errors.As(e, &sh) || sh.Satir != 4 || satirEki(9, e) != e {
		t.Fatalf("tipli satır hatası: %v", e)
	}
}

// Sonsuz ve tanımsız Ondalık değerler tensör biçimiyle yazılır; bilimsel yazım ayrıştırılır.
func TestOndalikYazdirmaVeBilimselYazim(t *testing.T) {
	for f, want := range map[float64]string{math.Inf(1): "+Sonsuz", math.Inf(-1): "-Sonsuz", math.NaN(): "NaN", 1e21: "1e+21", 2: "2.0", 0.5: "0.5"} {
		if got := (&FloatVal{Val: f}).Inspect(); got != want {
			t.Errorf("%v: beklenen %s, alınan %s", f, want, got)
		}
	}
	_, val, err := runTROut(t, `sonuç [1e-5, 3.2E+4, 2e3 == 2000, 1e-5 * 1e5]`)
	if err != nil || val.Inspect() != "[1e-05, 32000.0, doğru, 1.0]" {
		t.Fatalf("bilimsel yazım: %v %v", val, err)
	}
}

// atanabilirDeger indeksli atama ve dış biçime dönüşüm kancalarını sınar: a[i] = d, i
// konumuna d yazar; a[i] okuması yazılmış değeri (yoksa 0) verir.
type atanabilirDeger struct{ hucre map[int64]Value }

func (a *atanabilirDeger) Type() ValueType { return "Atanabilir" }
func (a *atanabilirDeger) Inspect() string { return "atanabilir" }
func (a *atanabilirDeger) IsTruthy() bool  { return true }
func (a *atanabilirDeger) Indeks(i Value) (Value, error) {
	if v, ok := a.hucre[i.(*IntVal).Val]; ok {
		return v, nil
	}
	return NewInt(0), nil
}
func (a *atanabilirDeger) IndekseAta(i, d Value) error {
	n := i.(*IntVal).Val
	if n < 0 {
		return errors.New("negatif indeks")
	}
	a.hucre[n] = d
	return nil
}
func (a *atanabilirDeger) DuzDeger() Value {
	return &ListVal{Elements: []Value{a.hucre[0], a.hucre[1]}}
}

func TestIndeksAtamaVeDuzDegerKancalari(t *testing.T) {
	a := &atanabilirDeger{hucre: map[int64]Value{}}
	vmx := New()
	vmx.GlobalEnv.Define("a", a, false)
	vmx.GlobalEnv.Define("l", &ListVal{Elements: []Value{a}}, false)
	p := parser.New(lexer.New("a[0] = 5\na[1] += 2\na[1] *= 4\nl[0][0] -= 1\na[-1] = 3"))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	_, err := vmx.Run(prog)
	if err == nil || err.Error() != "Satır 5: negatif indeks" {
		t.Fatalf("kanca hatası satır önekli olmalı: %v", err)
	}
	if got := TRToGoValue(a); fmt.Sprint(got) != "[4 8]" {
		t.Fatalf("atamalar ya da düz değer: %v", got)
	}
}

// yinelenenDeger n öğeli (öğe i: i*10) bir dış değerdir; n < 0 ise döngü kurulamaz.
type yinelenenDeger struct{ n int }

func (y *yinelenenDeger) Type() ValueType { return "Yinelenen" }
func (y *yinelenenDeger) Inspect() string { return "yinelenen" }
func (y *yinelenenDeger) IsTruthy() bool  { return true }
func (y *yinelenenDeger) YinelemeUzunlugu() (int, error) {
	if y.n < 0 {
		return 0, errors.New("üzerinde gezilemez")
	}
	return y.n, nil
}
func (y *yinelenenDeger) YinelemeOgesi(i int) (Value, error) { return NewInt(int64(i * 10)), nil }

// Yinelenebilir kancası 'döngü x içinde', 'döngü i, x içinde' ve liste üretecinde
// kullanılır; kanca hatası satır bilgisi taşır.
func TestYinelenebilirKancasi(t *testing.T) {
	calistirKod := func(kod string) (*VM, error) {
		vmx := New()
		vmx.GlobalEnv.Define("y", &yinelenenDeger{n: 4}, false)
		vmx.GlobalEnv.Define("bozuk", &yinelenenDeger{n: -1}, false)
		p := parser.New(lexer.New(kod))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		_, err := vmx.Run(prog)
		return vmx, err
	}
	vmx, err := calistirKod(`
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
	for ad, want := range map[string]string{
		"toplamı": "60", "çiftler": "[[0, 0], [1, 10], [2, 20]]", "üretilen": "[11, 21, 31]",
	} {
		if got, _ := vmx.GlobalEnv.Get(ad); got == nil || got.Inspect() != want {
			t.Errorf("%s: beklenen %s, alınan %v", ad, want, got)
		}
	}
	if _, err := calistirKod("\ndöngü x içinde bozuk { }"); err == nil || err.Error() != "Satır 2: üzerinde gezilemez" {
		t.Errorf("döngü kanca hatası satır önekli olmalı: %v", err)
	}
	if _, err := calistirKod("\n\nyazdır([x döngü x içinde bozuk])"); err == nil || err.Error() != "Satır 3: üzerinde gezilemez" {
		t.Errorf("üreteç kanca hatası satır önekli olmalı: %v", err)
	}
}
