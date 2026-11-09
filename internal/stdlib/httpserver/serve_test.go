package httpserver

import (
	"net"
	"strconv"
	"strings"
	"testing"

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
