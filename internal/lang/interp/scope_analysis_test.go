package interp

import (
	"sync"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Aynı ayrıştırılmış program birçok VM'de aynı anda derlenip çalıştırılabilmelidir: blok
// kapsam bayrakları eskiden ağaca (ast.BlockStatement.Flags) tembelce yazılıyordu ve iki VM
// aynı bloğu aynı anda derlerken yarışıyordu. Önbellek artık derleyicinin; ağaca yazılmaz.
// Yarış 'go test -race' ile görülür (tools/full_check.sh --kip=yaris, Linux); yarış
// dedektörü olmayan yerde de test her VM'in doğru sonucu verdiğini sınar.
func TestOneProgramCompiledInParallelVMs(t *testing.T) {
	const code = `
		fonksiyon sayaç_yap() {
			değişken n = 0
			sonuç fonksiyon() { n += 1; sonuç n }
		}
		değişken toplam_ = 0
		döngü i içinde 1..=20 {
			değişken kare = i * i
			eğer kare % 2 == 0 { değişken yarım = kare / 2; toplam_ += yarım } değilse { toplam_ += kare }
		}
		değişken s = sayaç_yap()
		değişken j = 0
		iken j < 5 { değişken k = j; s(); j += 1 }
		değişken kapanışlar = [(fonksiyon() { sonuç x }) döngü x içinde 1..=3]
		yapı Nokta { x = 0
			fonksiyon iki_kat() { değişken d = kendi.x * 2; sonuç d } }
		sonuç [toplam_, s(), kapanışlar[2](), Nokta(4).iki_kat()]`
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	const want = "[2100, 6, 3, 8]"
	const vms = 8
	var wg sync.WaitGroup
	results := make([]string, vms)
	errs := make([]error, vms)
	for i := range vms {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := New()
			val, err := m.Run(prog)
			m.WaitAsync()
			errs[i] = err
			if val != nil {
				results[i] = val.Inspect()
			}
		}()
	}
	wg.Wait()
	for i := range vms {
		if errs[i] != nil || results[i] != want {
			t.Errorf("VM %d: %s bekleniyordu, %q / %v alındı", i, want, results[i], errs[i])
		}
	}
}
