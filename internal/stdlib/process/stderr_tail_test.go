package process

import (
	"strings"
	"testing"
)

func noMask(s string) string { return s }

// Tampon sınırlıdır: çok büyük çıktının yalnız sonu kalır, bütün çıktı tutulmaz (21.4).
func TestStderrTailIsBounded(t *testing.T) {
	tail := &stderrTail{longest: func() int { return 0 }}
	chunk := []byte(strings.Repeat("x", 1000))
	for range 10000 {
		if _, err := tail.Write(chunk); err != nil {
			t.Fatal(err)
		}
		if len(tail.buf) > 2*stderrTailLimit {
			t.Fatalf("tampon %d bayt, sınır %d", len(tail.buf), 2*stderrTailLimit)
		}
	}
	_, _ = tail.Write([]byte("son satır\n"))
	got := tail.text(noMask)
	if len(got) > stderrTailLimit || !strings.HasSuffix(got, "son satır") {
		t.Errorf("%d bayt, sonu %q", len(got), got[max(0, len(got)-20):])
	}
}

// Gizli değer sınırın neresine düşerse düşsün iletide kalanı görünmez: önce süzülür, sonra kesilir (19.4).
func TestStderrTailMasksBeforeCutting(t *testing.T) {
	const secret = "hf_cokgizlideger12345"
	mask := func(s string) string { return strings.ReplaceAll(s, secret, "***") }
	longest := func() int { return len(secret) }
	for _, chunkSize := range []int{1, 7, 4096, 1 << 16} {
		for shift := -len(secret) - 2; shift <= len(secret)+2; shift++ {
			for _, repeat := range []int{0, 1, 3} {
				tail := &stderrTail{longest: longest}
				before := strings.Repeat("a", 3*(stderrTailLimit+len(secret)))
				after := strings.Repeat("b", stderrTailLimit+shift) + strings.Repeat(secret+"c", repeat)
				text := before + secret + after
				for len(text) > 0 {
					n := min(chunkSize, len(text))
					_, _ = tail.Write([]byte(text[:n]))
					text = text[n:]
				}
				got := tail.text(mask)
				for n := 4; n < len(secret); n++ {
					if strings.Contains(got, secret[:n]) || strings.Contains(got, secret[len(secret)-n:]) {
						t.Fatalf("parça %d, kayma %d, tekrar %d: gizli değerin %d baytı göründü", chunkSize, shift, repeat, n)
					}
				}
			}
		}
	}
}

// Geçersiz UTF-8 ve satır sonu dışı denetim karakteri iletiye girmez; Türkçe harf ve satır sonu kalır (21.4).
func TestStderrTailPrintable(t *testing.T) {
	for _, d := range []struct{ in, want string }{
		{"hata\r\nsatır\r\n", "hata\nsatır"},
		{"\x1b[31mkırmızı\x1b[0m", "[31mkırmızı[0m"},
		{"a\x00b\x07c\x7fd", "abcd"},
		{"a\tb", "a b"},
		{"ilerleme\r%50", "ilerleme\n%50"},
		{"yarım \xff bayt", "yarım � bayt"},
		{"\xc2\x9b31m", "31m"},
		{"\n\n", ""},
	} {
		tail := &stderrTail{longest: func() int { return 0 }}
		_, _ = tail.Write([]byte(d.in))
		if got := tail.text(noMask); got != d.want {
			t.Errorf("%q: gelen %q, beklenen %q", d.in, got, d.want)
		}
	}
}

// Tamponun düşen başı bir adres bölgesinin "://"'sunu götürünce kalan kimlik kısmı bağlamsız kalırdı
// (19.4): pencerenin '@' taşıyan başı ilk bölge sınırına kadar atılır, '@'siz baş kalır. A en az 4090
// bayttır: toplam 8192'yi aşınca tampon son 4096 bayta (B'ye) iner; daha kısa A'da kesim kurulmazdı.
func TestStderrTailDropsACutAddress(t *testing.T) {
	for _, c := range []struct{ start, want string }{
		{"u:ab@h kalan", " kalan"},
		{"uzunsözcük kalan", "uzunsözcük kalan"},
	} {
		tail := &stderrTail{longest: func() int { return 0 }}
		b := c.start + strings.Repeat("x", stderrTailLimit-len(c.start))
		_, _ = tail.Write([]byte(strings.Repeat("a", 4090) + "http://" + b))
		if !tail.cut {
			t.Fatalf("%q: tampon kesilmedi", c.start)
		}
		if got := tail.text(noMask); !strings.HasPrefix(got, c.want) || strings.Contains(got, "ab@") {
			t.Errorf("%q: gelen %q…", c.start, got[:min(len(got), 20)])
		}
	}
}

// Kesme çok baytlı harfi ortasından bölmez.
func TestStderrTailKeepsWholeRunes(t *testing.T) {
	tail := &stderrTail{longest: func() int { return 0 }}
	_, _ = tail.Write([]byte("x" + strings.Repeat("ş", stderrTailLimit)))
	if got := tail.text(noMask); strings.ContainsRune(got, '�') {
		t.Errorf("harf bölündü: %q", got[:8])
	}
}
