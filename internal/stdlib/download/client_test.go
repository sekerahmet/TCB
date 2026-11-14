package download

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
		if !messages.Has(requestError("indir()", &url.URL{Host: host}, err), messages.H7712) {
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
	if !messages.Has(requestError("indir()", u, err), messages.H7714) {
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
