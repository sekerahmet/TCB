package debugger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// dapClient, testlerde düzenleyicinin yerine geçen küçük bir DAP istemcisidir.
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
		machine := v.(map[string]interface{})
		out[machine["name"].(string)] = machine["value"].(string)
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

const dapProgram = `TR_var toplam = 0
TR_var liste = [{"ad": "Ali", "maas": 60}, {"ad": "Ayşe", "maas": 90}]
TR_func ekle_maas(k) {
    TR_return k["maas"] + 10
}
TR_for i TR_in 0..2 {
    toplam += ekle_maas(liste[i])
}
TR_print("toplam: \(toplam)")
`

// Düzenleyicinin tipik bir hata ayıklama oturumu: koşullu kesme noktası, çağrı yığını,
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
	table := c.request("trTable", map[string]interface{}{"expression": "liste"})
	if fmt.Sprint(table["columns"]) != "[ad maas]" || len(table["rows"].([]interface{})) != 2 {
		t.Fatalf("2 satırlı [ad maas] tablosu bekleniyordu: %v", table)
	}

	// İzleme Masası: tek tabloda tüm değişkenler, tablolar işaretli
	panel := c.request("trVariables", nil)
	if int(panel["line"].(float64)) != 9 {
		t.Fatalf("panel 9. satırı göstermeli: %v", panel["line"])
	}
	found := map[string]map[string]interface{}{}
	for _, d := range panel["variables"].([]interface{}) {
		dm := d.(map[string]interface{})
		found[dm["name"].(string)] = dm
	}
	if l := found["liste"]; l == nil || l["type"] != "Tablo [2×2]" || l["table"] != true || l["scope"] != "Genel" || l["kind"] != "table" {
		t.Fatalf("liste Tablo [2×2] olarak işaretlenmeli: %v", found["liste"])
	}
	if tp := found["toplam"]; tp == nil || tp["value"] != "1100" || tp["table"] != false || tp["kind"] != "number" {
		t.Fatalf("toplam = 1100 skaler değişken olmalı: %v", found["toplam"])
	}
	if _, var_ := found["yazdır"]; var_ {
		t.Fatal("yerleşik fonksiyonlar panelde listelenmemeli")
	}

	// Hata ayıklama konsolunda deyim çalıştırma
	c.request("evaluate", map[string]interface{}{"expression": "TR_sum = 5", "context": "repl"})

	c.request("continue", map[string]interface{}{"threadId": 1})
	exited := c.event("exited")
	c.event("terminated")
	if exited["exitCode"].(float64) != 0 {
		t.Fatalf("çıkış kodu 0 olmalı: %v", exited)
	}
	if !strings.Contains(c.output.String(), "toplam: 5") {
		t.Fatalf("program çıktısı hata ayıklama konsoluna gelmeli: %q", c.output.String())
	}
}

// İzleme noktası ve iz noktası (durmadan mesaj yazan kesme noktası).
func TestDAPWatchpointAndLogpoint(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "TR_var a = 1\nTR_var b = 2\na = 5\nb = 3\nTR_print(\"bitti\")\n")
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

// Satıra zıpla (düzenleyicinin "imlece zıpla" komutu) ve hatalı programın bildirilmesi.
func TestDAPGotoAndLaunchErrors(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "TR_var x = 1\nx = 2\nx = 3\nTR_print(x)\n")
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

	// Sözdizimi hatalı program: launch başarısız olur, hata ayıklama konsoluna yazılır
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

// launch'tan önce gelen setBreakpoints istekleri birbirini silmemeli; başka bir dosyanın
// kesme noktası hata ayıklanan dosyada tetiklenmemeli.
func TestDAPBreakpointsPerFile(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "TR_var a = 1\na = 2\na = 3\na = 4\nTR_print(a)\n")
	other := filepath.Join(t.TempDir(), "modul.tr")
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": path}, "breakpoints": []interface{}{map[string]interface{}{"line": 2}}})
	c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": other}, "breakpoints": []interface{}{map[string]interface{}{"line": 4}}})
	c.request("launch", map[string]interface{}{"program": path})
	c.request("configurationDone", nil)
	c.event("stopped")
	if line := c.topFrameLine(); line != 2 {
		t.Fatalf("ana dosyanın 2. satırında durmalıydı, %d", line)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
	if !strings.Contains(c.output.String(), "4") {
		t.Fatalf("program sonuna kadar çalışmalıydı: %q", c.output.String())
	}
	// Launch sonrası başka dosyanın kesme noktaları doğrulanmamış (verified: false) döner
	c2 := startDAP(t)
	c2.launch(path, false)
	reply := c2.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": other}, "breakpoints": []interface{}{map[string]interface{}{"line": 1}}})
	if bp := reply["breakpoints"].([]interface{})[0].(map[string]interface{}); bp["verified"] != false {
		t.Fatalf("başka dosyanın kesme noktası doğrulanmamalı: %v", bp)
	}
}

// goto şu anki bloğun dışına zıplayamaz: hedef önerilmez, istek açık bir hatayla reddedilir
// ("zıplandı" deyip başka satırda sürmez).
func TestDAPGotoOutsideBlockRejected(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "TR_var x = 0\nTR_func f() {\n    x = 1\n    x = 2\n}\nf()\nx = 7\nTR_print(x)\n")
	c.launch(path, false, map[string]interface{}{"line": 3})
	c.event("stopped")
	targets := c.request("gotoTargets", map[string]interface{}{"source": map[string]interface{}{"path": path}, "line": 7})
	if n := len(targets["targets"].([]interface{})); n != 0 {
		t.Fatalf("bloğun dışındaki satır hedef olarak önerilmemeli: %v", targets)
	}
	if n := len(c.request("gotoTargets", map[string]interface{}{"source": map[string]interface{}{"path": path}, "line": 4})["targets"].([]interface{})); n != 1 {
		t.Fatalf("aynı bloktaki satır hedef olmalı")
	}
	seq := c.send("goto", map[string]interface{}{"threadId": 1, "targetId": 7})
	reply := c.next(func(m map[string]interface{}) bool {
		return m["type"] == "response" && int(m["request_seq"].(float64)) == seq
	}, "goto yanıtı")
	if reply["success"] != false || !strings.Contains(fmt.Sprint(reply["message"]), "bloğun dışında") {
		t.Fatalf("blok dışına goto reddedilmeli: %v", reply)
	}
	// Program hâlâ 3. satırda duruyor
	if line := c.topFrameLine(); line != 3 {
		t.Fatalf("reddedilen goto sonrası 3. satırda kalmalı, %d", line)
	}
	c.request("goto", map[string]interface{}{"threadId": 1, "targetId": 4})
	c.event("stopped")
	if line := c.topFrameLine(); line != 4 {
		t.Fatalf("4. satıra zıplamalıydı, %d", line)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
}

// Çok büyük Content-Length bellek tüketmeden reddedilir.
func TestDAPContentLengthLimit(t *testing.T) {
	s := newDAPServer(strings.NewReader("Content-Length: 99999999999\r\n\r\n{}"), io.Discard)
	if _, err := s.readMessage(); !messages.Has(err, messages.H8101) {
		t.Fatalf("büyük ileti reddedilmeli: %v", err)
	}
}

// Eşzamanlı görev hataları hata ayıklama konsoluna "stderr" kategorisiyle gelir.
func TestDAPTaskError(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "TR_async {\n    TR_var x = 1 / 0\n}\nTR_wait_all()\nTR_print(\"bitti\")\n")
	c.launch(path, false)
	stderr := c.next(func(m map[string]interface{}) bool {
		if m["type"] != "event" || m["event"] != "output" {
			return false
		}
		b := m["body"].(map[string]interface{})
		return b["category"] == "stderr" && strings.Contains(b["output"].(string), "Eşzamanlı görev hatası")
	}, "görev hatası çıktısı")
	if stderr == nil {
		t.Fatal("görev hatası gelmedi")
	}
	c.event("terminated")
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
	if _, _, err := TableData(&interp.IntVal{Val: 5}); err == nil {
		t.Fatal("sayı için tablo hatası bekleniyordu")
	}
}

// Kural: oturum başladıktan sonra ikinci 'launch' reddedilir (success: false); çalışan makine
// ve hata ayıklayıcı yerinde kalır. Neden: launch durum alanlarını çalışan VM'in altından
// değiştirirdi (veri yarışı, asılı kalan ilk oturum).
func TestDAPSecondLaunchIsRejectedWhileSessionRuns(t *testing.T) {
	dir := t.TempDir()
	progPath := filepath.Join(dir, "ana.tr")
	if err := os.WriteFile(progPath, []byte("TR_var a = 1\nTR_var b = 2\nTR_print(a + b)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := startDAP(t)
	c.request("initialize", map[string]any{})
	c.request("launch", map[string]any{"program": progPath, "stopOnEntry": true})
	c.event("initialized")
	c.request("configurationDone", map[string]any{})
	c.event("stopped")
	seq := c.send("launch", map[string]any{"program": progPath})
	resp := c.next(func(m map[string]interface{}) bool {
		return m["type"] == "response" && int(m["request_seq"].(float64)) == seq
	}, "ikinci launch yanıtı")
	if resp["success"] == true {
		t.Fatal("çalışan oturum sırasında ikinci launch kabul edildi")
	}
}

// Kural: başlık satırı dapMaxHeaderLine ile sınırlıdır; satır sonu gelmeyen akış hata verir,
// bellek ayırmaya devam etmez. Neden: gövde sınırı (64 MB) başlık satırını kapsamıyordu.
func TestDAPHeaderLineIsBounded(t *testing.T) {
	s := newDAPServer(strings.NewReader(strings.Repeat("x", 4*dapMaxHeaderLine)), io.Discard)
	if _, err := s.readMessage(); !messages.Has(err, messages.H8108) {
		t.Fatalf("sınırsız başlık satırı reddedilmedi: %v", err)
	}
}
