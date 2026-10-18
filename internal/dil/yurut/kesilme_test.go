package yurut

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
)

// Kes programı bir sonraki güvenli noktada durdurur: Run ErrKesildi döner, dene/yakala
// kesilmeyi yakalamaz, ertelenen işler (fonksiyon, eşzamanlı görev, program) çalışır,
// eşzamanlı görevin kesilmesi hata olarak bildirilmez.
func TestKesilme(t *testing.T) {
	kod := `
değişken izler = []
ertele { izler.ekle("program") }
eşzamanlı {
    ertele { izler.ekle("görev") }
    döngü { }
}
fonksiyon iş() {
    ertele { izler.ekle("fonksiyon") }
    dene {
        hazır()
        döngü { }
    } yakala h {
        izler.ekle("yakalandı")
    }
}
iş()
izler.ekle("sürdü")
`
	p := ayristir.New(sozcuk.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	vmx := New()
	var hata bytes.Buffer
	vmx.Err = &hata
	hazir := make(chan struct{})
	vmx.RegisterBuiltin("hazır", func(*VM, []Value, map[string]Value) (Value, error) {
		close(hazir)
		return NULL, nil
	})
	go func() {
		<-hazir
		time.Sleep(5 * time.Millisecond)
		vmx.Kes()
	}()
	_, err := vmx.Run(prog)
	if !errors.Is(err, ErrKesildi) {
		t.Fatalf("ErrKesildi bekleniyordu: %v", err)
	}
	vmx.WaitAsync()
	izler, _ := vmx.GlobalEnv.Get("izler")
	metin := izler.Inspect()
	for _, iz := range []string{`"fonksiyon"`, `"program"`, `"görev"`} {
		if !strings.Contains(metin, iz) {
			t.Errorf("%s ertelenen işi çalışmadı: %s", iz, metin)
		}
	}
	for _, iz := range []string{"yakalandı", "sürdü"} {
		if strings.Contains(metin, iz) {
			t.Errorf("kesilmeden sonra %q olmamalıydı: %s", iz, metin)
		}
	}
	if hata.Len() > 0 {
		t.Errorf("görevin kesilmesi hata olarak bildirilmemeli: %s", hata.String())
	}
	if !vmx.Kesildi() || vmx.Kes() {
		t.Error("Kes yalnızca ilk çağrıda doğru dönmeli")
	}
}

// Kesmeyi erteleyen (kontrol noktası) yoksa KesmeIste programı Kes gibi durdurur; varsa
// yalnızca bayrak koyar, program SimdiKes çağrılınca o noktada durur.
func TestKesmeErtelenir(t *testing.T) {
	v := New()
	if v.KesmeIste() || !v.Kesildi() {
		t.Fatal("erteleyen yokken kesme hemen olmalıydı")
	}
	v = New()
	v.KesmeyiErtele()
	if !v.KesmeIste() || v.Kesildi() || !v.DurdurmaIstendi() {
		t.Fatalf("kesme ertelenmeliydi: kesildi=%v istendi=%v", v.Kesildi(), v.DurdurmaIstendi())
	}
	v.GlobalEnv.Define("dur", &BuiltinVal{Name: "dur", Fn: func(m *VM, _ []Value, _ map[string]Value) (Value, error) {
		m.SimdiKes()
		return NULL, nil
	}}, true)
	var out bytes.Buffer
	v.Out = &out
	p := ayristir.New(sozcuk.New("ertele { yazdır(\"ertelenen\") }\ndur()\nyazdır(\"sürdü\")\n"))
	prog := p.ParseProgram()
	if _, err := v.Run(prog); !errors.Is(err, ErrKesildi) || out.String() != "ertelenen\n" {
		t.Fatalf("SimdiKes programı o noktada durdurmalıydı: %v %q", err, out.String())
	}
}
