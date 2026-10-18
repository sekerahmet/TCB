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
