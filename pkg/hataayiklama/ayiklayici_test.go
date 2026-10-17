package hataayiklama

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"tr_dili/pkg/vm"
)

func TestDebuggerBreakpointToggleIsSafe(t *testing.T) {
	d := NewDebugger("t.tr", "a\nb", false)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 2000; i++ {
			d.toggleBreakpoint(i % 7)
		}
		close(done)
	}()
	for i := 0; i < 2000; i++ {
		_ = d.hasBreakpoint(i % 7)
	}
	<-done
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// ---------------------------------------------------------------------------
// Görsel debugger: yalnızca 127.0.0.1, erişim anahtarı ve Host denetimi
// ---------------------------------------------------------------------------

func TestWebDebuggerRequiresTokenAndLocalhost(t *testing.T) {
	machine := vm.New()
	machine.Out = io.Discard
	machine.GlobalEnv.Define("bakiye", &vm.IntVal{Val: 10}, false)

	d := NewDebugger("test.tr", "değişken bakiye = 10", true)
	port := freePort(t)
	if err := d.StartWebServer(machine, port); err != nil {
		t.Fatal(err)
	}
	defer d.StopWebServer()
	// VM'nin bir satırda duraklatılıp web komutu beklemesini taklit et
	resumed := make(chan struct{})
	go func() {
		d.waitForWebAction(machine, 1, machine.GlobalEnv)
		close(resumed)
	}()
	waitUntil(t, func() bool { d.mu.Lock(); defer d.mu.Unlock(); return d.paused })

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	marker := filepath.Join(t.TempDir(), "dbg_pwned.txt")
	evilBody := fmt.Sprintf(`{"name":"bakiye","value":"dosya_yaz(%q, \"x\")"}`, strings.ReplaceAll(marker, `\`, `\\`))

	do := func(method, path, body, token, host string) int {
		req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set(debugTokenHeader, token)
		}
		if host != "" {
			req.Host = host
		}
		req.Header.Set("Content-Type", "text/plain") // tarayıcının ön kontrolsüz gönderebileceği tür
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	if c := do("GET", "/", "", "", ""); c != http.StatusUnauthorized {
		t.Errorf("anahtarsız ana sayfa 401 vermeliydi, %d", c)
	}
	if c := do("GET", "/?anahtar="+d.webToken, "", "", ""); c != http.StatusOK {
		t.Errorf("anahtarlı ana sayfa 200 vermeliydi, %d", c)
	}
	if c := do("GET", "/api/state", "", "", ""); c != http.StatusUnauthorized {
		t.Errorf("anahtarsız /api/state 401 vermeliydi, %d", c)
	}
	if c := do("POST", "/api/modify", evilBody, "", ""); c != http.StatusUnauthorized {
		t.Errorf("anahtarsız /api/modify 401 vermeliydi, %d", c)
	}
	if c := do("POST", "/api/modify", evilBody, "yanlis-anahtar", ""); c != http.StatusUnauthorized {
		t.Errorf("hatalı anahtarlı /api/modify 401 vermeliydi, %d", c)
	}
	if c := do("GET", "/api/state", "", d.webToken, "kotu-site.example:"+fmt.Sprint(port)); c != http.StatusForbidden {
		t.Errorf("yabancı Host (DNS rebinding) 403 vermeliydi, %d", c)
	}
	if c := do("GET", "/api/modify", "", d.webToken, ""); c != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/modify 405 vermeliydi, %d", c)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("yetkisiz istek kod çalıştırdı")
	}
	if v, _ := machine.GlobalEnv.Get("bakiye"); v.Inspect() != "10" {
		t.Fatalf("yetkisiz istek değişkeni değiştirdi: %s", v.Inspect())
	}

	// Yetkili istek hâlâ çalışmalı (canlı değişken değiştirme)
	if c := do("POST", "/api/modify", `{"name":"bakiye","value":"42"}`, d.webToken, ""); c != http.StatusOK {
		t.Fatalf("yetkili /api/modify 200 vermeliydi, %d", c)
	}
	if v, _ := machine.GlobalEnv.Get("bakiye"); v.Inspect() != "42" {
		t.Fatalf("yetkili değişiklik uygulanmadı: %s", v.Inspect())
	}

	// F8 ile devam ettikten sonra (program çalışırken) değişiklik ve adım komutları reddedilmeli
	if c := do("POST", "/api/step", `{"action":"f8"}`, d.webToken, ""); c != http.StatusOK {
		t.Fatalf("f8 200 vermeliydi, %d", c)
	}
	<-resumed
	if c := do("POST", "/api/modify", `{"name":"bakiye","value":"7"}`, d.webToken, ""); c != http.StatusConflict {
		t.Errorf("çalışırken /api/modify 409 vermeliydi, %d", c)
	}
	if c := do("POST", "/api/step", `{"action":"f5"}`, d.webToken, ""); c != http.StatusConflict {
		t.Errorf("çalışırken /api/step 409 vermeliydi (hayalet adım), %d", c)
	}
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("koşul 2 saniye içinde sağlanmadı")
}

func TestDebuggerJumpMessageUsesCorrectSuffix(t *testing.T) {
	d := NewDebugger("t.tr", strings.Repeat("x\n", 50), false)
	d.webWaitCh = make(chan webDebugAction, 1)
	for line, want := range map[int]string{6: "Satır 6'ya zıplandı", 9: "Satır 9'a zıplandı", 3: "Satır 3'e zıplandı", 40: "Satır 40'a zıplandı"} {
		d.webWaitCh <- webDebugAction{Action: "jump", Line: line}
		d.waitForWebAction(vm.New(), 1, vm.NewEnvironment(nil))
		if !strings.Contains(d.PauseReason, want) {
			t.Errorf("%q bekleniyordu, %q alındı", want, d.PauseReason)
		}
	}
}
