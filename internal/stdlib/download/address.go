package download

import (
	"net/netip"
	"net/url"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// loopback, adresi http ile kabul edilen tek yerel ana makinedir (28.11; 28.2'nin TR_host:'unun tek
// yazımı).
const loopback = "127.0.0.1"

// localPrefixes 28.11'in yerel adres listesidir: döngü, özel, bağlantı-yerel, paylaşılan adres alanı
// (RFC 6598) ve belirsiz adres. IPv4'ü taşıyan IPv6 adresi denetimden önce açılır (isLocal).
var localPrefixes = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("0.0.0.0/32"),
	netip.MustParsePrefix("::/128"),
}

// isLocal ip'nin 28.11'in yerel adreslerinden biri olup olmadığını söyler.
func isLocal(ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, p := range localPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// hostIP ana makine bir IP olarak yazılmışsa onu verir.
func hostIP(host string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return ip, err == nil
}

// shown adresin iletilerde yazılan biçimidir: şema, ana makine, kapı ve yol; kimlik kısmı, sorgu ve
// parça yazılmaz (28.11; imzalı adresin sorgusu bir süre geçerli bir kimliktir).
func shown(u *url.URL) string {
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

// allowedScheme adresin şemasının 28.11'e uyup uymadığını söyler: https, ya da ana makinesi tam
// olarak 127.0.0.1 olan http.
func allowedScheme(u *url.URL) bool {
	return u.Scheme == "https" || (u.Scheme == "http" && u.Hostname() == loopback)
}

// parseTarget indirmenin adresini ayrıştırır ve denetler (28.11): ayrışmayan ya da ana makinesiz
// adres H7708 (adres yazılmaz: ayrışmayan metinden kimliği ayıklamak güvenilmez), kimlik kısmı
// H7707, 127.0.0.1'in başka yazımıyla http H7706, öbür şemalar H7705.
func parseTarget(fn any, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return nil, messages.New(messages.H7708, fn)
	}
	if u.User != nil {
		return nil, messages.New(messages.H7707, fn, shown(u))
	}
	if allowedScheme(u) {
		return u, nil
	}
	if u.Scheme == "http" && looksLoopback(u.Hostname()) {
		return nil, messages.New(messages.H7706, fn, u.Host)
	}
	return nil, messages.New(messages.H7705, fn, shown(u))
}

// looksLoopback bu makineyi gösteren öbür yazımlardır: localhost, ::1 ve 127.0.0.0/8'in öbür
// adresleri.
func looksLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, ok := hostIP(host)
	return ok && ip.Unmap().IsLoopback()
}

// parseProxy vekilin adresini ayrıştırır: şeması http ya da https'tir (H7727); kimlik kısmı
// vekilin kimliğidir ve iletilerde yazılmaz.
func parseProxy(fn any, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, messages.New(messages.H7708, fn)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, messages.New(messages.H7727, fn, shown(u))
	}
	return u, nil
}
