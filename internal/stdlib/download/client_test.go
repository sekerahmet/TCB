package download

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// fakeResolver evil.test gibi adları verilen IP'lere çözer; "fe80::1%eth0" bölgeli adres verir.
type fakeResolver map[string]string

func (f fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	ip, ok := f[host]
	if !ok {
		return nil, &net.DNSError{Err: "not found", Name: host, IsNotFound: true}
	}
	addr, zone, _ := strings.Cut(ip, "%")
	return []net.IPAddr{{IP: net.ParseIP(addr), Zone: zone}}, nil
}

func useResolver(t *testing.T, r fakeResolver) {
	old := resolver
	resolver = r
	t.Cleanup(func() { resolver = old })
}

// TestClientRefusesLocalAddresses: yerel bir adrese çözülen ad her adımda reddedilir (H7712);
// yönlendirme yerel IP'ye gidemez (H7738), 127.0.0.1'den 127.0.0.1'e giden dışında (28.11). Bölgeli
// (zone) IPv6 adresi bölgesiz adresiyle denetlenir (DENETİM 759 Ş1).
func TestClientRefusesLocalAddresses(t *testing.T) {
	useResolver(t, fakeResolver{"evil.test": "127.0.0.1", "ten.test": "10.0.0.1", "mapped.test": "::ffff:127.0.0.1", "cgnat.test": "100.64.0.1", "v6.test": "::1", "zone.test": "fe80::1%eth0"})
	for _, host := range []string{"evil.test", "ten.test", "mapped.test", "cgnat.test", "v6.test", "zone.test"} {
		c := newClient("indir()", nil)
		_, err := c.Get("https://" + host + "/model.bin")
		if !messages.Has(requestError("indir()", &url.URL{Host: host}, err, ""), messages.H7712) {
			t.Errorf("%s: H7712 bekleniyordu, gelen %v", host, err)
		}
	}
	from, _ := url.Parse("https://huggingface.co/a")
	for _, target := range []string{"https://10.0.0.5/a", "https://[::1]/a", "https://100.64.0.1/a", "https://[::ffff:127.0.0.1]/a", "https://[fe80::1%25eth0]/a", "https://[::1%25lo]/a"} {
		to, _ := url.Parse(target)
		err := checkRedirect("indir()", &http.Request{URL: to, Header: http.Header{}}, []*http.Request{{URL: from}})
		if !messages.Has(err, messages.H7738) {
			t.Errorf("%s'e yönlendirme: H7738 bekleniyordu, gelen %v", target, err)
		}
	}
	loop, _ := url.Parse("http://127.0.0.1:9/a")
	same, _ := url.Parse("http://127.0.0.1:9/b")
	if err := checkRedirect("indir()", &http.Request{URL: same, Header: http.Header{}}, []*http.Request{{URL: loop}}); err != nil {
		t.Errorf("127.0.0.1'den 127.0.0.1'e yönlendirme kabul edilmeliydi: %v", err)
	}
	out, _ := url.Parse("https://huggingface.co/a")
	if err := checkRedirect("indir()", &http.Request{URL: out, Header: http.Header{}}, []*http.Request{{URL: loop}}); !messages.Has(err, messages.H7713) {
		t.Errorf("127.0.0.1'den dışarı: H7713 bekleniyordu, gelen %v", err)
	}
}

// TestClientRedirectLimit: 10 yönlendirme geçer, 11. H7710'dur; https'ten http'ye H7711; kimlikli
// yönlendirme H7707.
func TestClientRedirectLimit(t *testing.T) {
	first, _ := url.Parse("https://huggingface.co/a")
	next, _ := url.Parse("https://cdn.example.com/b")
	via := []*http.Request{{URL: first}}
	for len(via) < 10 {
		via = append(via, &http.Request{URL: next})
	}
	if err := checkRedirect("indir()", &http.Request{URL: next, Header: http.Header{}}, via); err != nil {
		t.Fatalf("10. yönlendirme geçmeliydi: %v", err)
	}
	via = append(via, &http.Request{URL: next})
	if err := checkRedirect("indir()", &http.Request{URL: next, Header: http.Header{}}, via); !messages.Has(err, messages.H7710) {
		t.Errorf("11. yönlendirme H7710 olmalı: %v", err)
	}
	plain, _ := url.Parse("http://cdn.example.com/b")
	if err := checkRedirect("indir()", &http.Request{URL: plain, Header: http.Header{}}, via[:1]); !messages.Has(err, messages.H7711) {
		t.Errorf("http'ye yönlendirme H7711 olmalı: %v", err)
	}
	user, _ := url.Parse("https://ad:p@cdn.example.com/b")
	if err := checkRedirect("indir()", &http.Request{URL: user, Header: http.Header{}}, via[:1]); !messages.Has(err, messages.H7707) {
		t.Errorf("kimlikli yönlendirme H7707 olmalı: %v", err)
	}
}

// TestClientRedirectDropsCredentials: Authorization ve Cookie her yönlendirmede, aynı alana da, silinir.
func TestClientRedirectDropsCredentials(t *testing.T) {
	from, _ := url.Parse("https://huggingface.co/a")
	to, _ := url.Parse("https://sub.huggingface.co/b")
	req := &http.Request{URL: to, Header: http.Header{"Authorization": {"Bearer x"}, "Cookie": {"a=b"}, "Range": {"bytes=5-"}}}
	if err := checkRedirect("indir()", req, []*http.Request{{URL: from}}); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" || req.Header.Get("Range") == "" {
		t.Errorf("kimlik üstbilgileri silinmeli, öbürleri kalmalı: %v", req.Header)
	}
}

// TestClientProxyIsExplicit: vekil ortamdan okunmaz; verilen vekil https isteğinde kullanılır.
func TestClientProxyIsExplicit(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://ortam.example:3128")
	t.Setenv("HTTP_PROXY", "http://ortam.example:3128")
	target, _ := url.Parse("https://huggingface.co/a")
	tr := newClient("indir()", nil).Transport.(*http.Transport)
	if u, _ := tr.Proxy(&http.Request{URL: target}); u != nil {
		t.Errorf("vekil verilmedi, ortamdaki %v kullanıldı", u)
	}
	p, _ := parseProxy("indir()", "http://sinama:Parola2026x@vekil.example:8080")
	tr = newClient("indir()", p).Transport.(*http.Transport)
	if u, _ := tr.Proxy(&http.Request{URL: target}); u == nil || u.Host != "vekil.example:8080" {
		t.Errorf("verilen vekil kullanılmalı: %v", u)
	}
	if _, err := parseProxy("indir()", "socks5://vekil.example:1080"); !messages.Has(err, messages.H7727) {
		t.Errorf("socks5 vekil H7727 olmalı: %v", err)
	}
}

// TestClientLoopbackSkipsProxy: 127.0.0.1'e giden istek vekil verilse de vekilden geçmez.
func TestClientLoopbackSkipsProxy(t *testing.T) {
	p, _ := parseProxy("indir()", "http://vekil.example:8080")
	tr := newClient("indir()", p).Transport.(*http.Transport)
	loop, _ := url.Parse("http://127.0.0.1:8000/a")
	if u, _ := tr.Proxy(&http.Request{URL: loop}); u != nil {
		t.Errorf("127.0.0.1 vekilden geçmemeli: %v", u)
	}
}

// TestClientVerifiesCertificates: sertifika doğrulanır (H7714); sınama kökü verilince geçer.
func TestClientVerifiesCertificates(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_, err := newClient("indir()", nil).Get(srv.URL)
	if !messages.Has(requestError("indir()", u, err, ""), messages.H7714) {
		t.Fatalf("doğrulanmayan sertifika H7714 olmalı: %v", err)
	}
	testRoots = srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	defer func() { testRoots = nil }()
	resp, err := newClient("indir()", nil).Get(srv.URL)
	if err != nil {
		t.Fatalf("sınama köküyle geçmeli: %v", err)
	}
	resp.Body.Close()
}

// TestClientIdentityEncodingOnly: istek Accept-Encoding: identity gönderir; sıkıştırılmış gövde H7718.
func TestClientIdentityEncodingOnly(t *testing.T) {
	var gotEncoding string
	s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		w.Write([]byte("x"))
	})
	target := slash(t.TempDir()) + "/a.bin"
	wantCode(t, dl(s.URL+"/a.bin", target, "TR_none", "TR_none"), messages.H7718)
	if gotEncoding != "identity" {
		t.Errorf("Accept-Encoding = %q, beklenen identity", gotEncoding)
	}
}

// TestClientMessagesHaveNoQuery: adres taşıyan iletiler kimlik kısmını, parolayı ve imzalı sorgunun
// değerini yazmaz (28.11; DENETİM 643 Ş3 (a): @ ve ?'ye değil, sırrın değerine bakılır).
func TestClientMessagesHaveNoQuery(t *testing.T) {
	const secret, user, sig = "Parola2026x", "sinama", "imza0123456789"
	s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/yonlen":
			http.Redirect(w, r, "http://huggingface.co/x?X-Amz-Signature="+sig, http.StatusFound)
		default:
			http.Error(w, "yok", http.StatusNotFound)
		}
	})
	dir := slash(t.TempDir())
	var texts []string
	for _, code := range []string{
		dl("https://"+user+":"+secret+"@huggingface.co/a?X-Amz-Signature="+sig, dir+"/a", "TR_none", "TR_none"),
		dl(s.URL+"/yonlen?X-Amz-Signature="+sig, dir+"/b", "TR_none", "TR_none"),
		dl(s.URL+"/yok?X-Amz-Signature="+sig, dir+"/c", "TR_none", "TR_none"),
		dl("https://huggingface.co/a", dir+"/d", "TR_none", "TR_none", "TR_proxy: "+q("socks5://"+user+":"+secret+"@vekil:1")),
	} {
		_, err := run(t, code)
		if err == nil {
			t.Fatalf("hata bekleniyordu: %s", code)
		}
		texts = append(texts, err.Error())
	}
	for _, text := range texts {
		for _, s := range []string{secret, user, sig} {
			if strings.Contains(text, s) {
				t.Errorf("ileti %q taşımamalı: %s", s, text)
			}
		}
	}
}

// multiResolver bir adı sırasıyla birden çok IP'ye çözer.
type multiResolver map[string][]string

func (f multiResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	var out []net.IPAddr
	for _, s := range f[host] {
		out = append(out, net.IPAddr{IP: net.ParseIP(s)})
	}
	if len(out) == 0 {
		return nil, &net.DNSError{Err: "not found", Name: host, IsNotFound: true}
	}
	return out, nil
}

// attempt dialAddr'a gelen bir denemedir: adres ve aldığı süre.
type attempt struct {
	addr  string
	share time.Duration
}

// blackHoleDial her denemeyi kaydeder; real'deki adrese gerçek sunucuya bağlanır, öbürlerinde son
// süreye kadar bekleyip süre hatası verir (SYN'e cevap vermeyen adres).
func blackHoleDial(t *testing.T, real map[string]string) *[]attempt {
	var mu sync.Mutex
	got := &[]attempt{}
	old := dialAddr
	dialAddr = func(ctx context.Context, network, addr string, deadline time.Time) (net.Conn, error) {
		mu.Lock()
		*got = append(*got, attempt{addr, time.Until(deadline)})
		mu.Unlock()
		if to, ok := real[addr]; ok {
			return old(ctx, network, to, deadline)
		}
		select {
		case <-time.After(time.Until(deadline)):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, &net.OpError{Op: "dial", Net: network, Err: os.ErrDeadlineExceeded}
	}
	t.Cleanup(func() { dialAddr = old })
	return got
}

// TestClientTriesTheNextAddress: ilk adres cevap vermezse sıradaki adrese (öbür aileye) geçilir ve
// indirme biter (28.10).
func TestClientTriesTheNextAddress(t *testing.T) {
	shortTimes(t, 600*time.Millisecond, time.Second, time.Second, 10*time.Second)
	body := []byte("ikinci adres")
	s := tlsServer(t, "tcp", "127.0.0.1:0", func(w http.ResponseWriter, r *http.Request) { w.Write(body) }, nil)
	useResolver(t, nil)
	resolver = multiResolver{"example.com": {"2001:db8::1", "192.0.2.2"}}
	got := blackHoleDial(t, map[string]string{"192.0.2.2:443": strings.TrimPrefix(s.URL, "https://")})
	dir := slash(t.TempDir())
	mustRun(t, dl("https://example.com/f", dir+"/a", q(sum(body)), strconv.Itoa(len(body))))
	if len(*got) != 2 || (*got)[0].addr != "[2001:db8::1]:443" || (*got)[1].addr != "192.0.2.2:443" {
		t.Errorf("önce IPv6, sonra IPv4 denenmeli: %v", *got)
	}
}

// TestClientInterleavesAddressFamilies: adresler aileleri dönüşümlü dizilir, ilk adresin ailesinden
// başlanır; her ailenin sırası korunur.
func TestClientInterleavesAddressFamilies(t *testing.T) {
	ips := func(ss ...string) []net.IPAddr {
		var out []net.IPAddr
		for _, s := range ss {
			out = append(out, net.IPAddr{IP: net.ParseIP(s)})
		}
		return out
	}
	str := func(a []net.IPAddr) string {
		var out []string
		for _, x := range a {
			out = append(out, x.IP.String())
		}
		return strings.Join(out, " ")
	}
	cases := []struct{ in, want []net.IPAddr }{
		{ips("2001:db8::1", "2001:db8::2", "192.0.2.1", "192.0.2.2", "192.0.2.3"), ips("2001:db8::1", "192.0.2.1", "2001:db8::2", "192.0.2.2", "192.0.2.3")},
		{ips("192.0.2.1", "2001:db8::1", "192.0.2.2"), ips("192.0.2.1", "2001:db8::1", "192.0.2.2")},
		{ips("192.0.2.1"), ips("192.0.2.1")},
		{nil, nil},
	}
	for _, c := range cases {
		if got := interleave(c.in); str(got) != str(c.want) {
			t.Errorf("interleave(%s) = %s; beklenen %s", str(c.in), str(got), str(c.want))
		}
	}
}

// TestClientConnectTimeIsShared: sekiz cevap vermeyen adreste bağlanmanın süresi adreslere
// paylaştırılır: hepsi denenir, toplam bağlanma süresinin içinde kalır, hata H7716 connect'tir.
func TestClientConnectTimeIsShared(t *testing.T) {
	shortTimes(t, 400*time.Millisecond, time.Second, time.Second, 10*time.Second)
	useResolver(t, nil)
	resolver = multiResolver{"example.com": {"2001:db8::1", "2001:db8::2", "2001:db8::3", "2001:db8::4", "192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4"}}
	got := blackHoleDial(t, nil)
	start := time.Now()
	err := wantCode(t, dl("https://example.com/f", slash(t.TempDir())+"/a", "TR_none", "TR_none"), messages.H7716)
	d := time.Since(start)
	if _, stage, _ := timeoutArgs(err); stage != stageConnect {
		t.Errorf("evre %q, beklenen connect", stage)
	}
	if len(*got) != 8 {
		t.Fatalf("sekiz adresin hepsi denenmeli: %v", *got)
	}
	if (*got)[0].share > 100*time.Millisecond || (*got)[1].addr != "192.0.2.1:443" {
		t.Errorf("ilk deneme süreden payını (400 ms / 8) almalı, ikinci deneme IPv4 olmalı: %v", *got)
	}
	if d > 900*time.Millisecond {
		t.Errorf("toplam bağlanma süresinin içinde kalmalı (400 ms): %v", d)
	}
}

// TestClientProxyDeadlineIsLifted: vekil bağlantısının son süresi CONNECT'in cevabında kalkar;
// üstbilgisi vekilin bağlanma süresinden sonra gelen indirme biter.
func TestClientProxyDeadlineIsLifted(t *testing.T) {
	shortTimes(t, 300*time.Millisecond, 3*time.Second, time.Second, 10*time.Second)
	body := []byte("tünel")
	s := tlsServer(t, "tcp6", "[::1]:0", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(700 * time.Millisecond)
		w.Write(body)
	}, nil)
	dir := slash(t.TempDir())
	mustRun(t, dl(s.URL+"/f", dir+"/a", q(sum(body)), strconv.Itoa(len(body)), "TR_proxy: "+q(tunnelProxy(t))))
	if got, _ := os.ReadFile(dir + "/a"); string(got) != string(body) {
		t.Errorf("tünelden tam inmeli: %q", got)
	}
}
