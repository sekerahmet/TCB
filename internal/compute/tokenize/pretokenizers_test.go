package tokenize

import (
	"reflect"
	"testing"
)

// Kural: findSubstring çakışmayan geçişleri sırayla bildirir; boş desen hiç geçiş bildirmez
// (tokenizer.json'dan boş düz desen gelebilir; korumasız arama sonsuz döngü yapardı).
func TestFindSubstringReportsEachPassageAndIgnoresEmptyPatterns(t *testing.T) {
	var got [][2]int
	collect := func(b, e int) { got = append(got, [2]int{b, e}) }
	findSubstring("aXbXXc", "X", collect)
	if want := [][2]int{{1, 2}, {3, 4}, {4, 5}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("geçişler %v, beklenen %v", got, want)
	}
	got = nil
	findSubstring("abab", "ab", collect)
	if want := [][2]int{{0, 2}, {2, 4}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("geçişler %v, beklenen %v", got, want)
	}
	got = nil
	findSubstring("abc", "", collect)
	if got != nil {
		t.Fatalf("boş desen geçiş bildirdi: %v", got)
	}
}
