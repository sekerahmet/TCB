package httpserver

import (
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ServeHTTP bir isteği işler (28.4). Geldiği yerin (28.1a), belirtecin (28.2), yolun (28.3), gövde
// sınırının (28.5) ve eşzamanlı istek sınırının (28.4) denetimleri bu sırayla Go'nun goroutine'inde, kilit
// alınmadan ve görev açılmadan yapılır: kimliksiz bir istek seli TR görevi açamaz. Geçen istek için görev
// açılır.
// İşleyici akışlı cevap (28.7) döndürürse blok aynı görevde çalışır; görev ancak akış başladıktan sonra
// istemci koparsa ya da yazma hata verirse iptal edilir (stop). Yayın akışının dinleyicisi görev bittikten
// sonra kilitsiz yazar ve eşzamanlı istek sınırına sayılmaz.
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.local {
		if code, text := s.originError(r); code != 0 {
			refuseUnread(w, r, code, text)
			return
		}
	}
	if s.token != "" && !s.isTokenless(r) && !s.tokenMatches(r) {
		w.Header()["Www-Authenticate"] = []string{"Bearer"}
		refuseUnread(w, r, http.StatusUnauthorized, messages.Text(messages.H7617))
		return
	}
	if !validPath(r.URL.Path) || !validQuery(r.URL.RawQuery) {
		refuseUnread(w, r, http.StatusBadRequest, messages.Text(messages.H7618))
		return
	}
	body, ok := s.readBody(w, r)
	if !ok {
		refuseUnread(w, r, http.StatusRequestEntityTooLarge, messages.Text(messages.H7619, s.maxBody))
		return
	}
	release := func() {}
	if s.maxConcurrent > 0 {
		if s.active.Add(1) > s.maxConcurrent {
			s.active.Add(-1)
			deny(w, http.StatusServiceUnavailable, messages.Text(messages.H7620, s.maxConcurrent))
			return
		}
		var counted sync.Once
		release = func() { counted.Do(func() { s.active.Add(-1) }) }
		defer release()
	}
	req := requestOf(r, body)
	stop := make(chan struct{})
	var once sync.Once
	cancel := func() { once.Do(func() { close(stop) }) }
	streamed := false
	var listener *Response
	val, err := s.host.RunUntil(stop, func(task *interp.VM) (interp.Value, error) {
		v, err := task.ApplyFunction(s.handler, []interp.Value{req}, nil, s.line)
		if res, ok := v.(*Response); err == nil && ok && res.stream != nil && res.stream.sub != nil {
			listener = res // yayın akışının dinleyicisi: görev biter, dinleyici kilitsiz yazar
			return nil, nil
		}
		if res, ok := v.(*Response); err == nil && ok && res.stream != nil {
			streamed = true
			return nil, s.stream(task, w, r, res, cancel)
		}
		return v, err
	})
	switch {
	case errors.Is(err, interp.ErrInterrupted) || errors.Is(err, interp.ErrHostClosed):
		panic(http.ErrAbortHandler) // iptal ya da kapanış: cevap gönderilmez (28.8)
	case err != nil && streamed:
		// Başlıklar gitti, 500 yazılamaz: akış düzgün bitirilmeden kesilir, istemci bozuk biten akışı görür (28.7).
		s.stderrLine(messages.Text(messages.H7613, req.method, req.path, err))
		panic(http.ErrAbortHandler)
	case err != nil:
		s.handlerFailed(w, req, err)
		return
	case streamed:
		return
	case listener != nil:
		release() // dinleyici TR_max_concurrent:'a sayılmaz; aboneliğin süresi sayıldı (28.4, 28.7)
		s.listen(w, r, listener)
		return
	}
	res, ok := val.(*Response)
	if !ok {
		s.handlerFailed(w, req, messages.New(messages.H7612, interp.CallPrefix(codes.FnServeHttp), val.Type(), ResponseType))
		return
	}
	res.write(w, r.Method == http.MethodHead)
}

// originError 127.0.0.1'e bağlı sunucunun isteğin geldiği yeri denetlemesidir (28.1a): Host, Origin ve
// Sec-Fetch-Site. Üçünü de taşımayan istek (tarayıcı olmayan istemci) geçer.
func (s *HTTPServer) originError(r *http.Request) (int, string) {
	self := "127.0.0.1:" + strconv.Itoa(s.port)
	if r.Host != self {
		return http.StatusForbidden, messages.Text(messages.H7615, "http://"+self)
	}
	for _, o := range r.Header.Values("Origin") {
		if o != "http://"+self {
			return http.StatusForbidden, messages.Text(messages.H7616)
		}
	}
	for _, f := range r.Header.Values("Sec-Fetch-Site") {
		if f != "same-origin" && f != "none" {
			return http.StatusForbidden, messages.Text(messages.H7616)
		}
	}
	return 0, ""
}

// isTokenless istek belirteç istemeyen bir yola mı (TR_tokenless_paths:, 28.2)? Yalnız GET ve HEAD; istek hedefinde
// `?` yoksa (`GET /?` de belirteç ister: RawQuery boş, ForceQuery doğru); çözülmüş yol (28.3'ün .TR_path'i) listedeki
// bir yola tam eşitse. Ham yol (RawPath, RequestURI) karşılaştırılmaz: `/%6Cang/tr.js` `/lang/tr.js`'dir; `//`,
// `/./`, sondaki `/` ve harf büyüklüğü eşleşmez. 28.1a'nın denetimi bundan önce, yolun ve sorgunun denetimi (28.3)
// sonra gelir; gövde okunmaz.
func (s *HTTPServer) isTokenless(r *http.Request) bool {
	get := r.Method == http.MethodGet || r.Method == http.MethodHead
	return get && r.URL.RawQuery == "" && !r.URL.ForceQuery && s.tokenless[r.URL.Path]
}

// tokenMatches Authorization: Bearer <belirteç> başlığını sabit sürede karşılaştırır (28.2).
func (s *HTTPServer) tokenMatches(r *http.Request) bool {
	got := r.Header.Values("Authorization")
	if len(got) != 1 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got[0]), []byte("Bearer "+s.token)) == 1
}

// validPath çözülmüş yol geçersiz UTF-8, denetim karakteri, \ ya da .. bileşeni taşımıyor mu (28.3)?
func validPath(p string) bool {
	if !utf8.ValidString(p) || strings.ContainsRune(p, '\\') || hasControl(p) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

// validQuery sorgunun çözülmüş anahtar ve değerleri geçerli UTF-8 mi? Sorgu değeri veridir, satır sonu
// taşıyabilir (28.3).
func validQuery(raw string) bool {
	_, ok := parseQuery(raw)
	return ok
}

// parseQuery sorguyu geliş sırasıyla çözer: aynı anahtarın değerleri sırasıyla listede.
func parseQuery(raw string) (*interp.DictVal, bool) {
	d := interp.NewDictVal()
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		key, err1 := url.QueryUnescape(k)
		val, err2 := url.QueryUnescape(v)
		if err1 != nil || err2 != nil || !utf8.ValidString(key) || !utf8.ValidString(val) {
			return nil, false
		}
		l, _ := d.Pairs[key].(*interp.ListVal)
		if l == nil {
			l = &interp.ListVal{}
			d.Set(key, l)
		}
		l.Elements = append(l.Elements, &interp.StringVal{Val: val})
	}
	return d, true
}

// readBody gövdeyi en çok maxBody bayt okur (28.5): bilinen Content-Length sınırı aşıyorsa hiç okunmaz,
// chunked gövdenin toplamı da sınıra sayılır; iki okuma arası 60 saniyeyi geçerse bağlantı kapanır.
func (s *HTTPServer) readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if r.ContentLength > s.maxBody {
		return nil, false
	}
	rc := http.NewResponseController(w)
	src := &deadlineReader{r: r.Body, set: func() { _ = rc.SetReadDeadline(time.Now().Add(bodyReadGap)) }}
	body, err := io.ReadAll(io.LimitReader(src, s.maxBody+1))
	if err != nil && int64(len(body)) <= s.maxBody {
		// İstemci koptu ya da sustu: işleyicinin hatası değil (28.9). Süre geçmiş kalır: Go bağlantıyı
		// kapatırken gövdenin kalanını okumaya çalışır, süresiz beklerdi (TestTimeoutsCloseTheConnection).
		_ = rc.SetReadDeadline(time.Now())
		panic(http.ErrAbortHandler)
	}
	_ = rc.SetReadDeadline(time.Time{})
	if int64(len(body)) > s.maxBody {
		return nil, false // gövde yarım kaldı: refuseUnread bağlantıyı kapatır
	}
	return body, true
}

// refuseUnread gövdesi okunmamış ya da yarım kalmış isteği işleyiciyi çağırmadan cevaplar ve bağlantıyı
// kapatır (28.5). Go, işleyici gövdeyi bitirmeden dönünce kalanını cevaptan önce ya da bağlantıyı kapatırken
// okumaya çalışır ve bu okumanın süresi yoktur: susan bir istemci bağlantıyı süresiz tutardı, belirteçsiz
// bir istekle de (TestTimeoutsCloseTheConnection). Connection: close boşaltmayı, geçmiş okuma süresi
// kapanıştaki okumayı keser. Gövdesiz istekte bağlantı açık kalır.
func refuseUnread(w http.ResponseWriter, r *http.Request, code int, text string) {
	if r.ContentLength != 0 { // gövdesi var ya da boyu bilinmiyor (chunked)
		w.Header()["Connection"] = []string{"close"}
		_ = http.NewResponseController(w).SetReadDeadline(time.Now())
	}
	deny(w, code, text)
}

// bodyReadGap iki gövde okuması arasında beklenen en uzun süredir (28.5); testler kısaltır.
var bodyReadGap = 60 * time.Second

// deadlineReader her okumadan önce bağlantının okuma süresini yeniden kurar.
type deadlineReader struct {
	r   io.Reader
	set func()
}

func (d *deadlineReader) Read(p []byte) (int, error) {
	d.set()
	return d.r.Read(p)
}

// requestOf Go'nun isteğinden TR_Request'tir: HEAD işleyiciye GET olarak gelir (28.3).
func requestOf(r *http.Request, body []byte) *Request {
	method := r.Method
	if method == http.MethodHead {
		method = http.MethodGet
	}
	query, _ := parseQuery(r.URL.RawQuery) // validQuery'den geçti
	return &Request{method: method, path: r.URL.Path, query: query, headers: headersOf(r), body: body}
}

// deny işleyiciyi çağırmadan cevaplar; gövde insan içindir, iç bilgi taşımaz.
func deny(w http.ResponseWriter, code int, text string) {
	h := w.Header()
	h["Content-Type"] = []string{"text/plain; charset=utf-8"}
	h["X-Content-Type-Options"] = []string{"nosniff"}
	w.WriteHeader(code)
	_, _ = io.WriteString(w, text+"\n")
}

// handlerFailed işleyicinin yakalanmayan hatasıdır (28.4): istemciye iç bilgisiz 500, standart hataya
// 8.4'ün kaçışı ve 19.4'ün maskesiyle; sunucu sürer, çıkış kodu değişmez.
func (s *HTTPServer) handlerFailed(w http.ResponseWriter, req *Request, err error) {
	s.stderrLine(messages.Text(messages.H7613, req.method, req.path, err))
	deny(w, http.StatusInternalServerError, messages.Text(messages.H7614))
}
