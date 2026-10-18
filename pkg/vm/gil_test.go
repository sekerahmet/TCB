package vm

import (
	"io"
	"testing"
	"time"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
)

// CPU-yoğun bir döngü sürerken eşzamanlı görev kilidi alabilmeli (kilit devri).
func TestKilitDevriEszamanliGorev(t *testing.T) {
	bas := time.Now()
	_, val, err := runTROut(t, `
		değişken bitti = yanlış
		eşzamanlı { bitti = doğru }
		değişken n = 0
		iken !bitti && n < 200000000 { n += 1 }
		sonuç n < 200000000
	`)
	if err != nil || val.Inspect() != "doğru" {
		t.Fatalf("eşzamanlı görev döngü bitmeden çalışmalıydı: %v %v (%v)", val, err, time.Since(bas))
	}
}

// Ana döngü sürerken bir istek görevi (web/arayüz işleyicisi gibi RunTask ile) makul sürede
// çalışabilmeli; döngü özyinelemeli çağrılardan oluşsa da.
func TestKilitDevriIstekGorevi(t *testing.T) {
	prog := parser.New(lexer.New(`
		değişken durdur = yanlış
		fonksiyon dolas(n) {
			eğer durdur || n == 0 { sonuç n }
			sonuç dolas(n - 1)
		}
		değişken tur = 0
		iken !durdur && tur < 100000000 {
			dolas(50)
			tur += 1
		}
		sonuç tur
	`)).ParseProgram()
	m := New()
	m.Out = io.Discard
	bitti := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		bitti <- err
	}()
	time.Sleep(50 * time.Millisecond) // döngü başlasın
	yanit := make(chan time.Duration, 1)
	go func() {
		bas := time.Now()
		_, _ = m.NewTaskVM(false).RunTask(func() (Value, error) {
			return NULL, m.GlobalEnv.Assign("durdur", TRUE)
		})
		yanit <- time.Since(bas)
	}()
	select {
	case d := <-yanit:
		if d > 2*time.Second {
			t.Fatalf("istek görevi çok geç çalıştı: %v", d)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("istek görevi ana döngü sürerken hiç çalışamadı")
	}
	if err := <-bitti; err != nil {
		t.Fatal(err)
	}
}
