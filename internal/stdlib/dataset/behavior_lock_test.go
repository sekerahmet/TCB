package dataset

import (
	"bytes"
	"context"
	"github.com/sekerahmet/TCB/internal/testkit"
	"io"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// gatedSource ilk Doldur çağrısında kapı açılana kadar bekleyen bir toplu iş kaynağıdır.
type gatedSource struct {
	gate chan struct{}
	n    int
}

func (k *gatedSource) Fill(ctx context.Context, ti *dataload.Batch) error {
	select {
	case <-k.gate:
	case <-ctx.Done():
		return ctx.Err()
	}
	if k.n >= 2 {
		return io.EOF
	}
	k.n++
	ti.B, ti.T = 1, 2
	ti.Data = append(ti.Data[:0], 1, 2, 3)
	return nil
}

// .sonraki() önden yükleyiciyi beklerken yorumlayıcı kilidini bırakmalıdır: bir görev
// sonraki()'de beklerken ana program ilerleyebilmeli ve kapıyı açabilmelidir. Kilit
// tutulsaydı program kilitlenirdi (test zaman aşımıyla düşer).
func TestNextReleasesLockWhileWaiting(t *testing.T) {
	resource := &gatedSource{gate: make(chan struct{})}
	o := (&Dataset{path: "sahte"}).newReader(true, 1, 2, 0, resource)
	defer o.resource.Close(time.Second)

	started := make(chan struct{}, 1)
	m := interp.New()
	m.Out = &bytes.Buffer{}
	m.GlobalEnv.Define("s", o, false)
	testBuiltin(m, "başladı", func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) {
		started <- struct{}{}
		return interp.Null, nil
	})
	testBuiltin(m, "görevi_bekle", func(m *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		m.Blocking(func() {
			<-started
			// Görev sonraki()'nin içinde önden yükleyiciyi beklemeye girene kadar beklenir
			// (uyku değil): görev oraya varmadan kapı açılsaydı test, sınadığını sınamadan
			// geçerdi.
			testkit.WaitGoroutineBlocked(t, false, []string{"[select"}, "(*Reader).nextTR", "(*Prefetcher).Take")
		})
		return interp.Null, nil
	})
	testBuiltin(m, "kapıyı_aç", func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) {
		close(resource.gate)
		return interp.Null, nil
	})
	p := parser.New(lexer.New(`
değişken alınan = yok
eşzamanlı {
    başladı()
    alınan = s.sonraki()
}
görevi_bekle()
kapıyı_aç()
bekle_hepsi()
`))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	done := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("program kilitlendi: sonraki() beklerken yorumlayıcı kilidi tutuluyor")
	}
	v, _ := m.GlobalEnv.Get("alınan")
	l, ok := v.(*interp.ListVal)
	if !ok || len(l.Elements) != 2 {
		t.Fatalf("[girdi, hedef] bekleniyordu: %v", v)
	}
}

// testBuiltin testin yardımcı yerleşiğini genel kapsama bağlar (kod listesinde olmayan,
// yalnızca bu testte geçen bir ad).
func testBuiltin(m *interp.VM, name string, fn interp.BuiltinFunc) {
	m.GlobalEnv.Define(name, &interp.BuiltinVal{Name: name, Fn: fn}, true)
}
