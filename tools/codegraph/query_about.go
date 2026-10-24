package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Bu dosya tests-for'un ilk grubunu kurar: hedefi konu edinen testler (deneme 8 S1).

// aboutTest bir hedefi konu edinen testtir ve nedeni.
type aboutTest struct {
	test   *Node
	reason string // "names it" ya da "expects H2427"
}

// aboutTests adayların içinden hedefi konu edinen testleri seçer (deneme 8 S1: çok kullanılan bir adda, yazdır'ı
// yalnızca sonucu görmek için çağıran testler, onu sınayanları gömüyordu). Bir test hedefi konu edinir:
//   - adı ya da belge yorumu hedefin adını anıyorsa (ad kodunun dildeki adı, işlevin kısa adı, uyguladığı ad kodları);
//   - hedefin ürettiği bir iletiyi bekliyorsa: Go testi o ileti koduna `uses_message` ile bağlı, TR testinin bir satırı
//     hem iletinin sabit metnini hem hedefin adını taşıyor (`yazdır() 'ayraç' adında bir parametre almaz` → H2427).
//
// Hedefin ürettiği iletiler: kendisinin, onu uygulayan Go işlevlerinin (ad kodu için) ve bunların doğrudan çağırdığı
// işlevlerin `uses_message` kenarları.
func (ix *index) aboutTests(n *Node, candidates []*Node) []aboutTest {
	names := ix.subjectNames(n)
	msgs := ix.producedMessages(n)
	var out []aboutTest
	seen := map[string]bool{}
	for _, t := range candidates {
		if t == nil || seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		if reason := ix.aboutReason(t, names, msgs); reason != "" {
			out = append(out, aboutTest{t, reason})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].test.Name < out[j].test.Name })
	return out
}

// subjectNames hedefin testlerde anılabilecek adlarıdır.
func (ix *index) subjectNames(n *Node) []string {
	var names []string
	switch n.Kind {
	case kindName:
		names = append(names, n.Aliases...)
	case kindFunc, kindMethod, kindTRFunc, kindTRMethod:
		names = append(names, n.Name[strings.LastIndex(n.Name, ".")+1:])
		for _, e := range ix.out[n.ID] {
			if c := ix.byID[e.To]; e.Kind == edgeUsesName && c != nil && implementable(c) {
				names = append(names, c.Aliases...)
			}
		}
	}
	var out []string
	for _, s := range names {
		if len([]rune(s)) >= 4 && !slices.Contains(out, s) { // kısa adlar (pi, x) her yerde geçer
			out = append(out, s)
		}
	}
	return out
}

// producedMessages hedefin ürettiği ileti düğümleridir.
func (ix *index) producedMessages(n *Node) []*Node {
	sources := []string{n.ID}
	if n.Kind == kindName {
		for _, e := range ix.in[n.ID] {
			if e.Kind == edgeUsesName && strings.HasPrefix(e.From, "go:") {
				sources = append(sources, e.From)
			}
		}
	}
	for _, s := range append([]string(nil), sources...) {
		for _, e := range ix.out[s] {
			if e.Kind == edgeCalls && e.Confidence == extracted {
				sources = append(sources, e.To)
			}
		}
	}
	own := map[string]bool{}
	for _, s := range sources {
		own[s] = true
	}
	var out []*Node
	for _, s := range sources {
		for _, e := range ix.out[s] {
			if m := ix.byID[e.To]; e.Kind == edgeUsesMessage && m != nil && !slices.Contains(out, m) && ix.producedOnlyBy(m, own) {
				out = append(out, m)
			}
		}
	}
	return out
}

// producedOnlyBy iletiyi üreten her (test olmayan) düğüm own'da mı: başka işlevlerin de ürettiği ileti hedefin değildir,
// onu bekleyen test hedefi konu edinmez (bağımsız denetim grup 6, S1: H2000 `{fn}: {error}` on iki işlevin sarmalayıcı
// iletisi, yüklemeyi sınayan test saveOnInterval'ın ilk satırına çıkıyordu).
func (ix *index) producedOnlyBy(m *Node, own map[string]bool) bool {
	for _, e := range ix.in[m.ID] {
		if p := ix.byID[e.From]; e.Kind == edgeUsesMessage && p != nil && !isTestNode(p) && !own[p.ID] {
			return false
		}
	}
	return true
}

func (ix *index) aboutReason(t *Node, names []string, msgs []*Node) string {
	title := testTitle(t)
	for _, name := range names {
		if hasWord(title, name) || hasWord(t.Doc, name) ||
			strings.HasPrefix(t.ID, "go:") && containsWords(nameWords(title), nameWords(name)) {
			return "names it"
		}
	}
	for _, m := range msgs {
		for _, e := range ix.out[t.ID] {
			if e.Kind == edgeUsesMessage && e.To == m.ID {
				return "expects " + m.Name
			}
		}
	}
	if t.Kind != kindTRTest || ix.root == "" || len(names) == 0 {
		return ""
	}
	// TR testi hatayı metniyle bekler; iletiyi kimin ürettiği çoğu zaman ortak bir yoldur (yerleşik çağırıcısı H2427'yi
	// her yerleşik için üretir). Bu yüzden işlev adını taşıyan ({fn}) bütün iletiler de bakılır: satır hem adı hem
	// iletinin sabit metnini taşıyorsa test o adın iletisini bekler.
	msgs = append(slices.Clone(msgs), ix.functionMessages()...)
	data, err := os.ReadFile(filepath.Join(ix.root, filepath.FromSlash(t.File)))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	best, bestLen := "", 0 // birden çok ileti uyarsa sabit metni en uzun olan (en belirgin) söylenir
	for no := t.Line; no <= max(t.End, t.Line) && no <= len(lines); no++ {
		line := lines[no-1]
		if !slices.ContainsFunc(names, func(name string) bool { return hasWord(line, name) }) {
			continue
		}
		for _, m := range msgs {
			if fixed := longestFixedPart(m.Detail); len(fixed) >= 12 && len(fixed) > bestLen && strings.Contains(line, fixed) &&
				fnSlotHolds(m.Detail, line, names) {
				best, bestLen = "expects "+m.Name, len(fixed)
			}
		}
	}
	return best
}

// testTitle testin adının hedefle karşılaştırılan kısmıdır: Go testinde paketsiz kısa ad (`TestTensorData`; paket adı
// `dataset_test` "Data"yı anmaz), TR testinde modül adı atılmış açıklama (bağımsız denetim grup 6, S1).
func testTitle(t *Node) string {
	if t.Kind == kindTRTest {
		return t.Name[strings.Index(t.Name, ": ")+2:]
	}
	return t.Name[strings.LastIndex(t.Name, ".")+1:]
}

// nameWords bir adın sözcükleridir, Türkçe katlanmış: alt çizgide, harf ve rakam olmayan karakterde ve büyük harf
// geçişinde bölünür (`TestTensorData` → test tensor data, `saveOnInterval` → save on interval, `HTTPServer` → http
// server).
func nameWords(s string) []string {
	var words []string
	runes := []rune(s)
	start := -1
	flush := func(end int) {
		if start >= 0 && end > start {
			words = append(words, codeindex.Fold(string(runes[start:end])))
		}
		start = -1
	}
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush(i)
			continue
		}
		if start >= 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(runes[i-1])) {
			flush(i)
		}
		if start < 0 {
			start = i
		}
	}
	flush(len(runes))
	return words
}

// containsWords sub, words içinde ardışık olarak geçiyor mu.
func containsWords(words, sub []string) bool {
	if len(sub) == 0 {
		return false
	}
	for i := 0; i+len(sub) <= len(words); i++ {
		if slices.Equal(words[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

// fnSlotHolds şablonda işlev adı yuvası ({fn}) varsa satırda o yuvada hedefin adlarından biri durur mu: yuvanın hemen
// önündeki ya da ardındaki sabit metin adla bitişik geçmeli (`{fn}() '{param}' adında ...` için `yazdır() '`). Yuvasız
// şablonda koşul yoktur. `metin.böl()`ün iletisini bekleyen satır `metin` yerleşiğini beklemez (bağımsız denetim grup 6,
// S1).
func fnSlotHolds(template, line string, names []string) bool {
	slots := templateSlot.FindAllStringIndex(template, -1)
	holds, has := false, false
	for i, at := range slots {
		if template[at[0]:at[1]] != "{fn}" {
			continue
		}
		has = true
		before, after := template[:at[0]], template[at[1]:]
		if i > 0 {
			before = template[slots[i-1][1]:at[0]]
		}
		if i+1 < len(slots) {
			after = template[at[1]:slots[i+1][0]]
		}
		for _, name := range names {
			if after != "" && strings.Contains(line, name+after) || before != "" && strings.Contains(line, before+name) {
				holds = true
			}
		}
	}
	return holds || !has
}

// functionMessages şablonu bir işlev adı ({fn}) taşıyan iletilerdir (sorgu başına bir kez hesaplanır).
func (ix *index) functionMessages() []*Node {
	if ix.fnMessages == nil {
		ix.fnMessages = []*Node{}
		for i := range ix.g.Nodes {
			if m := &ix.g.Nodes[i]; m.Kind == kindMessage && strings.Contains(m.Detail, "{fn}") {
				ix.fnMessages = append(ix.fnMessages, m)
			}
		}
	}
	return ix.fnMessages
}

// longestFixedPart ileti şablonunun yer tutucular ({fn}, {param}) arasındaki en uzun sabit parçasıdır.
func longestFixedPart(template string) string {
	best := ""
	for _, part := range templateSlot.Split(template, -1) {
		if len(part) > len(best) {
			best = part
		}
	}
	return best
}

var templateSlot = regexp.MustCompile(`\{[^{}]*\}`)

// printAboutTests hedefi konu edinen testleri ilk grupta yazar.
func (ix *index) printAboutTests(w io.Writer, list []aboutTest, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests about it (%d; their name or doc names it, or they expect a message it produces):\n", len(list))
	forEachLimited(w, list, limit, "    ", moreHint, func(a aboutTest) string {
		return fmt.Sprintf("%s  [%s]", describe(a.test), a.reason)
	})
}
