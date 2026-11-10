package httpserver

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Testler yalnız bu motoru kaydeder (stdlib'i içe aktarmak döngü olurdu); çekirdek yerleşikler var.
func init() { interp.AddEngine(Register) }

// testServer gerçek bağlantıyla sınanan bir sunucudur.
type testServer struct {
	s      *HTTPServer
	vm     *interp.VM
	errOut *syncBuffer
	calls  *atomic.Int64 // Go işleyicisinin çağrı sayısı
}

// syncBuffer standart hatanın test kopyasıdır: sunucunun goroutine'leri yazarken test okur (-race).
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// serveGo Go'da yazılmış bir işleyiciyle sunucu açar; args TR_serve_http'in adlı argümanlarıdır
// (", TR_port: 0" gibi). Etkileşimli VM program sonunda sunucuyu kapatmaz; test kapatır.
func serveGo(t *testing.T, args string, handler func(*Request) *Response) *testServer {
	t.Helper()
	calls := &atomic.Int64{}
	h := &interp.BuiltinVal{Name: "işleyici", Fn: func(_ *interp.VM, a []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		calls.Add(1)
		return handler(a[0].(*Request)), nil
	}}
	ts := serveTR(t, "TR_var s = TR_serve_http(h"+args+")", map[string]interp.Value{"h": h})
	ts.calls = calls
	return ts
}

// serveTR TR kodunu etkileşimli bir VM'de çalıştırır; kod s adında bir sunucu açar.
func serveTR(t *testing.T, code string, globals map[string]interp.Value) *testServer {
	t.Helper()
	v := interp.New()
	v.SetInteractive()
	errOut := &syncBuffer{}
	v.Out, v.Err = io.Discard, errOut
	for k, g := range globals {
		v.GlobalEnv.Define(k, g, false)
	}
	if _, err := v.Run(trrun.Parse(t, code)); err != nil {
		t.Fatalf("sunucu açılmadı: %v", err)
	}
	sv, _ := v.GlobalEnv.Get("s")
	s, ok := sv.(*HTTPServer)
	if !ok {
		t.Fatalf("s bir sunucu değil: %v", sv)
	}
	t.Cleanup(s.cut)
	return &testServer{s: s, vm: v, errOut: errOut}
}

// ok200 her isteğe "tamam" der.
func ok200(*Request) *Response {
	h := newHeaders()
	h.add("Content-Type", "text/plain; charset=utf-8")
	return &Response{status: 200, body: []byte("tamam"), headers: h}
}

func (ts *testServer) addr() string { return "127.0.0.1:" + strconv.Itoa(ts.s.port) }

// do isteği gönderir; header değerleri verildiği gibi gider (Host dahil).
func (ts *testServer) do(t *testing.T, method, path string, header map[string]string, body io.Reader) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+ts.addr()+path, body)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

// raw ham bir HTTP isteği yazar ve cevabı okur: Go'nun istemcisinin düzelttiği yollar için.
func (ts *testServer) raw(t *testing.T, request string) (*http.Response, string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", ts.addr(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.WriteString(c, request); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		t.Fatalf("cevap okunamadı (%q): %v", request, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

// get127 sunucunun kendi Host'uyla ham bir GET'tir.
func (ts *testServer) get127(path string) string {
	return "GET " + path + " HTTP/1.1\r\nHost: " + ts.addr() + "\r\nConnection: close\r\n\r\n"
}

// mustContain metinde parça yoksa testi düşürür.
func mustContain(t *testing.T, what, text, part string) {
	t.Helper()
	if !strings.Contains(text, part) {
		t.Fatalf("%s %q içermeli: %q", what, part, text)
	}
}
