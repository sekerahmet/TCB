package main

import (
	"os"
	"path/filepath"
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
