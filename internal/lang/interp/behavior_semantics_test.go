package interp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// runTROut programı çalıştırır ve yazdır() çıktısını da döndürür. Programın sonucu 'result' adlı
// genel değişkendir (programın en dışında 'TR_return' yazılmaz, 1.10); yoksa son değer.
func runTROut(t *testing.T, code string) (string, Value, error) {
	t.Helper()
	prog := mustParseProgram(t, code)
	var out bytes.Buffer
	machine := New()
	machine.Out = &out
	val, err := machine.Run(prog)
	machine.WaitAsync()
	return out.String(), programResult(machine, val), err
}

// Tanımsız bir isme atama eskiden en dış kapsamda sessizce genel değişken oluşturuyordu.
func TestAssignToUndeclaredNameIsError(t *testing.T) {
	_, _, err := runTROut(t, "TR_func f() { gizli = 42 }\nf()\nTR_print(gizli)")
	if !messages.Has(err, messages.H2448) {
		t.Fatalf("tanımsız değişken hatası bekleniyordu, alınan: %v", err)
	}

	_, _, err = runTROut(t, "toplm = 1")
	if !messages.Has(err, messages.H2448) {
		t.Fatalf("en dış kapsamda da hata bekleniyordu, alınan: %v", err)
	}

	// Tanımlı değişkenlere iç kapsamlardan atama çalışmaya devam etmeli.
	val, err := runTR(t, `
		TR_var sayaç = 0
		TR_func artır() { sayaç = sayaç + 1 }
		artır()
		artır()
		TR_for i TR_in 1..=3 { sayaç += i }
		TR_var result = sayaç
	`)
	if err != nil || val.Inspect() != "8" {
		t.Fatalf("8 bekleniyordu, alınan: %v, %v", val, err)
	}
}

func TestOperatorPrecedence(t *testing.T) {
	cases := []struct{ code, want string }{
		{"2 ** 3 ** 2", "512"},
		{"-2 ** 2", "-4"},
		{"(-2) ** 2", "4"},
		{"2.0 ** -1", "0.5"},
		{"2 * 3 ** 2", "18"},
		{"-2 * 3", "-6"},
		{"TR_not 1 == 2", "doğru"},
		{"TR_not 1 < 2", "yanlış"},
		{"TR_not TR_false TR_and TR_false", "yanlış"}, // (TR_not TR_false) TR_and TR_false
		{"TR_not TR_false == TR_false", "yanlış"},     // TR_not (TR_false == TR_false)
		{"2 ** 10", "1024"},
		{"2.0 ** 2", "4.0"},
	}
	for _, c := range cases {
		val, err := runTR(t, "TR_var result = "+c.code)
		if err != nil {
			t.Errorf("%s: beklenmeyen hata: %v", c.code, err)
			continue
		}
		if val.Inspect() != c.want {
			t.Errorf("%s = %s bekleniyordu, %s alındı", c.code, c.want, val.Inspect())
		}
	}

	val, _ := runTR(t, "TR_var result = 3 ** 2")
	if val.Type() != ValInt {
		t.Fatalf("Tam ** Tam sonucu Tam olmalı, %s alındı", val.Type())
	}
	if _, err := runTR(t, "TR_var result = 10 ** 40"); !messages.Has(err, messages.H2472) {
		t.Fatalf("taşma hatası bekleniyordu, alınan: %v", err)
	}
}

func TestNewlineEndsCallOrIndex(t *testing.T) {
	out, _, err := runTROut(t, "TR_var f = 5\n(TR_print)(3)\nTR_var l = [1, 2]\n[TR_print][0]([3, 4])\nTR_print(f, l)")
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if out != "3\n[3, 4]\n5 [1, 2]\n" {
		t.Fatalf("beklenmeyen çıktı: %q", out)
	}

	// Aynı satırdaki çağrı/indeks ve çok satırlı argümanlar çalışmaya devam etmeli.
	out, _, err = runTROut(t, "TR_var l = [10, 20]\nTR_print(\n  l[1],\n  l [0]\n)")
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if out != "20 10\n" {
		t.Fatalf("beklenmeyen çıktı: %q", out)
	}
}

func TestStringBracesAndEscapes(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_var ad = "Ali"
		TR_var p = {"ad": "Veli"}
		TR_print("{ad} = \(ad)")
		TR_print("\(p["ad"]) ve \(p["ad"])")
		TR_print("C:\\\(ad)")
		TR_print("sonra")
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	want := "{ad} = Ali\nVeli ve Veli\nC:\\Ali\nsonra\n"
	if out != want {
		t.Fatalf("çıktı %q bekleniyordu, %q alındı", want, out)
	}
}

// Test blokları normal çalıştırmada atlanır; doğrula/eşit_mi her yerde çalışır.
func TestTestBlocksSkippedAndAssertions(t *testing.T) {
	out, _, err := runTROut(t, `TR_test "atlanmalı" { TR_print("TEST ÇALIŞTI") }
TR_assert(1 + 1 == 2)
TR_assert_equal([1, 2], [1, 2])
TR_print("tamam")`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if strings.Contains(out, "TEST ÇALIŞTI") || !strings.Contains(out, "tamam") {
		t.Fatalf("çıktı yanlış: %q", out)
	}
	_, _, err = runTROut(t, `TR_assert_equal("5", 5)`)
	if !messages.Has(err, messages.H2458) {
		t.Fatalf("eşit_mi hata mesajı yanlış: %v", err)
	}
	_, _, err = runTROut(t, `TR_assert_equal("5", 5, "ek not")`)
	if !messages.Has(err, messages.H2517) {
		t.Fatalf("üç argümanlı eşit_mi hata mesajı yanlış: %v", err)
	}
	_, _, err = runTROut(t, "\n\nTR_assert(TR_false)")
	if !codeAt(err, messages.H2457, 3) {
		t.Fatalf("doğrula hata mesajı yanlış: %v", err)
	}
}

// Tam sayı toplama/çıkarma/çarpma taşması sessizce sarmak yerine hata verir ('**' gibi).
func TestIntegerOverflowIsError(t *testing.T) {
	for _, codeVal := range []string{"TR_var result = 9223372036854775807 + 1", "TR_var result = -9223372036854775807 - 2", "TR_var result = 4294967296 * 4294967296"} {
		if _, err := runTR(t, codeVal); !messages.Has(err, messages.H2473) {
			t.Fatalf("%s: taşma hatası bekleniyordu, alınan: %v", codeVal, err)
		}
	}
	val, err := runTR(t, "TR_var result = 9223372036854775806 + 1 - 5 * 2")
	if err != nil || val.Inspect() != "9223372036854775797" {
		t.Fatalf("sınır içindeki işlem bozuldu: %v %v", val, err)
	}
}

// Ondalık sayılarda '%' (kalan) desteklenir.
func TestFloatModulo(t *testing.T) {
	val, err := runTR(t, "TR_var result = 5.5 % 2")
	if err != nil || val.Inspect() != "1.5" {
		t.Fatalf("5.5 %% 2 = 1.5 bekleniyordu, alınan: %v %v", val, err)
	}
	if _, err := runTR(t, "TR_var result = 1.5 % 0"); err == nil {
		t.Fatal("sıfıra göre mod hata vermedi")
	}
}

// Yerleşik bir fonksiyon adıyla tanımlanan değişken sabit kalmamalı ('toplam += 5' çalışmalı);
// kullanıcının kendi 'sabit' tanımları ise korunmalı.
func TestShadowingBuiltinCreatesMutableVariable(t *testing.T) {
	val, err := runTR(t, `
		TR_var toplam = 0
		toplam += 5
		TR_var uzunluk = 1
		uzunluk = uzunluk + 1
		TR_var result = [toplam, uzunluk]`)
	if err != nil {
		t.Fatalf("yerleşik adı gölgeleyen TR_var değiştirilebilmeli: %v", err)
	}
	if val.Inspect() != "[5, 2]" {
		t.Fatalf("[5, 2] bekleniyordu, %s alındı", val.Inspect())
	}
	if _, err := runTR(t, "TR_const PI_DEGERI = 3\nPI_DEGERI = 4"); !messages.Has(err, messages.H2447) {
		t.Fatalf("kullanıcı sabiti korunmalı, alınan: %v", err)
	}
}

// Kural: intOp hesabı intFastFunc ile ortaktır; hızlı yol düşünce hata türü işleçten ve
// işlenenlerden seçilir (sıfıra bölme H2470, sıfıra mod H2471, taşma H2473, eksi tam üs H2603,
// kaydırma miktarı H2606).
// Neden: iki ayrı kopya tutulurken biri değişince öbürü sessizce ayrışabiliyordu.
func TestIntOpErrorsAndFastPathAgree(t *testing.T) {
	const minInt, maxInt = int64(-1 << 63), int64(1<<63 - 1)
	cases := []struct {
		code uint8
		op   string
		a, b int64
		want messages.Code
		val  string
	}{
		{opAdd, "+", maxInt, 1, messages.H2473, ""},
		{opSub, "-", minInt, 1, messages.H2473, ""},
		{opMul, "*", maxInt, 2, messages.H2473, ""},
		{opDiv, "/", 1, 0, messages.H2470, ""},
		{opFloorDiv, "//", minInt, -1, messages.H2473, ""},
		{opFloorDiv, "//", 1, 0, messages.H2470, ""},
		{opPow, "**", 2, -1, messages.H2603, ""},
		{opShl, "<<", 1, 64, messages.H2606, ""},
		{opShr, ">>", 1, -1, messages.H2606, ""},
		{opMod, "%", 1, 0, messages.H2471, ""},
		{opAdd, "+", 2, 3, 0, "5"},
		{opDiv, "/", 6, 3, 0, "2.0"},
		{opDiv, "/", minInt, -1, 0, "9.223372036854776e+18"},
		{opFloorDiv, "//", -7, 2, 0, "-4"},
		{opMod, "%", -7, 3, 0, "2"},
		{opShr, ">>", -8, 1, 0, "-4"},
		{opDiv, "/", 7, 2, 0, "3.5"},
		{opMod, "%", 7, 3, 0, "1"},
		{opLt, "<", 1, 2, 0, "doğru"},
		{opEq, "==", 2, 2, 0, "doğru"},
		{opPow, "**", 2, 10, 0, "1024"},
	}
	for _, c := range cases {
		val, ok, err := intOp(c.code, c.op, c.a, c.b, 1)
		if !ok {
			t.Fatalf("%d %s %d: işleç ele alınmalıydı", c.a, c.op, c.b)
		}
		if c.want != 0 {
			if !messages.Has(err, c.want) {
				t.Errorf("%d %s %d: %v kodu bekleniyordu, alınan: %v", c.a, c.op, c.b, c.want, err)
			}
			continue
		}
		if err != nil || val.Inspect() != c.val {
			t.Errorf("%d %s %d = %v, %v; %s bekleniyordu", c.a, c.op, c.b, val, err, c.val)
		}
	}
}

// programResult programın 'result' adlı genel değişkenidir; tanımlı değilse Run'ın verdiği değer.
func programResult(machine *VM, last Value) Value {
	if r, ok := machine.GlobalEnv.Get("result"); ok {
		return r
	}
	return last
}
