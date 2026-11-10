package httpserver

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// broadcastServer y adında bir yayıncı ve s adında bir sunucu kurar: "/" abone eder, "/gonder?v=x"
// x'i başka bir görevden gönderir, öbür yollar "tamam" der. decl yayıncının tanımıdır.
func broadcastServer(t *testing.T, decl, serveArgs string) (*testServer, *EventBroadcast) {
	t.Helper()
	ts := serveTR(t, decl+`
TR_func f(i: TR_Request) -> TR_Response {
    TR_if i.TR_path == "/" { TR_return y.TR_subscribe(i) }
    TR_if i.TR_path == "/gonder" {
        y.TR_send_event(i.TR_query["v"][0])
        TR_return TR_Response(TR_body: "gönderildi")
    }
    TR_return TR_Response(TR_body: "tamam")
}
TR_var s = TR_serve_http(f`+serveArgs+`)`, nil)
	yv, _ := ts.vm.GlobalEnv.Get("y")
	b, ok := yv.(*EventBroadcast)
	if !ok {
		t.Fatalf("y bir yayıncı değil: %v", yv)
	}
	return ts, b
}

// send test VM'sinden (programın ana görevi) TR kodu çalıştırır.
func send(t *testing.T, ts *testServer, code string) error {
	t.Helper()
	_, err := ts.vm.Run(trrun.Parse(t, code))
	return err
}

// subscribeRaw ham bağlantıyla abone olur; header ek başlık satırlarıdır ("Last-Event-ID: 3\r\n").
func subscribeRaw(t *testing.T, ts *testServer, method, header string) (*http.Response, *bufio.Reader, net.Conn) {
	t.Helper()
	c, err := net.DialTimeout("tcp", ts.addr(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	_ = c.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := io.WriteString(c, method+" / HTTP/1.1\r\nHost: "+ts.addr()+"\r\n"+header+"\r\n"); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(c), &http.Request{Method: method})
	if err != nil {
		t.Fatal(err)
	}
	return res, bufio.NewReader(res.Body), c
}

// frame bir SSE olayını okur (boş satıra kadar, sonu dahil); kimliğin yayıncı ön eki readFrame'de atılır.
func frame(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	f, err := readFrame(br)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// rawFrame bir SSE olayını olduğu gibi okur.
func rawFrame(br *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		line, err := br.ReadString('\n')
		b.WriteString(line)
		if err != nil {
			return b.String(), fmt.Errorf("olay okunamadı (%q): %v", b.String(), err)
		}
		if line == "\n" {
			return b.String(), nil
		}
	}
}

// stripPrefix kimlik satırının ön ekini (16 onaltılık karakter ve "-") denetler ve atar: beklentiler
// "id: 3" yazar; ön ekin kendisini TestBroadcastIDsBelongToTheirBroadcaster sınar. Kimliksiz olay aynen döner.
func stripPrefix(f string) (string, error) {
	rest, ok := strings.CutPrefix(f, "id: ")
	if !ok {
		return f, nil
	}
	if len(rest) < 17 || rest[16] != '-' || strings.Trim(rest[:16], "0123456789abcdef") != "" {
		return f, fmt.Errorf("kimliğin ön eki yok ya da biçimi yanlış: %.60q", f)
	}
	return "id: " + rest[17:], nil
}

// frameID olayın kimliğidir; yoksa -1.
func frameID(f string) int64 {
	if !strings.HasPrefix(f, "id: ") {
		return -1
	}
	n, _ := strconv.ParseInt(f[4:strings.IndexByte(f, '\n')], 10, 64)
	return n
}

// frameData olayın data satırıdır.
func frameData(f string) string {
	i := strings.Index(f, "data: ")
	return strings.TrimSuffix(f[i+6:], "\n\n")
}

const counting = `TR_var y = TR_EventBroadcast(TR_capacity: %d, TR_snapshot: TR_JSON(0), TR_reduce: (g, olay) => TR_JSON(TR_Int(g) + 1))`

// TestBroadcastWireFormat: yayın olayı id ve data satırıdır, veri maskeli JSON'dur (sözlük anahtarı da);
// görüntü tek olaydır, kimliği son kimlik, olay yoksa 0; Last-Event-ID: <ön ek>-0 yine görüntü verir (28.7 (f)).
func TestBroadcastWireFormat(t *testing.T) {
	ts, b := broadcastServer(t, fmt.Sprintf(counting, 10), "")
	ts.vm.AddSecret("gizli-deger-123")
	res, br, _ := subscribeRaw(t, ts, "GET", "")
	if res.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("başlıklar: %v", res.Header)
	}
	if f := frame(t, br); f != "id: 0\ndata: 0\n\n" {
		t.Fatalf("olay yokken görüntü: %q", f)
	}
	if err := send(t, ts, `y.TR_send_event("a")
y.TR_send_event({"gizli-deger-123": "x gizli-deger-123"})`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"id: 1\ndata: \"a\"\n\n", "id: 2\ndata: {\"***\":\"x ***\"}\n\n"} {
		if f := frame(t, br); f != want {
			t.Fatalf("olay %q, beklenen %q", f, want)
		}
	}
	for _, h := range []string{"", "Last-Event-ID: " + b.prefix + "-0\r\n"} {
		_, br2, _ := subscribeRaw(t, ts, "GET", h)
		if f := frame(t, br2); f != "id: 2\ndata: 2\n\n" {
			t.Fatalf("%q ile görüntü: %q", h, f)
		}
	}
}

// TestBroadcastReplayBounds: Last-Event-ID son kimlikse hiçbir şey gelmez; en eski saklananın bir
// eksiğiyse hepsi; ondan bir eksikse, son kimlikten büyükse, sıra sayı değilse ya da ön ek yoksa görüntü (28.7).
func TestBroadcastReplayBounds(t *testing.T) {
	ts, b := broadcastServer(t, fmt.Sprintf(counting, 3), "")
	p := b.prefix + "-"
	if err := send(t, ts, `TR_for k TR_in 1..6 { y.TR_send_event({"k": k}) }`); err != nil {
		t.Fatal(err)
	}
	// Saklananlar 3, 4, 5; görüntünün kimliği 5, verisi katlanan olay sayısı (5); olayın verisi {"k":5}.
	ev := func(k int) string { return fmt.Sprintf("id: %d\ndata: {\"k\":%d}\n\n", k, k) }
	snapshot := "id: 5\ndata: 5\n\n"
	cases := []struct {
		last string
		want []string
	}{
		{p + "5", nil}, {p + "2", []string{ev(3), ev(4), ev(5)}}, {p + "4", []string{ev(5)}},
		{p + "1", []string{snapshot}}, {p + "9", []string{snapshot}}, {p + "+3", []string{snapshot}}, {p + "x", []string{snapshot}},
		{"2", []string{snapshot}}, {p, []string{snapshot}},
	}
	var conns []*bufio.Reader
	for _, c := range cases {
		_, br, _ := subscribeRaw(t, ts, "GET", "Last-Event-ID: "+c.last+"\r\n")
		conns = append(conns, br)
	}
	if err := send(t, ts, `y.TR_send_event({"k": 6})`); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		var got []string
		for {
			f := frame(t, conns[i])
			if frameID(f) == 6 {
				break
			}
			got = append(got, f)
		}
		if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", c.want) {
			t.Errorf("Last-Event-ID %s: %q geldi, beklenen %q", c.last, got, c.want)
		}
	}
}

// TestBroadcastIDsBelongToTheirBroadcaster: olay kimliği yayıncının ön ekini taşır; yeniden başlayan
// sunucunun yeni yayıncısı (sayacı yine 1'den) eski yayıncının kimliğiyle dönen istemciye geri oynatmaz,
// görüntü verir. Kendi kimliğiyle dönen kaçırdıklarını alır (28.7; DENETİM 387 N2).
func TestBroadcastIDsBelongToTheirBroadcaster(t *testing.T) {
	old, ob := broadcastServer(t, fmt.Sprintf(counting, 10), "")
	cur, cb := broadcastServer(t, fmt.Sprintf(counting, 10), "")
	if ob.prefix == cb.prefix {
		t.Fatalf("iki yayıncının ön eki aynı: %q", ob.prefix)
	}
	for _, ts := range []*testServer{old, cur} {
		if err := send(t, ts, `TR_for k TR_in 1..4 { y.TR_send_event({"k": k}) }`); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		ts     *testServer
		prefix string
	}{{old, ob.prefix}, {cur, cb.prefix}} {
		_, br, _ := subscribeRaw(t, c.ts, "GET", "")
		if f, err := rawFrame(br); err != nil || !strings.HasPrefix(f, "id: "+c.prefix+"-3\n") {
			t.Fatalf("görüntünün kimliği yayıncının ön ekini taşımalı (%q): %q %v", c.prefix, f, err)
		}
	}
	cases := []struct {
		last string
		want []string
	}{
		{ob.prefix + "-1", []string{"id: 3\ndata: 3\n\n"}},
		{cb.prefix + "-1", []string{"id: 2\ndata: {\"k\":2}\n\n", "id: 3\ndata: {\"k\":3}\n\n"}},
	}
	for _, c := range cases {
		_, br, _ := subscribeRaw(t, cur, "GET", "Last-Event-ID: "+c.last+"\r\n")
		var got []string
		for range c.want {
			got = append(got, frame(t, br))
		}
		if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", c.want) {
			t.Errorf("Last-Event-ID %s: %q geldi, beklenen %q", c.last, got, c.want)
		}
	}
}

// TestBroadcastSubscribeRaceLosesNothing: gönderim sürerken abone olan her dinleyici görüntüyü (kimliği
// k, içinde k olay) ve ondan sonraki olayları tam bir kez alır (28.7 (c)).
func TestBroadcastSubscribeRaceLosesNothing(t *testing.T) {
	ts, _ := broadcastServer(t, fmt.Sprintf(counting, 1000), "")
	const total = 200
	sent := make(chan error, 1)
	go func() {
		sent <- send(t, ts, fmt.Sprintf(`TR_for k TR_in 1..%d { y.TR_send_event(k)
TR_sleep(0.001) }`, total+1))
	}()
	var wg sync.WaitGroup
	errs := make(chan string, 16)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		time.Sleep(time.Duration(i*20) * time.Millisecond)
		_, br, _ := subscribeRaw(t, ts, "GET", "")
		go func() {
			defer wg.Done()
			c := bufio.NewReader(br)
			first, err := readFrame(c)
			if err != nil {
				errs <- err.Error()
				return
			}
			k := frameID(first)
			if frameData(first) != strconv.FormatInt(k, 10) {
				errs <- fmt.Sprintf("görüntü %q: kimlik ile katlanan olay sayısı ayrı", first)
				return
			}
			for next := k + 1; next <= total; next++ {
				f, err := readFrame(c)
				if err != nil {
					errs <- err.Error()
					return
				}
				if frameID(f) != next || frameData(f) != strconv.FormatInt(next, 10) {
					errs <- fmt.Sprintf("görüntü %d'den sonra %d bekleniyordu: %q", k, next, f)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
}

// readFrame frame'in hata dönen biçimidir (goroutine'de t.Fatal yok).
func readFrame(br *bufio.Reader) (string, error) {
	f, err := rawFrame(br)
	if err != nil {
		return f, err
	}
	return stripPrefix(f)
}

// TestBroadcastFoldSleepMakesOthersWait: katlama kilidi bırakırsa (TR_sleep) öbür gönderen ve abone
// kilidi bırakarak bekler: iki gönderimin ikisi de katlanır, abone ikisini tam bir kez görür (28.7 (d)).
func TestBroadcastFoldSleepMakesOthersWait(t *testing.T) {
	ts, _ := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, olay) => {
    TR_sleep(0.2)
    TR_return TR_JSON(TR_Int(g) + 1)
})`, "")
	first := make(chan error, 1)
	go func() { first <- send(t, ts, `y.TR_send_event("ana")`) }()
	time.Sleep(50 * time.Millisecond) // katlama uyurken
	other := make(chan string, 1)
	go func() {
		_, body := ts.do(t, "GET", "/gonder?v=gorev", nil, nil)
		other <- body
	}()
	time.Sleep(50 * time.Millisecond)
	_, br, _ := subscribeRaw(t, ts, "GET", "")
	snap := frame(t, br)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if body := <-other; body != "gönderildi" {
		t.Fatalf("görevden gönderim: %q", body)
	}
	seen := frameID(snap)
	if frameData(snap) != strconv.FormatInt(seen, 10) {
		t.Fatalf("görüntü katlanan olay sayısını taşımalı: %q", snap)
	}
	for seen < 2 {
		f := frame(t, br)
		if frameID(f) != seen+1 {
			t.Fatalf("görüntü %q'dan sonra %d bekleniyordu: %q", snap, seen+1, f)
		}
		seen++
	}
	_, br2, _ := subscribeRaw(t, ts, "GET", "")
	if f := frame(t, br2); f != "id: 2\ndata: 2\n\n" {
		t.Fatalf("iki gönderimin ikisi de katlanmalı: %q", f)
	}
}

// TestBroadcastFoldErrors: katlama hatayla biterse ya da JSON vermezse gönderim çağrı hatasıdır, olay
// gitmez, görüntü önceki değerinde kalır; katlama aynı yayıncıya gönderirse çağrı hatası, asılma değil
// (28.7 (d)). Zorunlu argümanlar ve katlamanın parametre sayısı kurulurken denetlenir.
func TestBroadcastFoldErrors(t *testing.T) {
	ts, _ := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, olay) => {
    TR_if TR_String(olay) == "kötü" { TR_throw TR_Message("H7614") }
    TR_if TR_String(olay) == "metin" { TR_return "JSON değil" }
    TR_if TR_String(olay) == "kendine" { y.TR_send_event("iç") }
    TR_return TR_JSON(TR_Int(g) + 1)
})`, "")
	if err := send(t, ts, `y.TR_send_event("iyi")`); err != nil {
		t.Fatal(err)
	}
	for v, code := range map[string]messages.Code{"kötü": messages.H7639, "metin": messages.H7640, "kendine": messages.H7641} {
		done := make(chan error, 1)
		go func() { done <- send(t, ts, `y.TR_send_event("`+v+`")`) }()
		select {
		case err := <-done:
			if !messages.Has(err, code) {
				t.Errorf("%s: %s bekleniyordu: %v", v, code, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: gönderim asıldı", v)
		}
	}
	_, br, _ := subscribeRaw(t, ts, "GET", "")
	if f := frame(t, br); f != "id: 1\ndata: 1\n\n" {
		t.Fatalf("hatalı gönderimler olay ve görüntü bırakmamalı: %q", f)
	}
	if err := send(t, ts, `y.TR_send_event("iyi")`); err != nil {
		t.Fatalf("hatadan sonra yayıncı çalışmalı: %v", err)
	}
	if f := frame(t, br); f != "id: 2\ndata: \"iyi\"\n\n" {
		t.Fatalf("hatadan sonraki olay: %q", f)
	}
	for code, call := range map[messages.Code]string{
		messages.H7637: `TR_EventBroadcast(TR_capacity: 1, TR_snapshot: TR_JSON(0))`,
		messages.H7638: `TR_EventBroadcast(TR_capacity: 1, TR_snapshot: TR_JSON(0), TR_reduce: (g) => g)`,
		messages.H7628: `TR_EventBroadcast(TR_capacity: 0, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g)`,
		messages.H7604: `TR_EventBroadcast(TR_capacity: 1, TR_snapshot: {"a": 1}, TR_reduce: (g, _) => g)`,
	} {
		if err := send(t, ts, call); !messages.Has(err, code) {
			t.Errorf("%s: %s bekleniyordu: %v", call, code, err)
		}
	}
	// İletilerin önerdiği yazımlar çalışır.
	for _, ok := range []string{
		`TR_var y1 = TR_EventBroadcast(TR_capacity: 100, TR_snapshot: TR_JSON({}), TR_reduce: (g, olay) => olay)
y1.TR_send_event(1)`,
		`TR_var y2 = TR_EventBroadcast(TR_capacity: 1, TR_snapshot: TR_JSON(0), TR_reduce: (g, olay) => TR_JSON({"son": olay}))
y2.TR_send_event(1)`,
	} {
		if err := send(t, ts, ok); err != nil {
			t.Errorf("önerilen yazım çalışmadı: %v\n%s", err, ok)
		}
	}
}

// TestBroadcastSnapshotIsMaskedAgain (DENETİM 377 Ş1): maskeli akıştan geçmeyen, iki olaya bölünen gizli
// değer akışta iki yarım olarak gider (olay başına maske onu görmez), katlanan görüntüde *** olur;
// sonradan eklenen gizli değer de görüntüde maskelenir (28.7 (e)).
func TestBroadcastSnapshotIsMaskedAgain(t *testing.T) {
	ts, _ := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(""), TR_reduce: (g, olay) => TR_JSON(TR_String(g) + TR_String(olay)))`, "")
	ts.vm.AddSecret("gizli-deger-123")
	_, br, _ := subscribeRaw(t, ts, "GET", "")
	frame(t, br)
	if err := send(t, ts, `y.TR_send_event("gizli-de")
y.TR_send_event("ger-123 sonra-eklenen")`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"id: 1\ndata: \"gizli-de\"\n\n", "id: 2\ndata: \"ger-123 sonra-eklenen\"\n\n"} {
		if f := frame(t, br); f != want {
			t.Fatalf("akışta %q, beklenen %q", f, want)
		}
	}
	ts.vm.AddSecret("sonra-eklenen")
	_, br2, _ := subscribeRaw(t, ts, "GET", "")
	if f := frame(t, br2); f != "id: 2\ndata: \"*** ***\"\n\n" {
		t.Fatalf("görüntü yeniden maskelenmeli: %q", f)
	}
}

// TestSlowListenerOnlyItsConnectionCloses: okumayan dinleyicinin yalnız bağlantısı kapanır; gönderim
// ağı beklemez, öbür dinleyici her olayı alır, kesilen dinleyici Last-Event-ID ile dönünce kaçırdığını
// boşluksuz alır (28.7).
func TestSlowListenerOnlyItsConnectionCloses(t *testing.T) {
	old := slowWrite
	slowWrite = 200 * time.Millisecond
	t.Cleanup(func() { slowWrite = old })
	ts, b := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 1000, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g)
TR_var parça = "a" * 262144`, "")
	_, slow, sc := subscribeRaw(t, ts, "GET", "")
	_ = sc.(*net.TCPConn).SetReadBuffer(4096)
	_, fast, _ := subscribeRaw(t, ts, "GET", "")
	frame(t, slow)
	frame(t, fast)
	if err := send(t, ts, `y.TR_send_event(parça)`); err != nil {
		t.Fatal(err)
	}
	// Yavaş dinleyici ilk olayı okur, sonra okumayı bırakır.
	if f := frame(t, slow); frameID(f) != 1 {
		t.Fatalf("yavaş dinleyicinin ilk olayı: %.40q", f)
	}
	const total = 40
	got := make(chan error, 1)
	go func() {
		for k := int64(1); k <= total; k++ {
			f, err := readFrame(fast)
			if err != nil || frameID(f) != k {
				got <- fmt.Errorf("hızlı dinleyici %d bekliyordu: %.40q %v", k, f, err)
				return
			}
		}
		got <- nil
	}()
	start := time.Now()
	if err := send(t, ts, fmt.Sprintf(`TR_for k TR_in 2..%d { y.TR_send_event(parça) }`, total+1)); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("gönderim yavaş dinleyiciyi bekledi: %v", d)
	}
	if err := <-got; err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * slowWrite) // yavaş dinleyicinin yazması süreyi aşar, bağlantısı kapanır
	last := int64(1)
	for {
		f, err := readFrame(slow)
		if err != nil {
			break
		}
		last = frameID(f)
	}
	if last >= total {
		t.Fatalf("yavaş dinleyicinin bağlantısı kapanmalıydı, %d olayın hepsini aldı", total)
	}
	_, back, _ := subscribeRaw(t, ts, "GET", "Last-Event-ID: "+b.prefix+"-"+strconv.FormatInt(last, 10)+"\r\n")
	for k := last + 1; k <= total; k++ {
		if f := frame(t, back); frameID(f) != k {
			t.Fatalf("geri dönen dinleyici %d bekliyordu (son aldığı %d): %.40q", k, last, f)
		}
	}
	t.Logf("yavaş dinleyici %d olaydan sonra kesildi", last)
}

// TestListenerNotCountedInMaxConcurrent: açık dinleyici TR_max_concurrent: sınırına sayılmaz (28.4, 28.7).
func TestListenerNotCountedInMaxConcurrent(t *testing.T) {
	ts, _ := broadcastServer(t, fmt.Sprintf(counting, 10), ", TR_max_concurrent: 1")
	_, br, _ := subscribeRaw(t, ts, "GET", "")
	frame(t, br)
	if res, body := ts.do(t, "GET", "/x", nil, nil); res.StatusCode != 200 || body != "tamam" {
		t.Fatalf("dinleyici açıkken istek: %d %q", res.StatusCode, body)
	}
}

// TestBroadcastHeadRegistersNoListener: HEAD'de başlıklar gider, dinleyici kaydedilmez (28.3, 28.7).
func TestBroadcastHeadRegistersNoListener(t *testing.T) {
	ts, b := broadcastServer(t, fmt.Sprintf(counting, 10), "")
	res, _, _ := subscribeRaw(t, ts, "HEAD", "")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" || len(body) != 0 {
		t.Fatalf("HEAD: %d %v %q", res.StatusCode, res.Header, body)
	}
	b.mu.Lock()
	n := len(b.listeners)
	b.mu.Unlock()
	if n != 0 {
		t.Fatalf("HEAD'de %d dinleyici kaydedildi", n)
	}
}

// TestBroadcastHeartbeat: yayın akışının nabzı kimliksiz bir veri olayıdır, {"type":"heartbeat"};
// TR_heartbeat: verisini değiştirir, maskelenir; nabız saklanmaz (28.7).
func TestBroadcastHeartbeat(t *testing.T) {
	old := heartbeatInterval
	heartbeatInterval = 30 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = old })
	ts, b := broadcastServer(t, `TR_var y = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g)
TR_var y2 = TR_EventBroadcast(TR_capacity: 10, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g, TR_heartbeat: {"tür": "nabız gizli-deger-123"})`, "")
	_, br, _ := subscribeRaw(t, ts, "GET", "")
	frame(t, br)
	if f := frame(t, br); f != "data: {\"type\":\"heartbeat\"}\n\n" {
		t.Fatalf("nabız: %q", f)
	}
	b.mu.Lock()
	n := len(b.ring)
	b.mu.Unlock()
	if n != 0 {
		t.Fatalf("nabız saklanmamalı: %d", n)
	}
	yv, _ := ts.vm.GlobalEnv.Get("y2")
	if got := string(yv.(*EventBroadcast).beat); got != "data: {\"tür\":\"nabız gizli-deger-123\"}\n\n" {
		t.Fatalf("TR_heartbeat: %q", got)
	}
	ts.vm.AddSecret("gizli-deger-123")
	if err := send(t, ts, `TR_var y3 = TR_EventBroadcast(TR_capacity: 1, TR_snapshot: TR_JSON(0), TR_reduce: (g, _) => g, TR_heartbeat: "gizli-deger-123")`); err != nil {
		t.Fatal(err)
	}
	yv, _ = ts.vm.GlobalEnv.Get("y3")
	if got := string(yv.(*EventBroadcast).beat); got != "data: \"***\"\n\n" {
		t.Fatalf("nabzın verisi maskelenmeli: %q", got)
	}
}
