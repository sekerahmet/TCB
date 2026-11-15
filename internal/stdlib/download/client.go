package download

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Süreler (28.10): ad çözümü, bağlanma (vekilin CONNECT cevabı dahil), TLS el sıkışması ve ilk
// üstbilgi için ayrı ayrı 30 saniye; yönlendirmeler dahil ilk üstbilgiye kadar en çok 120 saniye; iki
// okuma arası hareketsizlik için 60 saniye. Testler kısaltır.
var (
	connectTimeout     = 30 * time.Second
	headerTimeout      = 30 * time.Second
	idleTimeout        = 60 * time.Second
	firstHeaderTimeout = 120 * time.Second
)

// minAttempt bir adrese bağlanma denemesinin en kısa süresidir: bağlanmanın süresi adreslere
// paylaştırılırken (Go'nun dialSerial kuralı) son süreyi aşmamak şartıyla. Testler kısaltır.
var minAttempt = 2 * time.Second

// maxRedirects 28.11'in yönlendirme sınırıdır.
const maxRedirects = 10

// resolver adları çözer; testler kendi çözümleyicisini koyar (evil.test → 127.0.0.1).
var resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
} = net.DefaultResolver

// dialAddr bir adrese deadline'a kadar bağlanır; testler kara delik adresi için kendi işlevini koyar
// (yerel adres yasağı çözümlenen adrese uygulandığı için testin sunucusu 127.0.0.1'de kalır).
var dialAddr = func(ctx context.Context, network, addr string, deadline time.Time) (net.Conn, error) {
	d := &net.Dialer{Deadline: deadline}
	return d.DialContext(ctx, network, addr)
}

// testRoots yalnız _test.go'dan kurulur: httptest'in TLS sunucusunun kökü. TR'de ek kök sertifikanın
// yazımı yoktur (28.11).
var testRoots *x509.CertPool

// newClient bir indirmenin istemcisini kurar (28.11). İstemci HTTP/1.1'dir: DialContext verilen
// taşıyıcıda Go, ForceAttemptHTTP2 olmadan HTTP/2'yi açmaz; H7739'un ayırımı buna bağlıdır. Evreler
// isteğin bağlamındaki izleyiciye yazılır (tracker); izleyici yalnız adı tutar, süreleri Go'nun
// alanları ve dial'ın son süreleri keser.
func newClient(fn any, proxy *url.URL) *http.Client {
	var proxyHost string
	if proxy != nil {
		proxyHost = proxy.Host
		if proxy.Port() == "" {
			proxyHost = net.JoinHostPort(proxy.Hostname(), map[string]string{"http": "80", "https": "443"}[proxy.Scheme])
		}
	}
	tr := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			// 127.0.0.1'e giden istek vekilden geçmez: vekil onu kendi makinesinde çözerdi.
			if proxy == nil || r.URL.Hostname() == loopback {
				return nil, nil
			}
			return proxy, nil
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dial(ctx, fn, proxyHost, network, addr)
		},
		// CONNECT'in cevabı geldi: vekil bağlantısının son süresi kalkar, TLS ve üstbilgi kendi
		// süreleriyle sürer (dial'ın vekil kolu).
		OnProxyConnectResponse: func(ctx context.Context, _ *url.URL, _ *http.Request, _ *http.Response) error {
			trackerOf(ctx).liftProxyDeadline()
			return nil
		},
		TLSClientConfig:       &tls.Config{RootCAs: testRoots, MinVersion: tls.VersionTLS12},
		DisableCompression:    true,
		ResponseHeaderTimeout: headerTimeout,
		TLSHandshakeTimeout:   connectTimeout,
	}
	return &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return checkRedirect(fn, req, via)
		},
	}
}

// dial taşıyıcının bağlanmasıdır (28.10, 28.11). Ad, bağlamı connectTimeout'lu bir alt bağlamla
// çözülür (evre dns); çözülen her adres yerel adres listesinden geçer; bağlanma (evre connect)
// adresleri sırayla dener. Vekile bağlanmada bağlantıya bağlanma evresinin son süresi konur: Go
// CONNECT'in cevabını kendi 1 dakikasıyla beklerdi; süre CONNECT'in cevabında kalkar.
func dial(ctx context.Context, fn any, proxyHost, network, addr string) (net.Conn, error) {
	t := trackerOf(ctx)
	if proxyHost != "" && addr == proxyHost {
		t.set(stageConnect)
		deadline := time.Now().Add(connectTimeout)
		conn, err := dialAddr(ctx, network, addr, deadline) // vekilin kuralı: yerel olabilir
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(deadline)
		t.holdProxy(conn)
		return conn, nil
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if _, ok := hostIP(host); ok {
		// Adreste doğrudan yazılmış IP: yerelse yalnız ilk adımda gelir, yönlendirmeyi
		// CheckRedirect reddeder.
		t.set(stageConnect)
		return dialAddr(ctx, network, addr, time.Now().Add(connectTimeout))
	}
	t.set(stageDNS)
	lookup, cancel := context.WithTimeout(ctx, connectTimeout)
	ips, err := resolver.LookupIPAddr(lookup, host)
	cancel()
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, &net.DNSError{Err: "no address", Name: host, IsNotFound: true}
	}
	for _, a := range ips {
		if ip, ok := netip.AddrFromSlice(a.IP); ok && isLocal(ip) {
			return nil, messages.New(messages.H7712, fn, host, ip.Unmap().String())
		}
	}
	// Denetlenen IP'lere bağlanılır: ad yeniden çözülmez (DNS yeniden bağlama).
	t.set(stageConnect)
	return dialEach(ctx, network, interleave(ips), port, time.Now().Add(connectTimeout))
}

// dialEach adreslere sırayla bağlanır: bağlanmanın kalan süresi kalan adreslere paylaştırılır, her
// deneme en az minAttempt alır ama deadline'ı aşmaz (Go'nun dialSerial kuralı). IPv6'sı bozuk ağda
// sıradaki IPv4 adresine böyle geçilir. Son denemenin hatası döner: süre dolduysa o bir süre
// hatasıdır.
func dialEach(ctx context.Context, network string, ips []net.IPAddr, port string, deadline time.Time) (net.Conn, error) {
	var last error
	for i, a := range ips {
		left := time.Until(deadline)
		if left <= 0 {
			break
		}
		share := min(max(left/time.Duration(len(ips)-i), minAttempt), left)
		conn, err := dialAddr(ctx, network, net.JoinHostPort(a.IP.String(), port), time.Now().Add(share))
		if err == nil {
			return conn, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, last
}

// interleave adresleri aileleri dönüşümlü dizer (IPv6, IPv4, IPv6, ...), ilk adresin ailesinden
// başlar; her ailenin kendi sırası korunur.
func interleave(ips []net.IPAddr) []net.IPAddr {
	if len(ips) == 0 {
		return ips
	}
	firstV4 := ips[0].IP.To4() != nil
	var same, other []net.IPAddr
	for _, a := range ips {
		if (a.IP.To4() != nil) == firstV4 {
			same = append(same, a)
		} else {
			other = append(other, a)
		}
	}
	out := make([]net.IPAddr, 0, len(ips))
	for i := 0; i < len(same) || i < len(other); i++ {
		if i < len(same) {
			out = append(out, same[i])
		}
		if i < len(other) {
			out = append(out, other[i])
		}
	}
	return out
}

// checkRedirect yönlendirmenin her adımını 28.11'in kurallarından geçirir ve kimlik üstbilgilerini
// siler (Go aynı alana ve alt alan adına taşırdı).
func checkRedirect(fn any, req *http.Request, via []*http.Request) error {
	from := via[len(via)-1].URL
	if len(via) > maxRedirects {
		return messages.New(messages.H7710, fn, shown(via[0].URL))
	}
	to := req.URL
	if to.User != nil {
		return messages.New(messages.H7707, fn, shown(to))
	}
	if !allowedScheme(to) {
		return messages.New(messages.H7711, fn, shown(from), shown(to))
	}
	fromLoop, toLoop := from.Hostname() == loopback, to.Hostname() == loopback
	if fromLoop && !toLoop {
		return messages.New(messages.H7713, fn, shown(to))
	}
	if ip, ok := hostIP(to.Hostname()); ok && isLocal(ip) && !(fromLoop && toLoop) {
		return messages.New(messages.H7738, fn, shown(from), shown(to))
	}
	req.Header.Del("Authorization")
	req.Header.Del("Cookie")
	return nil
}

// requestError istemcinin hatasını kodlu iletiye çevirir: kuralların iletisi olduğu gibi geçer;
// sertifika H7714, süre H7716 (asılan evreyle; stage stageTotal ise zincirin tavanı), öbürleri H7715.
// Konak asılan isteğinkidir (*url.Error'ın adresi; yönlendirmede CDN'in konağı), yoksa u'nunki.
// İşletim sisteminin metni iletiye girmez (28.9 gibi).
func requestError(fn any, u *url.URL, err error, stage string) error {
	var coded *messages.Error
	if errors.As(err, &coded) {
		return coded
	}
	host := u.Host
	var ue *url.Error
	if errors.As(err, &ue) {
		if failed, perr := url.Parse(ue.URL); perr == nil && failed.Host != "" {
			host = failed.Host
		}
	}
	if stage == stageTotal {
		return messages.New(messages.H7716, fn, host, stageTotal)
	}
	var certErr *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &certErr) || errors.As(err, &unknown) || errors.As(err, &hostErr) || errors.As(err, &invalid) {
		return messages.New(messages.H7714, fn, host)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return messages.New(messages.H7716, fn, host, stage)
	}
	return messages.New(messages.H7715, fn, host)
}

// H7716'nın evreleri (28.10): iletinin stage seçicisinin değerleri, dilden bağımsız.
const (
	stageDNS     = "dns"
	stageConnect = "connect"
	stageTLS     = "tls"
	stageHeader  = "header"
	stageTotal   = "total" // zincirin tavanı (firstHeaderTimeout)
)

// tracker bir istek zincirinin o anki evresidir; süreyi kesmez, yalnız H7716'ya adını verir. Vekil
// bağlantısını da tutar: CONNECT'in cevabı gelince (OnProxyConnectResponse) son süresi kalkar. Go'nun
// kancası bağlantıyı vermez, ama dial'ın ve kancanın bağlamı isteğin bağlamından türer ve değerlerini
// taşır; izleyici oradan bulunur (trackerOf). Her fetch kendi taşıyıcısını kurar, iki istek aynı
// izleyiciyi paylaşmaz.
type tracker struct {
	mu    sync.Mutex
	stage string
	proxy net.Conn
}

type trackerKey struct{}

// withTracker bağlama izleyiciyi ve evreleri ilerleten httptrace kancalarını koyar.
func withTracker(ctx context.Context, t *tracker) context.Context {
	ctx = context.WithValue(ctx, trackerKey{}, t)
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		TLSHandshakeStart: func() { t.set(stageTLS) },
		WroteRequest:      func(httptrace.WroteRequestInfo) { t.set(stageHeader) },
	})
}

// trackerOf bağlamın izleyicisidir; izleyicisiz bağlamda (testin doğrudan istemcisi) boş bir izleyici.
func trackerOf(ctx context.Context) *tracker {
	if t, ok := ctx.Value(trackerKey{}).(*tracker); ok {
		return t
	}
	return &tracker{}
}

func (t *tracker) set(stage string) {
	t.mu.Lock()
	t.stage = stage
	t.mu.Unlock()
}

func (t *tracker) current() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stage
}

func (t *tracker) holdProxy(c net.Conn) {
	t.mu.Lock()
	t.proxy = c
	t.mu.Unlock()
}

// liftProxyDeadline vekil bağlantısının bağlanma son süresini kaldırır.
func (t *tracker) liftProxyDeadline() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.proxy != nil {
		t.proxy.SetDeadline(time.Time{})
		t.proxy = nil
	}
}
