package main

import (
	"fmt"
	"sort"
	"strings"
)

// fieldAccess bir alan erişiminin sorgu satırıdır.
type fieldAccessLine struct {
	site  Site
	owner *Node
}

// fieldAccesses bir alan düğümüne gelen okuma ve yazma kenarlarının erişim yerlerini verir
// (read+write iki kenarda da vardır; yinelenenler atılır). mode "read" ya da "write" ise yalnızca
// o tür erişimler kalır.
func (ix *index) fieldAccesses(n *Node, mode string) []fieldAccessLine {
	seen := map[string]bool{}
	var out []fieldAccessLine
	for _, e := range ix.in[n.ID] {
		if e.Kind != edgeReadsField && e.Kind != edgeWritesField {
			continue
		}
		for _, s := range e.Sites {
			key := fmt.Sprintf("%s|%s:%d|%s", e.From, s.File, s.Line, s.Mode)
			if seen[key] {
				continue
			}
			seen[key] = true
			isRead := s.Mode == modeRead || s.Mode == modeReadWrite
			isWrite := s.Mode != modeRead
			if (mode == "read" && !isRead) || (mode == "write" && !isWrite) {
				continue
			}
			out = append(out, fieldAccessLine{s, ix.byID[e.From]})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].site, out[j].site
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return out
}

// printFieldUses bir struct alanının okunduğu ve yazıldığı yerleri yazar: dosya:satır, kip ve
// erişen işlev; func literal içindeyse "işlev > func literal@Lnn". Alanlar go/types ile
// çözülür: başka tiplerin aynı adlı alanları karışmaz.
func (ix *index) printFieldUses(n *Node, o *queryOpts) {
	w := o.stdout
	fmt.Fprintln(w, describe(n))
	if n.Detail != "" {
		fmt.Fprintf(w, "  type %s\n", n.Detail)
	}
	all := ix.fieldAccesses(n, "")
	counts := map[string]int{}
	funcs := map[string]bool{}
	for _, a := range all {
		counts[a.site.Mode]++
		funcs[a.owner.ID] = true
	}
	// Erişimden başka kullanımlar (JSON'un yansımayla okuması: encodes) erişimlerin ardından gruplanır; yalnız onlarla
	// kullanılan alan "hiç okunmuyor" görünmez (profiler.Snapshot.Start yalnız speed.json'a yazılıyor).
	var other []*Edge
	for _, e := range ix.in[n.ID] {
		if e.Kind != edgeReadsField && e.Kind != edgeWritesField && e.Kind != edgeContains {
			other = append(other, e)
		}
	}
	defer ix.printGroups(w, "  <-", other, fromOf, o.limit, true)
	if len(all) == 0 {
		if len(other) == 0 {
			fmt.Fprintln(w, "  never read or written outside its declaration")
		} else {
			fmt.Fprintln(w, "  no direct read or write outside its declaration")
		}
		return
	}
	fmt.Fprintf(w, "  %d accesses in %d functions: %s\n", len(all), len(funcs), modeSummary(counts))
	list := ix.fieldAccesses(n, o.mode)
	lines := make([]string, len(list))
	for i, a := range list {
		who := a.owner.Name
		if a.site.Closure != "" {
			who = ix.byID[a.site.Closure].Name
		}
		lines[i] = fmt.Sprintf("%s:%d  %-10s  %s", a.site.File, a.site.Line, a.site.Mode, who)
	}
	if o.mode != "" {
		fmt.Fprintf(w, "  %s accesses (%d):\n", o.mode, len(list))
	}
	printLimited(w, lines, o.limit, "    ")
}

func modeSummary(counts map[string]int) string {
	var parts []string
	for _, m := range []string{modeRead, modeWrite, modeReadWrite, modeElemWrite, modeAddr, modeLiteral} {
		if counts[m] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[m], m))
		}
	}
	return strings.Join(parts, ", ")
}

// printFieldTable bir tipin alanlarını okuma ve yazma sayılarıyla yazar (`fields <tip>`).
func (ix *index) printFieldTable(t *Node, o *queryOpts) {
	fmt.Fprintln(o.stdout, describe(t))
	var lines []string
	for _, e := range ix.out[t.ID] {
		f := ix.byID[e.To]
		if e.Kind != edgeContains || f.Kind != kindField {
			continue
		}
		all := ix.fieldAccesses(f, "")
		reads, writes := len(ix.fieldAccesses(f, "read")), len(ix.fieldAccesses(f, "write"))
		lines = append(lines, fmt.Sprintf("%s [%s]: %d reads, %d writes (%d accesses)", f.Name[strings.LastIndex(f.Name, ".")+1:], f.Detail, reads, writes, len(all)))
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		fmt.Fprintln(o.stdout, "  no struct fields")
		return
	}
	printLimited(o.stdout, lines, o.limit, "  ")
}

// printCaptures bir işlevin (ya da func literal'in) içindeki func literal'lerin dış kapsamdan
// yakaladığı değişkenleri ve tiplerini yazar; --type yalnızca tipinde o metin geçenleri gösterir.
func (ix *index) printCaptures(n *Node, o *queryOpts) int {
	var closures []*Node
	seen := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		for _, e := range ix.out[id] {
			if c := ix.byID[e.To]; e.Kind == edgeContains && c.Kind == kindClosure && !seen[c.ID] {
				seen[c.ID] = true
				closures = append(closures, c)
				walk(c.ID)
			}
		}
	}
	if n.Kind == kindClosure {
		closures = append(closures, n)
	}
	walk(n.ID)
	sort.SliceStable(closures, func(i, j int) bool { return closures[i].Line < closures[j].Line })
	fmt.Fprintln(o.stdout, describe(n))
	printed := 0
	for _, c := range closures {
		var caps []string
		for _, entry := range c.Captured {
			if o.typeFilter == "" || strings.Contains(entry, o.typeFilter) {
				caps = append(caps, entry)
			}
		}
		if o.typeFilter != "" && len(caps) == 0 {
			continue
		}
		printed++
		fmt.Fprintf(o.stdout, "  %s [%s-%d]\n", c.Name, where(c.File, c.Line), c.End)
		if len(caps) == 0 {
			fmt.Fprintln(o.stdout, "    captures nothing")
		}
		printLimited(o.stdout, caps, o.limit, "    ")
	}
	if printed == 0 {
		fmt.Fprintln(o.stdout, "  no func literals"+map[bool]string{true: " capturing " + o.typeFilter, false: ""}[o.typeFilter != ""])
	}
	return exitOK
}
