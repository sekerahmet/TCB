package yurut

import (
	"io"
	"testing"
	"time"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
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
	prog := ayristir.New(sozcuk.New(`
		değişken durdur = yanlış
		fonksiyon dolas(n) {
			eğer durdur || n == 0 { sonuç n }
			sonuç dolas(n - 1)
		}
		değişken tur = 0
		döngü_başladı()
		iken !durdur && tur < 100000000 {
			dolas(50)
			tur += 1
		}
		sonuç tur
	`)).ParseProgram()
	m := New()
	m.Out = io.Discard
	// Süreye değil olaya bakılır: istek, döngü gerçekten başladıktan sonra gönderilir (aksi
	// hâlde döngü başlamadan çalışan istek hiçbir şeyi sınamadan geçerdi).
	basladi := make(chan struct{})
	m.RegisterBuiltin("döngü_başladı", func(*VM, []Value, map[string]Value) (Value, error) {
		close(basladi)
		return NULL, nil
	})
	type sonuc struct {
		val Value
		err error
	}
	bitti := make(chan sonuc, 1)
	go func() {
		val, err := m.Run(prog)
		bitti <- sonuc{val, err}
	}()
	select {
	case <-basladi:
	case s := <-bitti:
		t.Fatalf("program döngüye girmeden bitti: %v", s.err)
	}
	yanit := make(chan time.Duration, 1)
	go func() {
		bas := time.Now()
		_, _ = m.NewTaskVM(false).RunTask(func() (Value, error) {
			return NULL, m.GlobalEnv.Assign("durdur", TRUE)
		})
		yanit <- time.Since(bas)
	}()
	// İsteğin döngü sürerken çalıştığını döngünün kendisi gösterir: istek 'durdur'u
	// ayarlamasaydı döngü sınırına (100000000 tur) kadar sürerdi. Süre sınırı yalnızca
	// takılmaya karşıdır, zamanlama ölçmez.
	select {
	case <-yanit:
	case <-time.After(2 * time.Minute):
		t.Fatal("istek görevi ana döngü sürerken hiç çalışamadı")
	}
	s := <-bitti
	if s.err != nil {
		t.Fatal(s.err)
	}
	if tur, ok := s.val.(*IntVal); !ok || tur.Val >= 100000000 {
		t.Fatalf("istek döngü bitmeden çalışmalıydı; döngü %v tur sürdü", s.val)
	}
}
