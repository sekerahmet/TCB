package yz

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuiltinsRejectBadSizesWithoutPanic(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `sonuç [aralık(-5), aralık(5, 2), aralık(3)]`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != "[[], [], [0, 1, 2]]" {
		t.Fatalf("[[], [], [0, 1, 2]] bekleniyordu, %s alındı", val.Inspect())
	}

	for _, code := range []string{
		`aralık(10000000000)`,
		`sıfırlar(-2, 2)`,
		`sıfırlar(-3)`,
		`birler(0, 4)`,
		`birim_matris(-1)`,
		`rastgele_matris(2, -7)`,
		`sıfırlar(100000, 100000)`,
	} {
		_, _, err := runTRWithin(t, 5*time.Second, code)
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", code)
		} else if strings.Contains(err.Error(), "çökme önlendi") {
			t.Errorf("%s: panik yerine düzgün hata bekleniyordu: %v", code, err)
		}
	}
}

// yz_sor eskiden sunucu kapalıyken uydurma bir "yanıt" döndürüyordu.
func TestYzSorReportsUnreachableServer(t *testing.T) {
	port := freePort(t) // dinleyen yok
	_, err := runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: "http://127.0.0.1:%d/api/generate")`, port))
	if err == nil || !strings.Contains(err.Error(), "ulaşılamadı") {
		t.Fatalf("ulaşılamadı hatası bekleniyordu, alınan: %v", err)
	}

	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
	}))
	defer notFound.Close()
	_, err = runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: %q)`, notFound.URL))
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("404 hatası bekleniyordu, alınan: %v", err)
	}

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"response": "  Merhaba!  "}`)
	}))
	defer ok.Close()
	val, err := runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: %q)`, ok.URL))
	if err != nil || val.Inspect() != "Merhaba!" {
		t.Fatalf("\"Merhaba!\" bekleniyordu, alınan: %v, %v", val, err)
	}
}

func TestMatrixIndexingAndAssignment(t *testing.T) {
	code := `
		değişken M = matris([[1, 2], [3, 4]])
		M[0][1] = 10
		M[1][0] += 5
		M[1] = [8, 9]
		sonuç [M[0][1], M[1][0], M[1][1]]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Matris indeksleme/atama hatası: %v", err)
	}
	if val.Inspect() != "[10.0, 8.0, 9.0]" {
		t.Fatalf("[10.0, 8.0, 9.0] bekleniyordu, %s alındı", val.Inspect())
	}
}

func TestAutogradAndNeuralNet(t *testing.T) {
	code := `
		değişken x = tensör(3.0, türev_izle: doğru)
		değişken y = x * x + 2 * x + 1
		geri_yayılım(y)
		sonuç [y.değer, x.gradyan]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("Autograd hatası: %v", err)
	}
	if val.Inspect() != "[16.0, 8.0]" {
		t.Fatalf("[16.0, 8.0] bekleniyordu, %s alındı", val.Inspect())
	}
}

// yz_sor'un HTTP kısmı web paketinin ortak istemcisini kullanır: zaman aşımı "ulaşılamadı"
// diye yanlış yönlendirmek yerine açıkça bildirilir.
func TestYzSorTimeout(t *testing.T) {
	eski := yzSorZamanAsimi
	yzSorZamanAsimi = 200 * time.Millisecond
	defer func() { yzSorZamanAsimi = eski }()

	birak := make(chan struct{})
	yavas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-birak:
		case <-time.After(5 * time.Second):
		}
	}))
	defer yavas.Close()
	defer close(birak)

	_, err := runTR(t, fmt.Sprintf(`sonuç yz_sor("merhaba", adres: %q)`, yavas.URL))
	if err == nil || !strings.Contains(err.Error(), "zaman aşımı") || strings.Contains(err.Error(), "ulaşılamadı") {
		t.Fatalf("zaman aşımı hatası bekleniyordu, alınan: %v", err)
	}
}
