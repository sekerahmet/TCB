package main

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// linkScans çözülen taramaları `scan` düğümü yapar ve onlara ulaşan testlerden `scans` kenarı çeker (zincir kuralı).
// Düğümün Detail'i kapsamdır (taban|dahil|hariç, virgülle); Name okunur biçimidir.
func linkScans(b *builder, idx *goIndex, reach *testReach) {
	for _, r := range idx.scans.done {
		tests := reach.testsReaching(r.from)
		if len(tests) == 0 {
			continue
		}
		b.addNode(Node{ID: r.id, Kind: kindScan, Name: r.scope.String(), File: r.file, Line: r.line, Detail: r.scope.encode()})
		for _, t := range tests {
			b.addEdge(t, r.id, edgeScans, inferred, r.file, r.line)
		}
	}
}

func (s scanScope) encode() string {
	return strings.Join(s.bases, ",") + "|" + strings.Join(s.inc, ",") + "|" + strings.Join(s.exc, ",")
}

func decodeScope(detail string) scanScope {
	parts := strings.SplitN(detail, "|", 3)
	split := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.Split(s, ",")
	}
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return scanScope{split(parts[0]), split(parts[1]), split(parts[2])}
}

// String kapsamın okunur biçimidir: "*.go under cmd, internal, not *_test.go".
func (s scanScope) String() string {
	what := "every file"
	if len(s.inc) > 0 {
		what = suffixList(s.inc)
	}
	where := "the repository"
	if !(len(s.bases) == 1 && s.bases[0] == ".") {
		where = strings.Join(s.bases, ", ")
	}
	out := what + " under " + where
	if len(s.exc) > 0 {
		out += ", not " + suffixList(s.exc)
	}
	return out
}

// suffixList ekleri kalıp olarak yazar: uzantı ya da ek `*.go`, `*_test.go`; tam ad olduğu gibi (`README.md`).
func suffixList(list []string) string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s
		if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "_") {
			out[i] = "*" + s
		}
	}
	return strings.Join(out, ", ")
}

// covers dosya kapsamda mı: bir tabanın altında, dahil eklerden biriyle biter (dahil yoksa her dosya), hariç eklerin
// hiçbiriyle bitmez.
func (s scanScope) covers(file string) bool {
	under := false
	for _, b := range s.bases {
		if b == "." || file == b || strings.HasPrefix(file, b+"/") {
			under = true
			break
		}
	}
	if !under {
		return false
	}
	name := path.Base(file)
	matches := func(list []string) bool {
		for _, suf := range list {
			if strings.HasSuffix(name, suf) {
				return true
			}
		}
		return false
	}
	return (len(s.inc) == 0 || matches(s.inc)) && !matches(s.exc)
}

// scanHit bir dosyayı kapsayan tarama ve onu yapan testler.
type scanHit struct {
	scan  *Node
	tests []*Node
}

// scansCovering dosyayı kapsayan taramalardır, en çok test taşıyan önce.
func (ix *index) scansCovering(file string) []scanHit {
	var out []scanHit
	for i := range ix.g.Nodes {
		n := &ix.g.Nodes[i]
		if n.Kind != kindScan || !decodeScope(n.Detail).covers(file) {
			continue
		}
		hit := scanHit{scan: n}
		for _, e := range ix.in[n.ID] {
			if t := ix.byID[e.From]; e.Kind == edgeScans && t != nil {
				hit.tests = append(hit.tests, t)
			}
		}
		sort.Slice(hit.tests, func(i, j int) bool { return hit.tests[i].Name < hit.tests[j].Name })
		out = append(out, hit)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].tests) != len(out[j].tests) {
			return len(out[i].tests) > len(out[j].tests)
		}
		return out[i].scan.ID < out[j].scan.ID
	})
	return out
}

// namesPerScan bir tarama satırında yazılan en çok test adıdır (--all hepsini yazar).
const namesPerScan = 8

// printScans dosyayı kapsayan depo geneli taramaları kalıp başına bir satırla yazar: her dosyada aynı uzun test listesi
// olmasın diye satırda en çok namesPerScan test adı yazılır.
func (ix *index) printScans(w io.Writer, file string, limit int) int {
	hits := ix.scansCovering(file)
	if len(hits) == 0 {
		return 0
	}
	tests := map[string]bool{}
	for _, h := range hits {
		for _, t := range h.tests {
			tests[t.ID] = true
		}
	}
	fmt.Fprintf(w, "  <- repository-wide checks that scan it (%d tests in %d scans; each walks every file of its pattern, its rule may still skip this one):\n", len(tests), len(hits))
	forEachLimited(w, hits, limit, "    ", moreHint, func(h scanHit) string {
		names := make([]string, 0, len(h.tests))
		for _, t := range h.tests {
			names = append(names, t.Name)
		}
		if limit > 0 && len(names) > namesPerScan {
			names = append(names[:namesPerScan], fmt.Sprintf("+%d", len(h.tests)-namesPerScan))
		}
		return fmt.Sprintf("%s (%s:%d): %s  [INFERRED]", h.scan.Name, h.scan.File, h.scan.Line, strings.Join(names, ", "))
	})
	return len(tests)
}
