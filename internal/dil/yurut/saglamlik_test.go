package yurut

import (
	"io"
	"strings"
	"testing"
	"time"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
)

// runTRWithin kodu bir süre sınırıyla çalıştırır; kilitlenme (deadlock) testi asmak yerine başarısız olur.
func runTRWithin(t *testing.T, limit time.Duration, code string) (*VM, Value, error) {
	t.Helper()
	p := ayristir.New(sozcuk.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	machine := New()
	machine.Out = io.Discard
	type result struct {
		val Value
		err error
	}
	done := make(chan result, 1)
	go func() {
		val, err := machine.Run(prog)
		machine.WaitAsync()
		done <- result{val, err}
	}()
	select {
	case r := <-done:
		return machine, r.val, r.err
	case <-time.After(limit):
		t.Fatalf("program %v içinde bitmedi (kilitlenme?)", limit)
		return nil, nil, nil
	}
}

// ---------------------------------------------------------------------------
// Çökmeler: panik yerine anlaşılır hata
// ---------------------------------------------------------------------------

func TestRunRecoversFromBuiltinPanic(t *testing.T) {
	machine := New()
	machine.Out = io.Discard
	machine.RegisterBuiltin("patla", func(y *VM, args []Value, named map[string]Value) (Value, error) {
		var m map[string]int
		m["x"] = 1 // nil map paniği
		return NULL, nil
	})
	prog := ayristir.New(sozcuk.New(`
		değişken çalıştı = yanlış
		ertele { çalıştı = doğru }
		patla()`)).ParseProgram()
	_, err := machine.Run(prog)
	if err == nil || !strings.Contains(err.Error(), "çökme önlendi") {
		t.Fatalf("panik hataya dönüştürülmeliydi, alınan: %v", err)
	}
	if v, _ := machine.GlobalEnv.Get("çalıştı"); v.Inspect() != "doğru" {
		t.Fatal("panik sonrasında da 'ertele' blokları çalışmalı")
	}
	// Panik sonrası kilit bırakılmış olmalı: aynı VM yeniden kullanılabilir (REPL)
	if _, err := machine.Run(ayristir.New(sozcuk.New("1 + 1")).ParseProgram()); err != nil {
		t.Fatalf("panik sonrası VM kullanılamıyor: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Eşzamanlılık
// ---------------------------------------------------------------------------

// Eskiden: "fatal error: concurrent map writes" ile süreç çöküyordu (recover edilemez).
func TestAsyncConcurrentDictAndListWrites(t *testing.T) {
	_, val, err := runTRWithin(t, 20*time.Second, `
		değişken d = {}
		değişken l = []
		döngü i içinde 0..3000 {
			eşzamanlı {
				d[metin(i)] = i
				l.ekle(i)
			}
		}
		bekle_hepsi()
		sonuç [d.anahtarlar().uzunluk, l.uzunluk]`)
	if err != nil {
		t.Fatal(err)
	}
	if val.Inspect() != "[3000, 3000]" {
		t.Fatalf("[3000, 3000] bekleniyordu, %s alındı", val.Inspect())
	}
}

// Eskiden: 20000 artırımdan yalnızca ~900'ü kalıyordu (kayıp güncelleme).
func TestAsyncCounterIncrementIsAtomic(t *testing.T) {
	_, val, err := runTRWithin(t, 20*time.Second, `
		değişken s = 0
		döngü i içinde 0..5000 {
			eşzamanlı { s += 1 }
		}
		bekle_hepsi()
		sonuç s`)
	if err != nil {
		t.Fatal(err)
	}
	if val.Inspect() != "5000" {
		t.Fatalf("5000 bekleniyordu, %s alındı", val.Inspect())
	}
}

// Kilit bekle() sırasında bırakıldığı için görevler hâlâ iç içe (eşzamanlı) ilerlemeli.
// bekle(sn) kilidi bıraktığı için görevler aynı anda bekler. Süre ölçülmez (yüklü makinede
// ya da yarış dedektörü altında kararsız olurdu): aynı anda bekleyen görev sayısı sayılır;
// görevler sıraya girseydi en çok 1 olurdu. Süre sınırı yalnızca takılmaya karşıdır.
func TestAsyncTasksStillOverlapDuringSleep(t *testing.T) {
	_, val, err := runTRWithin(t, 30*time.Second, `
		değişken biten = 0
		değişken bekleyen = 0
		değişken en_cok = 0
		döngü i içinde 0..20 {
			eşzamanlı {
				bekleyen += 1
				en_cok = maks(en_cok, bekleyen)
				bekle(0.2)
				bekleyen -= 1
				biten += 1
			}
		}
		bekle_hepsi()
		sonuç [biten, en_cok]`)
	if err != nil {
		t.Fatal(err)
	}
	l, ok := val.(*ListVal)
	if !ok || l.Elements[0].Inspect() != "20" {
		t.Fatalf("20 görev bitmeliydi: %s", val.Inspect())
	}
	if n, _ := l.Elements[1].(*IntVal); n == nil || n.Val < 2 {
		t.Fatalf("görevler aynı anda beklemeliydi; aynı anda bekleyen en çok %s görev (sıraya girmişler)", l.Elements[1].Inspect())
	}
}

// Eskiden: "fatal error: all goroutines are asleep - deadlock!"
func TestWaitAllInsideAsyncDoesNotDeadlock(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		değişken sonra = yanlış
		eşzamanlı {
			bekle_hepsi()
			sonra = doğru
		}
		bekle_hepsi()
		sonuç sonra`)
	if err != nil {
		t.Fatal(err)
	}
	// Görev hatayla (bekle_hepsi reddedildi) sonlanır; sonraki satır çalışmaz ama program kilitlenmez
	if val.Inspect() != "yanlış" {
		t.Fatalf("yanlış bekleniyordu, %s alındı", val.Inspect())
	}
}

func TestDeferInsideAsyncRunsWhenTaskEnds(t *testing.T) {
	machine, val, err := runTRWithin(t, 5*time.Second, `
		değişken sira = []
		ertele { sira.ekle("program-ertele") }
		fonksiyon f() {
			eşzamanlı {
				ertele { sira.ekle("görev-ertele") }
				sira.ekle("görev")
			}
			bekle_hepsi()
			sira.ekle("f-son")
		}
		f()
		eşzamanlı {
			bekle(0.05)
			sira.ekle("geç-görev")
		}
		sonuç sira`)
	if err != nil {
		t.Fatal(err)
	}
	_ = val
	got, _ := machine.GlobalEnv.Get("sira")
	want := `["görev", "görev-ertele", "f-son", "geç-görev", "program-ertele"]`
	if got.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, got.Inspect())
	}
}

func TestPanicInsideAsyncDoesNotCrashProcess(t *testing.T) {
	machine := New()
	machine.Out = io.Discard
	machine.RegisterBuiltin("patla", func(y *VM, args []Value, named map[string]Value) (Value, error) {
		panic("görev içinde beklenmedik hata")
	})
	prog := ayristir.New(sozcuk.New(`
		değişken tamam = yanlış
		eşzamanlı { patla() }
		bekle_hepsi()
		tamam = doğru`)).ParseProgram()
	if _, err := machine.Run(prog); err != nil {
		t.Fatalf("ana program etkilenmemeli: %v", err)
	}
	if v, _ := machine.GlobalEnv.Get("tamam"); v.Inspect() != "doğru" {
		t.Fatal("görev paniğinden sonra ana program devam etmeliydi")
	}
}

// Test raporu K1: sınırsız özyineleme Go yığınını taşırıp süreci yakalanamaz biçimde
// çökertiyordu. Artık sınırda 'dene/yakala' ile yakalanabilen bir hata döner.
func TestDeepRecursionIsCatchableError(t *testing.T) {
	_, _, err := runTRWithin(t, 30*time.Second, "fonksiyon f(n) { sonuç f(n + 1) }\nf(0)")
	if err == nil || !strings.Contains(err.Error(), "Özyineleme derinliği sınırı aşıldı") {
		t.Fatalf("özyineleme sınırı hatası bekleniyordu, alınan: %v", err)
	}
	_, val, err := runTRWithin(t, 30*time.Second, `
		fonksiyon f(n) { sonuç f(n + 1) }
		değişken yakalandı = yanlış
		dene { f(0) } yakala hata { yakalandı = doğru }
		sonuç yakalandı`)
	if err != nil || val.Inspect() != "doğru" {
		t.Fatalf("özyineleme hatası dene/yakala ile yakalanmalıydı: %v / %v", val, err)
	}
	// Sınırın altındaki derin ama sonlu özyineleme çalışmaya devam etmeli
	_, val, err = runTRWithin(t, 30*time.Second, "fonksiyon say(n) { eğer n == 0 { sonuç 0 } sonuç 1 + say(n - 1) }\nsonuç say(5000)")
	if err != nil || val.Inspect() != "5000" {
		t.Fatalf("5000 derinlik çalışmalı: %v / %v", val, err)
	}
}

// Test raporu Y4: eksik argüman sessizce 'yok' oluyor, fazlası yok sayılıyordu.
func TestUserFunctionArityIsChecked(t *testing.T) {
	cases := []struct{ code, want string }{
		{"fonksiyon topla(a, b) { sonuç a + b }\ntopla(1)", "Satır 2: 'topla' fonksiyonu 2 argüman bekler, 1 verildi (eksik: 'b')"},
		{"fonksiyon topla(a, b) { sonuç a + b }\ntopla(1, 2, 3)", "Satır 2: 'topla' fonksiyonu en fazla 2 argüman alır, 3 verildi"},
		{"fonksiyon f(isim) { sonuç isim }\nf(isimm: 1)", "'f' fonksiyonu 'isimm' adında bir parametre almaz. Bunu mu demek istediniz: 'isim'?"},
		{"değişken g = (x, y) => x + y\ng(1)", "İsimsiz fonksiyon 2 argüman bekler, 1 verildi"},
		{"fonksiyon iki(a, b) { sonuç a }\n5 |> iki", "'iki' fonksiyonu 2 argüman bekler, 1 verildi"},
	}
	for _, c := range cases {
		_, _, err := runTRWithin(t, 5*time.Second, c.code)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q içeren hata bekleniyordu, alınan: %v", c.want, err)
		}
	}
	// Varsayılan değerler, adlandırılmış argümanlar ve geri çağrılar (fazladan argüman alabilir) çalışmalı
	_, val, err := runTRWithin(t, 5*time.Second, `
		fonksiyon selam(ad, ek = "!") { sonuç ad + ek }
		değişken a = selam("Ali")
		değişken b = selam(ek: "?", ad: "Ayşe")
		değişken c = [1, 2, 3].filtrele((x) => x > 1)
		sonuç [a, b, c]`)
	if err != nil || val.Inspect() != `["Ali!", "Ayşe?", [2, 3]]` {
		t.Fatalf("geçerli çağrılar çalışmalı: %v / %v", val, err)
	}
}
