package download

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
