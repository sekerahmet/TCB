package httpserver

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

const handlerTR = "TR_func f(i: TR_Request) -> TR_Response { TR_return TR_Response() }\n"

// TestServeArgumentErrors: adres ayrıştırılmış hâliyle denetlenir, dış adres belirteç ister, belirteç
// en az 16 karakter ve [0-9A-Za-z_-], kapı 0–65535, sınırlar pozitif; hatada sunucu açılmaz (28.1,
// 28.2, 28.4, 28.5).
func TestServeArgumentErrors(t *testing.T) {
	cases := map[string]messages.Code{
		`TR_host: "localhost"`:                               messages.H7621,
		`TR_host: "::1"`:                                     messages.H7621,
		`TR_host: "[::1]"`:                                   messages.H7621,
		`TR_host: "0:0:0:0:0:0:0:1"`:                         messages.H7621,
		`TR_host: "127.0.0.2"`:                               messages.H7621,
		`TR_host: "127.0.0.2", TR_token: "abcdefghijklmnop"`: messages.H7621,
		`TR_host: "sunucu.example"`:                          messages.H7622,
		`TR_host: "0.0.0.0"`:                                 messages.H7623,
		`TR_host: "0.0.0.0", TR_token: ""`:                   messages.H7624,
		`TR_token: "abcdefghijklmno"`:                        messages.H7624,
		`TR_token: "abcdefghijklmnop!"`:                      messages.H7624,
		`TR_token: "abcdefghijklmnopğ"`:                      messages.H7624,
		`TR_port: -1`:                                        messages.H7627,
		`TR_port: 65536`:                                     messages.H7627,
		`TR_max_concurrent: 0`:                               messages.H7628,
		`TR_max_body_bytes: -1`:                              messages.H7628,
		`TR_max_body_bytes: 1.5`:                             messages.H7628,
		`TR_port: "80"`:                                      messages.H7604,
		// TR_public_paths: (28.2): geçersiz yol ve belirteçsiz liste (boş da olsa: liste bir şey korumaz); boş liste
		// belirteçli sunucuda hata değildir (TestServeSuggestionsWork).
		`TR_token: "abcdefghijklmnop", TR_public_paths: ["index.html"]`: messages.H7660,
		`TR_token: "abcdefghijklmnop", TR_public_paths: ["/a?b=1"]`:     messages.H7660,
		`TR_token: "abcdefghijklmnop", TR_public_paths: ["/a/../b"]`:    messages.H7660,
		`TR_token: "abcdefghijklmnop", TR_public_paths: ["/a\\b"]`:      messages.H7660,
		`TR_token: "abcdefghijklmnop", TR_public_paths: [""]`:           messages.H7660,
		`TR_public_paths: ["/"]`:                                        messages.H7661,
		`TR_public_paths: []`:                                           messages.H7661,
		`TR_token: "abcdefghijklmnop", TR_public_paths: "/"`:            messages.H7604,
		`TR_token: "abcdefghijklmnop", TR_public_paths: [1]`:            messages.H7604,
	}
	for args, want := range cases {
		o := trrun.Exec(t, handlerTR+"TR_serve_http(f, "+args+")")
		if !messages.Has(o.Err, want) {
			t.Fatalf("%s: %s bekleniyordu, alınan %v", args, want, o.Err)
		}
	}
	if o := trrun.Exec(t, `TR_serve_http(1)`); o.Err == nil {
		t.Fatal("işleyici olmayan argüman hatadır")
	}
}

// TestServeSuggestionsWork: iletilerin önerdiği yazım çalışır (yerel adres ve belirteç).
func TestServeSuggestionsWork(t *testing.T) {
	o := trrun.Exec(t, handlerTR+`TR_serve_http(f, TR_host: "localhost")`)
	mustContain(t, "H7621", o.Err.Error(), `"127.0.0.1"`)
	o = trrun.Exec(t, handlerTR+`TR_serve_http(f, TR_host: "0.0.0.0")`)
	mustContain(t, "H7623", o.Err.Error(), "TR_secret_token()")
	// Dış adrese bağlanmak bu makinede güvenlik duvarını uyarır; önerinin belirteç yarısı yerelde koşar.
	trrun.MustExec(t, handlerTR+`TR_var s = TR_serve_http(f, TR_host: "127.0.0.1", TR_token: TR_secret_token())
TR_var result = s.TR_close()`)
	// H7660'ın ve H7661'in önerdiği yazım (28.2, TR_public_paths:); belirteçli sunucuda boş liste hata değildir.
	o = trrun.Exec(t, handlerTR+`TR_serve_http(f, TR_token: "abcdefghijklmnop", TR_public_paths: ["index.html"])`)
	mustContain(t, "H7660", o.Err.Error(), `TR_public_paths: ["/", "/index.html"]`)
	o = trrun.Exec(t, handlerTR+`TR_serve_http(f, TR_public_paths: ["/"])`)
	mustContain(t, "H7661", o.Err.Error(), "TR_token: TR_secret_token()")
	trrun.MustExec(t, handlerTR+`TR_var s = TR_serve_http(f, TR_token: TR_secret_token(), TR_public_paths: ["/", "/index.html"])
TR_var result = s.TR_close()`)
	trrun.MustExec(t, handlerTR+`TR_var s = TR_serve_http(f, TR_token: TR_secret_token(), TR_public_paths: [])
TR_var result = s.TR_close()`)
}

// TestPortInUse: kullanılan kapı çağrı hatasıdır (H7625), ileti işletim sisteminin metnini taşımaz (28.9).
func TestPortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	o := trrun.Exec(t, handlerTR+"TR_serve_http(f, TR_port: "+port+")")
	if !messages.Has(o.Err, messages.H7625) {
		t.Fatalf("kullanılan kapı H7625 vermeli: %v", o.Err)
	}
	if text := o.Err.Error(); strings.Contains(text, "bind") || strings.Contains(text, "Only one usage") || strings.Contains(text, "address already") {
		t.Fatalf("ileti işletim sisteminin metnini taşımamalı: %s", text)
	}
}

// TestPortZeroPicksAPort: kapı verilmezse ya da 0 ise işletim sistemi seçer; gerçek kapı s.TR_port (28.1).
func TestPortZeroPicksAPort(t *testing.T) {
	ts := serveGo(t, ", TR_port: 0", ok200)
	if ts.s.port <= 0 {
		t.Fatalf("gerçek kapı: %d", ts.s.port)
	}
	if res, _ := ts.do(t, "GET", "/", nil, nil); res.StatusCode != 200 {
		t.Fatalf("seçilen kapıda istek: %d", res.StatusCode)
	}
}

// TestServerLogIsCoded: Go'nun sunucu günlüğü kodlu iletiye çevrilir, 8.4'ün kaçışıyla (28.4).
func TestServerLogIsCoded(t *testing.T) {
	ts := serveGo(t, "", ok200)
	ts.s.srv.ErrorLog.Printf("http: deneme \x1b[31m")
	out := ts.errOut.String()
	if !strings.Contains(out, messages.Text(messages.H7629, "")[:10]) || strings.Contains(out, "\x1b") {
		t.Fatalf("günlük satırı kodlu ve kaçışlı olmalı: %q", out)
	}
}

// TestTimeoutDefaults 28.5'in süre sınırlarını sabitler: başlık 60 saniye, iki gövde okuması arası 60
// saniye, boşta bağlantı 5 saniye; cevap yazmanın ve isteğin bütününün süre sınırı yoktur (akış kesilmez).
func TestTimeoutDefaults(t *testing.T) {
	if headerTimeout != 60*time.Second || bodyReadGap != 60*time.Second || idleTimeout != 5*time.Second {
		t.Fatalf("süre sınırları %v, %v, %v; 28.5 60, 60 ve 5 saniye diyor", headerTimeout, bodyReadGap, idleTimeout)
	}
	ts := serveGo(t, "", ok200)
	srv := ts.s.srv
	if srv.ReadHeaderTimeout != headerTimeout || srv.IdleTimeout != idleTimeout || srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Fatalf("sunucunun süreleri: başlık %v, boşta %v, okuma %v, yazma %v", srv.ReadHeaderTimeout, srv.IdleTimeout, srv.ReadTimeout, srv.WriteTimeout)
	}
}

// TestTimeoutsCloseTheConnection: başlık süresinde gelmezse, iki gövde okuması arası süreyi geçerse ya da
// bağlantı boşta süreyi aşarsa bağlantı kapanır; işleyici yalnız tam gelen istekte çağrılır (28.5).
// Süreler testte kısaltılır.
func TestTimeoutsCloseTheConnection(t *testing.T) {
	oldH, oldI, oldB := headerTimeout, idleTimeout, bodyReadGap
	headerTimeout, idleTimeout, bodyReadGap = 200*time.Millisecond, 200*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { headerTimeout, idleTimeout, bodyReadGap = oldH, oldI, oldB })
	ts := serveGo(t, "", ok200)
	limited := serveGo(t, ", TR_max_body_bytes: 10", ok200)
	tokened := serveGo(t, `, TR_token: "abcdefghijklmnop"`, ok200)
	head := func(s *testServer, method, path, rest string) string {
		return method + " " + path + " HTTP/1.1\r\nHost: " + s.addr() + "\r\n" + rest
	}
	// Süreyle kapananlar ~200 ms bekler; gövdesi okunmadan reddedilenler cevaptan hemen sonra kapanır,
	// istemci gövdeyi göndermeden sussa da (öncesinde 5 saniyelik istemci sınırına kadar asılı kalıyordu).
	cases := []struct {
		name  string
		s     *testServer
		req   string
		first string // cevabın ilk satırı; boşsa cevap yok
		timed bool
	}{
		{"başlık", ts, head(ts, "GET", "/", ""), "", true},
		{"gövde", ts, head(ts, "POST", "/", "Content-Length: 10\r\n\r\nab"), "", true},
		{"boşta", ts, head(ts, "GET", "/", "\r\n"), "HTTP/1.1 200", true},
		{"413, Content-Length sınırı aşıyor, gövde yok", limited, head(limited, "POST", "/", "Content-Length: 100\r\n\r\n"), "HTTP/1.1 413", false},
		{"413, chunked sınırı aşıp susuyor", limited, head(limited, "POST", "/", "Transfer-Encoding: chunked\r\n\r\n14\r\n"+strings.Repeat("a", 20)+"\r\n"), "HTTP/1.1 413", false},
		{"401, belirteçsiz, gövde yok", tokened, head(tokened, "POST", "/", "Content-Length: 100\r\n\r\n"), "HTTP/1.1 401", false},
		{"400, geçersiz yol, gövde yok", ts, head(ts, "POST", "/a/../b", "Content-Length: 100\r\n\r\n"), "HTTP/1.1 400", false},
		{"403, başka Host, gövde yok", ts, "POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 100\r\n\r\n", "HTTP/1.1 403", false},
	}
	for _, tc := range cases {
		c, err := net.DialTimeout("tcp", tc.s.addr(), 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_ = c.SetDeadline(time.Now().Add(5 * time.Second))
		start := time.Now()
		if _, err := io.WriteString(c, tc.req); err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(bufio.NewReader(c)) // bağlantı kapanınca biter; 5 saniyede kapanmazsa süre hatası
		d := time.Since(start)
		c.Close()
		if d > 3*time.Second || (tc.timed && d < 150*time.Millisecond) {
			t.Errorf("%s: bağlantı %v sonra kapandı", tc.name, d)
		}
		if line, _, _ := strings.Cut(string(got), "\r\n"); !strings.HasPrefix(line, tc.first) || (tc.first == "") != (len(got) == 0) {
			t.Errorf("%s: cevap %q, ilk satır %q bekleniyordu", tc.name, got, tc.first)
		}
	}
	if ts.calls.Load() != 1 || limited.calls.Load() != 0 || tokened.calls.Load() != 0 {
		t.Fatalf("işleyici yalnız tam gelen istekte çağrılmalı: %d %d %d", ts.calls.Load(), limited.calls.Load(), tokened.calls.Load())
	}
}
