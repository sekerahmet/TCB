package interp

import (
	"testing"
	"time"
)

// Faz 2 — dilin imzası: 'o' zamiri ve Türkçe hata mesajları

func TestTrailingBlockWithOPronoun(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		TR_var sayılar = [1, 2, 3, 4, 5, 6]
		TR_var a = sayılar.TR_filter { TR_it > 2 }.TR_map { TR_it * TR_it }
		TR_var b = sayılar.TR_filter { TR_it % 2 == 0 }
		TR_var c = [{"ad": "Ali"}, {"ad": "Ayşe"}].TR_map { TR_it["ad"] }
		TR_var d = [[1, 2], [3, 4]].TR_map { TR_it.TR_map { TR_it * 10 } }
		TR_func uygula(f) { TR_return f(21) }
		TR_var e = uygula { TR_it * 2 }
		TR_return [a, b, c, d, e]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[[9, 16, 25, 36], [2, 4, 6], ["Ali", "Ayşe"], [[10, 20], [30, 40]], 42]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
}

func TestTrailingBlockEquivalentToLambda(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		TR_var l = [5, 3, 8, 1]
		TR_return l.TR_filter { TR_it > 2 } == l.TR_filter((x) => x > 2)`)
	if err != nil {
		t.Fatal(err)
	}
	if val.Inspect() != "doğru" {
		t.Fatalf("sondaki blok ile ok fonksiyonu aynı sonucu vermeli, %s alındı", val.Inspect())
	}
}

// Kontrol yapısı başlıklarında '{' gövdenin başlangıcıdır; sondaki blok olarak okunmamalı.
func TestControlHeadersDoNotTakeTrailingBlocks(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `
		TR_var sayılar = [1, 2, 3, 4, 5, 6]
		TR_var izler = []
		TR_if sayılar.TR_length > 3 {
			izler.TR_append("eğer")
		}
		TR_for s TR_in sayılar.TR_filter((x) => x < 3) {
			izler.TR_append("döngü-\(s)")
		}
		TR_var i = 0
		TR_for i < sayılar.TR_length {
			i += 3
		}
		izler.TR_append("iken-\(i)")
		TR_var sonuç_eşle = TR_match sayılar.TR_length {
			6 => "altı"
			_ => "başka"
		}
		izler.TR_append(sonuç_eşle)
		# Parantez içinde sondaki blok başlıkta da kullanılabilir
		TR_if (sayılar.TR_filter { TR_it > 5 }).TR_length == 1 {
			izler.TR_append("parantez")
		}
		TR_return izler`)
	if err != nil {
		t.Fatal(err)
	}
	want := `["eğer", "döngü-1", "döngü-2", "iken-6", "altı", "parantez"]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
}
