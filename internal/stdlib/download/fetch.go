package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// chunk bir okumanın en büyük boyudur; iptal en geç bir chunk sonra görülür.
const chunk = 1 << 20

// progressEvery ilerleme geri çağrısının en sık aralığıdır (28.10: en çok 0,25 saniyede bir).
var progressEvery = 250 * time.Millisecond

// newHash indirmenin karmasını kurar; testler yavaş bir karmayla iptalin bir okuma sınırında
// durduğunu sınar.
var newHash = sha256.New

// errRestart, sürdürme isteğine uymayan cevapta (Range'e 200 dışı ya da yanlış konum, 416) .part
// sıfırlandığını ve aynı çağrının baştan indiğini söyler.
var errRestart = errors.New("restart")

// await work'ü kendi goroutine'inde, yorumlayıcının kilidi bırakılarak koşar; iptal, süre sınırı ya
// da durdurma gelince stop çağrılır ve work bitene kadar beklenir (AwaitExternal, 10.9).
func await(m *interp.VM, stop func(), work func()) error {
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		work()
	}()
	return m.AwaitExternal(ready, stop)
}

// hashPart .part'ın içeriğini baştan h'ye yazar. İptal, süre sınırı ve durdurma bir okuma sınırında
// durdurur (en çok bir chunk sonra) ve await'in hatası döner; readErr diskin hatasıdır (28.10, 10.9).
func hashPart(m *interp.VM, p *part, h hash.Hash) (readErr, err error) {
	var stopped atomic.Bool
	err = await(m, func() { stopped.Store(true) }, func() { readErr = p.readFrom0(h, &stopped) })
	return readErr, err
}

// run indirmeyi .part'ın kilidi tutulurken yapar ve indirilen dosyanın sha256'sını verir.
func (c *call) run(m *interp.VM, p *part) (string, error) {
	have, err := p.size()
	if err != nil {
		return "", c.diskError()
	}
	if have > 0 && !c.hasHash {
		// Karmasız çağrı sürdürmez: başka koşudan kalan baş sessizce yeni sonla birleşirdi.
		if err := p.reset(); err != nil {
			return "", c.diskError()
		}
		have = 0
	}
	if have > 0 && c.hasSize && have == c.size {
		// Çökmeden kalan tam .part: istek yapılmaz, karma tutarsa yerine konur.
		h := newHash()
		readErr, err := hashPart(m, p, h)
		if err != nil {
			return "", err
		}
		if readErr == nil && hex.EncodeToString(h.Sum(nil)) == c.hash {
			return c.hash, c.place(p)
		}
		if err := p.reset(); err != nil {
			return "", c.diskError()
		}
		have = 0
	}
	for range 3 {
		sum, err := c.fetch(m, p, have)
		if !errors.Is(err, errRestart) {
			return sum, err
		}
		have = 0
	}
	return "", messages.New(messages.H7709, c.fn, http.StatusRequestedRangeNotSatisfiable, shown(c.url))
}

// fetch tek bir istek yapar; have > 0 ise Range ile sürdürür.
func (c *call) fetch(m *interp.VM, p *part, have int64) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url.String(), nil)
	if err != nil {
		return "", messages.New(messages.H7708, c.fn)
	}
	req.Header.Set("Accept-Encoding", "identity")
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
	}
	var resp *http.Response
	var doErr error
	client := newClient(c.fn, c.proxy)
	if err := await(m, cancel, func() { resp, doErr = client.Do(req) }); err != nil {
		return "", err
	}
	if doErr != nil {
		return "", requestError(c.fn, c.url, doErr)
	}
	defer resp.Body.Close()
	total, err := c.accept(p, resp, have)
	if err != nil {
		return "", err
	}
	h := newHash()
	if have > 0 && resp.StatusCode == http.StatusPartialContent {
		readErr, err := hashPart(m, p, h)
		if err != nil {
			return "", err
		}
		if readErr != nil {
			return "", c.diskError()
		}
	} else {
		have = 0
	}
	got, err := c.readBody(m, p, resp, h, have, total, cancel)
	if err != nil {
		return "", err
	}
	return c.finish(p, resp, h, got)
}

// accept cevabı denetler ve dosyanın toplam boyunu (bilinmiyorsa -1) verir. Sürdürmeye uymayan
// cevapta .part sıfırlanır: 200 gövdesiyle baştan iner, yanlış konum ve 416 errRestart'tır.
func (c *call) accept(p *part, resp *http.Response, have int64) (int64, error) {
	total := resp.ContentLength
	switch {
	case have > 0 && resp.StatusCode == http.StatusPartialContent:
		start, all, ok := contentRange(resp.Header.Get("Content-Range"))
		if !ok || start != have {
			return 0, c.restart(p)
		}
		total = all
	case have > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		return 0, c.restart(p)
	case resp.StatusCode == http.StatusOK:
		if have > 0 {
			if err := p.reset(); err != nil {
				return 0, c.diskError()
			}
		}
	default:
		return 0, messages.New(messages.H7709, c.fn, resp.StatusCode, shown(resp.Request.URL))
	}
	if enc := resp.Header.Get("Content-Encoding"); enc != "" && !strings.EqualFold(enc, "identity") {
		return 0, messages.New(messages.H7718, c.fn, enc)
	}
	if c.hasSize && total > c.size {
		if err := p.reset(); err != nil {
			return 0, c.diskError()
		}
		return 0, messages.New(messages.H7719, c.fn, c.size)
	}
	return total, nil
}

func (c *call) restart(p *part) error {
	if err := p.reset(); err != nil {
		return c.diskError()
	}
	return errRestart
}

// contentRange "bytes a-b/toplam" başlığını okur.
func contentRange(v string) (start, total int64, ok bool) {
	rest, found := strings.CutPrefix(v, "bytes ")
	if !found {
		return 0, 0, false
	}
	span, all, found := strings.Cut(rest, "/")
	from, _, found2 := strings.Cut(span, "-")
	if !found || !found2 {
		return 0, 0, false
	}
	start, err1 := strconv.ParseInt(from, 10, 64)
	total, err2 := strconv.ParseInt(all, 10, 64)
	if all == "*" {
		total, err2 = -1, nil
	}
	return start, total, err1 == nil && err2 == nil
}

// readBody gövdeyi okuma sınırlarıyla .part'a yazar ve .part'taki bayt sayısını verir. Her okuma
// kilit bırakılarak yapılır; hareketsizlik zamanlayıcısı her okumada yenilenir.
func (c *call) readBody(m *interp.VM, p *part, resp *http.Response, h hash.Hash, have, total int64, cancel func()) (int64, error) {
	var idle atomic.Bool
	timer := time.AfterFunc(idleTimeout, func() { idle.Store(true); cancel() })
	defer timer.Stop()
	buf := make([]byte, chunk)
	got, last := have, time.Now()
	for {
		var n int
		var readErr, writeErr error
		over := false
		timer.Reset(idleTimeout)
		if err := await(m, cancel, func() {
			n, readErr = resp.Body.Read(buf)
			if c.hasSize && got+int64(n) > c.size {
				over = true
				return
			}
			if n > 0 {
				if _, writeErr = p.f.Write(buf[:n]); writeErr == nil {
					h.Write(buf[:n])
				}
			}
		}); err != nil {
			return 0, err
		}
		if over {
			if err := p.reset(); err != nil {
				return 0, c.diskError()
			}
			return 0, messages.New(messages.H7719, c.fn, c.size)
		}
		if writeErr != nil {
			return 0, c.diskError()
		}
		got += int64(n)
		final := readErr == io.EOF
		if c.progress != nil && (final || time.Since(last) >= progressEvery) {
			if err := c.report(m, got, total); err != nil {
				return 0, err
			}
			last = time.Now()
		}
		switch {
		case final:
			return got, nil
		case readErr == nil:
		case idle.Load():
			return 0, messages.New(messages.H7717, c.fn, c.url.Host, c.resumeKey())
		case errors.Is(readErr, io.ErrUnexpectedEOF):
			return 0, messages.New(messages.H7720, c.fn, got, c.sizeText(total), c.resumeKey())
		default:
			return 0, requestError(c.fn, c.url, readErr)
		}
	}
}

// report ilerleme fonksiyonunu f(alınan, toplam) ile çağırır; hatası indirmeyi durdurur.
func (c *call) report(m *interp.VM, got, total int64) error {
	var all interp.Value = interp.Null
	switch {
	case c.hasSize:
		all = interp.NewInt(c.size)
	case total >= 0:
		all = interp.NewInt(total)
	}
	_, err := m.ApplyFunction(c.progress, []interp.Value{interp.NewInt(got), all}, nil, m.BuiltinLine())
	return err
}

// finish gövde bittikten sonra boyu ve karmayı denetler ve hedefi yerine koyar.
func (c *call) finish(p *part, resp *http.Response, h hash.Hash, got int64) (string, error) {
	closeDelimited := resp.ProtoMajor == 1 && resp.ContentLength == -1 && len(resp.TransferEncoding) == 0
	if closeDelimited && !c.hasSize && !c.hasHash {
		return "", messages.New(messages.H7739, c.fn)
	}
	if c.hasSize && got < c.size {
		return "", messages.New(messages.H7720, c.fn, got, c.sizeText(-1), c.resumeKey())
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if c.hasHash && sum != c.hash {
		if err := p.reset(); err != nil {
			return "", c.diskError()
		}
		return "", messages.New(messages.H7721, c.fn, c.hash, sum)
	}
	return sum, c.place(p)
}

// place .part'ı kilit tutulurken hedefin yerine koyar.
func (c *call) place(p *part) error {
	if err := p.place(c.target); err != nil {
		return messages.New(messages.H7728, c.fn, c.userPath)
	}
	return nil
}

func (c *call) diskError() error { return messages.New(messages.H7728, c.fn, c.userPath+".part") }

// resumeKey H7717 ve H7720'nin seçimidir: karmalı çağrı sürdürür.
func (c *call) resumeKey() string {
	if c.hasHash {
		return "yes"
	}
	return "no"
}

// sizeText H7720'nin beklenen boyudur: TR_size:, yoksa sunucunun bildirdiği toplam; ikisi de yoksa boş
// (empty dalı).
func (c *call) sizeText(total int64) any {
	switch {
	case c.hasSize:
		return c.size
	case total >= 0:
		return total
	}
	return ""
}
