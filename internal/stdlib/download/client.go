package download

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Süreler (28.10): bağlantı ve ilk üstbilgi için 30 saniye, iki okuma arası hareketsizlik için 60
// saniye. Testler kısaltır.
var (
	connectTimeout = 30 * time.Second
	headerTimeout  = 30 * time.Second
	idleTimeout    = 60 * time.Second
)

// maxRedirects 28.11'in yönlendirme sınırıdır.
const maxRedirects = 10

// resolver adları çözer; testler kendi çözümleyicisini koyar (evil.test → 127.0.0.1).
var resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
} = net.DefaultResolver

// testRoots yalnız _test.go'dan kurulur: httptest'in TLS sunucusunun kökü. TR'de ek kök sertifikanın
// yazımı yoktur (28.11).
var testRoots *x509.CertPool

// newClient bir indirmenin istemcisini kurar (28.11). İstemci HTTP/1.1'dir: DialContext verilen
// taşıyıcıda Go, ForceAttemptHTTP2 olmadan HTTP/2'yi açmaz; H7739'un ayırımı buna bağlıdır.
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
			d := &net.Dialer{Timeout: connectTimeout}
			if proxyHost != "" && addr == proxyHost {
				return d.DialContext(ctx, network, addr) // vekilin kuralı: yerel olabilir
			}
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if _, ok := hostIP(host); ok {
				// Adreste doğrudan yazılmış IP: yerelse yalnız ilk adımda gelir, yönlendirmeyi
				// CheckRedirect reddeder.
				return d.DialContext(ctx, network, addr)
			}
			ips, err := resolver.LookupIPAddr(ctx, host)
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
			// Denetlenen IP'ye bağlanılır: ad yeniden çözülmez (DNS yeniden bağlama).
			return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
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
// sertifika H7714, süre H7716, öbürleri H7715. İşletim sisteminin metni iletiye girmez (28.9 gibi).
func requestError(fn any, u *url.URL, err error) error {
	var coded *messages.Error
	if errors.As(err, &coded) {
		return coded
	}
	var certErr *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &certErr) || errors.As(err, &unknown) || errors.As(err, &hostErr) || errors.As(err, &invalid) {
		return messages.New(messages.H7714, fn, u.Host)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return messages.New(messages.H7716, fn, u.Host)
	}
	return messages.New(messages.H7715, fn, u.Host)
}
