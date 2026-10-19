package interp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

func runTR(t *testing.T, code string) (Value, error) {
	t.Helper()
	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	machine := New()
	val, err := machine.Run(prog)
	machine.WaitAsync()
	return val, err
}

func TestCoreLanguageAndMultiAssign(t *testing.T) {
	code := `
		değişken a, b = 10, 20
		a, b = b, a
		değişken kareler = [x * x döngü x içinde 1..=5 eğer x % 2 == 1]
		sonuç [a, b, toplam(kareler)]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Beklenmeyen hata: %v", err)
	}
	if val.Inspect() != "[20, 10, 35]" {
		t.Fatalf("[20, 10, 35] bekleniyordu, %s alındı", val.Inspect())
	}
}

func TestModuleImportWithAliasAndCircularProtection(t *testing.T) {
	tmpDir := t.TempDir()
	modPath := filepath.Join(tmpDir, "yardimci.tr")
	err := os.WriteFile(modPath, []byte(`
		sabit PI_SAYISI = 3.14
		fonksiyon iki_kati(x) { sonuç x * 2 }
	`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	mainFile := filepath.Join(tmpDir, "ana.tr")
	code := `
		aktar "yardimci.tr" olarak yrd
		sonuç yrd.iki_kati(21)
	`
	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()
	machine := New()
	machine.SourceFile = mainFile
	val, err := machine.Run(prog)
	if err != nil {
		t.Fatalf("Modül aktarım hatası: %v", err)
	}
	if val.Inspect() != "42" {
		t.Fatalf("42 bekleniyordu, %s alındı", val.Inspect())
	}

	// Döngüsel aktarım (circular import) testi
	circA := filepath.Join(tmpDir, "a.tr")
	circB := filepath.Join(tmpDir, "b.tr")
	_ = os.WriteFile(circA, []byte(`aktar "b.tr"`), 0644)
	_ = os.WriteFile(circB, []byte(`aktar "a.tr"`), 0644)

	l2 := lexer.New(`aktar "a.tr"`)
	p2 := parser.New(l2)
	prog2 := p2.ParseProgram()
	m2 := New()
	m2.SourceFile = circA
	_, err = m2.Run(prog2)
	if err == nil || !strings.Contains(err.Error(), "döngüsel aktarım") {
		t.Fatalf("Döngüsel aktarım hatası bekleniyordu, alınan: %v", err)
	}
}

// TestNewTaskVMYigini: eşzamanlı görevler başlatıldıkları yerin çağrı yığınını devralır;
// kopya bağımsızdır.
func TestNewTaskVMStack(t *testing.T) {
	mainSrc := New()
	mainSrc.CallStack = append(mainSrc.CallStack, StackFrame{FuncName: "dış", Line: 3}, StackFrame{FuncName: "iç", Line: 7})
	job := mainSrc.NewTaskVM()
	if job.StackDepth() != 2 || !job.asyncTask || job.CallStack[1].FuncName != "iç" {
		t.Fatalf("eşzamanlı görev üst yığını devralmalı: %+v", job.CallStack)
	}
	// Kopya bağımsız olmalı: görevin yığını büyüdüğünde ana yığın değişmez
	job.CallStack = append(job.CallStack, StackFrame{FuncName: "görev"})
	if mainSrc.StackDepth() != 2 {
		t.Fatalf("ana yığın değişmemeli, derinlik=%d", mainSrc.StackDepth())
	}
}

func TestConcurrentCallStackSafety(t *testing.T) {
	code := `
		fonksiyon hesapla(n) {
			sonuç n * n
		}
		döngü i içinde 1..=20 {
			eşzamanlı {
				hesapla(i)
			}
		}
		bekle_hepsi()
		sonuç doğru
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Eşzamanlı çalışma hatası: %v", err)
	}
	if val.Inspect() != "doğru" {
		t.Fatalf("doğru bekleniyordu, %s alındı", val.Inspect())
	}
}

// CPU-yoğun bir döngü sürerken eşzamanlı görev kilidi alabilmeli (kilit devri).
func TestLockHandoffConcurrentTask(t *testing.T) {
	begin := time.Now()
	_, val, err := runTROut(t, `
		değişken bitti = yanlış
		eşzamanlı { bitti = doğru }
		değişken n = 0
		iken !bitti && n < 200000000 { n += 1 }
		sonuç n < 200000000
	`)
	if err != nil || val.Inspect() != "doğru" {
		t.Fatalf("eşzamanlı görev döngü bitmeden çalışmalıydı: %v %v (%v)", val, err, time.Since(begin))
	}
}

// Ana döngü özyinelemeli çağrılardan oluşsa da (kilit fonksiyon çağrısı girişinde de devredilir)
// eşzamanlı görev döngü sürerken çalışabilmeli. Süreye değil olaya bakılır: görev 'durdur'u
// ayarlamasaydı döngü sınırına (100000000 tur) kadar sürerdi.
func TestLockHandoffRecursiveLoop(t *testing.T) {
	_, val, err := runTROut(t, `
		değişken durdur = yanlış
		fonksiyon dolas(n) {
			eğer durdur || n == 0 { sonuç n }
			sonuç dolas(n - 1)
		}
		eşzamanlı { durdur = doğru }
		değişken tur = 0
		iken !durdur && tur < 100000000 {
			dolas(50)
			tur += 1
		}
		sonuç tur
	`)
	if err != nil {
		t.Fatal(err)
	}
	if kind, ok := val.(*IntVal); !ok || kind.Val >= 100000000 {
		t.Fatalf("görev döngü bitmeden çalışmalıydı; döngü %v tur sürdü", val)
	}
}

type testKey struct{}

// Görev değeri yalnızca kendi VM görünümünde görünür; yeni görevler boş başlar.
func TestTaskValue(t *testing.T) {
	v := New()
	if v.TaskValue(testKey{}) != nil {
		t.Fatal("başlangıçta değer olmamalı")
	}
	v.SetTaskValue(testKey{}, 7)
	if v.TaskValue(testKey{}) != 7 {
		t.Fatalf("değer: %v", v.TaskValue(testKey{}))
	}
	if g := v.NewTaskVM(); g.TaskValue(testKey{}) != nil {
		t.Errorf("yeni görev değeri devralmamalı: %v", g.TaskValue(testKey{}))
	}
	g := v.NewTaskVM()
	g.SetTaskValue(testKey{}, 9)
	if v.TaskValue(testKey{}) != 7 {
		t.Error("görevin değeri ana görünüme sızdı")
	}
	v.SetTaskValue(testKey{}, nil)
	if v.TaskValue(testKey{}) != nil {
		t.Error("nil değer silmeli")
	}
}

// RegisterBuiltin testlerin kendi yardımcı yerleşiklerini verilen adla genel kapsama kaydeder.
// Kod listesini atlayan bir kapı olduğu için yalnızca bu paketin testlerinde vardır; dilin
// yerleşikleri RegisterBuiltinKod ile sözlük koduyla kaydedilir (adları internal/lang/codes'da).
func (v *VM) RegisterBuiltin(name string, fn BuiltinFunc) {
	v.GlobalEnv.Define(name, &BuiltinVal{Name: name, Fn: fn}, true)
}

// Eşzamanlı görev hataları sabit os.Stderr'e yazılıyor, düzenleyici oturumunda ve tr test
// çıktısında görünmüyordu. VM'in hata yazıcısına (Err) gider.
func TestTaskErrorGoesToErrWriter(t *testing.T) {
	var out, failure bytes.Buffer
	m := New()
	m.Out = &out
	m.Err = &failure
	prog := parser.New(lexer.New("eşzamanlı {\n    değişken x = 1 / 0\n}\nbekle_hepsi()\n")).ParseProgram()
	if _, err := m.Run(prog); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(failure.String(), "Eşzamanlı görev hatası (satır 1'de başlatılan): Satır 2: Sıfıra bölme hatası") {
		t.Fatalf("görev hatası Err'e yazılmalı: %q (çıktı %q)", failure.String(), out.String())
	}
	if New().Err == nil {
		t.Fatal("varsayılan hata yazıcısı standart hata olmalı")
	}
}
