package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Yerel HTTP sunucuları (web_sunucu, görsel arayüz) için ortak koruma.
//
// Bu sunucular yalnızca 127.0.0.1'de dinler; ama kullanıcının tarayıcısında açık olan
// herhangi bir web sitesi yine de onlara istek gönderebilir. Bu katman şunları engeller:
//   - DNS rebinding: Host başlığı localhost / 127.0.0.1 / [::1] ve doğru port olmalıdır.
//   - Başka siteden okuma: CORS başlığı gönderilmez; tarayıcı yanıtı başka siteye vermez.
//   - CSRF: başka bir siteden (başka bir porttaki yerel siteler dahil) gelen BÜTÜN istekler,
//     GET dahil, reddedilir; böylece <img src="http://localhost:8080/sil"> gibi bir bağlantı
//     da işleyiciyi çalıştıramaz. Tarayıcılar bunu Sec-Fetch-Site ve Origin başlıklarıyla
//     bildirir; bu başlıkları göndermeyen eski tarayıcılarda Referer'a bakılır.
//   - Kaynak tüketimi: istek gövdesi boyutu ve başlık okuma süresi sınırlıdır.

// maxRequestBody tek bir istek gövdesinin en büyük boyutudur (10 MB).
const maxRequestBody = 10 << 20

// hataGunlugu sunucu tarafındaki iç hataların (işleyici hatası, JSON'a çevrilemeyen yanıt)
// yazıldığı yerdir; istemciye yalnızca genel bir mesaj gider. Testler değiştirebilir.
var hataGunlugu io.Writer = os.Stderr

// jsonHata istemciye {"hata": mesaj} biçiminde geçerli bir JSON hata yanıtı yazar.
func jsonHata(rw http.ResponseWriter, durum int, mesaj string, ekler ...string) {
	govde := map[string]string{"hata": mesaj}
	for i := 0; i+1 < len(ekler); i += 2 {
		govde[ekler[i]] = ekler[i+1]
	}
	b, _ := json.Marshal(govde) // metinlerden oluşan sözlük her zaman yazılabilir
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(durum)
	_, _ = rw.Write(b)
}

// localHostAllowed Host başlığının bu bilgisayardaki sunucunun kendisi olup olmadığını söyler.
// Bütün yerel sunucular (web_sunucu, görsel arayüz) bu tek denetimi kullanır.
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

// baskaSitedenMi isteğin tarayıcıda açık başka bir siteden (başka bir porttaki yerel site
// dahil) gelip gelmediğini söyler.
func baskaSitedenMi(req *http.Request, port int) bool {
	origin := req.Header.Get("Origin")
	if origin == "null" || (origin != "" && !sameLocalOrigin(origin, port)) {
		return true
	}
	sf := req.Header.Get("Sec-Fetch-Site")
	if sf == "cross-site" || sf == "same-site" {
		return true
	}
	// Sec-Fetch-Site ve Origin göndermeyen eski tarayıcılar için: yabancı Referer
	if sf == "" && origin == "" {
		if ref := req.Header.Get("Referer"); ref != "" && !sameLocalOrigin(ref, port) {
			return true
		}
	}
	return false
}

// guardLocalHTTP bir yerel sunucunun tüm isteklerini yukarıdaki kurallarla süzer.
func guardLocalHTTP(port int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if !localHostAllowed(req.Host, port) {
			http.Error(rw, "yasak: geçersiz Host başlığı", http.StatusForbidden)
			return
		}
		if baskaSitedenMi(req, port) {
			// Ön kontrol (preflight) dahil: CORS izni verilmez, işleyici çalışmaz.
			http.Error(rw, "yasak: başka bir siteden gelen istek reddedildi (adresi tarayıcının adres çubuğuna kendiniz yazın)", http.StatusForbidden)
			return
		}
		if req.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusNoContent)
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
