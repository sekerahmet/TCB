package download

import (
	"context"
	"crypto/sha256"
	"hash"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

var zeroTime time.Time

// shortIdle süreleri test için kısaltır.
func shortIdle(t *testing.T) {
	oldIdle, oldHeader := idleTimeout, headerTimeout
	idleTimeout, headerTimeout = 300*time.Millisecond, 300*time.Millisecond
	t.Cleanup(func() { idleTimeout, headerTimeout = oldIdle, oldHeader })
}

// TestDownloadResumesOnlyWithHash: karmalı çağrı .part'tan Range ile sürer; karmasız çağrı .part'ı
// sıfırlar ve baştan iner.
func TestDownloadResumesOnlyWithHash(t *testing.T) {
	body := []byte("0123456789")
	s := fileServer(t, body)
	dir := slash(t.TempDir())
	os.WriteFile(dir+"/a.part", body[:4], 0o644)
	mustRun(t, dl(s.URL+"/a", dir+"/a", q(sum(body)), "10"))
	os.WriteFile(dir+"/b.part", []byte("XXXX"), 0o644)
	mustRun(t, dl(s.URL+"/b", dir+"/b", "TR_none", "TR_none"))
	if b, _ := os.ReadFile(dir + "/b"); string(b) != string(body) {
		t.Errorf("karmasız çağrı baştan inmeli: %q", b)
	}
	if r := s.ranges(); len(r) != 2 || r[0] != "bytes=4-" || r[1] != "" {
		t.Errorf("Range'ler %v, beklenen [bytes=4- \"\"]", r)
	}
}

// TestDownloadRestartsOnWrongRange: Range'e 200 gelirse ya da Content-Range'in başı .part'ın boyu
// değilse .part sıfırlanır ve aynı çağrı baştan iner.
func TestDownloadRestartsOnWrongRange(t *testing.T) {
	body := []byte("0123456789")
	ignore := newServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write(body) })
	wrong := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Range", "bytes 2-9/10")
			w.WriteHeader(http.StatusPartialContent)
			w.Write(body[2:])
			return
		}
		w.Write(body)
	})
	dir := slash(t.TempDir())
	for name, s := range map[string]*server{"ignore": ignore, "wrong": wrong} {
		os.WriteFile(dir+"/"+name+".part", body[:4], 0o644)
		mustRun(t, dl(s.URL+"/a", dir+"/"+name, q(sum(body)), "10"))
		if b, _ := os.ReadFile(dir + "/" + name); string(b) != string(body) {
			t.Errorf("%s: hedef %q", name, b)
		}
	}
}

// TestDownloadHashMismatchResetsPart: karma tutmazsa H7721; .part sıfırlanır, hedef yerine konmaz.
func TestDownloadHashMismatchResetsPart(t *testing.T) {
	s := fileServer(t, []byte("bozuk"))
	dir := slash(t.TempDir())
	wantCode(t, dl(s.URL+"/a", dir+"/a", q(sum([]byte("doğru"))), "TR_none"), messages.H7721)
	if partSize(dir+"/a") != 0 || exists(dir+"/a") {
		t.Errorf(".part 0 bayt olmalı (%d), hedef olmamalı", partSize(dir+"/a"))
	}
}

// TestDownloadSizeExceededStopsEarly: Content-Length boyu aşarsa gövde okunmadan, bilinmeyen boyda
// aşan bayt yazılmadan H7719; .part sıfırlanır.
func TestDownloadSizeExceededStopsEarly(t *testing.T) {
	big := []byte(strings.Repeat("x", 100))
	s := fileServer(t, big)
	chunked := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(big[:50])
		w.(http.Flusher).Flush()
		w.Write(big[50:])
	})
	dir := slash(t.TempDir())
	wantCode(t, dl(s.URL+"/a", dir+"/a", "TR_none", "10"), messages.H7719)
	wantCode(t, dl(chunked.URL+"/a", dir+"/b", "TR_none", "60"), messages.H7719)
	if partSize(dir+"/a") != 0 || partSize(dir+"/b") != 0 {
		t.Errorf(".part'lar sıfırlanmalı: %d, %d", partSize(dir+"/a"), partSize(dir+"/b"))
	}
	mustRun(t, dl(fileServer(t, nil).URL+"/a", dir+"/bos", q(sum(nil)), "0"))
}

// TestDownloadCancelKeepsPart: süre sınırıyla (10.7) iptal edilen indirme bir okuma sınırında durur;
// .part kalır, hedef yok; aynı karmalı çağrı Range ile sürer.
func TestDownloadCancelKeepsPart(t *testing.T) {
	body := []byte(strings.Repeat("y", 1000))
	release := make(chan struct{})
	s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			http.ServeContent(w, r, "f.bin", zeroTime, strings.NewReader(string(body)))
			return
		}
		w.Header().Set("Content-Length", "1000")
		w.Write(body[:400])
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer close(release)
	dir := slash(t.TempDir())
	call := dl(s.URL+"/a", dir+"/a", q(sum(body)), "1000")
	wantCode(t, "TR_var g = TR_async { "+call+" }\ng.TR_wait(TR_timeout: 0.5)", messages.H2921)
	if partSize(dir+"/a") != 400 || exists(dir+"/a") {
		t.Fatalf(".part 400 bayt kalmalı (%d), hedef olmamalı", partSize(dir+"/a"))
	}
	mustRun(t, call)
	if r := s.ranges(); r[len(r)-1] != "bytes=400-" {
		t.Errorf("ikinci çağrı sürmeli: %v", r)
	}
}

// slowHash her MiB'ında 200 ms bekleyen sha256'dır.
type slowHash struct{ hash.Hash }

func (s slowHash) Write(b []byte) (int, error) {
	time.Sleep(time.Duration(len(b)) * 200 * time.Millisecond / chunk)
	return s.Hash.Write(b)
}

// TestDownloadCancelStopsPartHash: var olan .part'ın karması da bir okuma sınırında (en çok 1 MiB)
// durur: süre sınırı (10.7) bütün .part'ın karmasını beklemez; .part kalır, hedef yok (28.10; DENETİM
// 759 N1). .part tam boyda olduğu için istek yapılmaz.
func TestDownloadCancelStopsPartHash(t *testing.T) {
	old := newHash
	newHash = func() hash.Hash { return slowHash{sha256.New()} }
	t.Cleanup(func() { newHash = old })
	body := []byte(strings.Repeat("z", 16*chunk))
	dir := slash(t.TempDir())
	if err := os.WriteFile(dir+"/a.part", body, 0o644); err != nil {
		t.Fatal(err)
	}
	call := dl("https://huggingface.co/a", dir+"/a", q(sum(body)), strconv.Itoa(len(body)))
	start := time.Now()
	wantCode(t, "TR_var g = TR_async { "+call+" }\ng.TR_wait(TR_timeout: 0.1)", messages.H2921)
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Errorf("iptal .part'ın bütün karmasını bekledi: %v (bütün karma 3,2 sn)", d)
	}
	if partSize(dir+"/a") != int64(len(body)) || exists(dir+"/a") {
		t.Errorf(".part %d bayt kalmalı (%d), hedef olmamalı", len(body), partSize(dir+"/a"))
	}
}

// TestDownloadTimeouts: ilk üstbilgi süresi H7716, hareketsizlik H7717 (.part kalır).
func TestDownloadTimeouts(t *testing.T) {
	shortIdle(t)
	slow := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	dir := slash(t.TempDir())
	wantCode(t, dl(slow.URL+"/a", dir+"/a", "TR_none", "TR_none"), messages.H7716)
	stall := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.Write([]byte("abc"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	wantCode(t, dl(stall.URL+"/a", dir+"/b", "TR_none", "TR_none"), messages.H7717)
	if partSize(dir+"/b") != 3 {
		t.Errorf(".part 3 bayt kalmalı: %d", partSize(dir+"/b"))
	}
}

// TestDownloadProgressCalls: ilerleme fonksiyonu sonda alınan == toplam ile çağrılır; hatası
// indirmeyi durdurur.
func TestDownloadProgressCalls(t *testing.T) {
	body := []byte(strings.Repeat("z", 5000))
	s := fileServer(t, body)
	dir := slash(t.TempDir())
	out := mustRun(t, "TR_var son = []\n"+dl(s.URL+"/a", dir+"/a", "TR_none", "TR_none", "TR_on_progress: (a, t) => { son = [a, t] }")+"\nTR_print(son)")
	if strings.TrimSpace(out) != "[5000, 5000]" {
		t.Errorf("son çağrı %q, beklenen [5000, 5000]", out)
	}
	if _, err := run(t, dl(s.URL+"/a", dir+"/b", "TR_none", "TR_none", `TR_on_progress: (a, t) => [1][5]`)); err == nil || exists(dir+"/b") {
		t.Errorf("ilerleme fonksiyonunun hatası indirmeyi durdurmalı: %v", err)
	}
}

// TestDownloadCloseDelimitedBodyNeedsSizeOrHash: Content-Length'li ve parçalı cevapta erken kapanma
// H7720'dir; kapanışla biten gövde boysuz ve karmasız çağrıda H7739'dur, hedef yerine konmaz.
func TestDownloadCloseDelimitedBodyNeedsSizeOrHash(t *testing.T) {
	dir := slash(t.TempDir())
	withLength := rawServer(t, "HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\n0123456789")
	chunkedEarly := rawServer(t, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\n0123456789\r\n")
	closeEnd := rawServer(t, "HTTP/1.1 200 OK\r\nConnection: close\r\n\r\n0123456789")
	wantCode(t, dl(withLength, dir+"/a", "TR_none", "TR_none"), messages.H7720)
	wantCode(t, dl(chunkedEarly, dir+"/b", "TR_none", "TR_none"), messages.H7720)
	wantCode(t, dl(closeEnd, dir+"/c", "TR_none", "TR_none"), messages.H7739)
	if exists(dir+"/c") || partSize(dir+"/c") != 10 {
		t.Errorf("H7739'da hedef olmamalı, .part kalmalı: %d", partSize(dir+"/c"))
	}
	mustRun(t, dl(closeEnd, dir+"/d", "TR_none", "10"))
}

// blockingResolver bağlamı bitene kadar dönmez (cevap vermeyen DNS).
type blockingResolver struct{}

func (blockingResolver) LookupIPAddr(ctx context.Context, _ string) ([]net.IPAddr, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// stallHeader cevap vermeyen işleyicidir: TLS biter, ilk üstbilgi gelmez.
func stallHeader(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }

// TestDownloadNameResolutionTimeout: dönmeyen ad çözümü bağlanmanın süresinde H7716 (evre dns) olur
// (28.10).
func TestDownloadNameResolutionTimeout(t *testing.T) {
	shortTimes(t, 300*time.Millisecond, time.Second, time.Second, 10*time.Second)
	old := resolver
	resolver = blockingResolver{}
	t.Cleanup(func() { resolver = old })
	start := time.Now()
	err := wantCode(t, dl("https://raw.example.test/f", slash(t.TempDir())+"/a", "TR_none", "TR_none"), messages.H7716)
	if host, stage, _ := timeoutArgs(err); host != "raw.example.test" || stage != stageDNS {
		t.Errorf("konak %q, evre %q; beklenen raw.example.test, dns", host, stage)
	}
	if !strings.Contains(err.Error(), "adı 30 saniyede çözülmedi") {
		t.Errorf("ileti evreyi söylemeli: %v", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("ad çözümü süresinde kesilmeli: %v", d)
	}
}

// TestDownloadTimeoutNamesTheStage: her evre kendi süresinde H7716 olur ve ileti evreyi adıyla söyler:
// CONNECT'e cevap vermeyen vekil connect, TLS'te ve ilk üstbilgide asılan sunucu IPv4'te ve IPv6'da
// tls ve header (28.10).
func TestDownloadTimeoutNamesTheStage(t *testing.T) {
	shortTimes(t, 300*time.Millisecond, 300*time.Millisecond, time.Second, 10*time.Second)
	head4 := tlsServer(t, "tcp", "127.0.0.1:0", stallHeader, nil)
	head6 := tlsServer(t, "tcp6", "[::1]:0", stallHeader, nil)
	cases := []struct{ name, url, proxy, stage, text string }{
		{"vekil", "https://[::1]:9/f", "http://" + silentListener(t, "tcp", "127.0.0.1:0"), stageConnect, "ile bağlantı 30 saniyede kurulmadı"},
		{"tls-v4", "https://" + silentListener(t, "tcp", "127.0.0.1:0") + "/f", "", stageTLS, "ile TLS el sıkışması 30 saniyede bitmedi"},
		{"tls-v6", "https://" + silentListener(t, "tcp6", "[::1]:0") + "/f", "", stageTLS, "ile TLS el sıkışması 30 saniyede bitmedi"},
		{"header-v4", head4.URL + "/f", "", stageHeader, "ilk üstbilgiyi 30 saniyede göndermedi"},
		{"header-v6", head6.URL + "/f", "", stageHeader, "ilk üstbilgiyi 30 saniyede göndermedi"},
	}
	dir := slash(t.TempDir())
	for _, c := range cases {
		var extra []string
		if c.proxy != "" {
			extra = append(extra, "TR_proxy: "+q(c.proxy))
		}
		start := time.Now()
		_, err := run(t, dl(c.url, dir+"/"+c.name, "TR_none", "TR_none", extra...))
		_, stage, ok := timeoutArgs(err)
		if !ok || stage != c.stage || !strings.Contains(err.Error(), c.text) {
			t.Errorf("%s: H7716 %s bekleniyordu (%q), gelen evre %q: %v", c.name, c.stage, c.text, stage, err)
		}
		if d := time.Since(start); d > 3*time.Second {
			t.Errorf("%s: evre kendi süresinde kesilmeli: %v", c.name, d)
		}
	}
}

// TestDownloadTimeoutNamesTheFailingHost: yönlendirilen ikinci sunucu üstbilgide asılırsa ileti onun
// konağını yazar, ilk adresinkini değil.
func TestDownloadTimeoutNamesTheFailingHost(t *testing.T) {
	shortTimes(t, time.Second, 300*time.Millisecond, time.Second, 10*time.Second)
	second := tlsServer(t, "tcp", "127.0.0.1:0", stallHeader, nil)
	first := tlsServer(t, "tcp", "127.0.0.1:0", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL+"/f", http.StatusFound)
	}, nil)
	err := wantCode(t, dl(first.URL+"/f", slash(t.TempDir())+"/a", "TR_none", "TR_none"), messages.H7716)
	want := strings.TrimPrefix(second.URL, "https://")
	if host, stage, _ := timeoutArgs(err); host != want || stage != stageHeader {
		t.Errorf("konak %q, evre %q; beklenen %q, header", host, stage, want)
	}
}

// TestDownloadRedirectChainCap: her adımı kendi süresinin içinde kalan ama toplamı tavanı aşan
// yönlendirme zinciri H7716'nın total dalıdır; konak tavan dolarken süren isteğinkidir (28.10).
func TestDownloadRedirectChainCap(t *testing.T) {
	shortTimes(t, time.Second, time.Second, time.Second, 600*time.Millisecond)
	second := tlsServer(t, "tcp", "127.0.0.1:0", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(400 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		w.Write([]byte("son"))
	}, nil)
	first := tlsServer(t, "tcp", "127.0.0.1:0", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond)
		http.Redirect(w, r, second.URL+"/f", http.StatusFound)
	}, nil)
	err := wantCode(t, dl(first.URL+"/f", slash(t.TempDir())+"/a", "TR_none", "TR_none"), messages.H7716)
	want := strings.TrimPrefix(second.URL, "https://")
	if host, stage, _ := timeoutArgs(err); host != want || stage != stageTotal {
		t.Errorf("konak %q, evre %q; beklenen %q, total", host, stage, want)
	}
	if !strings.Contains(err.Error(), "için yönlendirmeler dahil ilk üstbilgi 120 saniyede alınmadı") {
		t.Errorf("ileti tavanı söylemeli: %v", err)
	}
}

// TestDownloadCapSparesTheBody: tavan ilk üstbilgi gelince kalkar; gövde tavandan uzun sürse de
// (hareketsizlik süresinin içinde) indirme biter (28.10: gövdeye toplam sınır yok).
func TestDownloadCapSparesTheBody(t *testing.T) {
	shortTimes(t, time.Second, time.Second, time.Second, 300*time.Millisecond)
	body := []byte(strings.Repeat("g", 5*1024))
	s := tlsServer(t, "tcp", "127.0.0.1:0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		for i := range 5 {
			w.Write(body[i*1024 : (i+1)*1024])
			w.(http.Flusher).Flush()
			time.Sleep(150 * time.Millisecond)
		}
	}, nil)
	dir := slash(t.TempDir())
	start := time.Now()
	mustRun(t, dl(s.URL+"/f", dir+"/a", q(sum(body)), strconv.Itoa(len(body))))
	if d := time.Since(start); d < 600*time.Millisecond {
		t.Fatalf("gövde tavandan uzun sürmeliydi: %v", d)
	}
	if got, _ := os.ReadFile(dir + "/a"); string(got) != string(body) {
		t.Errorf("dosya tam inmeli: %d bayt", len(got))
	}
}

// TestDownloadCapRaceCountsAsTotal: tavanın zamanlayıcısı Do'nun dönüşüyle yarışıp çoktan koştuysa
// istek tavana takılmış sayılır: H7716 total, gövde okunmaz (DENETİM 795 N1).
func TestDownloadCapRaceCountsAsTotal(t *testing.T) {
	shortTimes(t, time.Second, time.Second, time.Second, 100*time.Millisecond)
	old := afterDo
	afterDo = func() { time.Sleep(300 * time.Millisecond) }
	t.Cleanup(func() { afterDo = old })
	body := []byte("tam")
	s := tlsServer(t, "tcp", "127.0.0.1:0", func(w http.ResponseWriter, r *http.Request) { w.Write(body) }, nil)
	dir := slash(t.TempDir())
	err := wantCode(t, dl(s.URL+"/f", dir+"/a", q(sum(body)), "3"), messages.H7716)
	if _, stage, _ := timeoutArgs(err); stage != stageTotal {
		t.Errorf("evre %q, beklenen total", stage)
	}
	if exists(dir+"/a") || partSize(dir+"/a") > 0 {
		t.Errorf("gövde okunmamalı: hedef %v, .part %d", exists(dir+"/a"), partSize(dir+"/a"))
	}
}

// TestDownloadCancelIsNotATimeout: ilk üstbilgi beklenirken görevin iptali H2920, TR_wait'in süre
// sınırı H2921'dir; ikisi de H7716 değildir (10.9, 10.7).
func TestDownloadCancelIsNotATimeout(t *testing.T) {
	shortTimes(t, time.Second, 5*time.Second, time.Second, 10*time.Second)
	s := tlsServer(t, "tcp", "127.0.0.1:0", stallHeader, nil)
	dir := slash(t.TempDir())
	cancelled := "TR_var g = TR_async { " + dl(s.URL+"/f", dir+"/a", "TR_none", "TR_none") + " }\nTR_sleep(0.2)\ng.TR_cancel()\ng.TR_wait()"
	err := wantCode(t, cancelled, messages.H2920)
	if messages.Has(err, messages.H7716) {
		t.Errorf("iptal süre hatası olmamalı: %v", err)
	}
	limited := "TR_var g = TR_async { " + dl(s.URL+"/f", dir+"/b", "TR_none", "TR_none") + " }\ng.TR_wait(TR_timeout: 0.2)"
	if err := wantCode(t, limited, messages.H2921); messages.Has(err, messages.H7716) {
		t.Errorf("süre sınırı H7716 olmamalı: %v", err)
	}
}

// TestDownloadClosesIdleConnections: indirme bitince bağlantı boşta kalmaz, kapanır (her indirme kendi
// taşıyıcısını kurar; CloseIdleConnections).
func TestDownloadClosesIdleConnections(t *testing.T) {
	var mu sync.Mutex
	states := map[net.Conn]http.ConnState{}
	body := []byte("bağlantı")
	s := tlsServer(t, "tcp", "127.0.0.1:0", func(w http.ResponseWriter, r *http.Request) { w.Write(body) },
		func(c net.Conn, st http.ConnState) {
			mu.Lock()
			states[c] = st
			mu.Unlock()
		})
	dir := slash(t.TempDir())
	for i := range 3 {
		mustRun(t, dl(s.URL+"/f", dir+"/"+strconv.Itoa(i), q(sum(body)), strconv.Itoa(len(body))))
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		open := 0
		for _, st := range states {
			if st != http.StateClosed {
				open++
			}
		}
		n := len(states)
		mu.Unlock()
		if n == 3 && open == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("3 bağlantının hepsi kapanmalı: %d bağlantı, %d açık", n, open)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
