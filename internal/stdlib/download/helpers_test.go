package download

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/stdlib/files"
)

// Testlerdeki VM'ler indirmeyi ve sha256'yı (22.5) içersin.
func init() {
	interp.AddEngine(files.Register)
	interp.AddEngine(Register)
}

// run bir TR programını çalıştırır; hatayı ve çıktıyı verir.
func run(t *testing.T, code string) (string, error) {
	t.Helper()
	p := parser.New(lexer.NewBody(code, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), code)
	}
	var out, warn bytes.Buffer
	m := interp.New()
	m.Out, m.Err = &out, &warn
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), err
}

// mustRun programın hatasız bitmesini bekler ve çıktısını verir.
func mustRun(t *testing.T, code string) string {
	t.Helper()
	out, err := run(t, code)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\n%s", err, code)
	}
	return out
}

// wantCode programın verilen kodu taşıyan bir hatayla durmasını bekler; hatayı verir.
func wantCode(t *testing.T, code string, want messages.Code) error {
	t.Helper()
	_, err := run(t, code)
	if err == nil || !messages.Has(err, want) {
		t.Fatalf("%s bekleniyordu, gelen: %v\n%s", want, err, code)
	}
	return err
}

// sum baytların sha256'sıdır.
func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// slash yolu TR metnine koymak için ileri eğik çizgiyle yazar.
func slash(p string) string { return filepath.ToSlash(p) }

// dl bir indirme çağrısının TR yazımıdır; ek adlı argümanlar extra'dadır.
func dl(url, target, hash, size string, extra ...string) string {
	args := []string{fmt.Sprintf("%q", url), fmt.Sprintf("%q", target), "TR_sha256: " + hash, "TR_size: " + size}
	return "TR_download(" + strings.Join(append(args, extra...), ", ") + ")"
}

// q bir Go metnini TR metni olarak yazar.
func q(s string) string { return fmt.Sprintf("%q", s) }

// server elle yazılan cevaplarla bir httptest sunucusudur; gelen istekleri kaydeder.
type server struct {
	*httptest.Server
	mu       sync.Mutex
	requests []*http.Request
}

func newServer(t *testing.T, h http.HandlerFunc) *server {
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.Clone(context.Background()))
		s.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *server) ranges() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, r := range s.requests {
		out = append(out, r.Header.Get("Range"))
	}
	return out
}

// rawServer her bağlantıya verilen ham HTTP/1.1 cevabını yazar ve bağlantıyı kapatır.
func rawServer(t *testing.T, response string) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				br := bufio.NewReader(c)
				for {
					line, err := br.ReadString('\n')
					if err != nil || line == "\r\n" {
						break
					}
				}
				c.Write([]byte(response))
			}()
		}
	}()
	return "http://" + l.Addr().String() + "/f.bin"
}

// partSize .part'ın boyudur; yoksa -1.
func partSize(target string) int64 {
	fi, err := os.Stat(target + ".part")
	if err != nil {
		return -1
	}
	return fi.Size()
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// shortTimes evre sürelerini, tavanı ve adres başına en kısa denemeyi test için kısaltır.
func shortTimes(t *testing.T, connect, header, idle, first time.Duration) {
	oc, oh, oi, of, om := connectTimeout, headerTimeout, idleTimeout, firstHeaderTimeout, minAttempt
	connectTimeout, headerTimeout, idleTimeout, firstHeaderTimeout, minAttempt = connect, header, idle, first, 20*time.Millisecond
	t.Cleanup(func() {
		connectTimeout, headerTimeout, idleTimeout, firstHeaderTimeout, minAttempt = oc, oh, oi, of, om
	})
}

// tlsServer network'te (tcp ya da tcp6) bir httptest TLS sunucusu açar ve kökünü testRoots'a koyar;
// cs verilirse bağlantıların durumunu izler.
func tlsServer(t *testing.T, network, addr string, h http.HandlerFunc, cs func(net.Conn, http.ConnState)) *httptest.Server {
	t.Helper()
	l, err := net.Listen(network, addr)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewUnstartedServer(h)
	s.Listener.Close()
	s.Listener = l
	s.Config.ConnState = cs
	s.StartTLS()
	t.Cleanup(s.Close)
	testRoots = s.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	t.Cleanup(func() { testRoots = nil })
	return s
}

// silentListener TCP'yi kabul eder, hiç yazmaz (TLS el sıkışmasında ya da CONNECT'te asılan uç).
func silentListener(t *testing.T, network, addr string) string {
	t.Helper()
	l, err := net.Listen(network, addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	var mu sync.Mutex
	var conns []net.Conn
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	})
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	return l.Addr().String()
}

// tunnelProxy CONNECT'e 200 verip hedefe tünel kuran bir vekildir; adresini verir.
func tunnelProxy(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				br := bufio.NewReader(c)
				req, err := http.ReadRequest(br)
				if err != nil || req.Method != http.MethodConnect {
					return
				}
				up, err := net.Dial("tcp", req.Host)
				if err != nil {
					return
				}
				defer up.Close()
				c.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
				go io.Copy(up, br)
				io.Copy(c, up)
			}()
		}
	}()
	return "http://" + l.Addr().String()
}

// timeoutArgs H7716'nın konağını ve evresini verir; hata H7716 değilse ok false.
func timeoutArgs(err error) (host, stage string, ok bool) {
	var e *messages.Error
	if !errors.As(err, &e) || e.Code != messages.H7716 || len(e.Args) != 3 {
		return "", "", false
	}
	host, _ = e.Args[1].(string)
	stage, _ = e.Args[2].(string)
	return host, stage, true
}
