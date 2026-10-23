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
	var out []*Node
	for _, s := range sources {
		for _, e := range ix.out[s] {
			if m := ix.byID[e.To]; e.Kind == edgeUsesMessage && m != nil && !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	return out
}

func (ix *index) aboutReason(t *Node, names []string, msgs []*Node) string {
	for _, name := range names {
		if hasWord(t.Name, name) || hasWord(t.Doc, name) ||
			strings.HasPrefix(t.ID, "go:") && strings.Contains(strings.ToLower(t.Name), strings.ToLower(name)) {
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
			if fixed := longestFixedPart(m.Detail); len(fixed) >= 12 && len(fixed) > bestLen && strings.Contains(line, fixed) {
				best, bestLen = "expects "+m.Name, len(fixed)
			}
		}
	}
	return best
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
