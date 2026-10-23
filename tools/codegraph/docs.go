package main

import (
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// walkFiles root altında adı suffix ile biten dosyaların köke göre yollarını verir ('/' ile, sıralı): depo araçlarının
// tek listesi (codeindex.ListFiles: git'in gördüğü dosyalar, izlenmeyen yeni dosyalar dahil; tek atlama listesi).
func walkFiles(root, suffix string) []string {
	return codeindex.ListFiles(root, nil, codeindex.DefaultSkipDirs, true, func(rel string) bool { return strings.HasSuffix(rel, suffix) })
}

func docID(rel string) string           { return "doc:" + rel }
func headingID(rel, slug string) string { return "heading:" + rel + "#" + slug }

// docSet bütün Markdown dosyaları ve başlıklarının kimlikleridir.
type docSet struct {
	docs map[string]*mdDoc
	slug map[string]map[string]string // dosya -> bağlantı parçası -> başlık kimliği
}

// extractMarkdown depodaki bütün .md dosyalarını düğüm yapar (dosya + başlıklar), bağlantıları
// `links_to` kenarına, "Denetim:" maddelerini kural düğümüne çevirir. Go çıkarımından sonra
// çalışır: bağlantılar Go dosya ve paket düğümlerine de gidebilir.
func extractMarkdown(root string, b *builder, idx *goIndex) {
	set := &docSet{docs: map[string]*mdDoc{}, slug: map[string]map[string]string{}}
	rels := walkFiles(root, ".md")
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		d := parseMarkdown(rel, string(data))
		set.docs[rel] = d
		addDocNodes(b, d, set)
	}
	for _, rel := range rels {
		if d := set.docs[rel]; d != nil {
			resolveLinks(root, b, set, d)
			addRules(b, idx, set, d)
		}
	}
}

// ownerID bir başlığın (ya da başlıktan öncesi için dosyanın) düğüm kimliğidir.
func ownerID(d *mdDoc, owner int) string {
	if owner < 0 {
		return docID(d.rel)
	}
	return headingID(d.rel, d.headings[owner].slug)
}

func addDocNodes(b *builder, d *mdDoc, set *docSet) {
	b.addNode(Node{ID: docID(d.rel), Kind: kindDoc, Name: d.rel, File: d.rel, Line: 1})
	set.slug[d.rel] = map[string]string{}
	for i, h := range d.headings {
		id := headingID(d.rel, h.slug)
		end := d.lines // bölüm, aynı ya da üst düzeyden sonraki başlığa kadar sürer
		for _, next := range d.headings[i+1:] {
			if next.level <= h.level {
				end = next.line - 1
				break
			}
		}
		set.slug[d.rel][h.slug] = id
		b.addNode(Node{ID: id, Kind: kindHeading, Name: h.text, File: d.rel, Line: h.line, End: end,
			Detail: d.rel + " h" + strconv.Itoa(h.level)})
		b.addEdge(ownerID(d, h.owner), id, edgeContains, extracted, d.rel, h.line)
	}
}

func resolveLinks(root string, b *builder, set *docSet, d *mdDoc) {
	for _, l := range d.links {
		to, conf, ok := resolveTarget(root, b, set, d.rel, l.target)
		from := ownerID(d, l.owner)
		if !ok {
			if to != "skip" {
				b.warn("broken link: " + d.rel + ":" + strconv.Itoa(l.line) + " -> " + l.target)
			}
			continue
		}
		b.addEdge(from, to, edgeLinksTo, conf, d.rel, l.line)
	}
}

// resolveTarget bir bağlantı hedefini düğüm kimliğine çevirir. ok=false ve to="skip": dış
// bağlantı ya da kökün dışı (sayılmaz); to="": hedef bulunamadı (kırık bağlantı).
func resolveTarget(root string, b *builder, set *docSet, from, target string) (to, conf string, ok bool) {
	target = strings.Trim(target, "<>")
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return "skip", "", false
	}
	pathPart, anchor, _ := strings.Cut(target, "#")
	if u, err := url.PathUnescape(pathPart); err == nil {
		pathPart = u
	}
	p := from
	if pathPart != "" {
		p = path.Join(path.Dir(from), pathPart)
	}
	if p == ".." || strings.HasPrefix(p, "../") {
		return "skip", "", false
	}
	if _, isDoc := set.docs[p]; isDoc {
		if anchor == "" {
			return docID(p), extracted, true
		}
		if id, found := set.slug[p][anchor]; found {
			return id, extracted, true
		}
		return docID(p), inferred, true // bağlantı parçası başlıklarda yok: dosyaya bağla
	}
	if _, found := b.nodes[fileID(p)]; found {
		return fileID(p), extracted, true
	}
	if _, found := b.nodes[trFileID(p)]; found {
		return trFileID(p), extracted, true
	}
	pkg := b.module
	if q := strings.Trim(p, "/"); q != "." && q != "" {
		pkg += "/" + q
	}
	if _, found := b.nodes[pkgID(pkg)]; found {
		return pkgID(pkg), extracted, true
	}
	if _, isDoc := set.docs[path.Join(p, "README.md")]; isDoc {
		return docID(path.Join(p, "README.md")), inferred, true
	}
	st, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
	if err != nil {
		return "", "", false
	}
	detail := ""
	if st.IsDir() {
		detail = "dir"
	}
	b.addNode(Node{ID: "file:" + p, Kind: kindFile, Name: p, File: p, Detail: detail})
	return "file:" + p, extracted, true
}
