package vm

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
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
	if err == nil || !strings.Contains(err.Error(), "tanımsız değişken: 'gizli'") {
		t.Fatalf("tanımsız değişken hatası bekleniyordu, alınan: %v", err)
	}

	_, _, err = runTROut(t, "toplm = 1")
	if err == nil || !strings.Contains(err.Error(), "önce 'değişken' ile tanımlayın") {
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
	if _, err := runTR(t, "sonuç 10 ** 40"); err == nil || !strings.Contains(err.Error(), "taşma") {
		t.Fatalf("taşma hatası bekleniyordu, alınan: %v", err)
	}
}

// d[1] ile d["1"] eskiden Inspect() ile aynı anahtara düşüyordu.
func TestDictKeysAreTypeAware(t *testing.T) {
	val, err := runTR(t, `
		değişken d = {1: "tam", "1": "metin"}
		d[doğru] = "mantık"
		d["doğru"] = "metin2"
		sonuç [d[1], d["1"], d[doğru], d["doğru"], uzunluk(d), d.içerir(1), d.içerir("2")]
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `["tam", "metin", "mantık", "metin2", 4, doğru, yanlış]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}

	// Döngü ve anahtarlar() özgün anahtar tiplerini geri vermeli.
	val, err = runTR(t, `
		değişken d = {1: "a", "b": 2}
		değişken tipler = []
		döngü k, v içinde d { tipler = tipler + [tip(k)] }
		sonuç [tipler, d.anahtarlar(), [tip(k) döngü k içinde d]]
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `[["Tam", "Metin"], [1, "b"], ["Tam", "Metin"]]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}

	// Yazdırma ve JSON dönüşümü metin anahtarlar için değişmemeli.
	out, _, err := runTROut(t, `
		değişken d = {"ad": "Ali", 1: "bir"}
		yazdır(d)
		yazdır(json_metin({"ad": "Ali"}))
		değişken j = json_çöz("{\"x\": 1}")
		yazdır(j["x"])
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	want := "{\"ad\": \"Ali\", 1: \"bir\"}\n{\"ad\":\"Ali\"}\n1\n"
	if out != want {
		t.Fatalf("çıktı %q bekleniyordu, %q alındı", want, out)
	}

	// Metin olmayan anahtarlar Go/JSON tarafına görünen hâlleriyle aktarılır.
	d := NewDictVal()
	d.SetValue(&IntVal{Val: 7}, &StringVal{Val: "yedi"})
	m := trToGoValue(d).(map[string]interface{})
	if m["7"] != "yedi" {
		t.Fatalf("trToGoValue anahtarı \"7\" olmalı, alınan: %v", m)
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

// yz_sor eskiden sunucu kapalıyken uydurma bir "yanıt" döndürüyordu.
func TestYzSorReportsUnreachableServer(t *testing.T) {
	port := freePort(t) // dinleyen yok
	_, err := runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: "http://127.0.0.1:%d/api/generate")`, port))
	if err == nil || !strings.Contains(err.Error(), "ulaşılamadı") {
		t.Fatalf("ulaşılamadı hatası bekleniyordu, alınan: %v", err)
	}

	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
	}))
	defer notFound.Close()
	_, err = runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: %q)`, notFound.URL))
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("404 hatası bekleniyordu, alınan: %v", err)
	}

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"response": "  Merhaba!  "}`)
	}))
	defer ok.Close()
	val, err := runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: %q)`, ok.URL))
	if err != nil || val.Inspect() != "Merhaba!" {
		t.Fatalf("\"Merhaba!\" bekleniyordu, alınan: %v, %v", val, err)
	}
}

// Test blokları normal çalıştırmada atlanır; doğrula/eşit_mi her yerde çalışır.
func TestTestBlocksSkippedAndAssertions(t *testing.T) {
	out, _, err := runTROut(t, `test "atlanmalı" { yazdır("TEST ÇALIŞTI") }
doğrula(1 + 1 == 2)
esit_mi([1, 2], [1, 2])
yazdır("tamam")`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if strings.Contains(out, "TEST ÇALIŞTI") || !strings.Contains(out, "tamam") {
		t.Fatalf("çıktı yanlış: %q", out)
	}
	_, _, err = runTROut(t, `eşit_mi("5", 5)`)
	if err == nil || !strings.Contains(err.Error(), `beklenen "5" (Metin), gerçek 5 (Tam)`) {
		t.Fatalf("eşit_mi hata mesajı yanlış: %v", err)
	}
	_, _, err = runTROut(t, "\n\ndoğrula(yanlış)")
	if err == nil || !strings.Contains(err.Error(), "Satır 3: doğrulama başarısız") {
		t.Fatalf("doğrula hata mesajı yanlış: %v", err)
	}
}
