package httpserver

import (
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// TestOriginIsChecked: 127.0.0.1'e bağlı sunucu Host, Origin ve Sec-Fetch-Site'ı işleyiciden önce
// denetler (28.1a); reddedilen istekte işleyici çağrılmaz. Üçünü de taşımayan istemci geçer.
func TestOriginIsChecked(t *testing.T) {
	ts := serveGo(t, "", ok200)
	self := "http://" + ts.addr()
	cases := []struct {
		name   string
		header map[string]string
		method string
		code   int
	}{
		{"başlıksız istemci", nil, "GET", 200},
		{"Host localhost", map[string]string{"Host": "localhost:" + ts.addr()[len("127.0.0.1:"):]}, "GET", 403},
		{"Host başka ad", map[string]string{"Host": "saldirgan.example:" + ts.addr()[len("127.0.0.1:"):]}, "GET", 403},
		{"Host kapısız", map[string]string{"Host": "127.0.0.1"}, "GET", 403},
		{"Origin null", map[string]string{"Origin": "null"}, "POST", 403},
		{"Origin başka kök", map[string]string{"Origin": "http://saldirgan.example"}, "POST", 403},
		{"Origin başka kapı", map[string]string{"Origin": "http://127.0.0.1:1"}, "POST", 403},
		{"Origin kendi kökü", map[string]string{"Origin": self}, "POST", 200},
		{"Sec-Fetch-Site cross-site", map[string]string{"Sec-Fetch-Site": "cross-site"}, "GET", 403},
		{"Sec-Fetch-Site same-site", map[string]string{"Sec-Fetch-Site": "same-site"}, "GET", 403},
		{"Sec-Fetch-Site same-origin", map[string]string{"Sec-Fetch-Site": "same-origin"}, "GET", 200},
		{"Sec-Fetch-Site none", map[string]string{"Sec-Fetch-Site": "none"}, "GET", 200},
		{"OPTIONS başka kök", map[string]string{"Origin": "http://saldirgan.example"}, "OPTIONS", 403},
	}
	passed := int64(0)
	for _, c := range cases {
		res, body := ts.do(t, c.method, "/", c.header, nil)
		if res.StatusCode != c.code {
			t.Fatalf("%s: %d, beklenen %d (%q)", c.name, res.StatusCode, c.code, body)
		}
		if c.code == 200 {
			passed++
		}
		for k := range res.Header {
			if strings.HasPrefix(k, "Access-Control-") {
				t.Fatalf("%s: sunucu kendiliğinden %s göndermemeli", c.name, k)
			}
		}
		if res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: nosniff eksik", c.name)
		}
		if strings.HasPrefix(c.name, "Host") {
			mustContain(t, c.name+" gövdesi", body, self)
		}
	}
	if got := ts.calls.Load(); got != passed {
		t.Fatalf("işleyici %d kez çağrıldı, yalnız geçen %d istekte çağrılmalı", got, passed)
	}
}

// TestTokenIsChecked: belirteçli sunucu her istekte (OPTIONS dahil) Authorization: Bearer'ı denetler;
// eksik ya da yanlışsa 401 ve WWW-Authenticate: Bearer, işleyici çağrılmaz (28.2).
func TestTokenIsChecked(t *testing.T) {
	const token = "abcdefghijklmnop_-0123"
	ts := serveGo(t, `, TR_token: "`+token+`"`, ok200)
	cases := []struct {
		name   string
		method string
		auth   string
		code   int
	}{
		{"eksik", "GET", "", 401},
		{"yanlış", "GET", "Bearer abcdefghijklmnop_-0124", 401},
		{"kısa önek", "GET", "Bearer " + token[:10], 401},
		{"başka şema", "GET", "Basic " + token, 401},
		{"OPTIONS belirteçsiz", "OPTIONS", "", 401},
		{"doğru", "GET", "Bearer " + token, 200},
		{"OPTIONS doğru", "OPTIONS", "Bearer " + token, 200},
	}
	for _, c := range cases {
		h := map[string]string{}
		if c.auth != "" {
			h["Authorization"] = c.auth
		}
		res, _ := ts.do(t, c.method, "/", h, nil)
		if res.StatusCode != c.code {
			t.Fatalf("%s: %d, beklenen %d", c.name, res.StatusCode, c.code)
		}
		if c.code == 401 && res.Header.Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("%s: WWW-Authenticate: Bearer eksik", c.name)
		}
	}
	if got := ts.calls.Load(); got != 2 {
		t.Fatalf("işleyici %d kez çağrıldı, 2 bekleniyordu", got)
	}
	if strings.Contains(ts.vm.Mask("x "+token), token) {
		t.Fatal("belirteç gizli değer olarak kaydedilmeli (19.4)")
	}
}

// TestPublicPathsNeedNoToken: TR_public_paths:'teki tam yol GET ve HEAD'de belirteçsiz açılır; aynı yola başka
// yöntem, listede olmayan yol ve yolun öneki belirteç ister; 28.1a'nın denetimi listedeki yolda da önce gelir,
// yolun ve sorgunun denetimi (28.3) de (28.2).
func TestPublicPathsNeedNoToken(t *testing.T) {
	const token = "abcdefghijklmnop_-0123"
	ts := serveGo(t, `, TR_token: "`+token+`", TR_public_paths: ["/", "/lang/tr.js"]`, ok200)
	self := "http://" + ts.addr()
	cases := []struct {
		name, method, path string
		header             map[string]string
		code               int
	}{
		{"listedeki kök, GET", "GET", "/", nil, 200},
		{"listedeki dosya, GET", "GET", "/lang/tr.js", nil, 200},
		{"listedeki dosya, HEAD", "HEAD", "/lang/tr.js", nil, 200},
		{"listedeki dosya, sorgulu", "GET", "/lang/tr.js?v=1", nil, 200},
		{"aynı kaynaktan sayfa", "GET", "/", map[string]string{"Origin": self, "Sec-Fetch-Site": "same-origin"}, 200},
		{"listedeki yola POST", "POST", "/", nil, 401},
		{"listedeki yola OPTIONS", "OPTIONS", "/", nil, 401},
		{"listede olmayan yol", "GET", "/events", nil, 401},
		{"listedeki yolun uzantısı", "GET", "/lang/tr.js/x", nil, 401},
		{"listedeki yolun öneki", "GET", "/lang/", nil, 401},
		{"listedeki yolun başka yazımı", "GET", "/lang//tr.js", nil, 401},
		{"yabancı Origin listedeki yolda", "GET", "/", map[string]string{"Origin": "http://saldirgan.example"}, 403},
		{"başka site listedeki yolda", "GET", "/", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"geçersiz sorgu listedeki yolda", "GET", "/?a=%ff", nil, 400},
		{"listede olmayan yol, belirteçli", "GET", "/events", map[string]string{"Authorization": "Bearer " + token}, 200},
	}
	want := int64(0)
	for _, c := range cases {
		res, _ := ts.do(t, c.method, c.path, c.header, nil)
		if res.StatusCode != c.code {
			t.Fatalf("%s: %d, beklenen %d", c.name, res.StatusCode, c.code)
		}
		if c.code == 401 && res.Header.Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("%s: WWW-Authenticate: Bearer eksik", c.name)
		}
		if c.code == 200 {
			want++
		}
	}
	if got := ts.calls.Load(); got != want {
		t.Fatalf("işleyici %d kez çağrıldı, %d bekleniyordu: reddedilen istek işleyiciye gitmez", got, want)
	}
}

// TestTokenComparisonIsConstantTime: belirteç sabit sürede karşılaştırılır (crypto/subtle, 28.2). Süre
// ölçüsü gürültülüdür; kaynak denetlenir: karşılaştırma ConstantTimeCompare'dir, == değil.
func TestTokenComparisonIsConstantTime(t *testing.T) {
	src, err := os.ReadFile("guard.go")
	if err != nil {
		t.Fatal(err)
	}
	_, body, _ := strings.Cut(string(src), "func (s *HTTPServer) tokenMatches(")
	body, _, _ = strings.Cut(body, "\n}\n")
	if !strings.Contains(body, "subtle.ConstantTimeCompare(") || strings.Contains(body, "== \"Bearer") || strings.Contains(body, "s.token ==") {
		t.Fatalf("tokenMatches sabit süreli karşılaştırmalı:\n%s", body)
	}
}

// TestPathIsChecked: çözülmüş yolda geçersiz UTF-8, denetim karakteri, \ ya da .. bileşeni 400'dür;
// sorguda yalnız geçersiz UTF-8 400'dür, satır sonu veridir (28.3). İşleyici reddedilende çağrılmaz.
func TestPathIsChecked(t *testing.T) {
	var lastQuery []string
	ts := serveGo(t, "", func(r *Request) *Response {
		if l, ok := r.query.Pairs["x"]; ok {
			lastQuery = nil
			for _, e := range l.(*interp.ListVal).Elements {
				lastQuery = append(lastQuery, e.(*interp.StringVal).Val)
			}
		}
		return ok200(r)
	})
	bad := []string{"/..%5Cx", "/a%5Cb", "/a/../b", "/..", "/a%00b", "/a%0Ab", "/%ff", "/a?x=%ff", "/a?%ff=1"}
	for _, p := range bad {
		if res, _ := ts.raw(t, ts.get127(p)); res.StatusCode != 400 {
			t.Fatalf("%s: %d, 400 bekleniyordu", p, res.StatusCode)
		}
	}
	if ts.calls.Load() != 0 {
		t.Fatalf("reddedilen yolda işleyici çağrıldı: %d", ts.calls.Load())
	}
	if res, _ := ts.raw(t, ts.get127("/a%20b?x=1%0A2&x=3")); res.StatusCode != 200 {
		t.Fatalf("satır sonu taşıyan sorgu geçmeli: %d", res.StatusCode)
	}
	if len(lastQuery) != 2 || lastQuery[0] != "1\n2" || lastQuery[1] != "3" {
		t.Fatalf("sorgunun değerleri geliş sırasıyla olmalı: %q", lastQuery)
	}
}

// FuzzValidPath: yol denetimi hiçbir girdide paniklemez; geçen yol geçerli UTF-8'dir, \, denetim
// karakteri ve .. bileşeni taşımaz (28.3). Tohumlar sıradan testle koşar.
func FuzzValidPath(f *testing.F) {
	for _, s := range []string{"/", "/a b", "/..", "/a/../b", `/a\b`, "/a\x00", "/\xff", "/ş/ğ", "/a/./b", "//", "/....", "/..a"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if !validPath(p) {
			return
		}
		if !utf8.ValidString(p) || strings.ContainsRune(p, '\\') || hasControl(p) || slices.Contains(strings.Split(p, "/"), "..") {
			t.Fatalf("geçmemesi gereken yol geçti: %q", p)
		}
	})
}

// TestBodyLimit: gövde sınırı aşılırsa 413, işleyici çağrılmaz (28.5): bilinen Content-Length sınırı
// aşıyorsa gövde hiç okunmaz; chunked gövdenin toplamı da sınıra sayılır.
func TestBodyLimit(t *testing.T) {
	var got int
	ts := serveGo(t, ", TR_max_body_bytes: 10", func(r *Request) *Response { got = len(r.body); return ok200(r) })
	// Gövdesi gönderilmeyen büyük Content-Length: okunsaydı sunucu beklerdi.
	start := time.Now()
	res, _ := ts.raw(t, "POST / HTTP/1.1\r\nHost: "+ts.addr()+"\r\nContent-Length: 11\r\n\r\n")
	if res.StatusCode != 413 || time.Since(start) > 5*time.Second {
		t.Fatalf("büyük Content-Length gövde okunmadan 413 almalı: %d, %v", res.StatusCode, time.Since(start))
	}
	chunked := "POST / HTTP/1.1\r\nHost: " + ts.addr() + "\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nabcdef\r\n6\r\nghijkl\r\n0\r\n\r\n"
	if res, _ := ts.raw(t, chunked); res.StatusCode != 413 {
		t.Fatalf("chunked toplamı sınırı aşınca 413 almalı: %d", res.StatusCode)
	}
	if ts.calls.Load() != 0 {
		t.Fatalf("sınırı aşan istekte işleyici çağrıldı")
	}
	exact := "POST / HTTP/1.1\r\nHost: " + ts.addr() + "\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n5\r\nabcde\r\n5\r\nfghij\r\n0\r\n\r\n"
	if res, _ := ts.raw(t, exact); res.StatusCode != 200 || got != 10 {
		t.Fatalf("sınırdaki gövde geçmeli: %d, %d bayt", res.StatusCode, got)
	}
}

// TestMaxConcurrent: sınır doluyken gelen istek beklemeden 503 alır, Retry-After'sız; görev açılmaz,
// işleyici çağrılmaz (28.4).
func TestMaxConcurrent(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	ts := serveGo(t, ", TR_max_concurrent: 1", func(r *Request) *Response {
		once.Do(func() { close(entered) })
		<-release
		return ok200(r)
	})
	done := make(chan int)
	go func() {
		res, _ := ts.do(t, "GET", "/", nil, nil)
		done <- res.StatusCode
	}()
	<-entered
	start := time.Now()
	res, _ := ts.do(t, "GET", "/", nil, nil)
	if res.StatusCode != 503 || res.Header.Get("Retry-After") != "" || time.Since(start) > 3*time.Second {
		t.Fatalf("sınır doluyken 503, Retry-After'sız, beklemeden: %d %q %v", res.StatusCode, res.Header.Get("Retry-After"), time.Since(start))
	}
	close(release)
	if code := <-done; code != 200 || ts.calls.Load() != 1 {
		t.Fatalf("ilk istek 200 almalı, işleyici bir kez çağrılmalı: %d, %d", code, ts.calls.Load())
	}
	if res, _ := ts.do(t, "GET", "/", nil, nil); res.StatusCode != 200 {
		t.Fatalf("sınır boşalınca istek geçmeli: %d", res.StatusCode)
	}
}

// TestHeadSendsHeadersOnly: HEAD isteğinde işleyici "GET" görür; sunucu GET'in başlıklarını ve
// Content-Length'ini yazar, gövdeyi göndermez (28.3).
func TestHeadSendsHeadersOnly(t *testing.T) {
	var seen string
	ts := serveGo(t, "", func(r *Request) *Response { seen = r.method; return ok200(r) })
	res, body := ts.raw(t, "HEAD / HTTP/1.1\r\nHost: "+ts.addr()+"\r\nConnection: close\r\n\r\n")
	if seen != "GET" || res.StatusCode != 200 || body != "" || res.Header.Get("Content-Length") != "5" {
		t.Fatalf("HEAD: işleyici %q gördü, %d, gövde %q, Content-Length %q", seen, res.StatusCode, body, res.Header.Get("Content-Length"))
	}
}

// TestHandlerFailureIs500: işleyicinin yakalanmayan hatası istemciye iç bilgisiz 500 olarak gider;
// hata standart hataya kodlu, kaçışlı ve maskeli yazılır; sunucu sürer (28.4).
func TestHandlerFailureIs500(t *testing.T) {
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_if i.TR_path == "/hata" { TR_throw TR_Message("H7614") }
    TR_return TR_Response(TR_body: "tamam")
}
TR_var s = TR_serve_http(f)`, nil)
	res, body := ts.do(t, "GET", "/hata", nil, nil)
	if res.StatusCode != 500 || strings.Contains(body, "H7614") || strings.Contains(body, "Satır") {
		t.Fatalf("500 iç bilgi taşımamalı: %d %q", res.StatusCode, body)
	}
	mustContain(t, "standart hata", ts.errOut.String(), `"/hata"`)
	if res, _ := ts.do(t, "GET", "/", nil, nil); res.StatusCode != 200 {
		t.Fatalf("hatadan sonra sunucu sürmeli: %d", res.StatusCode)
	}
	panics := serveGo(t, "", func(*Request) *Response { panic("kırıldı") })
	if res, _ := panics.do(t, "GET", "/", nil, nil); res.StatusCode != 500 {
		t.Fatalf("Go paniği 500 olmalı: %d", res.StatusCode)
	}
	if res, _ := panics.do(t, "GET", "/", nil, nil); res.StatusCode != 500 || panics.calls.Load() != 2 {
		t.Fatalf("panikten sonra sunucu sürmeli")
	}
}
