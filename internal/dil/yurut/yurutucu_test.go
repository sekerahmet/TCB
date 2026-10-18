package yurut

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
)

func runTR(t *testing.T, code string) (Value, error) {
	t.Helper()
	l := sozcuk.New(code)
	p := ayristir.New(l)
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
	l := sozcuk.New(code)
	p := ayristir.New(l)
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

	l2 := sozcuk.New(`aktar "a.tr"`)
	p2 := ayristir.New(l2)
	prog2 := p2.ParseProgram()
	m2 := New()
	m2.SourceFile = circA
	_, err = m2.Run(prog2)
	if err == nil || !strings.Contains(err.Error(), "döngüsel aktarım") {
		t.Fatalf("Döngüsel aktarım hatası bekleniyordu, alınan: %v", err)
	}
}

// TestNewTaskVMYigini: istek görevleri (async=false) ana programın çağrı yığınını kopyalamaz
// (kilitsiz okunması veri yarışı olurdu), eşzamanlı görevler (async=true) devralır.
func TestNewTaskVMYigini(t *testing.T) {
	ana := New()
	ana.CallStack = append(ana.CallStack, StackFrame{FuncName: "dış", Line: 3}, StackFrame{FuncName: "iç", Line: 7})

	istek := ana.NewTaskVM(false)
	if istek.StackDepth() != 0 || istek.asyncTask {
		t.Fatalf("istek görevi boş yığınla ve eşzamanlı olmadan başlamalı: derinlik=%d async=%v", istek.StackDepth(), istek.asyncTask)
	}

	gorev := ana.NewTaskVM(true)
	if gorev.StackDepth() != 2 || !gorev.asyncTask || gorev.CallStack[1].FuncName != "iç" {
		t.Fatalf("eşzamanlı görev üst yığını devralmalı: %+v", gorev.CallStack)
	}
	// Kopya bağımsız olmalı: görevin yığını büyüdüğünde ana yığın değişmez
	gorev.CallStack = append(gorev.CallStack, StackFrame{FuncName: "görev"})
	if ana.StackDepth() != 2 {
		t.Fatalf("ana yığın değişmemeli, derinlik=%d", ana.StackDepth())
	}

	// Ana program yığına yazarken istek görevi oluşturmak yarış üretmemeli (go test -race ile anlamlı)
	dur := make(chan struct{})
	bitti := make(chan struct{})
	go func() {
		defer close(bitti)
		for {
			select {
			case <-dur:
				return
			default:
				ana.CallStack = append(ana.CallStack, StackFrame{FuncName: "f"})
				ana.CallStack = ana.CallStack[:len(ana.CallStack)-1]
			}
		}
	}()
	for i := 0; i < 1000; i++ {
		if ana.NewTaskVM(false).StackDepth() != 0 {
			t.Fatal("istek görevi boş yığınla başlamalı")
		}
	}
	close(dur)
	<-bitti
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
