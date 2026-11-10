package httpserver

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// closeOn s.TR_close()'u VM'nin kilidini tutan bir programla çağırır.
func closeOn(t *testing.T, ts *testServer) string {
	t.Helper()
	if _, err := ts.vm.Run(trrun.Parse(t, "TR_var kapandı = [s.TR_close(), s.TR_close()]")); err != nil {
		t.Fatal(err)
	}
	v, _ := ts.vm.GlobalEnv.Get("kapandı")
	return v.Inspect()
}

// refused kapı artık bağlantı kabul etmiyor mu?
func refused(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return true
	}
	c.Close()
	return false
}

// TestCloseCancelsRunningHandlers: s.TR_close() yeni bağlantıyı kabul etmez, süren işleyici görevini
// iptal eder, en çok 2 saniye bekler; iptal edilen isteğin cevabı gönderilmez. İlk çağrı TR_true, ikinci
// TR_false verir (28.8).
func TestCloseCancelsRunningHandlers(t *testing.T) {
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_var n = 0
    TR_for k TR_in 0..2000000000 { n += 1 }
    TR_return TR_Response(TR_body: "bitti")
}
TR_var s = TR_serve_http(f)`, nil)
	answered := make(chan error, 1)
	go func() {
		_, err := (&http.Client{Timeout: 20 * time.Second}).Get("http://" + ts.addr() + "/")
		answered <- err
	}()
	time.Sleep(300 * time.Millisecond) // istek işleyicide
	start := time.Now()
	if got := closeOn(t, ts); got != "[true, false]" {
		t.Fatalf("kapat iki çağrıda [doğru, yanlış] vermeli: %s", got)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("kapanış en çok 2 saniye beklemeli: %v", d)
	}
	if err := <-answered; err == nil {
		t.Fatal("iptal edilen isteğin cevabı gönderilmemeli")
	}
	if !refused(ts.addr()) {
		t.Fatal("kapanan sunucu bağlantı kabul etmemeli")
	}
}

// TestCloseFinishesAWrittenResponse: kapanış yazılmakta olan cevabı tamamlar; boştaki sunucu hemen
// kapanır (28.8).
func TestCloseFinishesAWrittenResponse(t *testing.T) {
	ts := serveGo(t, "", ok200)
	if res, body := ts.do(t, "GET", "/", nil, nil); res.StatusCode != 200 || body != "tamam" {
		t.Fatalf("kapanıştan önce cevap tam gelmeli: %d %q", res.StatusCode, body)
	}
	start := time.Now()
	if got := closeOn(t, ts); got != "[true, false]" || time.Since(start) > time.Second {
		t.Fatalf("boştaki sunucu hemen kapanmalı: %s %v", got, time.Since(start))
	}
}

// TestProgramEndClosesTheServer: s.TR_wait() çağırmadan biten program açık sunucuyu kapatır; program
// asılı kalmaz (28.8).
func TestProgramEndClosesTheServer(t *testing.T) {
	v := interp.New()
	v.Out, v.Err = io.Discard, &bytes.Buffer{}
	start := time.Now()
	if _, err := v.Run(trrun.Parse(t, handlerTR+"TR_var s = TR_serve_http(f)\nTR_var kapı = s.TR_port")); err != nil {
		t.Fatal(err)
	}
	port, _ := v.GlobalEnv.Get("kapı")
	if !refused("127.0.0.1:"+port.Inspect()) || time.Since(start) > 3*time.Second {
		t.Fatalf("program sonunda sunucu kapanmalı (%v)", time.Since(start))
	}
}

// TestWaitReturnsStopRequested: durdurmayı üstlenen programda s.TR_wait() "durdurma istendi" hatasıyla
// döner, sunucu program kapatana kadar sürer (28.8, 1.6).
func TestWaitReturnsStopRequested(t *testing.T) {
	v := interp.New()
	out, w := io.Pipe()
	v.Out, v.Err = w, &bytes.Buffer{}
	code := handlerTR + `TR_handle_stop()
TR_var s = TR_serve_http(f)
TR_print(s.TR_port)
TR_var kod = TR_error_code { s.TR_wait() }`
	prog := trrun.Parse(t, code)
	done := make(chan error, 1)
	go func() {
		_, err := v.Run(prog)
		w.Close()
		done <- err
	}()
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.Copy(io.Discard, out) }()
	addr := "127.0.0.1:" + strings.TrimSpace(line)
	if refused(addr) {
		t.Fatal("sunucu açık olmalı")
	}
	time.Sleep(100 * time.Millisecond) // bekle başlasın
	v.RequestStop(2)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if c, _ := v.GlobalEnv.Get("kod"); c.Inspect() != "H2530" {
		t.Fatalf("bekle durdurma istendi hatasıyla dönmeli: %v", c)
	}
	if !refused(addr) {
		t.Fatal("program bitince sunucu kapanmalı")
	}
}

// TestCloseEndsOpenStreams: kapanış açık akışları kapatır ve 2 saniyeyi beklemez (28.8): yayın
// dinleyicisinin cevabı düzgün biter (istemci Last-Event-ID ile döner), istek akışının bloğu iptal
// edilir ve akışı düzgün bitirilmeden kesilir; blok durur.
func TestCloseEndsOpenStreams(t *testing.T) {
	say, n := counter()
	ts := serveTR(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g)
TR_func f(i: TR_Request) -> TR_Response {
    TR_if i.TR_path == "/" { TR_return y.TR_subscribe(i) }
    TR_return TR_stream_response((a) => {
        TR_for k TR_in 0..100000 {
            say()
            a.TR_send_event(k)
            TR_sleep(0.01)
        }
    })
}
TR_var s = TR_serve_http(f)`, map[string]interp.Value{"say": say})
	_, listener, _ := subscribeRaw(t, ts, "GET", "")
	frame(t, listener)
	res, _ := streamGet(t, ts, "GET", "/akis")
	stream := bufio.NewReader(res.Body)
	if line, err := stream.ReadString('\n'); err != nil || line != "data: 0\n" {
		t.Fatalf("ilk olay: %q %v", line, err)
	}
	start := time.Now()
	if got := closeOn(t, ts); got != "[true, false]" {
		t.Fatalf("kapat: %s", got)
	}
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Fatalf("açık akışlar kapanışı bekletmemeli: %v", d)
	}
	if rest, err := io.ReadAll(listener); err != nil || len(rest) != 0 {
		t.Fatalf("dinleyicinin cevabı düzgün bitmeli: %q %v", rest, err)
	}
	if _, err := io.ReadAll(stream); err == nil {
		t.Fatal("istek akışı düzgün bitirilmeden kesilmeli")
	}
	before := n.Load()
	time.Sleep(200 * time.Millisecond)
	if n.Load() != before {
		t.Fatal("kapanıştan sonra akışın bloğu durmalı")
	}
}
