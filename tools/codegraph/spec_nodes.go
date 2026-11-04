package main

import (
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/tools/internal/specref"
)

// Dil tanımının numaralı maddeleri (docs/spec/NN_*.md, `**17.7** ...`) grafikte düğümdür ve onlara yapılan her atıf
// (`**17.7**`, `dil tanımı 17.7`, `§17.7`, bağlantı metni, spec içi parantez anması) atıf yapan yerden maddeye bir kenardır.
// Tarayıcı `spec-refs` komutunun ve `tools/renumber`ın kullandığı tek tarayıcıdır (tools/internal/specref); burada
// yalnızca grafiğe çevrilir. Böylece "17.7 değişirse hangi belge, test ve yorum güncellenir" sorusu `who-uses 17.7` olur,
// "17.7'yi hangi testler sınıyor" sorusu `tests-for 17.7`; yeniden numaralamada etki listesi buradan gelir.

const (
	kindSpec     = "spec"
	edgeRefsSpec = "refs_spec"
)

// extractSpecRefs madde düğümlerini ve atıf kenarlarını kurar. Atıf yapan, atıfın satırını içeren en iç bildirimdir
// (işlev, metot, tür, TR işlevi, test, belge başlığı); satır bir bildirimin hemen üstündeki yorumdaysa o bildirim; hiçbiri
// yoksa dosyanın kendisi. Atıf yapan bir testse (Go testi ya da TR test bloğu) testten maddeye ayrıca `tests` kenarı
// çekilir: testin yorumunun andığı kural onun koruduğu kuraldır. TR test blokları adlarının numarayla başlamasıyla
// (`test "20.3 güncelle ..."`) maddeyi sınar.
func extractSpecRefs(root string, b *builder) {
	files, err := specref.Collect(root)
	if err != nil {
		b.warn("spec references skipped: " + err.Error())
		return
	}
	ix := specref.BuildIndex(files)
	memberAliases := specMemberAliases(files, ix)
	for _, id := range ix.Order {
		it := ix.Items[id]
		b.addNode(Node{ID: specID(id), Kind: kindSpec, Name: id, File: it.File, Line: it.Line, Detail: it.Title, Aliases: memberAliases[id]})
	}
	byFile := map[string][]*Node{}
	for _, n := range b.nodes {
		if n.File != "" && hasLineSpan(n.Kind) {
			byFile[n.File] = append(byFile[n.File], n)
		}
	}
	text := map[string][]string{}
	for _, f := range files {
		text[f.Path] = strings.Split(f.Text, "\n")
	}
	for _, m := range specref.Scan(files, ix) {
		if m.Kind == "definition" {
			continue
		}
		from := referrer(b, byFile[m.File], text[m.File], m.File, m.Line)
		for _, t := range m.Tokens {
			if _, ok := ix.Items[t.ID]; !ok {
				continue
			}
			b.addEdge(from, specID(t.ID), edgeRefsSpec, extracted, m.File, m.Line)
			if n := b.nodes[from]; n != nil && isTestNode(n) {
				b.addEdge(from, specID(t.ID), edgeTests, inferred, m.File, m.Line)
			}
		}
	}
	trTestsByNumber(b, ix)
}

func specID(item string) string { return "spec:" + item }

// referrer atıfın satırını içeren en iç düğümün kimliğini verir (yoksa satırın hemen altındaki bildirim, o da yoksa
// dosya düğümü).
func referrer(b *builder, nodes []*Node, lines []string, file string, line int) string {
	var best *Node
	for _, n := range nodes {
		if n.Line <= line && line <= max(n.End, n.Line) && (best == nil || n.End-n.Line < best.End-best.Line) {
			best = n
		}
	}
	if best == nil {
		best = declarationBelow(nodes, lines, line)
	}
	if best != nil {
		return best.ID
	}
	return fileNodeID(b, file)
}

// declarationBelow satırın doc yorumu olduğu bildirimi verir: satırdan sonra yalnızca yorum ve boş satır varsa ilk bildirim.
func declarationBelow(nodes []*Node, lines []string, line int) *Node {
	var next *Node
	for _, n := range nodes {
		if n.Line > line && (next == nil || n.Line < next.Line) {
			next = n
		}
	}
	if next == nil {
		return nil
	}
	for l := line; l < next.Line-1 && l < len(lines); l++ { // lines[l] = satır l+1
		if t := strings.TrimSpace(lines[l]); t != "" && !strings.HasPrefix(t, "//") && !strings.HasPrefix(t, "#") {
			return nil
		}
	}
	return next
}

// fileNodeID bir dosyanın düğüm kimliği (Go, TR ya da belge); düğüm yoksa dosya düğümü eklenir.
func fileNodeID(b *builder, file string) string {
	for _, id := range []string{fileID(file), trFileID(file), docID(file)} {
		if b.nodes[id] != nil {
			return id
		}
	}
	id := "file:" + file
	b.addNode(Node{ID: id, Kind: kindFile, Name: file, File: file, Line: 1})
	return id
}

// trTestsByNumber adı madde numarasıyla başlayan TR test bloklarını o maddeyi sınayan test sayar (kural: `test "4.2
// aritmetik: ..."`).
func trTestsByNumber(b *builder, ix *specref.Index) {
	var ids []string
	for id, n := range b.nodes {
		if n.Kind == kindTRTest {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := b.nodes[id]
		_, title, _ := strings.Cut(n.Name, ": ")
		item, _, _ := strings.Cut(title, " ")
		if _, ok := ix.Items[item]; ok && path.Ext(n.File) == ".tr" {
			b.addEdge(id, specID(item), edgeTests, inferred, n.File, n.Line)
		}
	}
}

var specDotMember = regexp.MustCompile(`\.TR_[\p{L}\p{N}_]+`)

type specMemberHit struct {
	id   string
	tok  string
	tier int
}

// specMemberAliases numaralı dil tanımı maddelerinde geçen `.TR_<ad>` üye yazımlarını toplar ve her yazımı en öncelikli
// katmandaki (0: maddenin Title özeti, 1: maddenin açılış satırı, 2: maddenin gövdesi) madde düğümlerine verir; böylece
// başka bir maddenin açılışında ya da gövdesinde örnek olarak anılan üye, kendi başlığında tanımlandığı maddeyi gölgelemez.
func specMemberAliases(files []specref.File, ix *specref.Index) map[string][]string {
	byFile := map[string]map[int]string{}
	for _, id := range ix.Order {
		it := ix.Items[id]
		if byFile[it.File] == nil {
			byFile[it.File] = map[int]string{}
		}
		byFile[it.File][it.Line] = id
	}
	var hits []specMemberHit
	for _, f := range files {
		if starts := byFile[f.Path]; len(starts) > 0 {
			hits = appendSpecFileMembers(hits, f.Text, starts, ix.Items)
		}
	}
	minTier := map[string]int{}
	for _, h := range hits {
		k := codeindex.Fold(h.tok)
		if cur, ok := minTier[k]; !ok || h.tier < cur {
			minTier[k] = h.tier
		}
	}
	out := map[string][]string{}
	for _, h := range hits {
		if h.tier == minTier[codeindex.Fold(h.tok)] && !slices.Contains(out[h.id], h.tok) {
			out[h.id] = append(out[h.id], h.tok)
		}
	}
	for _, list := range out {
		sort.Strings(list)
	}
	return out
}

func appendSpecFileMembers(hits []specMemberHit, text string, starts map[int]string, items map[string]specref.Item) []specMemberHit {
	var fence specref.Fence
	curID := ""
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, "\r")
		if fence.Line(line) {
			continue
		}
		if !fence.In() {
			if id, ok := starts[i+1]; ok {
				curID = id
			} else if t := strings.TrimSpace(line); strings.HasPrefix(t, "#") || isNumberedBoldLine(t) {
				curID = ""
			}
		}
		if curID == "" {
			continue
		}
		it := items[curID]
		for _, tok := range dotMemberTokens(line) {
			tier := 2
			if i+1 == it.Line {
				tier = 1
				if slices.Contains(dotMemberTokens(it.Title), tok) {
					tier = 0
				}
			}
			hits = append(hits, specMemberHit{id: curID, tok: tok, tier: tier})
		}
	}
	return hits
}

func dotMemberTokens(s string) []string {
	var out []string
	for _, loc := range specDotMember.FindAllStringIndex(s, -1) {
		if loc[0] == 0 || s[loc[0]-1] != '.' {
			out = append(out, s[loc[0]:loc[1]])
		}
	}
	return out
}

func isNumberedBoldLine(trimmed string) bool {
	return len(trimmed) > 2 && trimmed[0] == '*' && trimmed[1] == '*' && trimmed[2] >= '0' && trimmed[2] <= '9'
}
