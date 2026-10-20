package dataset

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"strings"
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
			nextWaiting(t)
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

// nextWaiting bir görevin .sonraki() içinde önden yükleyiciden toplu iş beklediğini
// (gorutinin kanal beklemesinde durduğunu) yığın dökümünden görene kadar bekler. Program
// ayrı bir gorutinde koştuğu için t.Fatal değil t.Error kullanılır. Süre sınırı yalnızca
// takılmaya karşıdır.
func nextWaiting(t *testing.T) {
	last := time.Now().Add(5 * time.Second)
	buf := make([]byte, 1<<20)
	for {
		stack := string(buf[:runtime.Stack(buf, true)])
		for _, g := range strings.Split(stack, "\n\n") {
			header, _, _ := strings.Cut(g, "\n")
			if strings.Contains(header, "[select") && strings.Contains(g, "(*Reader).nextTR") && strings.Contains(g, "(*Prefetcher).Take") {
				return
			}
		}
		if time.Now().After(last) {
			t.Errorf("görev sonraki() içinde beklemeye girmedi:\n%s", stack)
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// testBuiltin testin yardımcı yerleşiğini genel kapsama bağlar (kod listesinde olmayan,
// yalnızca bu testte geçen bir ad).
func testBuiltin(m *interp.VM, name string, fn interp.BuiltinFunc) {
	m.GlobalEnv.Define(name, &interp.BuiltinVal{Name: name, Fn: fn}, true)
}
