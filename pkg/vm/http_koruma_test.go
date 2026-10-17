package vm

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
)

// Güvenlik raporu Y1 / test raporu Y3: web_sunucu her siteye açıktı (CORS *, Host denetimi
// yok, text/plain ile CSRF). O2: '<' ile başlayan her yanıt HTML sayılıyordu (yansıyan XSS).
func TestWebServerRejectsCrossSiteAndForeignHost(t *testing.T) {
	port := freePort(t)
	machine := New()
	machine.Out = io.Discard
	prog := parser.New(lexer.New(fmt.Sprintf(`
		değişken sayac = 0
		değişken api = web_sunucu(port: %d)
		api.gönder("/ekle", fonksiyon(istek) {
			sayac += 1
			sonuç {"gelen": istek["veri"]}
		})
		api.al("/yansit", fonksiyon(istek) { sonuç "<b>" + istek["parametreler"]["ad"] + "</b>" })
		api.al("/belge", fonksiyon(istek) { sonuç "<!DOCTYPE html><p>tam belge</p>" })
		api.arka_planda_başlat()
		sonuç api`, port))).ParseProgram()
	val, err := machine.Run(prog)
	if err != nil {
		t.Fatal(err)
	}
	srv := val.(*WebServerVal)
	defer srv.server.Close()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	do := func(method, path, body string, headers map[string]string) (*http.Response, string) {
		req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
		for k, v := range headers {
			if k == "Host" {
				req.Host = v
			} else {
				req.Header.Set(k, v)
			}
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res, string(b)
	}
	sayac := func() string { v, _ := machine.GlobalEnv.Get("sayac"); return v.Inspect() }

	// 1. Başka siteden ön kontrol (preflight) reddedilir, CORS izni verilmez
	res, _ := do("OPTIONS", "/ekle", "", map[string]string{"Origin": "https://saldirgan.example", "Access-Control-Request-Method": "POST"})
	if res.StatusCode != http.StatusForbidden || res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("başka siteden preflight 403 ve CORS'suz olmalı: %d %q", res.StatusCode, res.Header.Get("Access-Control-Allow-Origin"))
	}
	// 2. Başka siteden text/plain POST (ön kontrolsüz CSRF) reddedilir, işleyici çalışmaz
	res, _ = do("POST", "/ekle", `{"x":1}`, map[string]string{"Origin": "https://saldirgan.example", "Content-Type": "text/plain"})
	if res.StatusCode != http.StatusForbidden || sayac() != "0" {
		t.Errorf("CSRF POST reddedilmeli (durum %d, sayac %s)", res.StatusCode, sayac())
	}
	res, _ = do("POST", "/ekle", `{}`, map[string]string{"Sec-Fetch-Site": "cross-site", "Content-Type": "application/json"})
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site: cross-site POST reddedilmeli: %d", res.StatusCode)
	}
	// 3. Yabancı Host (DNS rebinding) reddedilir
	res, _ = do("GET", "/belge", "", map[string]string{"Host": fmt.Sprintf("saldirgan.example:%d", port)})
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("yabancı Host 403 olmalı: %d", res.StatusCode)
	}
	// 4. Aynı makineden / aynı kaynaktan istekler çalışır; JSON yalnızca application/json ile çözülür
	res, body := do("POST", "/ekle", `{"ad":"Ali"}`, map[string]string{"Origin": base, "Content-Type": "application/json"})
	if res.StatusCode != 200 || !strings.Contains(body, `"ad": "Ali"`) || sayac() != "1" {
		t.Errorf("aynı kaynaktan JSON POST çalışmalı: %d %s sayac=%s", res.StatusCode, body, sayac())
	}
	res, body = do("POST", "/ekle", `{"ad":"Ali"}`, map[string]string{"Content-Type": "text/plain"})
	if res.StatusCode != 200 || !strings.Contains(body, `"gelen": "{\"ad\":\"Ali\"}"`) {
		t.Errorf("text/plain gövde metin olarak gelmeli: %d %s", res.StatusCode, body)
	}
	// 5. Kullanıcı girdisini yansıtan parça HTML olarak sunulmaz; tam belge sunulur
	res, _ = do("GET", "/yansit?ad=%3Cscript%3Ealert(1)%3C/script%3E", "", nil)
	if ct := res.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("yansıtılan parça text/html olmamalı ve nosniff olmalı: %q", ct)
	}
	res, _ = do("GET", "/belge", "", nil)
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") || res.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("tam HTML belgesi text/html ve çerçevelenemez olmalı: %q", ct)
	}
}

// Güvenlik raporu O5: görsel arayüz sunucusunun ana sayfası Host denetimsiz ve çerçevelenebilirdi.
func TestGUIServerHostCheckAndFraming(t *testing.T) {
	port := freePort(t)
	g := &GUIAppVal{Title: "Deneme", vmRef: New()}
	g.vmRef.Out = io.Discard
	if err := g.startGUIServer(port, true, false); err != nil {
		t.Fatal(err)
	}
	defer g.server.Close()
	get := func(host string) *http.Response {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", port), nil)
		if host != "" {
			req.Host = host
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	if res := get(fmt.Sprintf("saldirgan.example:%d", port)); res.StatusCode != http.StatusForbidden {
		t.Errorf("yabancı Host 403 olmalı: %d", res.StatusCode)
	}
	if res := get(""); res.StatusCode != 200 || res.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("yerel istek çalışmalı ve sayfa çerçevelenemez olmalı: %d %q", res.StatusCode, res.Header.Get("X-Frame-Options"))
	}
}
