package vm

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Yerel HTTP sunucuları (web_sunucu, görsel arayüz) için ortak koruma.
//
// Bu sunucular yalnızca 127.0.0.1'de dinler; ama kullanıcının tarayıcısında açık olan
// herhangi bir web sitesi yine de onlara istek gönderebilir. Bu katman şunları engeller:
//   - DNS rebinding: Host başlığı localhost / 127.0.0.1 / [::1] ve doğru port olmalıdır.
//   - Başka siteden okuma: CORS başlığı gönderilmez; tarayıcı yanıtı başka siteye vermez.
//   - CSRF: başka bir siteden gelen veri değiştiren istekler (POST/PUT/DELETE...) reddedilir.
//   - Kaynak tüketimi: istek gövdesi boyutu ve başlık okuma süresi sınırlıdır.

// maxRequestBody tek bir istek gövdesinin en büyük boyutudur (10 MB).
const maxRequestBody = 10 << 20

func localHostAllowed(host string, port int) bool {
	switch strings.ToLower(host) {
	case fmt.Sprintf("localhost:%d", port), fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("[::1]:%d", port):
		return true
	}
	return false
}

// sameLocalOrigin, Origin başlığının bu sunucunun kendisi olup olmadığını söyler.
func sameLocalOrigin(origin string, port int) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" {
		return false
	}
	return localHostAllowed(u.Host, port)
}

func isStateChanging(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// guardLocalHTTP bir yerel sunucunun tüm isteklerini yukarıdaki kurallarla süzer.
func guardLocalHTTP(port int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if !localHostAllowed(req.Host, port) {
			http.Error(rw, "yasak: geçersiz Host başlığı", http.StatusForbidden)
			return
		}
		crossSite := false
		if origin := req.Header.Get("Origin"); origin != "" && origin != "null" && !sameLocalOrigin(origin, port) {
			crossSite = true
		}
		if origin := req.Header.Get("Origin"); origin == "null" {
			crossSite = true
		}
		if sf := req.Header.Get("Sec-Fetch-Site"); sf == "cross-site" || sf == "same-site" {
			crossSite = true
		}
		if req.Method == http.MethodOptions {
			// Ön kontrol (preflight) isteklerine CORS izni verilmez.
			if crossSite {
				http.Error(rw, "yasak: başka bir siteden erişim", http.StatusForbidden)
				return
			}
			rw.WriteHeader(http.StatusNoContent)
			return
		}
		if crossSite && isStateChanging(req.Method) {
			http.Error(rw, "yasak: başka bir siteden gelen veri değiştiren istek reddedildi", http.StatusForbidden)
			return
		}
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("X-Frame-Options", "DENY")
		rw.Header().Set("Referrer-Policy", "no-referrer")
		req.Body = http.MaxBytesReader(rw, req.Body, maxRequestBody)
		next.ServeHTTP(rw, req)
	})
}

// newLocalServer yerel sunucular için zaman aşımları ayarlanmış bir http.Server döndürür.
func newLocalServer(port int, handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           guardLocalHTTP(port, handler),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// isFullHTMLDocument yanıt metninin tam bir HTML belgesi olup olmadığını söyler. Yalnızca
// bunlar text/html olarak sunulur; '<' ile başlayan herhangi bir metni HTML saymak,
// kullanıcı girdisini yansıtan işleyicilerde betik enjeksiyonuna (XSS) yol açıyordu.
func isFullHTMLDocument(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(t, "<!doctype html") || strings.HasPrefix(t, "<html")
}
