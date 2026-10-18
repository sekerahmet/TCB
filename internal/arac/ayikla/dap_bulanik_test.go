package ayikla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

// FuzzDAPIletisi: 'tr dap' standart girişinden gelen baytlar (çerçeve başlıkları ve JSON
// gövdeler) çözücüyü ve program başlamadan önceki istekleri çökertmemelidir. Programı
// başlatan, süreci bitiren ve diskteki dosyayı okuyan istekler (launch, configurationDone,
// disconnect, terminate) atlanır. Derin arama:
//
//	go test ./internal/arac/ayikla -run '^$' -fuzz FuzzDAPIletisi -fuzztime 5m
func FuzzDAPIletisi(f *testing.F) {
	ileti := func(govde string) string { return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(govde), govde) }
	f.Add([]byte(ileti(`{"seq":1,"type":"request","command":"initialize","arguments":{"adapterID":"tr"}}`)))
	f.Add([]byte(ileti(`{"seq":2,"type":"request","command":"setBreakpoints","arguments":{"source":{"path":"a.tr"},"breakpoints":[{"line":3,"condition":"i == 1"}]}}`) +
		ileti(`{"seq":3,"type":"request","command":"setDataBreakpoints","arguments":{"breakpoints":[{"dataId":"x"}]}}`)))
	f.Add([]byte(ileti(`{"seq":4,"type":"request","command":"evaluate","arguments":{"expression":"1+1"}}`) + ileti(`{"seq":5,"type":"request","command":"pause"}`)))
	f.Add([]byte("content-length:  2 \n\n{}Content-Length: -1\r\n\r\n"))

	f.Fuzz(func(t *testing.T, girdi []byte) {
		s := newDAPServer(bytes.NewReader(girdi), io.Discard)
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

// dapDuraklamaKomutlari program duraklatılmışken VM goroutine'inde işlenen isteklerdir.
var dapDuraklamaKomutlari = []string{"stackTrace", "scopes", "variables", "setVariable", "evaluate",
	"gotoTargets", "goto", "dataBreakpointInfo", "setDataBreakpoints", "trTablo", "trDegiskenler",
	"next", "stepIn", "stepOut", "pause", "threads", "setBreakpoints"}

// FuzzDAPDuraklamaIstegi: düzenleyicinin program duraklatılmışken gönderdiği istekler
// (değişken tutamakları, çerçeve kimlikleri, satır hedefleri) bozuk ya da kötü niyetli
// argümanlarla hata ayıklayıcıyı çökertmemeli ve asmamalıdır: her isteğe bir yanıt gelmeli,
// program sonra sonuna kadar çalışabilmelidir. Yan etkisiz kalsın diye fonksiyon çağrısı
// içerebilecek argümanlar ('(' ya da \u kaçışı) atlanır. Derin arama:
//
//	go test ./internal/arac/ayikla -run '^$' -fuzz FuzzDAPDuraklamaIstegi -fuzztime 5m
func FuzzDAPDuraklamaIstegi(f *testing.F) {
	f.Add(uint8(0), []byte(`{"threadId":1,"startFrame":-5,"levels":1000000}`))
	f.Add(uint8(2), []byte(`{"variablesReference":1,"start":-1,"count":-7}`))
	f.Add(uint8(3), []byte(`{"variablesReference":999,"name":"toplam","value":"1000"}`))
	f.Add(uint8(4), []byte(`{"expression":"liste[1][\"ad\"]","context":"watch","frameId":-3}`))
	f.Add(uint8(5), []byte(`{"source":{"path":"x"},"line":-1}`))
	f.Add(uint8(6), []byte(`{"threadId":1,"targetId":0}`))
	f.Add(uint8(7), []byte(`{"variablesReference":2,"name":"k"}`))
	f.Add(uint8(9), []byte(`{"ifade":"liste"}`))

	f.Fuzz(func(t *testing.T, secim uint8, arg []byte) {
		if bytes.ContainsAny(arg, "(") || bytes.Contains(arg, []byte(`\u`)) || len(arg) > 4096 {
			return
		}
		if !json.Valid(arg) {
			arg = []byte("null")
		}
		komut := dapDuraklamaKomutlari[int(secim)%len(dapDuraklamaKomutlari)]
		c := startDAP(t)
		yol := writeProgram(t, dapProgram)
		c.launch(yol, false, map[string]interface{}{"line": 4})
		c.event("stopped")

		c.seq++
		seq := c.seq
		govde := fmt.Sprintf(`{"seq":%d,"type":"request","command":%q,"arguments":%s}`, seq, komut, arg)
		fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n%s", len(govde), govde)
		c.next(func(m map[string]interface{}) bool {
			r, _ := m["request_seq"].(float64)
			return m["type"] == "response" && int(r) == seq
		}, komut+" yanıtı")

		// Program sonuna kadar sürdürülür (istek yeni duraklamalara yol açmış olabilir).
		c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": yol}, "breakpoints": []interface{}{}})
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
