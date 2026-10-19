package interp

import (
	"testing"
	"time"
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

// Ana döngü özyinelemeli çağrılardan oluşsa da (kilit fonksiyon çağrısı girişinde de devredilir)
// eşzamanlı görev döngü sürerken çalışabilmeli. Süreye değil olaya bakılır: görev 'durdur'u
// ayarlamasaydı döngü sınırına (100000000 tur) kadar sürerdi.
func TestKilitDevriOzyinelemeliDongu(t *testing.T) {
	_, val, err := runTROut(t, `
		değişken durdur = yanlış
		fonksiyon dolas(n) {
			eğer durdur || n == 0 { sonuç n }
			sonuç dolas(n - 1)
		}
		eşzamanlı { durdur = doğru }
		değişken tur = 0
		iken !durdur && tur < 100000000 {
			dolas(50)
			tur += 1
		}
		sonuç tur
	`)
	if err != nil {
		t.Fatal(err)
	}
	if tur, ok := val.(*IntVal); !ok || tur.Val >= 100000000 {
		t.Fatalf("görev döngü bitmeden çalışmalıydı; döngü %v tur sürdü", val)
	}
}
