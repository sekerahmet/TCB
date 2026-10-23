package debugger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

// FuzzDAPMessage: 'tr dap' standart girişinden gelen baytlar (çerçeve başlıkları ve JSON
// gövdeler) çözücüyü ve program başlamadan önceki istekleri çökertmemelidir. Programı
// başlatan, süreci bitiren ve diskteki dosyayı okuyan istekler (launch, configurationDone,
// disconnect, terminate) atlanır. Derin arama:
//
//	go test ./internal/devtools/debugger -run '^$' -fuzz FuzzDAPMessage -fuzztime 5m
func FuzzDAPMessage(f *testing.F) {
	message := func(body string) string { return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body) }
	f.Add([]byte(message(`{"seq":1,"type":"request","command":"initialize","arguments":{"adapterID":"tr"}}`)))
	f.Add([]byte(message(`{"seq":2,"type":"request","command":"setBreakpoints","arguments":{"source":{"path":"a.tr"},"breakpoints":[{"line":3,"condition":"i == 1"}]}}`) +
		message(`{"seq":3,"type":"request","command":"setDataBreakpoints","arguments":{"breakpoints":[{"dataId":"x"}]}}`)))
	f.Add([]byte(message(`{"seq":4,"type":"request","command":"evaluate","arguments":{"expression":"1+1"}}`) + message(`{"seq":5,"type":"request","command":"pause"}`)))
	f.Add([]byte("content-length:  2 \n\n{}Content-Length: -1\r\n\r\n"))

	f.Fuzz(func(t *testing.T, input []byte) {
		s := newDAPServer(bytes.NewReader(input), io.Discard)
		s.exit = func(int) { t.Fatal("süreç çıkışı istenmemeliydi") }
		for i := 0; i < 64; i++ {
			msg, err := s.readMessage()
			if err != nil {
				return
			}
			switch msg.Command {
			case "launch", "configurationDone", "disconnect", "terminate":
				continue
			}
			if msg.Type == "request" {
				s.dispatch(msg)
			}
		}
	})
}

// dapPauseCommands program duraklatılmışken VM goroutine'inde işlenen isteklerdir.
var dapPauseCommands = []string{"stackTrace", "scopes", "variables", "setVariable", "evaluate",
	"gotoTargets", "goto", "dataBreakpointInfo", "setDataBreakpoints", "trTablo", "trDegiskenler",
	"next", "stepIn", "stepOut", "pause", "threads", "setBreakpoints"}

// dapResumeCommands başarılı olunca programı sürdüren isteklerdir (yanıttan sonra bir duraklama gelir).
var dapResumeCommands = map[string]bool{"next": true, "stepIn": true, "stepOut": true, "goto": true}

// FuzzDAPPauseRequest: düzenleyicinin program duraklatılmışken gönderdiği istekler
// (değişken tutamakları, çerçeve kimlikleri, satır hedefleri) bozuk ya da kötü niyetli
// argümanlarla hata ayıklayıcıyı çökertmemeli ve asmamalıdır: her isteğe bir yanıt gelmeli,
// program sonra sonuna kadar çalışabilmelidir. Yan etkisiz kalsın diye fonksiyon çağrısı
// içerebilecek argümanlar ('(' ya da \u kaçışı) atlanır. Derin arama:
//
//	go test ./internal/devtools/debugger -run '^$' -fuzz FuzzDAPPauseRequest -fuzztime 5m
func FuzzDAPPauseRequest(f *testing.F) {
	f.Add(uint8(0), []byte(`{"threadId":1,"startFrame":-5,"levels":1000000}`))
	f.Add(uint8(2), []byte(`{"variablesReference":1,"start":-1,"count":-7}`))
	f.Add(uint8(3), []byte(`{"variablesReference":999,"name":"toplam","value":"1000"}`))
	f.Add(uint8(4), []byte(`{"expression":"liste[1][\"ad\"]","context":"watch","frameId":-3}`))
	f.Add(uint8(5), []byte(`{"source":{"path":"x"},"line":-1}`))
	f.Add(uint8(6), []byte(`{"threadId":1,"targetId":0}`))
	f.Add(uint8(7), []byte(`{"variablesReference":2,"name":"k"}`))
	f.Add(uint8(9), []byte(`{"ifade":"liste"}`))

	f.Fuzz(func(t *testing.T, choice uint8, arg []byte) {
		if bytes.ContainsAny(arg, "(") || bytes.Contains(arg, []byte(`\u`)) || len(arg) > 4096 {
			return
		}
		if !json.Valid(arg) {
			arg = []byte("null")
		}
		command := dapPauseCommands[int(choice)%len(dapPauseCommands)]
		c := startDAP(t)
		fpath := writeProgram(t, dapProgram)
		c.launch(fpath, false, map[string]interface{}{"line": 4})
		c.event("stopped")

		c.seq++
		seq := c.seq
		body := fmt.Sprintf(`{"seq":%d,"type":"request","command":%q,"arguments":%s}`, seq, command, arg)
		fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n%s", len(body), body)
		resp := c.next(func(m map[string]interface{}) bool {
			r, _ := m["request_seq"].(float64)
			return m["type"] == "response" && int(r) == seq
		}, command+" yanıtı")
		// Başarılı bir adımlama ya da goto programı çalışır bırakır; aşağıdaki temizlik istekleri
		// duraklama ister. Duraklama (ya da bitiş) beklenmezse sonuç makinenin hızına bağlı olurdu.
		if dapResumeCommands[command] && resp["success"] == true {
			m := c.next(func(m map[string]interface{}) bool {
				return m["type"] == "event" && (m["event"] == "terminated" || m["event"] == "stopped")
			}, command+" sonrası duraklama")
			if m["event"] == "terminated" {
				return
			}
		}

		// Program sonuna kadar sürdürülür (istek yeni duraklamalara yol açmış olabilir).
		c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": fpath}, "breakpoints": []interface{}{}})
		c.request("setDataBreakpoints", map[string]interface{}{"breakpoints": []interface{}{}})
		for i := 0; i < 100; i++ {
			c.send("continue", map[string]interface{}{"threadId": 1})
			m := c.next(func(m map[string]interface{}) bool {
				return m["type"] == "event" && (m["event"] == "terminated" || m["event"] == "stopped")
			}, "programın sürmesi")
			if m["event"] == "terminated" {
				return
			}
		}
		t.Fatal("program 100 sürdürmede bitmedi")
	})
}
