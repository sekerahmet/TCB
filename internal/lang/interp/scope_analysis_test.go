package interp

import (
	"sync"
	"testing"
)

// Aynı ayrıştırılmış program birçok VM'de aynı anda derlenip çalıştırılabilmelidir: blok
// kapsam bayrakları eskiden ağaca (ast.BlockStatement.Flags) tembelce yazılıyordu ve iki VM
// aynı bloğu aynı anda derlerken yarışıyordu. Önbellek artık derleyicinin; ağaca yazılmaz.
// Yarış 'go test -race' ile görülür (tools/full_check.sh --kip=yaris, Linux); yarış
// dedektörü olmayan yerde de test her VM'in doğru sonucu verdiğini sınar.
func TestOneProgramCompiledInParallelVMs(t *testing.T) {
	const code = `
		TR_func sayaç_yap() -> () -> TR_Int {
			TR_var n = 0
			TR_return () => { n += 1; TR_return n }
		}
		TR_var toplam_ = 0
		TR_for i TR_in 1..=20 {
			TR_var kare = i * i
			TR_if kare % 2 == 0 { TR_var yarım = kare // 2; toplam_ += yarım } TR_else { toplam_ += kare }
		}
		TR_var s = sayaç_yap()
		TR_var j = 0
		TR_for j < 5 { TR_var k = j; s(); j += 1 }
		TR_var kapanışlar = [(() => { TR_return x }) TR_for x TR_in 1..=3]
		TR_struct Nokta { x: TR_Int = 0
			TR_func iki_kat() -> TR_Int { TR_var d = TR_self.x * 2; TR_return d } }
		TR_var result = [toplam_, s(), kapanışlar[2](), Nokta(4).iki_kat()]`
	prog := mustParseProgram(t, code)
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
