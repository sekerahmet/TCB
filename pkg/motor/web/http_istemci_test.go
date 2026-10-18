package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Tüm sunucular httptest ile 127.0.0.1 üzerinde açılır; dış ağa bağlanılmaz.

func TestWebAlParsesJSONAndText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			fmt.Fprint(w, `{"b": 1, "a": [true, "x"]}`)
		case "/gonder":
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "yanlış istek", http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(r.Body)
			fmt.Fprintf(w, `{"alinan": %s}`, body)
		default:
			fmt.Fprint(w, "düz metin")
		}
	}))
	defer srv.Close()

	_, val, err := runTRWithin(t, 10*time.Second, fmt.Sprintf(`
		sonuç [web_al("%[1]s/json"), http_al("%[1]s/metin"), web_gönder("%[1]s/gonder", {"ad": "Ali"}), web_gonder("%[1]s/gonder", [1, 2])]`, srv.URL))
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	want := `[{"b": 1, "a": [doğru, "x"]}, "düz metin", {"alinan": {"ad": "Ali"}}, {"alinan": [1, 2]}]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
}

// 2xx dışı durum kodları eskiden hata sayfasının gövdesini sıradan bir yanıt gibi döndürüyordu.
func TestWebRequestsRejectNon2xx(t *testing.T) {
	uzun := strings.Repeat("ayrıntı ", 100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/uzun" {
			http.Error(w, uzun, http.StatusInternalServerError)
			return
		}
		http.Error(w, `{"hata": "bulunamadı"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	for _, tc := range []struct{ code, want string }{
		{fmt.Sprintf(`web_al("%s/yok")`, srv.URL), "404 Not Found durum kodu döndürdü"},
		{fmt.Sprintf(`web_gönder("%s/yok", {"x": 1})`, srv.URL), "web_gönder"},
		{fmt.Sprintf(`web_al("%s/uzun")`, srv.URL), "500 Internal Server Error"},
	} {
		_, _, err := runTRWithin(t, 10*time.Second, tc.code)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", tc.code, tc.want, err)
			continue
		}
		if !strings.Contains(err.Error(), "bulunamadı") && !strings.Contains(err.Error(), "ayrıntı") {
			t.Errorf("%s: hata gövde özetini içermeli: %v", tc.code, err)
		}
		if len(err.Error()) > 400 {
			t.Errorf("%s: gövde özeti kısaltılmalı (%d karakter): %v", tc.code, len(err.Error()), err)
		}
	}
}

func TestWebRequestBodyLimit(t *testing.T) {
	eski := EnFazlaGovde
	EnFazlaGovde = 1 << 10
	defer func() { EnFazlaGovde = eski }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := 1 << 10
		if r.URL.Path == "/buyuk" {
			n++
		}
		fmt.Fprint(w, strings.Repeat("a", n))
	}))
	defer srv.Close()

	_, val, err := runTRWithin(t, 10*time.Second, fmt.Sprintf(`sonuç uzunluk(web_al("%s/tam"))`, srv.URL))
	if err != nil || val.Inspect() != "1024" {
		t.Fatalf("sınırda yanıt okunmalıydı: %v, %v", val, err)
	}
	_, _, err = runTRWithin(t, 10*time.Second, fmt.Sprintf(`web_al("%s/buyuk")`, srv.URL))
	if err == nil || !strings.Contains(err.Error(), "yanıt gövdesi çok büyük (en fazla 1 KB") {
		t.Fatalf("boyut sınırı hatası bekleniyordu, alınan: %v", err)
	}
}

func TestWebRequestTimeout(t *testing.T) {
	eski := webZamanAsimi
	webZamanAsimi = 200 * time.Millisecond
	defer func() { webZamanAsimi = eski }()

	birak := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-birak:
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	defer close(birak)

	_, _, err := runTRWithin(t, 10*time.Second, fmt.Sprintf(`web_al("%s/yavas")`, srv.URL))
	if err == nil || !strings.Contains(err.Error(), "zaman aşımı: 200 ms içinde yanıt alınamadı") {
		t.Fatalf("zaman aşımı hatası bekleniyordu, alınan: %v", err)
	}
}

// Kapalı bir porta istek ağ hatası olarak bildirilir (eski mesaj biçimi korunur).
func TestWebRequestUnreachable(t *testing.T) {
	port := freePort(t)
	_, _, err := runTRWithin(t, 10*time.Second, fmt.Sprintf(`web_al("http://127.0.0.1:%d/")`, port))
	if err == nil || !strings.Contains(err.Error(), "Web bağlantı hatası") {
		t.Fatalf("bağlantı hatası bekleniyordu, alınan: %v", err)
	}
}

func TestGovdeOzetiKeepsUTF8(t *testing.T) {
	y := &Yanit{Govde: []byte("çççç\n  ğğ")}
	if got := y.GovdeOzeti(5); got != "çç..." {
		t.Fatalf("\"çç...\" bekleniyordu, %q alındı", got)
	}
	if got := y.GovdeOzeti(100); got != "çççç ğğ" {
		t.Fatalf("\"çççç ğğ\" bekleniyordu, %q alındı", got)
	}
}
