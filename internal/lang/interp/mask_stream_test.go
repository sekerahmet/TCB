package interp

import (
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

// FuzzMaskStream 19.4a'nın sözünü sınar: rastgele metin, rastgele bölme, örtüşen gizli değerler;
// parçaların çıktılarının birleşimi birleşimin VM.Mask'ına eşittir. Gizli değerler metnin küçük
// abecesinden seçilir ki örtüşsünler; bölme bayt konumundadır, çok baytlı harfin ortası dahil.
func FuzzMaskStream(f *testing.F) {
	f.Add("anahtar: abcdef12 bitti", "abcdef12", "", uint64(11))
	f.Add("xabcdefgh", "abcdef", "abcdefgh", uint64(7))
	f.Add("QQabcd", "QQabcd", "abcdWXYZ", uint64(0))
	f.Add("aaaaaaaaaaaaa", "aaaaaa", "", uint64(0x0102030405))
	f.Add("şifre_ğüç şifre_ğüç", "şifre_ğüç", "re_ğüç şi", uint64(0x0301020101))
	f.Fuzz(func(t *testing.T, text, s1, s2 string, cuts uint64) {
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
