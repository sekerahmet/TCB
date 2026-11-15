package httpserver

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
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
	// Yazarken kopan istemci hata değildir: standart hataya satır yazılmaz (28.9).
	if out := ts.errOut.String(); out != "" {
		t.Errorf("kopan istemci standart hataya satır yazdırmamalı: %q", out)
	}
}

// TestRequestStreamSlowWriteCloses: okumayan istemcide bir yazma slowWrite'ı aşınca istek akışının
// bağlantısı kapanır ve blok iptal edilir; standart hataya satır yazılmaz (28.7, 28.9).
func TestRequestStreamSlowWriteCloses(t *testing.T) {
	old := slowWrite
	slowWrite = 200 * time.Millisecond
	t.Cleanup(func() { slowWrite = old })
	say, n := counter()
	ts := serveTR(t, `TR_var parça = "a" * 262144
TR_func f(i: TR_Request) -> TR_Response {
    TR_return TR_stream_response((a) => {
        a.TR_send_event(0)
        TR_for k TR_in 1..100000 {
            say()
            a.TR_send_event(parça)
        }
    })
}
TR_var s = TR_serve_http(f)`, map[string]interp.Value{"say": say})
	res, c := streamGet(t, ts, "GET", "/")
	if line, err := bufio.NewReader(res.Body).ReadString('\n'); err != nil || line != "data: 0\n" {
		t.Fatalf("ilk olay: %q %v", line, err)
	}
	// İstemci okumayı bırakır: yazma takılır, süre aşılır, blok durur.
	deadline := time.Now().Add(5 * time.Second)
	for {
		before := n.Load()
		time.Sleep(3 * slowWrite)
		if n.Load() == before {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("yazma süresi aşılınca blok durmalı: sayaç %d", n.Load())
		}
	}
	// Blok yalnız takılmış değil, iptal edilmiş ve bağlantı kapanmıştır: istemci kalanı okuyunca akış
	// süre dolmadan biter ve blok yeniden yazmaya başlamaz (takılmış olsaydı okuma onu sürdürürdü).
	stopped := n.Load()
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, err := io.Copy(io.Discard, res.Body)
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() || n.Load() != stopped {
		t.Fatalf("yazma süresi aşılınca bağlantı kapanmalıydı: %v, sayaç %d → %d", err, stopped, n.Load())
	}
	if out := ts.errOut.String(); out != "" {
		t.Errorf("takılan istemci standart hataya satır yazdırmamalı: %q", out)
	}
}

// TestPlainResponseClientGoneWritesNoLine: düz cevabın gövdesini yazarken kopan istemci de hata
// değildir: standart hataya satır yazılmaz, sunucu öbür istekleri cevaplamayı sürdürür (28.9).
func TestPlainResponseClientGoneWritesNoLine(t *testing.T) {
	ts := serveTR(t, `TR_var büyük = "a" * 16000000
TR_func f(i: TR_Request) -> TR_Response {
    TR_if i.TR_path == "/büyük" { TR_return TR_Response(TR_body: büyük) }
    TR_return TR_Response(TR_body: "tamam")
}
TR_var s = TR_serve_http(f)`, nil)
	res, c := streamGet(t, ts, "GET", "/büyük")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("durum %d", res.StatusCode)
	}
	c.Close() // gövdenin çoğu yazılmadan
	time.Sleep(300 * time.Millisecond)
	res2, _ := streamGet(t, ts, "GET", "/")
	body, err := io.ReadAll(res2.Body)
	if err != nil || string(body) != "tamam" {
		t.Fatalf("sunucu sürmeli: %q %v", body, err)
	}
	if out := ts.errOut.String(); out != "" {
		t.Errorf("kopan istemci standart hataya satır yazdırmamalı: %q", out)
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

// TestBroadcastReplayIsMaskedAgain: abonelikten önce eklenen gizli değer geri oynatılan olayda *** olur;
// halkadaki olay dinleyiciye yazılırken (since) de yeniden maskelenir (28.7: maske yazıldığı anda).
func TestBroadcastReplayIsMaskedAgain(t *testing.T) {
	ts, b := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g)`, "")
	if err := send(t, ts, `y.TR_send_event("bir")
y.TR_send_event({"metin": "olay gizli-deger-456", "gizli-deger-456": 1})`); err != nil {
		t.Fatal(err)
	}
	ts.vm.AddSecret("gizli-deger-456")
	_, br, _ := subscribeRaw(t, ts, "GET", "Last-Event-ID: "+b.prefix+"-1\r\n")
	if f := frame(t, br); f != "id: 2\ndata: {\"metin\":\"olay ***\",\"***\":1}\n\n" {
		t.Fatalf("geri oynatılan olay yeniden maskelenmeli: %q", f)
	}
	if err := send(t, ts, `y.TR_send_event("canlı gizli-deger-789")`); err != nil {
		t.Fatal(err)
	}
	ts.vm.AddSecret("gizli-deger-789")
	frames, _, ok := b.since(2)
	if !ok || len(frames) != 1 || !strings.HasSuffix(string(frames[0]), "data: \"canlı ***\"\n\n") {
		t.Fatalf("halkadaki olay yazılırken yeniden maskelenmeli: %q", frames)
	}
}

// TestHeartbeatIsMaskedAtEachWrite: TR_heartbeat:'ın verisi her yazılışta maskelenir (28.7): akış ya da
// yayıncı kurulduktan sonra eklenen gizli değer nabızda *** olur; iki kipte.
func TestHeartbeatIsMaskedAtEachWrite(t *testing.T) {
	old := heartbeatInterval
	heartbeatInterval = 30 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = old })
	ts, _ := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g, TR_heartbeat: "nabız gizli-deger-321")`, "")
	_, br, _ := subscribeRaw(t, ts, "GET", "")
	frame(t, br)
	if f := frame(t, br); f != "data: \"nabız gizli-deger-321\"\n\n" {
		t.Fatalf("yayın nabzı: %q", f)
	}
	ts.vm.AddSecret("gizli-deger-321")
	if f := frame(t, br); f != "data: \"nabız ***\"\n\n" {
		t.Fatalf("yayın nabzı yeniden maskelenmeli: %q", f)
	}
	rs := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_return TR_stream_response((a) => { TR_sleep(1) }, TR_heartbeat: "istek nabzı gizli-deger-654")
}
TR_var s = TR_serve_http(f)`, nil)
	res, _ := streamGet(t, rs, "GET", "/")
	rb := bufio.NewReader(res.Body)
	if f, _ := rawFrame(rb); f != "data: \"istek nabzı gizli-deger-654\"\n\n" {
		t.Fatalf("istek nabzı: %q", f)
	}
	rs.vm.AddSecret("gizli-deger-654")
	if f, _ := rawFrame(rb); f != "data: \"istek nabzı ***\"\n\n" {
		t.Fatalf("istek nabzı yeniden maskelenmeli: %q", f)
	}
}

// TestRemaskKeepsTheBytes: yeniden maske yalnız metin sözcüklerini değiştirir (DENETİM 775 Ş2): gizli değer
// içermeyen olay, alakasız bir gizli değer eklenince bayt bayt aynı kalır (ondalığın yazımı, 2^53'ten
// büyük tam sayı, kaçışlı metin, anahtar sırası); gizli değer taşıyan metin eventData'nın yazımıyla ***.
func TestRemaskKeepsTheBytes(t *testing.T) {
	v := interp.New()
	data := []byte(`{"z":1.0,"a":12345678901234567890,"e":1e3,"n":-0.0,"m":"xç\n<\"","l":[true,null,"gizli-deger-987 son"]}`)
	if got := remask(v, data); string(got) != string(data) {
		t.Fatalf("gizli değer yokken baytlar değişmemeli: %s", got)
	}
	v.AddSecret("alakasız-değer")
	if got := remask(v, data); string(got) != string(data) {
		t.Fatalf("alakasız gizli değerde baytlar değişmemeli: %s", got)
	}
	v.AddSecret("gizli-deger-987")
	want := `{"z":1.0,"a":12345678901234567890,"e":1e3,"n":-0.0,"m":"xç\n<\"","l":[true,null,"*** son"]}`
	if got := remask(v, data); string(got) != want {
		t.Fatalf("yalnız gizli değerli metin değişmeli:\n%s\n%s", got, want)
	}
}

// forgottenCase TestForgottenMaskIsCaught'un bir akışıdır: events TR'de olayların listesi, want her olayın
// verisi, warn H7644'ün yolu (boş: uyarı yok, "-" yerine).
type forgottenCase struct {
	name, events string
	secret       bool
	want         []string
	warn         string // "-": uyarı yok
}

// TestForgottenMaskIsCaught: olay akışı her metin yaprağından 19.4a'nın akışının tutacağını tutar (28.7):
// iki olaya bölünen gizli değer ve adresin kimlik kısmı yeni olayda *** olur, standart hataya H7644 akış
// başına bir kez yazılır (yolu tırnaklı JSON göstergesi). Sınırı gelmemiş bölge en çok en uzun gizli değer
// + holdPad bayt tutulur: sınırdaki tutulan maskelenmez, bir bayt fazlası yeni olayda *** olur. Parçalar
// TR_mask_stream()'le gönderilince (H7644'ün önerisi) uyarı yoktur. İki kipte: istek akışı ve yayın.
func TestForgottenMaskIsCaught(t *testing.T) {
	long := "https://" + strings.Repeat("a", holdPad-len("://")) // tutulan "://a...": tam holdPad bayt
	cases := []forgottenCase{
		{"bölünen değer", `["anahtar gizli-de", "ger-123 bitti", "ger-123"]`, true,
			[]string{`"anahtar gizli-de"`, `"*** bitti"`, `"ger-123"`}, ""},
		{"bölünen adres", `[{"m": "bağlan https://ali:parola", "n": 1}, {"m": "123@sunucu/yol"}]`, false,
			[]string{`{"m":"bağlan https://ali:parola","n":1}`, `{"m":"***@sunucu/yol"}`}, "/m"},
		{"listede ve uyarı bir kez", `[["x", "anahtar gizli-de"], ["y", "ger-123"], ["", "gizli-de"], ["", "ger-123"]]`, true,
			[]string{`["x","anahtar gizli-de"]`, `["y","***"]`, `["","gizli-de"]`, `["","***"]`}, "/1"},
		{"anahtar tutulmaz", `[{"gizli-de": 1}, {"ger-123": 2}]`, true,
			[]string{`{"gizli-de":1}`, `{"ger-123":2}`}, "-"},
		{"sınırda", `["` + long + `", "/yol"]`, false,
			[]string{`"` + long + `"`, `"/yol"`}, "-"},
		{"sınırı aşan", `["` + long + `", "b", "c/yol"]`, false,
			[]string{`"` + long + `"`, `"***"`, `"c/yol"`}, ""},
		{"TR_mask_stream'le", "", true, []string{`"anahtar "`, `"*** bitti"`, `""`}, "-"},
	}
	for _, c := range cases {
		t.Run(c.name+"/istek akışı", func(t *testing.T) {
			body := `TR_for o TR_in ` + c.events + ` { a.TR_send_event(o) }`
			if c.events == "" {
				body = `TR_var m = TR_mask_stream()
        TR_for p TR_in ["anahtar gizli-de", "ger-123 bitti"] { a.TR_send_event(m.TR_step(p)) }
        a.TR_send_event(m.TR_finish())`
			}
			ts := serveTR(t, `TR_func f(i: TR_Request) -> TR_Response {
    TR_return TR_stream_response((a) => {
        `+body+`
    })
}
TR_var s = TR_serve_http(f)`, nil)
			withMaskStream(ts.vm)
			if c.secret {
				ts.vm.AddSecret("gizli-deger-123")
			}
			res, _ := streamGet(t, ts, "GET", "/")
			got, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("akış düzgün bitmeli: %v", err)
			}
			var want strings.Builder
			for _, w := range c.want {
				want.WriteString("data: " + w + "\n\n")
			}
			if string(got) != want.String() {
				t.Fatalf("akış:\n%q\nbeklenen:\n%q", got, want.String())
			}
			checkForgottenWarning(t, ts, c.warn)
		})
		t.Run(c.name+"/yayın", func(t *testing.T) {
			ts, b := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g)`, "")
			withMaskStream(ts.vm)
			if c.secret {
				ts.vm.AddSecret("gizli-deger-123")
			}
			code := `TR_for o TR_in ` + c.events + ` { y.TR_send_event(o) }`
			if c.events == "" {
				code = `TR_var m = TR_mask_stream()
TR_for p TR_in ["anahtar gizli-de", "ger-123 bitti"] { y.TR_send_event(m.TR_step(p)) }
y.TR_send_event(m.TR_finish())`
			}
			if err := send(t, ts, code); err != nil {
				t.Fatal(err)
			}
			frames, _, ok := b.since(0)
			if !ok || len(frames) != len(c.want) {
				t.Fatalf("halka: %q", frames)
			}
			for i, f := range frames {
				if got := frameData(string(f)); got != c.want[i] {
					t.Fatalf("%d. olay: %q, beklenen %q", i+1, got, c.want[i])
				}
			}
			checkForgottenWarning(t, ts, c.warn)
		})
	}
}

// withMaskStream v'ye TR_mask_stream'i tanımlar (H7644'ün önerisi koşsun diye): yerleşik süreç
// paketinindir; yalnız o alınır, sürecin öbür yerleşikleri ve ortamın adresleri (19.4) bu paketin testlerine
// girmez.
func withMaskStream(v *interp.VM) {
	src := interp.New()
	process.Register(src)
	for _, name := range codes.Spellings(codes.FnMaskStream) {
		b, _ := src.GlobalEnv.Get(name)
		v.GlobalEnv.Define(name, b, true)
	}
}

// checkForgottenWarning standart hatada H7644'ün path yoluyla tam bir kez durduğunu ("-": hiç durmadığını)
// denetler. İleti kodsuz yazılır (28.4'ün satırı gibi); sayım iletinin yoldan önceki başıyla yapılır.
func checkForgottenWarning(t *testing.T, ts *testServer, path string) {
	t.Helper()
	out := ts.errOut.String()
	head := strings.Split(messages.Text(messages.H7644, "§"), `"§"`)[0]
	n := strings.Count(out, head)
	if path == "-" {
		if n != 0 {
			t.Fatalf("uyarı olmamalı: %q", out)
		}
		return
	}
	line := messages.Text(messages.H7644, path)
	if n != 1 || !strings.Contains(out, line) {
		t.Fatalf("H7644 bir kez, yol %q: %q", path, out)
	}
}

// TestForgottenHoldSurvivesAFailedEvent: JSON'a yazılamayan olay gitmez ve tutulanı değiştirmez (28.7):
// bölünen gizli değerin arasına düşen hatalı olaydan sonra ikinci yarı yine *** olur.
func TestForgottenHoldSurvivesAFailedEvent(t *testing.T) {
	ts, b := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g)`, "")
	ts.vm.AddSecret("gizli-deger-123")
	if err := send(t, ts, `y.TR_send_event("anahtar gizli-de")
TR_var kod = TR_error_code { y.TR_send_event([(x) => x]) }
y.TR_send_event("ger-123 bitti")
TR_print(kod)`); err != nil {
		t.Fatal(err)
	}
	frames, _, ok := b.since(0)
	if !ok || len(frames) != 2 || frameData(string(frames[1])) != `"*** bitti"` {
		t.Fatalf("hatalı olaydan sonra tutulan kalmalı: %q", frames)
	}
	checkForgottenWarning(t, ts, "")
}
