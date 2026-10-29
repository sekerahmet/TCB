package testrunner

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var identityExceptionWordRe = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)

// identityExceptions tablosunda dil tanımından ya da durum belgesinden çıkmış ya da artık kod
// listesine girmiş bayat girdi kalmaz.
func TestIdentityExceptionsAreNotStale(t *testing.T) {
	var b strings.Builder
	for _, text := range languageSpec(t) {
		b.WriteString(text)
		b.WriteByte('\n')
	}
	b.WriteString(statusDoc(t))
	for _, p := range staleIdentityExceptions(identityExceptions, b.String()) {
		t.Error(p)
	}
}

// staleIdentityExceptions identityExceptions tablosundaki bayat girdileri sıralı olarak verir:
// sözcük metinde hiç geçmiyorsa ya da artık geçerli bir kimlikse (identityWord) bayattır.
func staleIdentityExceptions(exceptions map[string]string, text string) []string {
	seen := map[string]bool{}
	for _, w := range identityExceptionWordRe.FindAllString(text, -1) {
		seen[w] = true
	}
	var keys []string
	for w := range exceptions {
		keys = append(keys, w)
	}
	sort.Strings(keys)
	var stale []string
	for _, w := range keys {
		switch {
		case !seen[w]:
			stale = append(stale, fmt.Sprintf("%s: bayat: dil tanımında yok, silin", w))
		case identityWord(w):
			stale = append(stale, fmt.Sprintf("%s: bayat: artık kod, silin", w))
		}
	}
	return stale
}

// Bayat kimlik istisnası denetimi sahte girdiyle sınanır: metinde geçen yeni ad geçer, metinde
// olmayan ad ve artık kod olan ad yakalanır.
func TestStaleIdentityExceptions(t *testing.T) {
	fake := map[string]string{
		"TR_future_name":  "onaylı yeni ad",
		"TR_removed_name": "dil tanımından çıkmış ad",
		"TR_print":        "artık kod olan ad",
	}
	got := staleIdentityExceptions(fake, "Kuralda `TR_future_name` ve `TR_print` geçer.")
	want := []string{
		"TR_print: bayat: artık kod, silin",
		"TR_removed_name: bayat: dil tanımında yok, silin",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("alınan: %v, beklenen: %v", got, want)
	}
}
