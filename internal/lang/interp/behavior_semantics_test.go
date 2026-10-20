package interp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// runTROut programı çalıştırır ve yazdır() çıktısını da döndürür.
func runTROut(t *testing.T, code string) (string, Value, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	var out bytes.Buffer
	machine := New()
	machine.Out = &out
	val, err := machine.Run(prog)
	machine.WaitAsync()
	return out.String(), val, err
}

// Tanımsız bir isme atama eskiden en dış kapsamda sessizce genel değişken oluşturuyordu.
func TestAssignToUndeclaredNameIsError(t *testing.T) {
	_, _, err := runTROut(t, "fonksiyon f() { gizli = 42 }\nf()\nyazdır(gizli)")
	if !messages.Has(err, messages.H2448) {
		t.Fatalf("tanımsız değişken hatası bekleniyordu, alınan: %v", err)
	}

	_, _, err = runTROut(t, "toplm = 1")
	if !messages.Has(err, messages.H2448) {
		t.Fatalf("en dış kapsamda da hata bekleniyordu, alınan: %v", err)
	}

	// Tanımlı değişkenlere iç kapsamlardan atama çalışmaya devam etmeli.
	val, err := runTR(t, `
		değişken sayaç = 0
		fonksiyon artır() { sayaç = sayaç + 1 }
		artır()
		artır()
		döngü i içinde 1..=3 { sayaç += i }
		sonuç sayaç
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
		{"2 ** -1", "0.5"},
		{"2 * 3 ** 2", "18"},
		{"-2 * 3", "-6"},
		{"değil 1 == 2", "doğru"},
		{"değil 1 < 2", "yanlış"},
		{"değil yanlış ve yanlış", "yanlış"}, // (değil yanlış) ve yanlış
		{"!yanlış == yanlış", "yanlış"},      // '!' sıkı bağlanır: (!yanlış) == yanlış
		{"2 ** 10", "1024"},
		{"2.0 ** 2", "4.0"},
	}
	for _, c := range cases {
		val, err := runTR(t, "sonuç "+c.code)
		if err != nil {
			t.Errorf("%s: beklenmeyen hata: %v", c.code, err)
			continue
		}
		if val.Inspect() != c.want {
			t.Errorf("%s = %s bekleniyordu, %s alındı", c.code, c.want, val.Inspect())
		}
	}

	val, _ := runTR(t, "sonuç 3 ** 2")
	if val.Type() != VAL_INT {
		t.Fatalf("Tam ** Tam sonucu Tam olmalı, %s alındı", val.Type())
	}
	if _, err := runTR(t, "sonuç 10 ** 40"); !messages.Has(err, messages.H2472) {
		t.Fatalf("taşma hatası bekleniyordu, alınan: %v", err)
	}
}

func TestNewlineEndsCallOrIndex(t *testing.T) {
	out, _, err := runTROut(t, "değişken f = 5\n(1 + 2) |> yazdır\ndeğişken l = [1, 2]\n[3, 4] |> yazdır\nyazdır(f, l)")
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if out != "3\n[3, 4]\n5 [1, 2]\n" {
		t.Fatalf("beklenmeyen çıktı: %q", out)
	}

	// Aynı satırdaki çağrı/indeks ve çok satırlı argümanlar çalışmaya devam etmeli.
	out, _, err = runTROut(t, "değişken l = [10, 20]\nyazdır(\n  l[1],\n  l [0]\n)")
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if out != "20 10\n" {
		t.Fatalf("beklenmeyen çıktı: %q", out)
	}
}

func TestStringBracesAndEscapes(t *testing.T) {
	out, _, err := runTROut(t, `
		değişken ad = "Ali"
		değişken p = {"ad": "Veli"}
		yazdır("\{ad\} = {ad}")
		yazdır("{p["ad"]} ve {p['ad']}")
		yazdır("C:\\{ad}")
		yazdır("sonra")
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
	out, _, err := runTROut(t, `test "atlanmalı" { yazdır("TEST ÇALIŞTI") }
doğrula(1 + 1 == 2)
eşit_mi([1, 2], [1, 2])
yazdır("tamam")`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if strings.Contains(out, "TEST ÇALIŞTI") || !strings.Contains(out, "tamam") {
		t.Fatalf("çıktı yanlış: %q", out)
	}
	_, _, err = runTROut(t, `eşit_mi("5", 5)`)
	if !messages.Has(err, messages.H2458) {
		t.Fatalf("eşit_mi hata mesajı yanlış: %v", err)
	}
	_, _, err = runTROut(t, "\n\ndoğrula(yanlış)")
	if !codeAt(err, messages.H2457, 3) {
		t.Fatalf("doğrula hata mesajı yanlış: %v", err)
	}
}

// Tam sayı toplama/çıkarma/çarpma taşması sessizce sarmak yerine hata verir ('**' gibi).
func TestIntegerOverflowIsError(t *testing.T) {
	for _, codeVal := range []string{"sonuç 9223372036854775807 + 1", "sonuç -9223372036854775807 - 2", "sonuç 4294967296 * 4294967296"} {
		if _, err := runTR(t, codeVal); !messages.Has(err, messages.H2473) {
			t.Fatalf("%s: taşma hatası bekleniyordu, alınan: %v", codeVal, err)
		}
	}
	val, err := runTR(t, "sonuç 9223372036854775806 + 1 - 5 * 2")
	if err != nil || val.Inspect() != "9223372036854775797" {
		t.Fatalf("sınır içindeki işlem bozuldu: %v %v", val, err)
	}
}

// Ondalık sayılarda '%' (kalan) desteklenir.
func TestFloatModulo(t *testing.T) {
	val, err := runTR(t, "sonuç 5.5 % 2")
	if err != nil || val.Inspect() != "1.5" {
		t.Fatalf("5.5 %% 2 = 1.5 bekleniyordu, alınan: %v %v", val, err)
	}
	if _, err := runTR(t, "sonuç 1.5 % 0"); err == nil {
		t.Fatal("sıfıra göre mod hata vermedi")
	}
}

// Yerleşik bir fonksiyon adıyla tanımlanan değişken sabit kalmamalı ('toplam += 5' çalışmalı);
// kullanıcının kendi 'sabit' tanımları ise korunmalı.
func TestShadowingBuiltinCreatesMutableVariable(t *testing.T) {
	val, err := runTR(t, `
		değişken toplam = 0
		toplam += 5
		değişken uzunluk = 1
		uzunluk = uzunluk + 1
		sonuç [toplam, uzunluk]`)
	if err != nil {
		t.Fatalf("yerleşik adı gölgeleyen değişken değiştirilebilmeli: %v", err)
	}
	if val.Inspect() != "[5, 2]" {
		t.Fatalf("[5, 2] bekleniyordu, %s alındı", val.Inspect())
	}
	if _, err := runTR(t, "sabit PI_DEGERI = 3\nPI_DEGERI = 4"); !messages.Has(err, messages.H2447) {
		t.Fatalf("kullanıcı sabiti korunmalı, alınan: %v", err)
	}
}
