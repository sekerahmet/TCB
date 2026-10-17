package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tr_dili/pkg/checker"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
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
		sonuç [a, b, kareler.toplam()]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Beklenmeyen hata: %v", err)
	}
	if val.Inspect() != "[20, 10, 35]" {
		t.Fatalf("[20, 10, 35] bekleniyordu, %s alındı", val.Inspect())
	}
}

func TestMatrixIndexingAndAssignment(t *testing.T) {
	code := `
		değişken M = matris([[1, 2], [3, 4]])
		M[0][1] = 10
		M[1][0] += 5
		M[1] = [8, 9]
		sonuç [M[0][1], M[1][0], M[1][1]]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Matris indeksleme/atama hatası: %v", err)
	}
	if val.Inspect() != "[10.0, 8.0, 9.0]" {
		t.Fatalf("[10.0, 8.0, 9.0] bekleniyordu, %s alındı", val.Inspect())
	}
}

func TestAutogradAndNeuralNet(t *testing.T) {
	code := `
		değişken x = tensör(3.0, türev_izle: doğru)
		değişken y = x * x + 2 * x + 1
		geri_yayılım(y)
		sonuç [y.değer, x.gradyan]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Autograd hatası: %v", err)
	}
	if val.Inspect() != "[16.0, 8.0]" {
		t.Fatalf("[16.0, 8.0] bekleniyordu, %s alındı", val.Inspect())
	}
}

func TestSQLTurkishCaseAndUTF8Safety(t *testing.T) {
	code := `
		değişken vt = veritabanı_aç(":bellek:")
		vt.ekle("çalışanlar", [
			{"id": 1, "ad": "Işık", "şehir": "İstanbul", "maaş": 90000},
			{"id": 2, "ad": "Ali",  "şehir": "Ankara",   "maaş": 60000},
			{"id": 3, "ad": "İrem", "şehir": "İstanbul", "maaş": 95000}
		])
		// Küçük harfli Türkçe SQL ('ı', 'İ', 'ş') ve standart SQL AND/OR testi
		değişken s1 = vt.sorgula("seç ad, maaş tablo çalışanlar nerede şehir = 'İstanbul' AND maaş >= 90000 sırala maaş azalan limit 2")
		değişken silinen = vt.sorgula("sil tablo çalışanlar nerede id <> 1 ve id <> 3")
		sonuç [s1.uzunluk, s1[0]["ad"], silinen, vt.say("çalışanlar")]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("SQL hatası: %v", err)
	}
	if val.Inspect() != `[2, "İrem", 1, 2]` {
		t.Fatalf(`[2, "İrem", 1, 2] bekleniyordu, %s alındı`, val.Inspect())
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

func TestCheckerAndBytecode(t *testing.T) {
	code := `
		sabit MAX: Tam = 100
		değişken a, b = 1, 2
		değişken liste = [x * 2 döngü x içinde 1..=5 eğer x > 2]
		dene {
			eğer a > 0 {
				a += 1
			} değilse eğer a == 0 {
				a = 0
			} değilse {
				fırlat "hata"
			}
		} yakala h {
			yazdır(h)
		}
	`
	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parser hatası: %v", p.Errors())
	}
	chk := checker.New()
	if errs := chk.CheckProgram(prog); len(errs) > 0 {
		t.Fatalf("Checker hatası: %v", errs)
	}
	chunk := CompileToBytecode(prog)
	out := chunk.Disassemble()
	if !strings.Contains(out, "LİSTE_ÜRETECİ") || !strings.Contains(out, "DENE_BAŞLAT") {
		t.Fatalf("Bytecode çıktısında beklenen komutlar eksik:\n%s", out)
	}
}
