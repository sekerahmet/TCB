package vm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// dapClient, testlerde VS Code'un yerine geçen küçük bir DAP istemcisidir.
type dapClient struct {
	t       *testing.T
	w       io.Writer
	seq     int
	msgs    chan map[string]interface{}
	backlog []map[string]interface{}
	output  strings.Builder
}

func startDAP(t *testing.T) *dapClient {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	s := newDAPServer(inR, outW)
	s.exit = func(int) {}
	go func() { _ = s.serve() }()

	c := &dapClient{t: t, w: inW, msgs: make(chan map[string]interface{}, 256)}
	go func() {
		r := bufio.NewReader(outR)
		for {
			length := -1
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimSpace(line)
				if line == "" && length >= 0 {
					break
				}
				if strings.HasPrefix(line, "Content-Length:") {
					length, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")))
				}
			}
			buf := make([]byte, length)
			if _, err := io.ReadFull(r, buf); err != nil {
				return
			}
			var m map[string]interface{}
			_ = json.Unmarshal(buf, &m)
			c.msgs <- m
		}
	}()
	t.Cleanup(func() { inW.Close(); outW.Close() })
	return c
}

func (c *dapClient) send(command string, args interface{}) int {
	c.seq++
	data, _ := json.Marshal(map[string]interface{}{"seq": c.seq, "type": "request", "command": command, "arguments": args})
	fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return c.seq
}

// next, koşulu sağlayan ilk iletiyi bekler; diğerlerini sonrası için saklar.
func (c *dapClient) next(match func(map[string]interface{}) bool, what string) map[string]interface{} {
	c.t.Helper()
	for i, m := range c.backlog {
		if match(m) {
			c.backlog = append(c.backlog[:i], c.backlog[i+1:]...)
			return m
		}
	}
	timeout := time.After(10 * time.Second)
	for {
		select {
		case m := <-c.msgs:
			if m["type"] == "event" && m["event"] == "output" {
				body := m["body"].(map[string]interface{})
				c.output.WriteString(body["output"].(string))
			}
			if match(m) {
				return m
			}
			c.backlog = append(c.backlog, m)
		case <-timeout:
			c.t.Fatalf("%s 10 saniye içinde gelmedi; çıktı: %s", what, c.output.String())
			return nil
		}
	}
}

func (c *dapClient) request(command string, args interface{}) map[string]interface{} {
	c.t.Helper()
	seq := c.send(command, args)
	resp := c.next(func(m map[string]interface{}) bool {
		return m["type"] == "response" && int(m["request_seq"].(float64)) == seq
	}, command+" yanıtı")
	if resp["success"] != true {
		c.t.Fatalf("%s başarısız: %v", command, resp["message"])
	}
	body, _ := resp["body"].(map[string]interface{})
	return body
}

func (c *dapClient) event(name string) map[string]interface{} {
	c.t.Helper()
	m := c.next(func(m map[string]interface{}) bool { return m["type"] == "event" && m["event"] == name }, name+" olayı")
	body, _ := m["body"].(map[string]interface{})
	return body
}

func (c *dapClient) topFrameLine() int {
	c.t.Helper()
	st := c.request("stackTrace", map[string]interface{}{"threadId": 1})
	frames := st["stackFrames"].([]interface{})
	return int(frames[0].(map[string]interface{})["line"].(float64))
}

func (c *dapClient) evaluate(expr string) string {
	c.t.Helper()
	return c.request("evaluate", map[string]interface{}{"expression": expr, "context": "watch"})["result"].(string)
}

func (c *dapClient) scopeRef(name string) int {
	c.t.Helper()
	for _, s := range c.request("scopes", map[string]interface{}{"frameId": 1})["scopes"].([]interface{}) {
		sc := s.(map[string]interface{})
		if sc["name"] == name {
			return int(sc["variablesReference"].(float64))
		}
	}
	c.t.Fatalf("%q kapsamı yok", name)
	return 0
}

func (c *dapClient) varsOf(ref int) map[string]string {
	c.t.Helper()
	out := map[string]string{}
	for _, v := range c.request("variables", map[string]interface{}{"variablesReference": ref})["variables"].([]interface{}) {
		vm := v.(map[string]interface{})
		out[vm["name"].(string)] = vm["value"].(string)
	}
	return out
}

func writeProgram(t *testing.T, code string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "program.tr")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func (c *dapClient) launch(path string, stopOnEntry bool, breakpoints ...map[string]interface{}) {
	c.t.Helper()
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("launch", map[string]interface{}{"program": path, "stopOnEntry": stopOnEntry})
	if breakpoints == nil {
		breakpoints = []map[string]interface{}{}
	}
	c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": path}, "breakpoints": breakpoints})
	c.request("configurationDone", nil)
}

const dapProgram = `değişken toplam = 0
değişken liste = [{"ad": "Ali", "maas": 60}, {"ad": "Ayşe", "maas": 90}]
fonksiyon ekle_maas(k) {
    sonuç k["maas"] + 10
}
döngü i içinde 0..2 {
    toplam += ekle_maas(liste[i])
}
yazdır("toplam: {toplam}")
`

// VS Code'un tipik bir hata ayıklama oturumu: koşullu kesme noktası, çağrı yığını,
// değişkenler, değer değiştirme, adımlama, tablo görünümü ve programın bitişi.
func TestDAPDebugSession(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, dapProgram)
	c.launch(path, false, map[string]interface{}{"line": 7, "condition": "i == 1"})

	stopped := c.event("stopped")
	if stopped["reason"] != "breakpoint" {
		t.Fatalf("kesme noktası nedeni bekleniyordu: %v", stopped)
	}
	if line := c.topFrameLine(); line != 7 {
		t.Fatalf("7. satırda durması bekleniyordu, %d", line)
	}
	if got := c.varsOf(c.scopeRef("Yerel"))["i"]; got != "1" {
		t.Fatalf("koşullu kesme noktası i == 1 iken durmalıydı, i = %s", got)
	}
	if got := c.evaluate("toplam"); got != "70" {
		t.Fatalf("toplam 70 olmalıydı, %s", got)
	}

	// ✏️ değeri çalışırken değiştir
	set := c.request("setVariable", map[string]interface{}{"variablesReference": c.scopeRef("Genel"), "name": "toplam", "value": "1000"})
	if set["value"] != "1000" {
		t.Fatalf("setVariable 1000 döndürmeliydi: %v", set)
	}

	// F11: fonksiyonun içine gir, yığında iki çerçeve olmalı
	c.request("stepIn", map[string]interface{}{"threadId": 1})
	c.event("stopped")
	st := c.request("stackTrace", map[string]interface{}{"threadId": 1})
	frames := st["stackFrames"].([]interface{})
	top := frames[0].(map[string]interface{})
	if top["name"] != "ekle_maas()" || int(top["line"].(float64)) != 4 || len(frames) != 2 {
		t.Fatalf("ekle_maas() içinde 4. satır ve 2 çerçeve bekleniyordu: %v", frames)
	}
	if caller := frames[1].(map[string]interface{}); int(caller["line"].(float64)) != 7 {
		t.Fatalf("çağıran çerçeve 7. satırı göstermeli: %v", caller)
	}

	// Shift+F11: fonksiyondan çık → döngü bitti, 9. satır
	c.request("stepOut", map[string]interface{}{"threadId": 1})
	c.event("stopped")
	if line := c.topFrameLine(); line != 9 {
		t.Fatalf("fonksiyondan çıkınca 9. satıra gelmeli, %d", line)
	}
	if got := c.evaluate("toplam"); got != "1100" {
		t.Fatalf("değiştirilen değerle toplam 1100 olmalı, %s", got)
	}

	// Liste değişkeni açılabilir olmalı ve tablo görünümü üretilmeli
	ev := c.request("evaluate", map[string]interface{}{"expression": "liste", "context": "watch"})
	children := c.varsOf(int(ev["variablesReference"].(float64)))
	if !strings.Contains(children["[1]"], "Ayşe") {
		t.Fatalf("liste[1] Ayşe'yi içermeli: %v", children)
	}
	tablo := c.request("trTablo", map[string]interface{}{"ifade": "liste"})
	if fmt.Sprint(tablo["sutunlar"]) != "[ad maas]" || len(tablo["satirlar"].([]interface{})) != 2 {
		t.Fatalf("2 satırlı [ad maas] tablosu bekleniyordu: %v", tablo)
	}

	// İzleme Masası: tek tabloda tüm değişkenler, tablolar işaretli
	panel := c.request("trDegiskenler", nil)
	if int(panel["satir"].(float64)) != 9 {
		t.Fatalf("panel 9. satırı göstermeli: %v", panel["satir"])
	}
	bulundu := map[string]map[string]interface{}{}
	for _, d := range panel["degiskenler"].([]interface{}) {
		dm := d.(map[string]interface{})
		bulundu[dm["ad"].(string)] = dm
	}
	if l := bulundu["liste"]; l == nil || l["tip"] != "Tablo [2×2]" || l["tablo"] != true || l["kapsam"] != "Genel" || l["tur"] != "tablo" {
		t.Fatalf("liste Tablo [2×2] olarak işaretlenmeli: %v", bulundu["liste"])
	}
	if tp := bulundu["toplam"]; tp == nil || tp["deger"] != "1100" || tp["tablo"] != false || tp["tur"] != "sayi" {
		t.Fatalf("toplam = 1100 skaler değişken olmalı: %v", bulundu["toplam"])
	}
	if _, var_ := bulundu["yazdır"]; var_ {
		t.Fatal("yerleşik fonksiyonlar panelde listelenmemeli")
	}

	// Debug Console'da deyim çalıştırma
	c.request("evaluate", map[string]interface{}{"expression": "toplam = 5", "context": "repl"})

	c.request("continue", map[string]interface{}{"threadId": 1})
	exited := c.event("exited")
	c.event("terminated")
	if exited["exitCode"].(float64) != 0 {
		t.Fatalf("çıkış kodu 0 olmalı: %v", exited)
	}
	if !strings.Contains(c.output.String(), "toplam: 5") {
		t.Fatalf("program çıktısı Debug Console'a gelmeli: %q", c.output.String())
	}
}

// İzleme noktası ve iz noktası (durmadan mesaj yazan kesme noktası).
func TestDAPWatchpointAndLogpoint(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "değişken a = 1\ndeğişken b = 2\na = 5\nb = 3\nyazdır(\"bitti\")\n")
	c.launch(path, true, map[string]interface{}{"line": 4, "logMessage": "b şu an {b}, a = {a}"})

	if r := c.event("stopped")["reason"]; r != "entry" {
		t.Fatalf("başlangıçta durmalıydı: %v", r)
	}
	c.request("next", map[string]interface{}{"threadId": 1})
	c.event("stopped")
	info := c.request("dataBreakpointInfo", map[string]interface{}{"variablesReference": c.scopeRef("Genel"), "name": "a"})
	if info["dataId"] != "a" {
		t.Fatalf("a için izleme noktası kimliği bekleniyordu: %v", info)
	}
	c.request("setDataBreakpoints", map[string]interface{}{"breakpoints": []interface{}{map[string]interface{}{"dataId": "a"}}})

	c.request("continue", map[string]interface{}{"threadId": 1})
	stopped := c.event("stopped")
	if stopped["reason"] != "data breakpoint" {
		t.Fatalf("izleme noktasında durmalıydı: %v", stopped)
	}
	if line := c.topFrameLine(); line != 4 {
		t.Fatalf("a 3. satırda değişti, 4. satırdan önce durmalı; %d", line)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
	if !strings.Contains(c.output.String(), "b şu an 2, a = 5") {
		t.Fatalf("iz noktası mesajı yazılmalıydı: %q", c.output.String())
	}
}

// Satıra zıpla (VS Code "Jump to Cursor") ve hatalı programın bildirilmesi.
func TestDAPGotoAndLaunchErrors(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "değişken x = 1\nx = 2\nx = 3\nyazdır(x)\n")
	c.launch(path, false, map[string]interface{}{"line": 3})
	c.event("stopped")
	if got := c.evaluate("x"); got != "2" {
		t.Fatalf("3. satırdan önce x = 2 olmalı, %s", got)
	}
	targets := c.request("gotoTargets", map[string]interface{}{"source": map[string]interface{}{"path": path}, "line": 2})
	id := targets["targets"].([]interface{})[0].(map[string]interface{})["id"]
	c.request("goto", map[string]interface{}{"threadId": 1, "targetId": id})
	stopped := c.event("stopped")
	if stopped["reason"] != "goto" || c.topFrameLine() != 2 {
		t.Fatalf("2. satıra zıplamalıydı: %v", stopped)
	}
	if !strings.Contains(stopped["description"].(string), "2'ye") {
		t.Fatalf("açıklama doğru ekle yazılmalı: %v", stopped["description"])
	}
	c.request("disconnect", nil)

	// Sözdizimi hatalı program: launch başarısız olur, hata Debug Console'a yazılır
	c2 := startDAP(t)
	bad := writeProgram(t, "değişken a = \nyazdır(a\n")
	c2.request("initialize", map[string]interface{}{"adapterID": "tr"})
	seq := c2.send("launch", map[string]interface{}{"program": bad})
	resp := c2.next(func(m map[string]interface{}) bool {
		return m["type"] == "response" && int(m["request_seq"].(float64)) == seq
	}, "launch yanıtı")
	if resp["success"] != false || !strings.Contains(c2.output.String(), "Program başlatılamadı") {
		t.Fatalf("hatalı program başlatılmamalı ve hata yazılmalı: %v / %q", resp, c2.output.String())
	}
}

func TestTableData(t *testing.T) {
	_, val, err := runTRWithin(t, 5*time.Second, `sonuç [{"ad": "Ali", "yaş": 30}, {"ad": "Ayşe", "şehir": "İzmir"}]`)
	if err != nil {
		t.Fatal(err)
	}
	cols, rows, err := TableData(val)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(cols) != "[ad yaş şehir]" || fmt.Sprint(rows) != "[[Ali 30 ] [Ayşe  İzmir]]" {
		t.Fatalf("beklenmeyen tablo: %v %v", cols, rows)
	}
	if _, _, err := TableData(&IntVal{Val: 5}); err == nil {
		t.Fatal("sayı için tablo hatası bekleniyordu")
	}
}
