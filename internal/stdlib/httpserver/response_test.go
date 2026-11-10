package httpserver

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// rawGoServer Go'nun sunucusunu verilen işleyiciyle açar ve ham cevabı verir: Go'nun davranışını
// sabitlemek için.
func rawGoServer(t *testing.T, method string, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	c, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(c, method+" / HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n")
	b, _ := io.ReadAll(bufio.NewReader(c))
	return string(b)
}

// TestGoChangesHeadersSilently Go'nun sunucusunun sessizce yaptıklarını sabitler (28.3): geçersiz adlı
// başlığı siler, değerdeki satır sonunu boşluğa çevirir, 1xx'i ara cevap olarak yazıp sürer, gövdesiz
// cevapta Content-Length'i kendiliğinden koymaz. Go değişirse bu test düşer; TR'nin kuralları bunlar
// yüzünden vardır (TestResponseRejectsWhatGoChanges).
func TestGoChangesHeadersSilently(t *testing.T) {
	out := rawGoServer(t, "GET", func(w http.ResponseWriter, _ *http.Request) {
		w.Header()["Bad Name"] = []string{"x"}
		w.Header()["X-Satir"] = []string{"a\r\nX-Enjekte: b"}
		w.WriteHeader(200)
	})
	if strings.Contains(out, "Bad Name") || !strings.Contains(out, "X-Satir: a  X-Enjekte: b") {
		t.Fatalf("Go geçersiz adı silmeli, satır sonunu boşluğa çevirmeli:\n%s", out)
	}
	out = rawGoServer(t, "GET", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(103)
		w.WriteHeader(200)
		_, _ = io.WriteString(w, "son")
	})
	if !strings.HasPrefix(out, "HTTP/1.1 103") || !strings.Contains(out, "HTTP/1.1 200") {
		t.Fatalf("Go 1xx'i ara cevap yazıp sürmeli:\n%s", out)
	}
	out = rawGoServer(t, "HEAD", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	if strings.Contains(out, "Content-Length") {
		t.Fatalf("Go gövdesiz HEAD cevabında Content-Length koymamalı (TR kendisi yazar):\n%s", out)
	}
}

// TestResponseRejectsWhatGoChanges: Go'nun sessizce sildiği ya da değiştirdiği cevap, kurulurken
// çalışma hatasıdır (28.3).
func TestResponseRejectsWhatGoChanges(t *testing.T) {
	cases := []struct {
		code string
		want messages.Code
	}{
		{`TR_Response(TR_status: 103)`, messages.H7607},
		{`TR_Response(TR_status: 600)`, messages.H7607},
		{`TR_Response(TR_status: 199)`, messages.H7607},
		{`TR_Response(TR_headers: {"Bad Name": "x"})`, messages.H7609},
		{`TR_Response(TR_headers: {"": "x"})`, messages.H7609},
		{`TR_Response(TR_headers: {"X-A": "a\nX-B: b"})`, messages.H7611},
		{`TR_Response(TR_headers: {"X-A": "a\rb"})`, messages.H7611},
		{`TR_Response(TR_headers: {"X-A": "a\u{0}"})`, messages.H7611},
		{`TR_Response(TR_headers: {"X-A": "a", "x-a": "b"})`, messages.H7610},
		{`TR_Response(TR_body: "a", TR_json: 1)`, messages.H7608},
		{`TR_Response(TR_body: 1)`, messages.H7604},
		{`TR_Response(TR_status: "200")`, messages.H7604},
		{`TR_Response(TR_status: 204, TR_body: "a")`, messages.H7631},
		{`TR_Response(TR_status: 205, TR_body: TR_Bytes([1]))`, messages.H7631},
		{`TR_Response(TR_status: 304, TR_json: {})`, messages.H7631},
		{`TR_Response(TR_status: 204, TR_json: TR_none)`, messages.H7631},
		{`TR_Response(TR_headers: {"Content-Length": "3"})`, messages.H7632},
		{`TR_Response(TR_headers: {"transfer-encoding": "chunked"})`, messages.H7632},
		{`TR_Response(TR_headers: {"Connection": "keep-alive"})`, messages.H7633},
		{`TR_Response(TR_headers: {"Connection": "close, upgrade"})`, messages.H7633},
		{`TR_Response(TR_headers: {"connection": ""})`, messages.H7633},
	}
	for _, c := range cases {
		o := trrun.Exec(t, c.code)
		if !messages.Has(o.Err, c.want) {
			t.Fatalf("%s: %s bekleniyordu, alınan %v", c.code, c.want, o.Err)
		}
	}
}

// TestNoBodyStatusesAndServerHeaders: 204, 205 ve 304 gövdesiz kurulur; boş metin gövde değildir;
// iletilerin önerdiği yazımlar (gövdeli cevap için 200, Connection: close) çalışır; Connection: close
// harf büyüklüğüne bakılmadan geçer ve close olarak okunur (28.3).
func TestNoBodyStatusesAndServerHeaders(t *testing.T) {
	o := trrun.MustExec(t, `TR_var a = TR_Response(TR_status: 204)
TR_var b = TR_Response(TR_status: 205, TR_body: "")
TR_var c = TR_Response(TR_status: 304, TR_headers: {"ETag": "\"x\""})
TR_var d = TR_Response(TR_status: 200, TR_body: "a")
TR_var e = TR_Response(TR_headers: {"Connection": "close"})
TR_var f = TR_Response(TR_headers: {"connection": "Close"})
TR_var result = [a.TR_status, b.TR_body.TR_length, c.TR_headers["ETag"], d.TR_status, e.TR_headers["Connection"], f.TR_headers["Connection"]]`)
	if got := o.Value.Inspect(); got != `[204, 0, "\"x\"", 200, "close", "close"]` {
		t.Fatalf("gövdesiz cevaplar ve Connection: %s", got)
	}
}

// exchange bağlantıyı kapatmayı istemeden (HTTP/1.1, Connection'sız) bir GET yazar; cevabı ve sunucunun
// cevaptan sonra bağlantıyı kapatıp kapatmadığını verir.
func exchange(t *testing.T, addr, path string) (*http.Response, string, bool) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	_, _ = io.WriteString(c, "GET "+path+" HTTP/1.1\r\nHost: "+addr+"\r\n\r\n")
	br := bufio.NewReader(c)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("cevap okunamadı: %v", err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	_, err = br.ReadByte()
	return res, string(b), err == io.EOF
}

// TestGoClosesOnlyOnLowercaseClose Go'nun davranışını sabitler (28.3): işleyicinin yazdığı
// Connection: close cevaptan sonra bağlantıyı kapatır, Connection: Close kapatmaz. TR bu yüzden close
// yazar (TestConnectionCloseClosesTheConnection).
func TestGoClosesOnlyOnLowercaseClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Connection"] = []string{strings.TrimPrefix(r.URL.Path, "/")}
	}))
	defer srv.Close()
	addr := srv.Listener.Addr().String()
	if _, _, closed := exchange(t, addr, "/close"); !closed {
		t.Fatal("Go Connection: close'da bağlantıyı kapatmalı")
	}
	if _, _, closed := exchange(t, addr, "/Close"); closed {
		t.Fatal("Go Connection: Close'da bağlantıyı açık tutuyordu; değiştiyse TR'nin close yazması gereksizleşir")
	}
}

// TestConnectionCloseClosesTheConnection: Connection: close (harf büyüklüğü ne olursa olsun) cevapta
// close yazılır ve sunucu cevaptan sonra bağlantıyı kapatır; 204 gövdesiz ve Content-Length'siz gider (28.3).
func TestConnectionCloseClosesTheConnection(t *testing.T) {
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_if i.TR_path == "/bos" { TR_return TR_Response(TR_status: 204) }
    TR_if i.TR_path == "/acik" { TR_return TR_Response(TR_body: "x") }
    TR_return TR_Response(TR_body: "x", TR_headers: {"connection": i.TR_query["c"][0]})
}
TR_var s = TR_serve_http(f)`, nil)
	for _, p := range []string{"/?c=close", "/?c=Close", "/?c=CLOSE"} {
		res, body, closed := exchange(t, ts.addr(), p)
		// Go'nun istemcisi Connection: close'u başlıktan silip res.Close'a koyar.
		if body != "x" || !res.Close || !closed {
			t.Fatalf("%s: Close %v, gövde %q, kapandı %v", p, res.Close, body, closed)
		}
	}
	if _, _, closed := exchange(t, ts.addr(), "/acik"); closed {
		t.Fatal("Connection verilmeyen cevapta bağlantı açık kalmalı")
	}
	res, body, _ := exchange(t, ts.addr(), "/bos")
	if res.StatusCode != 204 || body != "" || res.Header.Get("Content-Length") != "" {
		t.Fatalf("204: %d %q Content-Length %q", res.StatusCode, body, res.Header.Get("Content-Length"))
	}
}

// TestResponseValues: varsayılan durum 200; metin text/plain, baytlar octet-stream, json
// application/json; verilen content-type varsayılanı ezer; okunur üyeler (28.3).
func TestResponseValues(t *testing.T) {
	o := trrun.MustExec(t, `TR_var a = TR_Response(TR_body: "ş")
TR_var b = TR_Response(TR_body: TR_Bytes([1, 2]), TR_status: 201)
TR_var c = TR_Response(TR_json: {"a": [1, TR_none]})
TR_var d = TR_Response(TR_body: "<p>", TR_headers: {"content-type": "text/html"})
TR_var e = TR_Response()
TR_var result = [a.TR_status, a.TR_headers["Content-Type"], a.TR_body.TR_length, b.TR_status, b.TR_headers["content-type"],
    TR_String(c.TR_body), c.TR_headers["Content-Type"], d.TR_headers["Content-Type"], e.TR_body.TR_length, e.TR_headers.TR_get("Content-Type") ?? "-"]`)
	want := `[200, "text/plain; charset=utf-8", 2, 201, "application/octet-stream", "{\"a\":[1,null]}", "application/json", "text/html", 0, "-"]`
	if got := o.Value.Inspect(); got != want {
		t.Fatalf("cevap değerleri:\n%s\nbeklenen\n%s", got, want)
	}
}

// TestResponseIsWrittenAsGiven: sunucu cevabı başlıklarıyla yazar; işleyici vermedikçe nosniff ekler,
// verdiği content-type'ın yanına Go ikinci bir Content-Type eklemez (28.3).
func TestResponseIsWrittenAsGiven(t *testing.T) {
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_if i.TR_path == "/kendi" { TR_return TR_Response(TR_body: "x", TR_headers: {"x-content-type-options": "kendi"}) }
    TR_return TR_Response(TR_body: "<b>", TR_headers: {"content-type": "text/html; charset=utf-8", "Cache-Control": "no-store"})
}
TR_var s = TR_serve_http(f)`, nil)
	res, body := ts.do(t, "GET", "/", nil, nil)
	if body != "<b>" || len(res.Header.Values("Content-Type")) != 1 || res.Header.Get("Content-Type") != "text/html; charset=utf-8" ||
		res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("cevap verildiği gibi yazılmalı: %v %q", res.Header, body)
	}
	if res, _ := ts.do(t, "GET", "/kendi", nil, nil); res.Header.Values("X-Content-Type-Options")[0] != "kendi" || len(res.Header.Values("X-Content-Type-Options")) != 1 {
		t.Fatalf("işleyicinin verdiği X-Content-Type-Options korunmalı: %v", res.Header)
	}
}

// TestRequestValues: TR_Request'in varsayılanları, adlı argümanları ve hataları; HEAD kurulmaz,
// geçersiz yöntem adı hatadır (28.3). Başlıklar harf büyüklüğüne bakmaz; olmayan başlık hatadır,
// ileti TR_get'i önerir ve önerilen yazım çalışır.
func TestRequestValues(t *testing.T) {
	o := trrun.MustExec(t, `TR_var i = TR_Request()
TR_var j = TR_Request(TR_method: "PATCH", TR_path: "/a b", TR_query: {"x": ["1", "2"]}, TR_headers: {"X-Ad": "a"}, TR_body: TR_Bytes("g"))
TR_var result = [i.TR_method, i.TR_path, i.TR_query, i.TR_body.TR_length, j.TR_method, j.TR_path, j.TR_query["x"], j.TR_headers["x-ad"], j.TR_headers.TR_get("X-AD"), j.TR_headers.TR_get("yok") ?? "-", TR_String(j.TR_body)]`)
	want := `["GET", "/", {}, 0, "PATCH", "/a b", ["1", "2"], "a", "a", "-", "g"]`
	if got := o.Value.Inspect(); got != want {
		t.Fatalf("istek değerleri:\n%s\nbeklenen\n%s", got, want)
	}
	for code, want := range map[string]messages.Code{
		`TR_Request(TR_method: "HEAD")`:           messages.H7605,
		`TR_Request(TR_method: "GE T")`:           messages.H7606,
		`TR_Request(TR_method: "")`:               messages.H7606,
		`TR_Request(TR_method: "ĞET")`:            messages.H7606,
		`TR_Request(TR_query: {"x": "1"})`:        messages.H7604,
		`TR_Request(TR_body: "metin")`:            messages.H7604,
		`TR_Request().TR_headers["Content-Type"]`: messages.H7603,
		`TR_Request().TR_headers[1]`:              messages.H7602,
	} {
		if o := trrun.Exec(t, code); !messages.Has(o.Err, want) {
			t.Fatalf("%s: %s bekleniyordu, alınan %v", code, want, o.Err)
		}
	}
	if o := trrun.MustExec(t, `TR_var result = TR_Request(TR_method: "GET").TR_headers.TR_get("Content-Type")`); o.Value.Inspect() != trrun.MustExec(t, "TR_var result = TR_none").Value.Inspect() {
		t.Fatalf("H7603'ün önerdiği yazım yok vermeli: %s", o.Value.Inspect())
	}
	if o := trrun.MustExec(t, "TR_var result = TR_Request(TR_method: \"!#$%&'*+-.^_`|~09azAZ\").TR_method"); o.Value.Inspect() != "!#$%&'*+-.^_`|~09azAZ" {
		t.Fatalf("RFC 9110 token'ı geçerli yöntem adıdır: %s", o.Value.Inspect())
	}
}

// TestRequestFromTheWire: sunucunun kurduğu istek: yol çözülmüş ve sorgusuz, sorgu sırasıyla, aynı
// başlığın değerleri virgülle birleşik, Host başlıklarda (28.3).
func TestRequestFromTheWire(t *testing.T) {
	var got *Request
	ts := serveGo(t, "", func(r *Request) *Response { got = r; return ok200(r) })
	ts.raw(t, "POST /a%20b?k=1&k=2&z= HTTP/1.1\r\nHost: "+ts.addr()+"\r\nX-Coklu: a\r\nx-coklu: b\r\nContent-Length: 3\r\nConnection: close\r\n\r\nabc")
	host, _ := got.headers.get("host")
	multi, _ := got.headers.get("X-COKLU")
	if got.method != "POST" || got.path != "/a b" || got.query.Inspect() != `{"k": ["1", "2"], "z": [""]}` || host != ts.addr() || multi != "a, b" || string(got.body) != "abc" {
		t.Fatalf("istek: %s %q %s host=%q çoklu=%q gövde=%q", got.method, got.path, got.query.Inspect(), host, multi, got.body)
	}
}
