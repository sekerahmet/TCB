package httpserver

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// counter TR koduna verilen, çağrıldıkça artan bir sayaçtır (say()).
func counter() (*interp.BuiltinVal, *atomic.Int64) {
	n := &atomic.Int64{}
	return &interp.BuiltinVal{Name: "say", Fn: func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) {
		n.Add(1)
		return interp.Null, nil
	}}, n
}

// streamGet akışı ham bağlantıyla ister; cevabın başlıklarını ve bağlantının okuyucusunu verir.
func streamGet(t *testing.T, ts *testServer, method, path string) (*http.Response, net.Conn) {
	t.Helper()
	c, err := net.DialTimeout("tcp", ts.addr(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	_ = c.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := io.WriteString(c, method+" "+path+" HTTP/1.1\r\nHost: "+ts.addr()+"\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(c), &http.Request{Method: method})
	if err != nil {
		t.Fatal(err)
	}
	return res, c
}

// TestRequestStreamWritesEvents: istek akışının her olayı yalnız data: satırıdır, event: ve id yoktur;
// veri 13.8'in JSON'udur, metinlerdeki gizli değer JSON'a çevrilmeden önce *** olur (28.7, 19.4).
func TestRequestStreamWritesEvents(t *testing.T) {
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_return TR_stream_response((a) => {
        a.TR_send_event({"parça": "mer"})
        a.TR_send_event("satır\nsonu gizli-deger-123")
        a.TR_send_event({"gizli-deger-123": [1, 2]})
    })
}
TR_var s = TR_serve_http(f)`, nil)
	ts.vm.AddSecret("gizli-deger-123")
	res, _ := streamGet(t, ts, "GET", "/")
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("akış düzgün bitmeli: %v", err)
	}
	if got := res.Header.Get("Content-Type"); got != "text/event-stream; charset=utf-8" || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("başlıklar: %v", res.Header)
	}
	want := "data: {\"parça\":\"mer\"}\n\n" + "data: \"satır\\nsonu ***\"\n\n" + "data: {\"***\":[1,2]}\n\n"
	if string(body) != want {
		t.Fatalf("akış:\n%q\nbeklenen:\n%q", body, want)
	}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "event:") {
			t.Fatalf("istek akışında id ve event satırı olmaz: %q", line)
		}
	}
}

// TestRequestStreamHeartbeat: açık akışa nabız gider; varsayılanı SSE yorum satırıdır (:), TR_heartbeat:
// verilirse yalnız data: satırı olan bir olaydır (28.7). Aralık testte kısaltılır.
func TestRequestStreamHeartbeat(t *testing.T) {
	old := heartbeatInterval
	heartbeatInterval = 30 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = old })
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_var blok = (a: TR_EventStream) => {
        a.TR_send_event(1)
        TR_sleep(0.3)
        a.TR_send_event(2)
    }
    TR_if i.TR_path == "/veri" { TR_return TR_stream_response(blok, TR_heartbeat: {"tür": "nabız"}) }
    TR_return TR_stream_response(blok)
}
TR_var s = TR_serve_http(f)`, nil)
	for path, beat := range map[string]string{"/": ":\n\n", "/veri": "data: {\"tür\":\"nabız\"}\n\n"} {
		res, _ := streamGet(t, ts, "GET", path)
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		// Blok son olaydan sonra da bir an sürer: akış kapanana kadar bir nabız daha gidebilir.
		text := string(body)
		last := strings.Index(text, "data: 2\n\n")
		if !strings.HasPrefix(text, "data: 1\n\n") || last < 0 || !strings.Contains(text[:last], beat) ||
			strings.ReplaceAll(text[last+len("data: 2\n\n"):], beat, "") != "" {
			t.Fatalf("%s: nabız %q olaylar arasında olmalı:\n%q", path, beat, text)
		}
		if strings.Contains(text, "id:") {
			t.Fatalf("%s: nabzın kimliği yoktur: %q", path, text)
		}
	}
}

// TestHeartbeatDefaultIs15Seconds nabzın varsayılan aralığını sabitler (28.7).
func TestHeartbeatDefaultIs15Seconds(t *testing.T) {
	if heartbeatInterval != 15*time.Second {
		t.Fatalf("nabız aralığı %v, 28.7 15 saniye diyor", heartbeatInterval)
	}
}

// TestRequestStreamClientGoneCancelsBlock: istemci koparsa bloğun görevi iptal edilir (28.7, 10.9):
// sayaç durur, görev biter.
func TestRequestStreamClientGoneCancelsBlock(t *testing.T) {
	for _, path := range []string{"/yazan", "/susan"} {
		t.Run(path, func(t *testing.T) { clientGoneCancels(t, path) })
	}
}

// clientGoneCancels: "/yazan"ın bloğu her turda yazar (kopan bağlantıya yazma iptal eder), "/susan"ınki
// ilk olaydan sonra yazmaz: onu yalnız isteğin bağlamı iptal eder (nabız 15 saniyede bir).
func clientGoneCancels(t *testing.T, path string) {
	say, n := counter()
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_return TR_stream_response((a) => {
        a.TR_send_event(0)
        TR_for k TR_in 1..100000 {
            say()
            TR_if i.TR_path == "/yazan" { a.TR_send_event(k) }
            TR_sleep(0.01)
        }
    })
}
TR_var s = TR_serve_http(f)`, map[string]interp.Value{"say": say})
	res, c := streamGet(t, ts, "GET", path)
	line, err := bufio.NewReader(res.Body).ReadString('\n')
	if err != nil || line != "data: 0\n" {
		t.Fatalf("ilk olay: %q %v", line, err)
	}
	c.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		before := n.Load()
		time.Sleep(200 * time.Millisecond)
		if n.Load() == before {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("istemci gidince blok durmalı: sayaç %d", n.Load())
		}
	}
}

// TestRequestStreamBlockErrorCutsTheStream: blok hatayla biterse hata standart hataya kodlu satırdır ve
// akış düzgün bitirilmeden kesilir: istemci yarım cevabı tam sanmaz (28.7).
func TestRequestStreamBlockErrorCutsTheStream(t *testing.T) {
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_return TR_stream_response((a) => {
        a.TR_send_event("ilk")
        TR_throw TR_Message("H7614")
    })
}
TR_var s = TR_serve_http(f)`, nil)
	res, _ := streamGet(t, ts, "GET", "/akis")
	body, err := io.ReadAll(res.Body)
	if err == nil {
		t.Fatalf("akış kesilmeli, düzgün bitti: %q", body)
	}
	if string(body) != "data: \"ilk\"\n\n" {
		t.Fatalf("kesilmeden önce gönderilen: %q", body)
	}
	mustContain(t, "standart hata", ts.errOut.String(), `"/akis"`)
}

// TestSendAfterBlockIsAnError: blok bittikten sonra saklanan akışa gönderme çağrı hatasıdır (28.7).
func TestSendAfterBlockIsAnError(t *testing.T) {
	ts := serveTR(t, `TR_var saklanan = TR_none
TR_func f(i: TR_Request) -> TR_Response {
    TR_return TR_stream_response((a) => {
        saklanan = a
        a.TR_send_event(1)
    })
}
TR_var s = TR_serve_http(f)`, nil)
	res, _ := streamGet(t, ts, "GET", "/")
	if body, err := io.ReadAll(res.Body); err != nil || string(body) != "data: 1\n\n" {
		t.Fatalf("akış: %q %v", body, err)
	}
	_, err := ts.vm.Run(trrun.Parse(t, "saklanan.TR_send_event(2)"))
	if !messages.Has(err, messages.H7636) {
		t.Fatalf("kapanmış akışa gönderme H7636 olmalı: %v", err)
	}
}

// TestHeadStartsNoStreamBlock: HEAD'de başlıklar gider, blok başlamaz (28.3, 28.7).
func TestHeadStartsNoStreamBlock(t *testing.T) {
	say, n := counter()
	ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_return TR_stream_response((a) => {
        say()
        a.TR_send_event(1)
    })
}
TR_var s = TR_serve_http(f)`, map[string]interp.Value{"say": say})
	res, _ := streamGet(t, ts, "HEAD", "/")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" || len(body) != 0 {
		t.Fatalf("HEAD: %d %v %q", res.StatusCode, res.Header, body)
	}
	if n.Load() != 0 {
		t.Fatal("HEAD'de blok başlamamalı")
	}
}

// TestStreamsAreNotStored: iki akış kipinin cevabı da `Cache-Control: no-store`'dur, GET'te ve HEAD'de (28.7): akış
// belirteçli ve kişiseldir, ara bellek ya da tarayıcı onu saklamaz (no-cache yalnız yeniden doğrulatır).
func TestStreamsAreNotStored(t *testing.T) {
	ts := serveTR(t, `TR_var y = TR_EventBroadcast(TR_capacity: 4, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g)
TR_func f(i: TR_Request) -> TR_Response {
    TR_if i.TR_path == "/yayin" { TR_return y.TR_subscribe(i) }
    TR_return TR_stream_response((akış) => { akış.TR_send_event(1) })
}
TR_var s = TR_serve_http(f)`, nil)
	for _, path := range []string{"/istek", "/yayin"} {
		for _, method := range []string{"GET", "HEAD"} {
			res, err := http.DefaultClient.Do(mustRequest(t, method, "http://"+ts.addr()+path))
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if got := res.Header.Get("Cache-Control"); got != "no-store" {
				t.Fatalf("%s %s: Cache-Control %q, beklenen no-store", method, path, got)
			}
		}
	}
}

func mustRequest(t *testing.T, method, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
