package download

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestClientOnlyHTTPS: yalnız https; http yalnız 127.0.0.1'de; öbür şemalar H7705 (28.11).
func TestClientOnlyHTTPS(t *testing.T) {
	for raw, want := range map[string]messages.Code{
		"http://huggingface.co/a":   messages.H7705,
		"ftp://example.com/a":       messages.H7705,
		"file:///etc/passwd":        messages.H7708,
		"https://":                  messages.H7708,
		"::bozuk":                   messages.H7708,
		"https://huggingface.co/a":  0,
		"http://127.0.0.1:8080/a.b": 0,
	} {
		_, err := parseTarget("indir()", raw)
		if want == 0 {
			if err != nil {
				t.Errorf("%s kabul edilmeliydi: %v", raw, err)
			}
			continue
		}
		if !messages.Has(err, want) {
			t.Errorf("%s: %s bekleniyordu, gelen %v", raw, want, err)
		}
	}
}

// TestClientLoopbackHTTPRule: 127.0.0.1'in öbür yazımları http ile H7706'dır (28.2'nin tek yazımı).
func TestClientLoopbackHTTPRule(t *testing.T) {
	for _, raw := range []string{"http://localhost:8080/a", "http://LOCALHOST/a", "http://[::1]:80/a", "http://127.0.0.2/a"} {
		if _, err := parseTarget("indir()", raw); !messages.Has(err, messages.H7706) {
			t.Errorf("%s: H7706 bekleniyordu, gelen %v", raw, err)
		}
	}
}

// TestClientRefusesUserInfo: hedef adreste kimlik kısmı H7707; ileti kimliği yazmaz (28.11).
func TestClientRefusesUserInfo(t *testing.T) {
	_, err := parseTarget("indir()", "https://sinama:Parola2026x@huggingface.co/a?X-Amz-Signature=imza123")
	if !messages.Has(err, messages.H7707) {
		t.Fatalf("H7707 bekleniyordu: %v", err)
	}
	for _, secret := range []string{"Parola2026x", "sinama", "imza123"} {
		if text := err.Error(); strings.Contains(text, secret) {
			t.Errorf("ileti %q taşımamalı: %s", secret, text)
		}
	}
}

// TestClientLocalAddressList: 28.11'in yerel adres listesi; IPv4'ü taşıyan IPv6 açılarak denetlenir.
func TestClientLocalAddressList(t *testing.T) {
	for s, want := range map[string]bool{
		"127.0.0.1": true, "127.9.9.9": true, "::1": true, "10.0.0.1": true, "172.16.0.1": true,
		"192.168.1.1": true, "fd00::1": true, "169.254.1.1": true, "fe80::1": true, "100.64.0.1": true,
		"0.0.0.0": true, "::": true, "::ffff:127.0.0.1": true, "::ffff:10.0.0.1": true,
		"8.8.8.8": false, "100.128.0.1": false, "2606:4700::1111": false, "198.18.0.1": false,
	} {
		if got := isLocal(netip.MustParseAddr(s)); got != want {
			t.Errorf("isLocal(%s) = %v, beklenen %v", s, got, want)
		}
	}
}
