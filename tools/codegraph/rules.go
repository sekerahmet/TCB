package main

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	backtick  = regexp.MustCompile("`([^`]+)`")
	boldLead  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	identOnly = regexp.MustCompile(`^\w+$`)
	// listMarker madde işaretidir ("- ", "* ", "1. "); kural adı ondan sonra başlar.
	listMarker = regexp.MustCompile(`^([-*+]|\d+\.)\s+`)
)

// addRules d'deki her "Denetim:" maddesini ve "Kural | Test" tablosu satırını kural düğümü
// yapar; koruyan testlere (ya da test dosyalarına) `guarded_by` kenarı çeker.
func addRules(b *builder, idx *goIndex, set *docSet, d *mdDoc) {
	for _, r := range d.rules {
		id := "rule:" + d.rel + ":" + strconv.Itoa(r.line)
		b.addNode(Node{ID: id, Kind: kindRule, Name: ruleName(r.text), File: d.rel, Line: r.line, Detail: ruleDetail(r.text)})
		b.addEdge(ownerID(d, r.owner), id, edgeContains, extracted, d.rel, r.line)
		files, hints, names := guardTokens(d.rel, r.guards, b)
		for _, f := range files {
			b.addEdge(id, fileID(f), edgeGuardedBy, extracted, d.rel, r.line)
		}
		for _, name := range names {
			tests, conf := resolveTests(idx, name, hints, path.Dir(d.rel))
			if len(tests) == 0 {
				b.warn("rule " + d.rel + ":" + strconv.Itoa(r.line) + ": test not found: " + name)
			}
			for _, t := range tests {
				b.addEdge(id, t.id, edgeGuardedBy, conf, d.rel, r.line)
			}
		}
	}
}

// ruleName kuralın kısa adıdır: kalın başlık varsa o (sondaki nokta atılır), yoksa ilk sözcükler.
func ruleName(text string) string {
	text = strings.TrimSpace(listMarker.ReplaceAllString(strings.TrimSpace(text), ""))
	if m := boldLead.FindStringSubmatch(text); m != nil && strings.HasPrefix(text, "**") {
		return strings.TrimRight(strings.TrimSpace(m[1]), ".:")
	}
	// Başta kalın ad yoksa ilk cümle (en çok 70 karakter); ortadaki kalın sözcükler ad değildir.
	text = strings.NewReplacer("`", "", "**", "").Replace(text)
	if i := strings.Index(text, ". "); i > 0 {
		text = text[:i]
	}
	return truncate(strings.TrimSpace(text), 70)
}

func ruleDetail(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "**", ""), "`", "")
	return truncate(strings.Join(strings.Fields(text), " "), 300)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// guardTokens koruma metnindeki ters tırnaklı simgeleri sınıflar: var olan test dosyaları
// (files), ad çözümünü daraltan yol ipuçları (hints) ve test işlevi adları (names).
func guardTokens(docRel, guards string, b *builder) (files, hints, names []string) {
	docDir := path.Dir(docRel)
	for _, m := range backtick.FindAllStringSubmatch(guards, -1) {
		tok := m[1]
		switch {
		case strings.HasSuffix(tok, ".go"):
			rel := tok
			if !strings.Contains(tok, "/") {
				rel = path.Join(docDir, tok)
			}
			if b.nodes[fileID(rel)] == nil {
				rel = path.Join(docDir, tok) // kökten verilen yol yoksa belge klasörüne göre dene
			}
			if b.nodes[fileID(rel)] != nil {
				files = append(files, rel)
				hints = append(hints, rel)
			}
		case strings.Contains(tok, "/") && !strings.ContainsAny(tok, " \t"):
			hints = append(hints, strings.TrimSuffix(tok, "/"))
		case identOnly.MatchString(tok) && isTestName(tok):
			names = append(names, tok)
		}
	}
	return files, hints, names
}

// resolveTests bir test adını işleve çözer: önce ipucu yolunun altındakiler, sonra belgenin
// klasörünün altındakiler (EXTRACTED); ikisi de tutmazsa, en çok üç yerde tanımlıysa depo geneli
// (INFERRED).
func resolveTests(idx *goIndex, name string, hints []string, docDir string) ([]*testFunc, string) {
	cands := idx.byName[name]
	pick := func(match func(*testFunc) bool) []*testFunc {
		var out []*testFunc
		for _, c := range cands {
			if match(c) {
				out = append(out, c)
			}
		}
		return out
	}
	if out := pick(func(t *testFunc) bool { return underAny(t.file, hints) }); len(out) > 0 {
		return out, extracted
	}
	if docDir != "." {
		if out := pick(func(t *testFunc) bool { return underAny(t.file, []string{docDir}) }); len(out) > 0 {
			return out, extracted
		}
	}
	if len(cands) > 0 && len(cands) <= 3 {
		return cands, inferred
	}
	return nil, ""
}

func underAny(file string, dirs []string) bool {
	for _, d := range dirs {
		if file == d || strings.HasPrefix(file, d+"/") {
			return true
		}
	}
	return false
}
