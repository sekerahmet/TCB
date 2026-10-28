package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Bu dosya konumla sorgu: `dosya:satır` ve `dosya:satır:sütun` (gopls'in ve tr dizin'in konum biçimi) query, who-uses,
// tests-for ve history'de satırı içeren en içteki düğüme çözülür; `query -` grep'in çıktısını (`yol:satır:metin`) satır
// satır okur ve her satıra o düğümü yazar. Metin araması codegraph'ta yoktur (grep, ripgrep var); bu köprü bulunan
// yeri grafiğe taşır (README, "Reading an answer": metnin içeriği grep ile aranır, sonra bulunan yer grafiğe sorulur).
//
// İçeren düğümün kuralı: satırı kapsayan (ilk satırdan son satıra) düğümlerden en dar olanı; kapanış (func literal)
// kendi işlevine sayılır, alan ve yöntem tipinden içtedir. Bildirimin hemen üstündeki yorum satırları (belge yorumu,
// araya boş satır girmeden) o bildirimindir. Hiçbir bildirimde olmayan satır (iki bildirim arası, içe aktarma) dosyanın
// kendisidir ve bu söylenir. `.tr` dosyasında sütun da verilmişse konum tr dizin'in kuralıyla (codeindex Reader.At:
// imlecin altındaki adın başvurduğu bildirim) çözülür; sütunsuz satır Go'daki gibi içeren bildirimdir.

// locationForm `yol:satır` ya da `yol:satır:sütun`, ardından isteğe bağlı `:metin` (grep -n, rg --vimgrep).
var locationForm = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+))?(?::.*)?$`)

// location bir konum sorgusunun çözümüdür.
type location struct {
	file      string
	line, col int
	node      *Node
	note      string // içeren bildirim yoksa ya da başvuru çözüldüyse açıklama
}

// locate q bir izlenen dosyada konumsa çözer; konum değilse ok=false. exact: q'da satırdan sonra metin olmamalı.
func (ix *index) locate(q string, exact bool) (loc location, ok bool) {
	m := locationForm.FindStringSubmatch(strings.TrimSpace(q))
	if m == nil {
		return loc, false
	}
	if exact && m[0] != strings.Join(nonEmpty(m[1], m[2], m[3]), ":") {
		return loc, false
	}
	file := path.Clean(filepath.ToSlash(m[1]))
	if filepath.IsAbs(m[1]) && ix.root != "" {
		if rel, err := filepath.Rel(ix.root, m[1]); err == nil {
			file = filepath.ToSlash(rel)
		}
	}
	fileNode, isFile := ix.fileTarget(file)
	if !isFile || fileNode == nil {
		return loc, false
	}
	loc = location{file: fileNode.File, node: fileNode}
	loc.line, _ = strconv.Atoi(m[2])
	loc.col, _ = strconv.Atoi(m[3])
	if loc.col > 0 && strings.HasSuffix(loc.file, ".tr") {
		if n := ix.trReference(loc); n != nil {
			loc.node, loc.note = n, "the name at that column refers to it (tr dizin's rule)"
			return loc, true
		}
	}
	if n := ix.containing(loc.file, loc.line); n != nil {
		loc.node = n
	} else {
		loc.note = fmt.Sprintf("line %d is in no declaration (between declarations, imports or the package clause): answering for the file", loc.line)
	}
	return loc, true
}

func nonEmpty(parts ...string) []string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// containing satırı içeren en dar düğümdür; yoksa satırın altındaki yorum bloğunun belgelediği bildirim; o da yoksa nil.
func (ix *index) containing(file string, line int) *Node {
	var best *Node
	starts := map[int]*Node{}
	for i := range ix.g.Nodes {
		n := &ix.g.Nodes[i]
		if n.File != file || n.Line <= 0 || !locatable(n.Kind) {
			continue
		}
		end := max(n.End, n.Line)
		if s := starts[n.Line]; s == nil || max(s.End, s.Line) < end {
			starts[n.Line] = n // aynı satırda başlayanlardan en genişi (tip, alanı değil) belgenin sahibidir
		}
		if n.Line <= line && line <= end && (best == nil || end-n.Line < max(best.End, best.Line)-best.Line) {
			best = n
		}
	}
	if best != nil {
		return best
	}
	lines := ix.fileLines(file)
	for l := line; l >= 1 && l <= len(lines); l++ {
		text := strings.TrimSpace(lines[l-1])
		if n := starts[l]; n != nil && l > line {
			return n
		}
		if !strings.HasPrefix(text, "//") {
			return nil // boş satır ya da kod: belge yorumu değil
		}
	}
	return nil
}

// locatable konumla bulunan düğüm türleridir: bildirimler ve belge bölümleri; dosya, paket, kapanış (kendi işlevine
// sayılır) ve tarama değil.
func locatable(kind string) bool {
	switch kind {
	case kindFile, kindPackage, kindClosure, kindScan, kindDoc:
		return false
	}
	return true
}

// fileLines dosyanın satırlarıdır (okunamazsa boş).
func (ix *index) fileLines(file string) []string {
	data, err := os.ReadFile(filepath.Join(ix.root, filepath.FromSlash(file)))
	if err != nil {
		return nil
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
}

// trReference `.tr` konumundaki adın başvurduğu bildirimin düğümüdür (codeindex Reader.At, tr dizin'in konum çözümü).
func (ix *index) trReference(loc location) *Node {
	idx, err := codeindex.Build(ix.root, codeindex.Options{Dirs: trRoots, Tracked: true})
	if err != nil {
		return nil
	}
	_, n := codeindex.NewReader(idx).At(loc.file, loc.line, loc.col)
	if n == nil {
		return nil
	}
	return ix.byID[graphID(*n)]
}

// printLocations grep çıktısını (`yol:satır[:sütun]:metin`) satır satır okur ve her satıra içeren düğümü yazar;
// konum olmayan satır olduğu gibi kalır (`query -`).
func (ix *index) printLocations(r io.Reader, w io.Writer) int {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		loc, ok := ix.locate(line, false)
		switch {
		case !ok:
			fmt.Fprintf(w, "%s\n    (not a location in a tracked file)\n", line)
		case loc.node.Kind == kindFile:
			fmt.Fprintf(w, "%s\n    %s  (no declaration)\n", line, describe(loc.node))
		default:
			fmt.Fprintf(w, "%s\n    %s\n", line, describe(loc.node))
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(w, "codegraph:", err)
		return exitFail
	}
	return exitOK
}
