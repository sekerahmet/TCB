package interp

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Faz 2 — dilin imzası: 'o' zamiri ve Türkçe hata mesajları

func TestTrailingBlockWithOPronoun(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		TR_var sayılar = [1, 2, 3, 4, 5, 6]
		TR_var a = sayılar.TR_filter((x) => x > 2).TR_map((x) => x * x)
		TR_var b = sayılar.TR_filter((x) => x % 2 == 0)
		TR_var c = [{"ad": "Ali"}, {"ad": "Ayşe"}].TR_map((x) => x["ad"])
		TR_var d = [[1, 2], [3, 4]].TR_map((x) => x.TR_map((x2) => x2 * 10))
		TR_func uygula(f: (TR_Int) -> TR_Int) -> TR_Int { TR_return f(21) }
		TR_var e = uygula((x) => x * 2)
		TR_var result = [a, b, c, d, e]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[[9, 16, 25, 36], [2, 4, 6], ["Ali", "Ayşe"], [[10, 20], [30, 40]], 42]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
}

func TestTrailingBlockEquivalentToLambda(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		TR_var l = [5, 3, 8, 1]
		TR_var result = l.TR_filter((x) => x > 2) == l.TR_filter((x) => x > 2)`)
	if err != nil {
		t.Fatal(err)
	}
	if val.Inspect() != "true" {
		t.Fatalf("sondaki blok ile ok fonksiyonu aynı sonucu vermeli, %s alındı", val.Inspect())
	}
}

// Kontrol yapısı başlıklarında '{' gövdenin başlangıcıdır; sondaki blok olarak okunmamalı.
func TestControlHeadersDoNotTakeTrailingBlocks(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		TR_var sayılar = [1, 2, 3, 4, 5, 6]
		TR_var izler = []
		TR_if sayılar.TR_length > 3 {
			izler.TR_append("eğer")
		}
		TR_for s TR_in sayılar.TR_filter((x) => x < 3) {
			izler.TR_append("döngü-\(s)")
		}
		TR_var i = 0
		TR_for i < sayılar.TR_length {
			i += 3
		}
		izler.TR_append("iken-\(i)")
		TR_var sonuç_eşle = TR_match sayılar.TR_length {
			6 => "altı"
			_ => "başka"
		}
		izler.TR_append(sonuç_eşle)
		# Parantez içinde sondaki blok başlıkta da kullanılabilir
		TR_if (sayılar.TR_filter((x) => x > 5)).TR_length == 1 {
			izler.TR_append("parantez")
		}
		TR_var result = izler`)
	if err != nil {
		t.Fatal(err)
	}
	want := `["eğer", "döngü-1", "döngü-2", "iken-6", "altı", "parantez"]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
}

// İki fonksiyon: f'nin ve g'nin ikinci parametresi varsayılanlı. f(1) tanımın adıyla çağrıdır,
// varsayılan kullanılabilir; yuvaya g konunca f(1) değer üzerinden çağrıdır (H2710, 6.9).
const twoDefaults = `TR_func f(x: TR_Int, y: TR_Int = 1) -> TR_Int {
    TR_return x + y
}
TR_func g(x: TR_Int, y: TR_Int = 10) -> TR_Int {
    TR_return x * 100 + y
}
TR_func h() -> TR_Int {
    TR_return f(1)
}
`

// Kural (6.9): çağrı tanımın adıyla yapılıyorsa varsayılan kullanılır; tanımın adına başka bir
// fonksiyon atanınca (f = g) aynı çağrı yeri yuvadaki değeri çağırır ve değer üzerinden çağrının
// kuralı çalışır. İç içe tanım her dış çağrıda yeni bir değerdir; tanımın adıyla çağrısı yine
// doğrudandır. (Çağrı yeri tanımı gövdesiyle tanır, compiler_call.go positionalCall.)
func TestACallByTheDefinitionsNameFollowsTheSlot(t *testing.T) {
	const source = twoDefaults + `TR_func dış(n: TR_Int) -> TR_Int {
    TR_func iç(x: TR_Int, y: TR_Int = n) -> TR_Int {
        TR_return x + y
    }
    TR_return iç(1)
}
TR_print(f(1), h(), dış(1), dış(5))
f = g
TR_print(f(1, 2), TR_error_code { f(1) }, TR_error_code { h() })
`
	out, _, err := runTROut(t, source)
	const want = "2 2 2 6\n102 H2710 H2710\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

// Kural (6.9, kabuk ve hata ayıklayıcı): her kabuk girdisi ayrı derlenir; sonraki girdi önceki
// girdinin tanımının adına yazabilir. Hata ayıklayıcının değerlendirmesi de (ExecStatements) tanımı
// yeniden yükleyebilir. Önceki girdinin derlenmiş çağrısı yuvadaki yeni değeri çağırır.
func TestAnEarlierInputsCallSeesALaterAssignment(t *testing.T) {
	machine := New()
	var out bytes.Buffer
	machine.Out = &out
	run := func(code string) {
		t.Helper()
		if _, err := machine.Run(mustParseProgram(t, code)); err != nil {
			t.Fatalf("%q: %v", code, err)
		}
	}
	run(twoDefaults)
	run("TR_print(h())\n")
	// Hata ayıklayıcının değerlendirmesi: tanım yeniden yüklenir, adı ve imzası aynı.
	redefine := mustParseProgram(t, "TR_func f(x: TR_Int, y: TR_Int = 50) -> TR_Int {\n    TR_return x + y\n}\n")
	if _, err := machine.ExecStatements(redefine.Statements, machine.GlobalEnv); err != nil {
		t.Fatal(err)
	}
	run("TR_print(h())\n")
	run("f = g\n")
	run("TR_print(TR_error_code { h() })\n")
	if got, want := out.String(), "2\n51\nH2710\n"; got != want {
		t.Fatalf("%q beklenirdi, gelen %q", want, got)
	}
}

// Kural (3.12): tek argümanlı tip çağrısı (callAt'in hızlı yolu) eski yolla aynı sonucu ve aynı hatayı
// verir: hatası çağrının satırındadır, argümanın hatası olduğu gibi geçer. Yapı tipi, argümansız ve
// iki argümanlı tip çağrısı eski yoldan gider.
func TestAOneArgumentTypeCallIsTheTypesCall(t *testing.T) {
	const source = `TR_struct N {
    x: TR_Int
}
TR_var l = [1, 2]
TR_var k = TR_List(l)
k.TR_append(3)
TR_print(TR_String(5), TR_Int("12"), TR_Float(2), l, k, TR_Type(N(x: 1)), N(x: 2).x)
TR_print(TR_error_code { TR_Int("a") }, TR_error_code { TR_Int(1 // 0) }, TR_error_code { TR_Dict() }, TR_error_code { TR_Range(3) })
TR_var y = TR_Int("b")
`
	out, _, err := runTROut(t, source)
	const want = "5 12 2.0 [1, 2] [1, 2, 3] N 2\nH2640 H2470 H2645 H2657\n"
	var lineErr *LineError
	if out != want || !messages.Has(err, messages.H2640) || !errors.As(err, &lineErr) || lineErr.Line != 9 {
		t.Fatalf("%q ve 9. satırda H2640 beklenirdi, gelen %q (%v)", want, out, err)
	}
}
