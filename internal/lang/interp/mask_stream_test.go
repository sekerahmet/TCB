package interp

import (
	"slices"
	"strings"
	"testing"
)

// streamOf parçaları bir akıştan geçirir, bütün çıktıların birleşimini verir.
func streamOf(v *VM, pieces ...string) string {
	m := v.NewMaskStream()
	var b strings.Builder
	for _, p := range pieces {
		out, _ := m.Step(p)
		b.WriteString(out)
	}
	b.WriteString(m.Finish())
	return b.String()
}

// TestMaskStreamFollowsTheBatchMask: 19.4a'nın sözü DENETİM 373'ün karşı örneklerinde ve tek başına
// aaaaaa'da tutar; iki parçaya bölünen gizli değer maskelenir.
func TestMaskStreamFollowsTheBatchMask(t *testing.T) {
	for _, c := range []struct {
		secrets []string
		pieces  []string
		want    string
	}{
		{[]string{"abcdef12"}, []string{"anahtar: ab", "cdef12", " bitti"}, "anahtar: *** bitti"},
		{[]string{"abcdef", "abcdefgh"}, []string{"xabcdef", "gh"}, "x***"},
		{[]string{"abcdef", "abcdefgh"}, []string{"xabcdef", "g"}, "x***g"},
		{[]string{"QQabcd", "abcdWXYZ"}, []string{"QQabcd"}, "***"},
		{[]string{"QQabcd", "abcdWXYZ"}, []string{"QQab", "cdWXYZ"}, "***WXYZ"},
		{[]string{"aaaaaa"}, []string{"aaa", "aaaaaaa", "aa"}, "******"},
		{[]string{"aaaaaa"}, []string{"aaaaa", "aaaaa"}, "***aaaa"},
		{[]string{"şifre_ğüç"}, []string{"x şif", "re_ğ", "\xc3", "\xbcç y"}, "x *** y"},
		{nil, []string{"ab", "cd"}, "abcd"},
	} {
		v := New()
		for _, s := range c.secrets {
			if !v.AddSecret(s) {
				t.Fatalf("%q kaydedilmedi", s)
			}
		}
		got := streamOf(v, c.pieces...)
		if want := v.Mask(strings.Join(c.pieces, "")); got != want {
			t.Errorf("%q %q: akış %q, toplu maske %q", c.secrets, c.pieces, got, want)
		}
		if got != c.want {
			t.Errorf("%q %q: %q, beklenen %q", c.secrets, c.pieces, got, c.want)
		}
	}
}

// TestMaskStreamHoldsOnlyAnIncompleteSecret: kararı kesinleşen baş hemen verilir; tutulan, en uzun
// gizli değerden kısadır.
func TestMaskStreamHoldsOnlyAnIncompleteSecret(t *testing.T) {
	v := New()
	v.AddSecret("abcdef12")
	m := v.NewMaskStream()
	if out, ok := m.Step("merhaba ab"); !ok || out != "merhaba " {
		t.Fatalf("Step = %q %v, beklenen \"merhaba \"", out, ok)
	}
	if out, _ := m.Step("cdeX"); out != "abcdeX" {
		t.Fatalf("tamamlanmayan baş verilmeli: %q", out)
	}
	if out, _ := m.Step("düz metin"); out != "düz metin" {
		t.Fatalf("gizli değerin başı olmayan metin tutulmamalı: %q", out)
	}
}

// TestMaskStreamAfterFinish: bittikten sonra Step ok false verir (çağıran H7634'ü yazar); ikinci
// Finish boş metin verir (19.4a susuyor; tutulan yoktur).
func TestMaskStreamAfterFinish(t *testing.T) {
	v := New()
	v.AddSecret("abcdef12")
	m := v.NewMaskStream()
	m.Step("x abc")
	if got := m.Finish(); got != "abc" {
		t.Fatalf("Finish = %q, beklenen \"abc\"", got)
	}
	if out, ok := m.Step("y"); ok || out != "" {
		t.Fatalf("bittikten sonra Step = %q %v", out, ok)
	}
	if got := m.Finish(); got != "" {
		t.Fatalf("ikinci Finish = %q, beklenen boş", got)
	}
}

// TestMaskStreamHoldsAnOpenAddress: gizli değer kayıtlı değilken de sınırı gelmemiş adres bölgesi
// "://"'dan başlayarak tutulur (19.4a, 19.4'ün bağlamlı maskesi); sondaki ":" ve ":/" da tutulur.
// İki parçaya bölünen kısa parola örtülür.
func TestMaskStreamHoldsAnOpenAddress(t *testing.T) {
	m := New().NewMaskStream()
	for _, step := range []struct{ piece, out string }{
		{"bak: https://", "bak: https"},
		{"ornek.com", ""},
		{" sonra", "://ornek.com sonra"},
		{" http://u:", " http"},
		{"ab@h", ""},
		{"/yol", "://u:***@h/yol"},
		{" son:", " son"},
		{"/", ""},
		{"/x:ab@y", ""},
	} {
		if out, ok := m.Step(step.piece); !ok || out != step.out {
			t.Fatalf("Step(%q) = %q %v, beklenen %q", step.piece, out, ok, step.out)
		}
	}
	if got := m.Finish(); got != "://x:***@y" {
		t.Fatalf("Finish = %q, beklenen \"://x:***@y\"", got)
	}
}

// FuzzMaskStream 19.4a'nın sözünü sınar: rastgele metin, rastgele bölme, örtüşen gizli değerler;
// parçaların çıktılarının birleşimi birleşimin VM.Mask'ına eşittir. Gizli değerler metnin küçük
// abecesinden seçilir ki örtüşsünler; bölme bayt konumundadır, çok baytlı harfin ortası dahil.
func FuzzMaskStream(f *testing.F) {
	f.Add("anahtar: abcdef12 bitti", "abcdef12", "", uint64(11))
	f.Add("xabcdefgh", "abcdef", "abcdefgh", uint64(7))
	f.Add("QQabcd", "QQabcd", "abcdWXYZ", uint64(0))
	f.Add("aaaaaaaaaaaaa", "aaaaaa", "", uint64(0x0102030405))
	f.Add("şifre_ğüç şifre_ğüç", "şifre_ğüç", "re_ğüç şi", uint64(0x0301020101))
	f.Add("bak: http://u:ab@h son", "", "", uint64(0x0203010405))
	f.Add("x https://ali:ab@h y", "https://ali", "", uint64(0x0102030102))
	f.Fuzz(func(t *testing.T, text, s1, s2 string, cuts uint64) {
		ClearRegisteredSecrets() // süreç boyu küme girdiden girdiye büyümesin
		v := New()
		v.AddSecret(s1)
		v.AddSecret(s2)
		var pieces []string
		rest := text
		for len(rest) > 0 {
			n := int(cuts%8) + 1
			cuts = cuts/8 | cuts<<61
			n = min(n, len(rest))
			pieces = append(pieces, rest[:n])
			rest = rest[n:]
		}
		if got, want := streamOf(v, pieces...), v.Mask(text); got != want {
			t.Fatalf("gizli %q %q, parçalar %q: akış %q, toplu maske %q", s1, s2, pieces, got, want)
		}
	})
}

// TestMaskJoined: unutulan maskenin birleşimi (28.7). Tutulan yoksa çıktı Mask'ınkidir; bölünen gizli
// değerin yeni olaydaki kısmı *** olur; kendi maskesinin örttüğü hiçbir zaman açılmaz; sınırı gelmemiş
// bölge en çok en uzun gizli değer (kaçışlı biçimi dahil) + pad bayt tutulur.
func TestMaskJoined(t *testing.T) {
	v := New()
	v.AddSecret("gizli-deger-123")
	cases := []struct {
		held, text, out, next string
		caught                bool
	}{
		{"", "a gizli-deger-123 b", "a *** b", "", false},
		{"", "son gizli-de", "son gizli-de", "gizli-de", false},
		{"gizli-de", "ger-123 x", "*** x", "", true},
		{"ab", "gizli-deger-123", "***", "", false},
		{"://ali:parola", "123@sunucu/yol", "***@sunucu/yol", "", true},
		{"", "https://" + strings.Repeat("a", 15+10-3), "https://" + strings.Repeat("a", 22), "://" + strings.Repeat("a", 22), false},
		{"", "https://" + strings.Repeat("a", 15+10-2), "https***", "", true},
		{"://" + strings.Repeat("a", 22), "bb", "***", "", true},
	}
	for _, c := range cases {
		out, next, caught := v.MaskJoined(c.held, c.text, 10)
		if out != c.out || next != c.next || caught != c.caught {
			t.Errorf("MaskJoined(%q, %q) = %q, %q, %v; beklenen %q, %q, %v", c.held, c.text, out, next, caught, c.out, c.next, c.caught)
		}
	}
	w := New()
	w.AddSecret("çok-gizli\x01")
	long := len(w.mask.Load().longestFirst()[0])
	if long <= len("çok-gizli\x01") || long != w.LongestSecret() {
		t.Fatalf("en uzun biçim kaçışlı olmalı ve LongestSecret'le aynı: %d, %d", long, w.LongestSecret())
	}
}

// TestMaskCutSkipKeepsTheDecision: maskCut'ın ilk bayt atlaması (valueMask.first) kararı değiştirmez:
// atlamalı ve atlamasız (first'ün her baytı açık) maske aynı aralıkları ve aynı kesimi verir. Gizli
// değerin ilk baytı metinde başka yerde de geçer ("g") ya da hiç geçmez ("Z"); akışın adımında
// (atEnd false) eksik baş da aynı yerde kesilir (DENETİM 791 N2). first her değerin (kaçışlı biçimler
// dahil) ilk baytını içerir.
func TestMaskCutSkipKeepsTheDecision(t *testing.T) {
	v := New()
	v.AddSecret("gizli-deger-123")
	v.AddSecret("Zq9-anahtar-77")
	v.AddSecret(`çok"gizli`)
	m := v.mask.Load()
	for _, g := range m.longestFirst() {
		if !m.first[g[0]] {
			t.Errorf("first %q'nin ilk baytını (%q) içermeli", g, g[0])
		}
	}
	all := *m
	for i := range all.first {
		all.first[i] = true
	}
	cases := []struct {
		text   string
		masked bool // metinde tam bir gizli değer var
	}{
		{"gg ağ gizli-deger-123 gizli gizli-de", true},
		{"gece gündüz gizli değil", false},
		{"gizli-deger-123gizli-deger-123", true},
		{"a Zq9-anahtar-77 b", true},
		{"son Zq9-ana", false},
		{"hiçbir şey yok", false},
		{`"çok\"gizli" ve çok`, true},
	}
	for _, c := range cases {
		for _, atEnd := range []bool{true, false} {
			skip, skipCut := maskCut(m, c.text, nil, -1, atEnd)
			full, fullCut := maskCut(&all, c.text, nil, -1, atEnd)
			if !slices.Equal(skip, full) || skipCut != fullCut {
				t.Errorf("maskCut(%q, atEnd %v): atlamalı %v %d, atlamasız %v %d", c.text, atEnd, skip, skipCut, full, fullCut)
			}
			if (len(skip) > 0) != c.masked {
				t.Errorf("maskCut(%q, atEnd %v) aralıkları %v; gizli değer var: %v", c.text, atEnd, skip, c.masked)
			}
		}
	}
}
