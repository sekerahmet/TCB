package yz

// Hakem denetimi (mimari.md 8, 18): yz_sor'un hata yolları sahte bir yerel sunucuyla
// (httptest, 127.0.0.1) sınanır; kullanıcıya görünen metinlerde başka ürün adı geçmez ve
// argüman hataları ortak biçimdedir.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// sahteSunucu metin üretme ve model listesi uç noktaları olan sahte bir yerel dil modeli
// sunucusudur; gelen üretme isteklerinin gövdelerini kaydeder.
type sahteSunucu struct {
	mu       sync.Mutex
	istekler []map[string]interface{}
	modeller string
	yanit    string
}

func (s *sahteSunucu) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/tags":
		if s.modeller == "" {
			http.Error(w, "yok", http.StatusNotFound)
			return
		}
		fmt.Fprint(w, s.modeller)
	case "/api/generate":
		b, _ := io.ReadAll(r.Body)
		var g map[string]interface{}
		_ = json.Unmarshal(b, &g)
		s.mu.Lock()
		s.istekler = append(s.istekler, g)
		s.mu.Unlock()
		fmt.Fprint(w, s.yanit)
	default:
		http.NotFound(w, r)
	}
}

func TestYzSorModelSecimiVeIstekGovdesi(t *testing.T) {
	s := &sahteSunucu{modeller: `{"models": [{"name": "ilk-model"}, {"name": "ikinci"}]}`, yanit: `{"response": " Selam "}`}
	srv := httptest.NewServer(s)
	defer srv.Close()
	adres := srv.URL + "/api/generate"
	val, err := runTR(t, fmt.Sprintf(`sonuç [yz_sor("merhaba", adres: %q), yz_sor("nasılsın", model: "seçilen", adres: %q)]`, adres, adres))
	if err != nil || val.Inspect() != `["Selam", "Selam"]` {
		t.Fatalf("%v %v", val, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.istekler) != 2 || s.istekler[0]["model"] != "ilk-model" || s.istekler[0]["prompt"] != "merhaba" ||
		s.istekler[1]["model"] != "seçilen" || s.istekler[0]["stream"] != false {
		t.Fatalf("istek gövdeleri: %v", s.istekler)
	}
}

func TestYzSorHataYollari(t *testing.T) {
	bos := httptest.NewServer(&sahteSunucu{modeller: `{"models": []}`})
	defer bos.Close()
	listesiz := httptest.NewServer(&sahteSunucu{})
	defer listesiz.Close()
	bozuk := httptest.NewServer(&sahteSunucu{yanit: `{"baska": 1}`})
	defer bozuk.Close()
	for _, d := range []struct{ kod, bekle string }{
		{fmt.Sprintf(`yz_sor("a", adres: %q)`, bos.URL+"/api/generate"), "yüklü model yok"},
		{fmt.Sprintf(`yz_sor("a", adres: %q)`, listesiz.URL+"/api/generate"), "listesi alınamadı"},
		{fmt.Sprintf(`yz_sor("a", model: "m", adres: %q)`, bozuk.URL+"/api/generate"), "'response' alanı"},
		{`yz_sor()`, "yz_sor() 1 argüman bekler, 0 verildi (kullanım: yz_sor(istem"},
		{`yz_sor("a", "b")`, "yz_sor() 1 argüman bekler, 2 verildi"},
		{`yz_sor(5)`, "yz_sor() 1. argüman olarak Metin (istem) bekler, Tam (5) verildi"},
		{`yz_sor("a", modl: "m")`, "model"},
	} {
		_, err := runTR(t, "sonuç "+d.kod)
		if err == nil || !strings.Contains(err.Error(), d.bekle) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", d.kod, d.bekle, err)
		}
	}
	// Ulaşılamayan sunucu: genel Türkçe tanım ve port; başka ürün adı yok
	port := freePort(t)
	_, err := runTR(t, fmt.Sprintf(`sonuç yz_sor("a", model: "m", adres: "http://127.0.0.1:%d/api/generate")`, port))
	if err == nil || !strings.Contains(err.Error(), "yerel dil modeli sunucusu") || !strings.Contains(err.Error(), "11434 portunda") {
		t.Fatalf("yol gösteren ulaşılamadı hatası bekleniyordu: %v", err)
	}
	for _, yasak := range []string{"llama", "ollama", "Ollama"} {
		if strings.Contains(err.Error(), yasak) {
			t.Fatalf("hata metninde ürün adı geçmemeli: %v", err)
		}
	}
}
