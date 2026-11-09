package codeindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// chainReader f0 ← f1 ← … ← f10 zincirli bir projedir: "yakın" testi f2'yi (f0'dan 3 adım), "derin" testi f10'u (11
// adım) çağırır.
func chainReader(t *testing.T) *Reader {
	t.Helper()
	dir := t.TempDir()
	var src strings.Builder
	src.WriteString("TR_func f0() -> TR_Int {\n    TR_return 1\n}\n")
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&src, "TR_func f%d() -> TR_Int {\n    TR_return f%d()\n}\n", i, i-1)
	}
	tests := "TR_import \"m\"\n\nTR_test \"yakın\" {\n    f2()\n}\n\nTR_test \"derin\" {\n    f10()\n}\n"
	for name, text := range map[string]string{"m.tr": src.String(), "m_test.tr": tests} {
		if err := os.WriteFile(filepath.Join(dir, name), trText(name, text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil || len(idx.Skipped) > 0 {
		t.Fatal(err, idx.Skipped)
	}
	return NewReader(idx)
}

// Kural (codegraph B1a, kademe 2): tested_by'ın listesi adım sınırındadır (TestMaxSteps); paketin içinde sınırın
// ötesinde ulaşılan test listeye girmez, Counts'ta sayılır ve girişiyle tek satırda yazılır. Giriş, ondan sorulan
// tested_by'ın o testi sınır içinde bulduğu düğümdür.
func TestATestBeyondTheStepLimitIsCountedNotListed(t *testing.T) {
	r := chainReader(t)
	res := r.TestedBy("f0", QueryOptions{})
	if len(res.Tests) != 1 || res.Tests[0].In.Name != "yakın" || res.Tests[0].Steps != 3 || res.Counts.Total != 1 {
		t.Fatalf("the list keeps the step limit: %+v", res.Tests)
	}
	c := res.Counts
	if c.BeyondLimit != 1 || len(c.BeyondEntries) != 1 {
		t.Fatalf("one test beyond the limit, one entry: %+v", c)
	}
	e := c.BeyondEntries[0]
	if e.Entry.Name != "f3" || e.Tests != 1 || e.Steps != 11 {
		t.Errorf("the entry is f3, 11-8 steps from f0: %+v", e)
	}
	if imp := r.Impact("f0", QueryOptions{}); imp.Counts.Tests != 1 || len(imp.Affected.Tests) != 1 {
		t.Errorf("impact's tests keep the step limit: %+v", imp.Counts)
	}
	from := r.TestedBy("m.f3", QueryOptions{}) // satırın önerdiği sorgu
	if !slices.ContainsFunc(from.Tests, func(u Use) bool { return u.In.Name == "derin" && u.Steps == TestMaxSteps }) {
		t.Errorf("tested_by of the entry lists the test within the limit: %+v", from.Tests)
	}
	text := Text(res)
	want := messages.Text(messages.H8880, 1, TestMaxSteps, "m.f3", 11, "--all")
	if !strings.Contains(text, want) {
		t.Errorf("text lacks the beyond line %q:\n%s", want, text)
	}
}

// Sınırın ötesinde testi olmayan düğümün JSON'u eskisi gibidir: yeni alanlar boşken yazılmaz.
func TestNoBeyondFieldsWithoutTestsBeyondTheLimit(t *testing.T) {
	res := chainReader(t).TestedBy("f9", QueryOptions{})
	data, err := json.Marshal(res.Counts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "beyond") {
		t.Errorf("counts without tests beyond the limit: %s", data)
	}
	if strings.Contains(Text(res), messages.Text(messages.H8880, 1, TestMaxSteps, "f1", 9, "--all")[:20]) {
		t.Errorf("no beyond line: %s", Text(res))
	}
}

// Birden çok giriş: en çok testli giriş adıyla, kalanı sayıyla (H8881).
func TestTheBeyondLineNamesTheEntryWithTheMostTests(t *testing.T) {
	c := &Counts{BeyondLimit: 5, BeyondEntries: []BeyondEntry{
		{Entry: Node{Name: "a", Qualified: "m.a"}, Tests: 3, Steps: 9},
		{Entry: Node{Name: "b"}, Tests: 2, Steps: 10},
	}}
	var b strings.Builder
	writeBeyond(&b, c)
	if want := messages.Text(messages.H8881, 5, TestMaxSteps, "m.a", 3, 9, 1, "--all"); !strings.Contains(b.String(), want) {
		t.Errorf("got %q, want %q", b.String(), want)
	}
}

// Kural: WalkCallersBeyond ilk TestMaxSteps katta WalkCallers'la aynı düğümleri aynı sırayla verir; ötesine yalnız
// başlangıcın paketinde geçer. Başka paketten gelen kol toplam TestMaxSteps adımda kalır.
func TestTheWalkBeyondTheLimitStaysInItsPackage(t *testing.T) {
	callers := map[string][]string{}
	for i := 1; i <= 12; i++ { // a/0 ← a/1 ← … ← a/12
		callers[fmt.Sprintf("a/%d", i-1)] = []string{fmt.Sprintf("a/%d", i)}
	}
	callers["a/3"] = append(callers["a/3"], "b/x") // b/x 4 adımda, b/y 5'te
	callers["b/x"] = []string{"b/y"}
	callers["a/7"] = append(callers["a/7"], "c/x") // c/x 8 adımda; c/y 9. adım olurdu
	callers["c/x"] = []string{"c/y"}
	pkgOf := func(id string) string { return id[:1] }
	walk := func(f func(string, func(string) []string, func(string) string, func(Reached) bool)) []Reached {
		var out []Reached
		f("a/0", func(id string) []string { return callers[id] }, pkgOf, func(r Reached) bool { out = append(out, r); return true })
		return out
	}
	limited, beyond := walk(WalkCallers), walk(WalkCallersBeyond)
	if len(limited) == 0 || len(beyond) <= len(limited) {
		t.Fatalf("limited %d, beyond %d", len(limited), len(beyond))
	}
	for i, r := range limited {
		if beyond[i] != r {
			t.Errorf("node %d: %+v, within the limit %+v", i, beyond[i], r)
		}
	}
	for _, r := range beyond[len(limited):] {
		if r.Steps <= TestMaxSteps || pkgOf(r.ID) != "a" || r.Entry != fmt.Sprintf("a/%d", r.Steps-TestMaxSteps) {
			t.Errorf("beyond the limit only in a, with its entry: %+v", r)
		}
	}
	if slices.ContainsFunc(beyond, func(r Reached) bool { return r.ID == "c/y" }) {
		t.Errorf("a chain through another package keeps the limit: %+v", beyond)
	}
}
