package assistant_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Ekranın gerçek sunucusu: sahte zincirli screen_demo.tr trlang ile açılır, test gerçek bağlantıyla konuşur
// (contract.md). Belirteçsiz yollar, 401 ve 403, akıştan bir tur, Last-Event-ID ile yeniden bağlanma ve aboneliğin
// bir gönderime denk gelmesi (katlama yavaşlatılır: parça ekranda tam bir kez).

const fakeScreen = "tools/assistant/screen_demo.tr"

var (
	trlangOnce sync.Once
	trlangPath string
	trlangErr  error
)

// trlang komutu bir kez derlenir (cmd/trlang/e2e'nin TestMain'i gibi).
func trlangBinary(t *testing.T) string {
	t.Helper()
	trlangOnce.Do(func() {
		dir, err := os.MkdirTemp("", "trlang-ekran")
		if err != nil {
			trlangErr = err
			return
		}
		trlangPath = filepath.Join(dir, "trlang")
		if os.PathSeparator == '\\' {
			trlangPath += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", trlangPath, "./cmd/trlang")
		cmd.Dir = reporoot.Path(".")
		if out, err := cmd.CombinedOutput(); err != nil {
			trlangErr = err
			trlangPath = string(out)
		}
	})
	if trlangErr != nil {
		t.Fatalf("trlang derlenemedi: %v\n%s", trlangErr, trlangPath)
	}
	return trlangPath
}

type screenServer struct {
	base, token string
}

var screenLine = regexp.MustCompile(`EKRAN (http://127\.0\.0\.1:[0-9]+)/ `)

// startScreen screen_demo.tr'yi açar; test bitince süreç kapanır.
func startScreen(t *testing.T, args ...string) screenServer {
	t.Helper()
	tokenFile := filepath.Join(t.TempDir(), "belirtec")
	cmd := exec.Command(trlangBinary(t), append([]string{fakeScreen, "--belirteç_dosyası", tokenFile}, args...)...)
	cmd.Dir = reporoot.Path(".")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	lines := bufio.NewScanner(out)
	var base string
	deadline := time.Now().Add(60 * time.Second)
	for base == "" || !strings.Contains(lines.Text(), "HAZIR") {
		if time.Now().After(deadline) || !lines.Scan() {
			t.Fatalf("sahte ekran açılmadı: %s", stderr.String())
		}
		if m := screenLine.FindStringSubmatch(lines.Text()); m != nil {
			base = m[1]
		}
	}
	go func() { _, _ = io.Copy(io.Discard, out) }()
	token, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), string(token)) {
		t.Fatal("belirteç standart hataya yazıldı")
	}
	return screenServer{base: base, token: string(token)}
}

func (s screenServer) request(t *testing.T, method, path string, header map[string]string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, s.base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (s screenServer) auth() map[string]string {
	return map[string]string{"Authorization": "Bearer " + s.token}
}

func (s screenServer) command(t *testing.T, id, typ, data string) string {
	t.Helper()
	h := s.auth()
	h["Content-Type"] = "application/json"
	res := s.request(t, "POST", "/command", h, `{"v":1,"command_id":"`+id+`","type":"`+typ+`","data":`+data+`}`)
	defer res.Body.Close()
	var reply struct {
		Status string `json:"status"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		t.Fatalf("komutun cevabı: %v", err)
	}
	if reply.Status == "accepted" {
		return "accepted"
	}
	return reply.Error.Code
}

type sseEvent struct {
	ID   string
	Type string
	Data map[string]any
}

// stream /events'i açar; olaylar kanala gelir (nabız atlanır).
func (s screenServer) stream(t *testing.T, lastID string) <-chan sseEvent {
	t.Helper()
	h := s.auth()
	if lastID != "" {
		h["Last-Event-ID"] = lastID
	}
	res := s.request(t, "GET", "/events", h, "")
	if res.StatusCode != 200 {
		t.Fatalf("/events: %d", res.StatusCode)
	}
	t.Cleanup(func() { res.Body.Close() })
	events := make(chan sseEvent, 1024)
	go func() {
		defer close(events)
		r := bufio.NewReader(res.Body)
		var id, data string
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case line == "":
				var e struct {
					Type string         `json:"type"`
					Data map[string]any `json:"data"`
				}
				if data != "" && json.Unmarshal([]byte(data), &e) == nil && e.Type != "heartbeat" {
					events <- sseEvent{ID: id, Type: e.Type, Data: e.Data}
				}
				id, data = "", ""
			case strings.HasPrefix(line, "id: "):
				id = line[4:]
			case strings.HasPrefix(line, "data: "):
				data = line[6:]
			}
		}
	}()
	return events
}

func next(t *testing.T, events <-chan sseEvent) sseEvent {
	t.Helper()
	select {
	case e, ok := <-events:
		if !ok {
			t.Fatal("akış kapandı")
		}
		return e
	case <-time.After(30 * time.Second):
		t.Fatal("akıştan olay gelmedi")
	}
	return sseEvent{}
}

// screenText ekranın cevap metnini kurar: session görüntüsünün turn_text'i, sonra append ve replace'ler.
type screenText struct{ text map[string]string }

func (v *screenText) apply(e sseEvent) {
	switch e.Type {
	case "session":
		v.text = map[string]string{}
		for _, raw := range e.Data["snapshot"].([]any) {
			x := raw.(map[string]any)
			v.apply(sseEvent{Type: x["type"].(string), Data: x["data"].(map[string]any)})
		}
	case "turn_text":
		if e.Data["role"] != "assistant" {
			return
		}
		turn := e.Data["turn_id"].(string)
		if e.Data["mode"] == "append" {
			v.text[turn] += e.Data["text"].(string)
		} else {
			v.text[turn] = e.Data["text"].(string)
		}
	}
}

var eventID = regexp.MustCompile(`^[0-9a-f]{16}-[0-9]+$`)

func TestScreenServerEndToEnd(t *testing.T) {
	s := startScreen(t)
	// Sayfa ve dil dosyası belirteçsizdir; akış ve komut belirteç ister; yabancı Origin 403.
	page := s.request(t, "GET", "/", nil, "")
	page.Body.Close()
	if page.StatusCode != 200 || page.Header.Get("Content-Security-Policy") != "frame-ancestors 'none'; connect-src 'self'" || page.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("GET /: %d %v", page.StatusCode, page.Header)
	}
	if res := s.request(t, "GET", "/lang/tr.js", nil, ""); res.StatusCode != 200 {
		t.Fatalf("GET /lang/tr.js: %d", res.StatusCode)
	}
	if res := s.request(t, "GET", "/events", nil, ""); res.StatusCode != 401 {
		t.Fatalf("belirteçsiz /events: %d", res.StatusCode)
	}
	if res := s.request(t, "POST", "/command", map[string]string{"Content-Type": "application/json"}, "{}"); res.StatusCode != 401 {
		t.Fatalf("belirteçsiz /command: %d", res.StatusCode)
	}
	h := s.auth()
	h["Origin"] = "http://saldirgan.example"
	if res := s.request(t, "GET", "/events", h, ""); res.StatusCode != 403 {
		t.Fatalf("yanlış Origin: %d", res.StatusCode)
	}
	if res := s.request(t, "GET", "/events", map[string]string{"Authorization": "Bearer yanlis-belirtec-0123456789"}, ""); res.StatusCode != 401 {
		t.Fatalf("yanlış belirteç: %d", res.StatusCode)
	}

	// Akıştan bir tur: session, turn_start, söz, cevap, turn_end, metrics.
	events := s.stream(t, "")
	first := next(t, events)
	if first.Type != "session" || !eventID.MatchString(first.ID) {
		t.Fatalf("ilk olay session ve kimliği <ön ek>-<sıra> olmalı: %+v", first)
	}
	view := &screenText{}
	view.apply(first)
	if got := s.command(t, "k-1", "send_text", `{"text":"merhaba dünya"}`); got != "accepted" {
		t.Fatalf("send_text: %s", got)
	}
	var types []string
	last := first.ID
	for {
		e := next(t, events)
		view.apply(e)
		types = append(types, e.Type)
		last = e.ID
		if e.Type == "turn_end" {
			if e.Data["reason"] != "done" {
				t.Fatalf("turn_end: %v", e.Data)
			}
			break
		}
	}
	if got := view.text["t-1"]; got != "Sahte cevap: merhaba dünya." {
		t.Fatalf("ekrandaki cevap %q", got)
	}
	if types[0] != "turn_start" {
		t.Fatalf("olaylar: %v", types)
	}
	if got := s.command(t, "k-1", "send_text", `{"text":"merhaba dünya"}`); got != "accepted" {
		t.Fatalf("aynı command_id ilk cevabı almalı: %s", got)
	}

	// Last-Event-ID ile yeniden bağlanan ekran session almaz, yalnız sonrasını alır.
	again := s.stream(t, last)
	if got := s.command(t, "k-2", "send_text", `{"text":"ikinci"}`); got != "accepted" {
		t.Fatalf("ikinci send_text: %s", got)
	}
	for {
		e := next(t, again)
		if e.Type == "session" {
			t.Fatal("tutulan kimlikle bağlanan ekran session almamalı")
		}
		view.apply(e)
		if e.Type == "turn_end" {
			break
		}
	}
	if got := view.text["t-2"]; got != "Sahte cevap: ikinci." {
		t.Fatalf("yeniden bağlanan ekrandaki cevap %q", got)
	}
}

// TestScreenSubscriptionMeetsASend: katlama yavaşken (her gönderim 0,1 sn bekler) tur sürerken açılan abonelik
// görüntüyü ve sonrasını alır; cevabın her parçası ekranda tam bir kez (28.7, DENETİM 377).
func TestScreenSubscriptionMeetsASend(t *testing.T) {
	s := startScreen(t, "--katlama_beklemesi", "0.1", "--parça_beklemesi", "0.01")
	if got := s.command(t, "k-1", "send_text", `{"text":"bir iki üç dört beş altı yedi sekiz"}`); got != "accepted" {
		t.Fatalf("send_text: %s", got)
	}
	time.Sleep(600 * time.Millisecond) // tur sürüyor: birkaç parça katlandı
	events := s.stream(t, "")
	view := &screenText{}
	first := next(t, events)
	if first.Type != "session" {
		t.Fatalf("ilk olay: %+v", first)
	}
	view.apply(first)
	if view.text["t-1"] == "" {
		t.Fatal("abonelik turun ortasına denk gelmedi: görüntüde cevap yok (bekleme kısaldı mı?)")
	}
	for {
		e := next(t, events)
		view.apply(e)
		if e.Type == "turn_end" {
			break
		}
	}
	if got := view.text["t-1"]; got != "Sahte cevap: bir iki üç dört beş altı yedi sekiz." {
		t.Fatalf("parça kayboldu ya da iki kez geldi: %q", got)
	}
}
