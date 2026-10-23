package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// Bu dosya `tests-for` çıktısında her Go testinin altına, testin koruduğu kuralın ilk satırını yazar. Kural düğümü
// olmayan testlerde bilgi test yorumunda durur: kaynak önce testin doc yorumudur, yoksa test dosyasının baş
// yorumudur (içe aktarmalardan sonraki ilk `//` bloğu). Yorum grafikte saklanmaz, sorgu anında dosyadan okunur;
// böylece grafik içeriği ve extractorRevision değişmez.

// maxNoteRunes bir not satırının en çok karakteri.
const maxNoteRunes = 160

// withTestNotes tests-for çıktısındaki Go testlerinin altına not yazar (root: depo kökü).
func (ix *index) withTestNotes(root string) {
	files := map[string][]string{}
	ix.note = func(n *Node) string {
		if !isTestNode(n) {
			return ""
		}
		lines, ok := files[n.File]
		if !ok {
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.File))); err == nil {
				lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
			}
			files[n.File] = lines
		}
		note := docFirstLine(lines, n.Line)
		if note == "" {
			note = headFirstLine(lines)
		}
		return note
	}
}

// docFirstLine satır numarası line (1'den) olan bildirimin hemen üstündeki `//` bloğunun ilk satırıdır.
func docFirstLine(lines []string, line int) string {
	decl := line - 1 // bildirimin satır dizisindeki indeksi
	if decl <= 0 || decl >= len(lines) {
		return ""
	}
	i := decl - 1
	for i >= 0 && strings.HasPrefix(strings.TrimSpace(lines[i]), "//") {
		i--
	}
	if i == decl-1 {
		return ""
	}
	return commentText(lines[i+1])
}

// headFirstLine dosyanın baş yorumudur: `package` ve içe aktarmalardan sonraki ilk `//` bloğunun ilk satırı
// (`//go:` yönergeleri sayılmaz).
func headFirstLine(lines []string) string {
	pastPackage, inImport := false, false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case !pastPackage:
			pastPackage = strings.HasPrefix(t, "package ")
		case inImport:
			inImport = t != ")"
		case strings.HasPrefix(t, "import ("):
			inImport = true
		case strings.HasPrefix(t, "import "):
		case strings.HasPrefix(t, "//go:"), t == "":
		case strings.HasPrefix(t, "//"):
			return commentText(t)
		default:
			return "" // yorumsuz ilk bildirim
		}
	}
	return ""
}

// commentText `// metin` satırının metnidir, uzunsa kesilir.
func commentText(line string) string {
	t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "//"))
	if utf8.RuneCountInString(t) > maxNoteRunes {
		t = string([]rune(t)[:maxNoteRunes]) + "…"
	}
	return t
}

// walkEdges n'yi bütün listesini gezerek koruyan testlerin kenarlarıdır: n bir veri dosyasıysa kendisine, bir ad ya
// da ileti kodu ise tanımlandığı dosya(lar)a gelen `walks` kenarları. Kod için tek bir girdisini arayan testler
// değil, listeyi gezen testler gösterilir: her girdiyi o korur.
func (ix *index) walkEdges(n *Node) []*Edge {
	var out []*Edge
	collect := func(id string) {
		for _, e := range ix.in[id] {
			if e.Kind == edgeWalks {
				out = append(out, e)
			}
		}
	}
	collect(n.ID)
	if n.Kind == kindName || n.Kind == kindMessage {
		for _, e := range ix.in[n.ID] {
			if e.Kind == edgeContains && ix.byID[e.From].Kind == kindFile {
				collect(e.From)
			}
		}
	}
	return out
}

// printWalks gezen testleri yazar; aynı test birden çok dosyayı geziyorsa (tr.json ve en.json) bir kez, dosyalarıyla.
func (ix *index) printWalks(w io.Writer, walks []*Edge, limit int) {
	if len(walks) == 0 {
		return
	}
	files := map[string][]string{}
	var firsts []*Edge
	for _, e := range walks {
		if _, seen := files[e.From]; !seen {
			firsts = append(firsts, e)
		}
		if name := path.Base(ix.byID[e.To].Name); !slices.Contains(files[e.From], name) {
			files[e.From] = append(files[e.From], name)
		}
	}
	sort.SliceStable(firsts, func(i, j int) bool { return ix.byID[firsts[i].From].Name < ix.byID[firsts[j].From].Name })
	fmt.Fprintf(w, "  <- tests that walk the whole list (%d, each entry is covered by them):\n", len(firsts))
	forEachLimited(w, firsts, limit, "    ", moreHint, func(e *Edge) string {
		sort.Strings(files[e.From])
		return ix.edgeLine(e, e.From, true) + "  (" + strings.Join(files[e.From], ", ") + ")"
	})
}
