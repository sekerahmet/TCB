package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// minFixedText bir ileti şablonunun bir satırda tanınması için en uzun sabit parçasının en az uzunluğudur (bayt): kısa
// sabit metinler (`{fn}: {error}`) her yerde geçer.
const minFixedText = 12

// messageText bir ileti şablonunun metin sabitinde aranan biçimidir: sabit parçalar sırayla, yuvalar arada herhangi bir
// metin.
type messageText struct {
	id    string // ileti düğümü (msg:H2418)
	re    *regexp.Regexp
	fixed int // sabit parçaların toplam uzunluğu: birden çok şablon uyarsa en belirgini seçilir
}

// trMessageTexts TR testinin iletiyi metniyle beklemesini koda bağlar: test bloğundaki bir metin sabiti bir iletinin
// şablonunun sabit parçalarını sırayla taşıyorsa (`h.içerir("'Nokta' yapısının 'yy' adında bir alanı yok")` → H2418)
// testten ileti düğümüne `uses_message` (çıkarım) kenarı. Birden çok şablon uyarsa sabit metni en uzun olanlar. Aday
// şablonlar en uzun sabit parçalarının en uzun sözcüğüyle bulunur: her metin sabitini her şablonla denemek kurulumu
// yavaşlatırdı.
func trMessageTexts(root string, idx *codeindex.Index, ids map[string]string, b *builder) {
	byWord := map[string][]*messageText{}
	for id, n := range b.nodes {
		if n.Kind != kindMessage {
			continue
		}
		longest := longestFixedPart(n.Detail)
		word := longestWord(longest)
		if len(longest) < minFixedText || word == "" {
			continue
		}
		parts := templateSlot.Split(n.Detail, -1)
		quoted := make([]string, len(parts))
		fixed := 0
		for i, p := range parts {
			quoted[i], fixed = regexp.QuoteMeta(p), fixed+len(p)
		}
		byWord[word] = append(byWord[word], &messageText{id: id, re: regexp.MustCompile(strings.Join(quoted, ".*?")), fixed: fixed})
	}
	files := map[string][]string{}
	for _, n := range idx.Nodes {
		if n.Kind != codeindex.KindTest || ids[n.ID] == "" {
			continue
		}
		lines, ok := files[n.Pos.File]
		if !ok {
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.Pos.File))); err == nil {
				lines = strings.Split(string(data), "\n")
			}
			files[n.Pos.File] = lines
		}
		for no := n.Pos.Line; no <= max(n.Pos.EndLine, n.Pos.Line) && no <= len(lines); no++ {
			for _, lit := range stringLiterals(lines[no-1]) {
				for _, m := range matchingMessages(lit, byWord) {
					b.addEdge(ids[n.ID], m.id, edgeUsesMessage, inferred, n.Pos.File, no)
				}
			}
		}
	}
}

// matchingMessages metin sabitine uyan şablonlardan sabit metni en uzun olanlardır.
func matchingMessages(lit string, byWord map[string][]*messageText) []*messageText {
	var best []*messageText
	for _, w := range literalWords(lit) {
		for _, m := range byWord[w] {
			switch {
			case len(best) > 0 && m.fixed < best[0].fixed, !m.re.MatchString(lit):
			case len(best) > 0 && m.fixed > best[0].fixed:
				best = []*messageText{m}
			default:
				if !containsMessage(best, m) {
					best = append(best, m)
				}
			}
		}
	}
	return best
}

func containsMessage(list []*messageText, m *messageText) bool {
	for _, x := range list {
		if x == m {
			return true
		}
	}
	return false
}

// stringLiterals satırdaki çift tırnaklı metin sabitlerinin içidir (kaçışlı tırnak sabiti bitirmez).
func stringLiterals(line string) []string {
	var out []string
	start := -1
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && start >= 0:
			i++
		case line[i] == '"' && start < 0:
			start = i + 1
		case line[i] == '"':
			out = append(out, line[start:i])
			start = -1
		}
	}
	return out
}

// literalWords metnin harf ve rakam dizileridir, yazıldığı gibi.
func literalWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// longestWord metnin en uzun sözcüğüdür (literalWords).
func longestWord(s string) string {
	best := ""
	for _, w := range literalWords(s) {
		if len(w) > len(best) {
			best = w
		}
	}
	return best
}
