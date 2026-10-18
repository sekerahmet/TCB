package veri

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/veriyolu"
)

// kapiliKaynak ilk Doldur çağrısında kapı açılana kadar bekleyen bir toplu iş kaynağıdır.
type kapiliKaynak struct {
	kapi chan struct{}
	n    int
}

func (k *kapiliKaynak) Doldur(ctx context.Context, ti *veriyolu.TopluIs) error {
	select {
	case <-k.kapi:
	case <-ctx.Done():
		return ctx.Err()
	}
	if k.n >= 2 {
		return io.EOF
	}
	k.n++
	ti.B, ti.T = 1, 2
	ti.Veri = append(ti.Veri[:0], 1, 2, 3)
	return nil
}

// .sonraki() önden yükleyiciyi beklerken yorumlayıcı kilidini bırakmalıdır: bir görev
// sonraki()'de beklerken ana program ilerleyebilmeli ve kapıyı açabilmelidir. Kilit
// tutulsaydı program kilitlenirdi (test zaman aşımıyla düşer).
func TestSonrakiBeklerkenKilitBirakilir(t *testing.T) {
	kaynak := &kapiliKaynak{kapi: make(chan struct{})}
	o := (&Kume{yol: "sahte"}).okuyucuKur(true, 1, 2, 0, kaynak)
	defer o.kaynak.Kapat(time.Second)

	basladi := make(chan struct{}, 1)
	m := yurut.New()
	m.Out = &bytes.Buffer{}
	m.GlobalEnv.Define("s", o, false)
	testYerlesigi(m, "başladı", func(*yurut.VM, []yurut.Value, map[string]yurut.Value) (yurut.Value, error) {
		basladi <- struct{}{}
		return yurut.NULL, nil
	})
	testYerlesigi(m, "görevi_bekle", func(m *yurut.VM, _ []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
		m.Blocking(func() {
			<-basladi
			// Görev sonraki()'nin içinde önden yükleyiciyi beklemeye girene kadar beklenir
			// (uyku değil): görev oraya varmadan kapı açılsaydı test, sınadığını sınamadan
			// geçerdi.
			sonrakiBeklemede(t)
		})
		return yurut.NULL, nil
	})
	testYerlesigi(m, "kapıyı_aç", func(*yurut.VM, []yurut.Value, map[string]yurut.Value) (yurut.Value, error) {
		close(kaynak.kapi)
		return yurut.NULL, nil
	})
	p := ayristir.New(sozcuk.New(`
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
	bitti := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		bitti <- err
	}()
	select {
	case err := <-bitti:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("program kilitlendi: sonraki() beklerken yorumlayıcı kilidi tutuluyor")
	}
	v, _ := m.GlobalEnv.Get("alınan")
	l, ok := v.(*yurut.ListVal)
	if !ok || len(l.Elements) != 2 {
		t.Fatalf("[girdi, hedef] bekleniyordu: %v", v)
	}
}

// sonrakiBeklemede bir görevin .sonraki() içinde önden yükleyiciden toplu iş beklediğini
// (gorutinin kanal beklemesinde durduğunu) yığın dökümünden görene kadar bekler. Program
// ayrı bir gorutinde koştuğu için t.Fatal değil t.Error kullanılır. Süre sınırı yalnızca
// takılmaya karşıdır.
func sonrakiBeklemede(t *testing.T) {
	son := time.Now().Add(5 * time.Second)
	buf := make([]byte, 1<<20)
	for {
		yigin := string(buf[:runtime.Stack(buf, true)])
		for _, g := range strings.Split(yigin, "\n\n") {
			baslik, _, _ := strings.Cut(g, "\n")
			if strings.Contains(baslik, "[select") && strings.Contains(g, "(*Okuyucu).sonraki") && strings.Contains(g, "(*OnYukleyici).Al") {
				return
			}
		}
		if time.Now().After(son) {
			t.Errorf("görev sonraki() içinde beklemeye girmedi:\n%s", yigin)
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// testYerlesigi testin yardımcı yerleşiğini genel kapsama bağlar (kod listesinde olmayan,
// yalnızca bu testte geçen bir ad).
func testYerlesigi(m *yurut.VM, ad string, fn yurut.BuiltinFunc) {
	m.GlobalEnv.Define(ad, &yurut.BuiltinVal{Name: ad, Fn: fn}, true)
}
