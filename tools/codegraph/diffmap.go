package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// hunk bir `git diff -U0` parçasıdır. added ve removed değişen satırların "+"/"-" önekli metnidir.
type hunk struct {
	file               string
	oldStart, oldCount int
	newStart, newCount int
	changedLines       []string
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// parseDiff `git diff -U0 --no-renames` çıktısını parçalara ayırır.
func parseDiff(out string) []hunk {
	var hs []hunk
	file := ""
	inHeader := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			inHeader = true
			file = line[strings.LastIndex(line, " b/")+3:]
		case inHeader && strings.HasPrefix(line, "+++ b/"):
			file = line[6:]
		case strings.HasPrefix(line, "@@"):
			inHeader = false
			if m := hunkHeader.FindStringSubmatch(line); m != nil {
				hs = append(hs, hunk{file: file, oldStart: atoi(m[1]), oldCount: count(m[2]),
					newStart: atoi(m[3]), newCount: count(m[4])})
			}
		case !inHeader && len(hs) > 0 && (line[:min(1, len(line))] == "+" || line[:min(1, len(line))] == "-"):
			h := &hs[len(hs)-1]
			h.changedLines = append(h.changedLines, line)
		}
	}
	return hs
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// count hunk başlığındaki satır sayısıdır; yazılmamışsa 1.
func count(s string) int {
	if s == "" {
		return 1
	}
	return atoi(s)
}

// touchSet değişikliklerin dokunduğu düğümlerdir.
type touchSet struct {
	nodes         map[string]*Node // kimlik -> düğüm
	codes         map[string]bool  // grafikte düğümü olmayan (yeni ya da silinmiş) kodların kimlikleri
	unmapped      map[string]int   // düğüme eşlenemeyen değişiklik: dosya -> hunk sayısı
	unmappedHunks []hunk           // eşlenemeyen Go/belge hunk'ları (dalda yeni bildirimleri adlandırmak için)
	files         map[string]bool  // dokunulan dosyalar
}

func newTouchSet() *touchSet {
	return &touchSet{nodes: map[string]*Node{}, codes: map[string]bool{}, unmapped: map[string]int{}, files: map[string]bool{}}
}

// span bir dosyadaki satır aralığı taşıyan düğümlerdir.
func (ix *index) spans() map[string][]*Node {
	m := map[string][]*Node{}
	for i := range ix.g.Nodes {
		n := &ix.g.Nodes[i]
		if hasLineSpan(n.Kind) && n.File != "" && n.Line > 0 {
			m[n.File] = append(m[n.File], n)
		}
	}
	return m
}

// touched hunk'ları düğümlere eşler. newSide=true ise yeni dosyadaki satırlar, değilse eski
// dosyadaki satırlar alınır (grafik hangisine uyuyorsa o). Sözlük JSON dosyalarında satır
// numarasına değil, değişen satırlardaki kod anahtarlarına bakılır.
func (ix *index) touched(hs []hunk, newSide bool) *touchSet {
	ts := newTouchSet()
	spans := ix.spans()
	for _, h := range hs {
		ts.files[h.file] = true
		if isDictionaryFile(h.file) {
			ix.touchCodes(ts, h)
			continue
		}
		start, n := h.oldStart, h.oldCount
		if newSide {
			start, n = h.newStart, h.newCount
		}
		lo, hi := start, start+max(n, 1)-1
		hit := false
		for _, node := range spans[h.file] {
			end := max(node.End, node.Line)
			// Sayı 0: satırlar start'tan SONRA eklendi/silindi; düğümün içindeyse (kapanışından
			// önceyse) değişmiş sayılır, hemen ardındaki ekleme sayılmaz.
			if n == 0 && node.Line <= start && start < end || n > 0 && node.Line <= hi && end >= lo {
				ts.nodes[node.ID] = node
				hit = true
			}
		}
		if !hit {
			ts.unmappedHunks = append(ts.unmappedHunks, h)
			ts.unmapped[h.file]++
		}
	}
	return ts
}

func isDictionaryFile(file string) bool {
	return strings.HasSuffix(file, ".json") && (strings.HasPrefix(file, messagesDir+"/") || strings.HasPrefix(file, codesDir+"/"))
}

// touchCodes bir sözlük hunk'ının değişen satırlarındaki kodları düğümlere (yoksa codes'a) ekler.
func (ix *index) touchCodes(ts *touchSet, h hunk) {
	found := false
	// Aynı kodun silinen ve eklenen satırı yalnızca sondaki virgülle ayrışıyorsa (komşu kod
	// eklenince) kod değişmemiştir.
	sides := map[string]map[string]string{}
	for _, l := range h.changedLines {
		if m := codeKeyDiff.FindStringSubmatch(l); m != nil {
			if sides[m[2]] == nil {
				sides[m[2]] = map[string]string{}
			}
			sides[m[2]][m[1]] = strings.TrimRight(strings.TrimSpace(l[1:]), ", ")
		}
	}
	for _, l := range h.changedLines {
		m := codeKeyDiff.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		found = true
		if s := sides[m[2]]; s["+"] != "" && s["+"] == s["-"] {
			continue
		}
		id := "name:" + m[2]
		if strings.HasPrefix(m[2], "H") {
			id = "msg:" + m[2]
		}
		if n := ix.byID[id]; n != nil {
			ts.nodes[id] = n
		} else {
			ts.codes[id] = true
		}
	}
	if !found {
		ts.unmapped[h.file]++
	}
}

// keys dokunulan düğüm ve kod kimliklerini sıralı verir.
func (ts *touchSet) keys() []string {
	var out []string
	for id := range ts.nodes {
		out = append(out, id)
	}
	for id := range ts.codes {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
