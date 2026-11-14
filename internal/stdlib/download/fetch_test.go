package download

import (
	"crypto/sha256"
	"hash"
	"net/http"
	"os"
	"strconv"
	"strings"
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
