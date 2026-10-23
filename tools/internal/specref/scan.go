// Package specref dil tanımı atıflarını bulur ve denetler: `docs/spec/NN_*.md` dosyalarındaki numaralı
// maddeler (`**15.3** ...`), bunlara yapılan atıflar (`**15.3**`, `dil tanımı 15.3`, `§15.3`, bağlantı
// metni `[15.3](...spec/...)`, spec dosyalarında parantez içi anmalar), numaradan sonra gelen Türkçe
// eklerin ses uyumu ve madde yeniden numaralama planı. `tools/codegraph spec-refs` denetler,
// `tools/renumber` yeniden numaralar; ikisi aynı tarayıcıyı kullanır.
//
// Layer: tools (tools); kullanıcıya gitmez.
package specref

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// File taranan bir dosyadır (Path köke göre, '/' ile).
type File struct {
	Path string
	Text string
}

// skipDirs taranmayan klasör adlarıdır.
var skipDirs = map[string]bool{".git": true, ".claude": true, ".codegraph": true, "node_modules": true, "testdata": true, "vendor": true}

// Collect kökteki .md, .go ve .tr dosyalarını okur: `.git` varsa yalnızca izlenenler (git ls-files; depo
// kökündeki izlenmeyen dosyalar kullanıcınındır), yoksa klasör dolaşılır. testdata taranmaz.
func Collect(root string) ([]File, error) {
	rels, ok := tracked(root)
	if !ok {
		rels = nil
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if p != root && skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			r, _ := filepath.Rel(root, p)
			rels = append(rels, filepath.ToSlash(r))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(rels)
	var out []File
	for _, r := range rels {
		switch strings.ToLower(filepath.Ext(r)) {
		case ".md", ".go", ".tr":
		default:
			continue
		}
		if inSkipped(r) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(r)))
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		out = append(out, File{Path: r, Text: string(data)})
	}
	return out, nil
}

// inSkipped yolun taranmayan bir klasörde olup olmadığı.
func inSkipped(rel string) bool {
	for _, part := range strings.Split(rel, "/")[:strings.Count(rel, "/")] {
		if skipDirs[part] {
			return true
		}
	}
	return false
}

// tracked git ls-files çıktısıdır; git yoksa ya da kök bir depo değilse ok=false.
func tracked(root string) ([]string, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, false
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return nil, false
	}
	var rels []string
	for _, n := range bytes.Split(out, []byte{0}) {
		if len(n) > 0 {
			rels = append(rels, string(n))
		}
	}
	return rels, true
}

// Item dil tanımının numaralı bir maddesidir.
type Item struct {
	ID    string // "15.3"
	File  string
	Line  int
	Title string // maddenin ilk sözcükleri (iki noktaya kadar)
}

// Index spec dosyalarındaki maddelerdir.
type Index struct {
	Items    map[string]Item
	Order    []string     // maddeler dosyadaki sırasıyla
	Chapters map[int]bool // en az bir numaralı maddesi olan bölümler
	Dups     []Item       // aynı numara ikinci kez tanımlanmış
}

var itemLine = regexp.MustCompile(`^\*\*(\d{1,2}\.\d{1,3}[a-z]?)\*\*\s*(.*)$`)

// IsSpecFile yol dil tanımı dosyası mı (docs/spec/NN_ad.md).
func IsSpecFile(path string) bool {
	return strings.HasPrefix(path, "docs/spec/") && strings.HasSuffix(path, ".md") && len(path) > len("docs/spec/NN_")
}

// BuildIndex spec dosyalarındaki satır başı `**N.M** ...` maddelerini toplar.
func BuildIndex(files []File) *Index {
	ix := &Index{Items: map[string]Item{}, Chapters: map[int]bool{}}
	for _, f := range files {
		if !IsSpecFile(f.Path) {
			continue
		}
		for i, line := range strings.Split(f.Text, "\n") {
			m := itemLine.FindStringSubmatch(strings.TrimRight(line, "\r"))
			if m == nil {
				continue
			}
			it := Item{ID: m[1], File: f.Path, Line: i + 1, Title: title(m[2])}
			if _, dup := ix.Items[it.ID]; dup {
				ix.Dups = append(ix.Dups, it)
				continue
			}
			ix.Items[it.ID] = it
			ix.Order = append(ix.Order, it.ID)
			ix.Chapters[chapterOf(it.ID)] = true
		}
	}
	return ix
}

// chapterOf "15.3" için 15.
func chapterOf(id string) int {
	n := 0
	for _, c := range id {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// title maddenin başlığıdır: ilk iki noktaya ya da cümle sonuna, yoksa sekiz sözcüğe kadar.
func title(s string) string {
	if i := strings.Index(s, ": "); i > 0 && i < 80 {
		return s[:i]
	}
	if i := strings.Index(s, ". "); i > 0 && i < 80 {
		return s[:i]
	}
	words := strings.Fields(s)
	if len(words) > 8 {
		words = append(words[:8], "…")
	}
	return strings.Join(words, " ")
}

// Token bir madde numarasının dosyadaki yeridir (bayt aralığı).
type Token struct {
	ID         string
	Start, End int
}

// Mention bir atıf (ya da madde tanımı) yeridir.
type Mention struct {
	File   string
	Line   int
	Kind   string // "definition", "strong" (açık atıf biçimi), "weak" (spec dosyalarında parantez içi anma)
	Tokens []Token
	Range  bool   // iki belirteç arasında aralık işareti var
	Style  string // "definition", "bold-inner" (**A–B**), "bold-split" (**A**–**B**), "keyword", "link", "weak"
	Start  int    // anmanın dosyadaki bayt aralığı
	End    int
	Suffix string // anmanın hemen ardından gelen kesme işaretli ek (kesme ve harfler), yoksa ""
	SufPos int    // ekin dosyadaki başlangıcı (kesme işaretinden sonra)
}

const tok = `\d{1,2}\.\d{1,3}[a-z]?`

var (
	reBold    = regexp.MustCompile(`\*\*(` + tok + `)(?:\s*([–-])\s*(` + tok + `))?\*\*(?:\s*([–-])\s*\*\*(` + tok + `)\*\*)?`)
	reKeyword = regexp.MustCompile(`(?i:dil tan.m\p{L}*|§)\s*\[?(` + tok + `)(?:\]\([^)]*\))?(?:\s*(?:,|ve|[–-])\s*(?:\[?` + tok + `)(?:\]\([^)]*\))?)*`)
	reLink    = regexp.MustCompile(`\[(` + tok + `)\]\([^)]*spec/[^)]*\)`)
	reToken   = regexp.MustCompile(tok)
	reWeak    = regexp.MustCompile(`(?:^|[\s(,;–-])(` + tok + `)`)
	reSuffix  = regexp.MustCompile(`^['’](\p{L}+)`)
)

// Scan bütün dosyalardaki atıfları ve madde tanımlarını bulur. Açık biçimler her dosyada aranır; spec
// dosyalarında ayrıca parantez içi ve virgüllü anmalar "weak" olarak alınır.
func Scan(files []File, ix *Index) []Mention {
	var out []Mention
	for _, f := range files {
		offset := 0
		fence := false
		for i, line := range strings.SplitAfter(f.Text, "\n") {
			body := strings.TrimRight(line, "\r\n")
			out = append(out, scanLine(f.Path, i+1, offset, body, maskCode(f.Path, body, &fence), ix)...)
			offset += len(line)
		}
	}
	return out
}

// lineScan bir satırın tarama durumudur.
type lineScan struct {
	path    string
	no      int
	offset  int // satırın dosyadaki bayt başlangıcı
	line    string
	covered []bool
	out     []Mention
}

// add bir anma kaydeder; belirteç konumları dosya düzeyine çevrilir.
func (s *lineScan) add(kind string, start, end int, rng bool, toks []Token) {
	for i := start; i < end && i < len(s.covered); i++ {
		s.covered[i] = true
	}
	m := Mention{File: s.path, Line: s.no, Style: kind, Tokens: toks, Range: rng, Start: s.offset + start, End: s.offset + end}
	for k := range m.Tokens {
		m.Tokens[k].Start += s.offset
		m.Tokens[k].End += s.offset
	}
	if sm := reSuffix.FindStringSubmatchIndex(s.line[end:]); sm != nil {
		m.Suffix = s.line[end+sm[2] : end+sm[3]]
		m.SufPos = s.offset + end + sm[2]
	}
	m.Kind = "strong"
	switch kind {
	case "definition":
		m.Kind = "definition"
	case "weak":
		m.Kind = "weak"
	}
	s.out = append(s.out, m)
}

// maskCode kod olan kısımları boşlukla örter (bayt uzunluğu korunur): numaralar yalnızca düz metinde atıftır,
// kod içinde (Markdown'da `...` aralıkları ve çit bloklar, Go'da // öncesi) örnek ya da veridir. fence, açık
// çit bloğunun durumunu satırdan satıra taşır.
func maskCode(path, line string, fence *bool) string {
	blank := func(s string) string { return strings.Repeat(" ", len(s)) }
	switch {
	case strings.HasSuffix(path, ".md"):
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			*fence = !*fence
			return blank(line)
		}
		if *fence {
			return blank(line)
		}
		var b strings.Builder
		in := false
		for _, part := range strings.SplitAfter(line, "`") {
			if in {
				b.WriteString(blank(part))
			} else {
				b.WriteString(part)
			}
			if strings.HasSuffix(part, "`") {
				in = !in
			}
		}
		return b.String()
	case strings.HasSuffix(path, ".go"):
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") {
			return line
		}
		if i := strings.Index(line, "//"); i >= 0 {
			return blank(line[:i]) + line[i:]
		}
		return blank(line)
	}
	return line
}

// scanLine bir satırdaki atıfları verir; offset satırın dosyadaki bayt başlangıcıdır.
func scanLine(path string, no, offset int, raw, line string, ix *Index) []Mention {
	if !hasNumberDotNumber(raw) {
		return nil // madde numarası içeremeyen satırda düzenli ifadeleri çalıştırmanın anlamı yok (satırların çoğu)
	}
	s := &lineScan{path: path, no: no, offset: offset, line: line, covered: make([]bool, len(line)+1)}
	if m := itemLine.FindStringSubmatchIndex(raw); m != nil && IsSpecFile(path) {
		s.add("definition", m[2]-2, m[3]+2, false, []Token{{ID: raw[m[2]:m[3]], Start: m[2], End: m[3]}})
	}
	for _, m := range reBold.FindAllStringSubmatchIndex(line, -1) {
		if overlaps(s.covered, m[0], m[1]) {
			continue
		}
		toks := []Token{{line[m[2]:m[3]], m[2], m[3]}}
		style := "bold-inner"
		if m[6] >= 0 { // **A–B**
			toks = append(toks, Token{line[m[6]:m[7]], m[6], m[7]})
		}
		if m[10] >= 0 { // **A**–**B**
			toks = append(toks, Token{line[m[10]:m[11]], m[10], m[11]})
			style = "bold-split"
		}
		s.add(style, m[0], m[1], len(toks) > 1, toks)
	}
	for _, m := range reKeyword.FindAllStringIndex(line, -1) {
		if !overlaps(s.covered, m[0], m[1]) {
			toks, rng := listTokens(line, m[0], m[1])
			s.add("keyword", m[0], m[1], rng, toks)
		}
	}
	for _, m := range reLink.FindAllStringSubmatchIndex(line, -1) {
		if !overlaps(s.covered, m[0], m[1]) {
			s.add("link", m[0], m[1], false, []Token{{line[m[2]:m[3]], m[2], m[3]}})
		}
	}
	s.weak(ix)
	return s.out
}

func overlaps(covered []bool, a, b int) bool {
	for i := a; i < b && i < len(covered); i++ {
		if covered[i] {
			return true
		}
	}
	return false
}

// listTokens bir anma aralığındaki bütün belirteçleri verir; ardışık iki belirteç arasında aralık işareti
// varsa rng doğrudur.
func listTokens(line string, start, end int) (toks []Token, rng bool) {
	for _, m := range reToken.FindAllStringIndex(line[start:end], -1) {
		toks = append(toks, Token{ID: line[start+m[0] : start+m[1]], Start: start + m[0], End: start + m[1]})
	}
	for k := 0; k+1 < len(toks); k++ {
		if strings.ContainsAny(line[toks[k].End:toks[k+1].Start], "–-") {
			rng = true
		}
	}
	return toks, rng
}

// weak açık biçimi olmayan, ama var olan bir bölümün maddesine benzeyen anmaları ekler (örn. "(karışık hassasiyet,
// 15.3)", Go yorumunda "(docs/spec/N_x.md, N.M)"); bölümün numaralı maddesi yoksa (1 ve 14) sayılmaz. Spec dışında yalnızca var olan
// bir maddenin numarası anma sayılır: "15.5 ms" gibi ondalıklar olmayan maddeyi ya da madde olmayanı anmaz.
func (s *lineScan) weak(ix *Index) {
	for _, m := range reWeak.FindAllStringSubmatchIndex(s.line, -1) {
		a, b := m[2], m[3]
		id := s.line[a:b]
		if overlaps(s.covered, a, b) || !standsAlone(s.line, a, b) || !ix.Chapters[chapterOf(id)] || looksNumeric(id) {
			continue
		}
		if _, exists := ix.Items[id]; !exists && !IsSpecFile(s.path) {
			continue // spec dışında çıplak sayı yalnızca var olan bir maddeyse anmadır (sürüm ve ondalık sayılardan ayrılır)
		}
		s.add("weak", a, b, false, []Token{{s.line[a:b], a, b}})
	}
}

// looksNumeric zayıf anma bir madde numarasından çok ondalık sayıya benziyor mu ("3.0", "2.7e-3"): madde
// numaraları 0 ile başlamaz ve zayıf anmada harfli sonek taşımaz.
func looksNumeric(id string) bool {
	m := id[strings.IndexByte(id, '.')+1:]
	return m[0] == '0' || m[len(m)-1] >= 'a'
}

// standsAlone belirteç daha büyük bir sayının parçası değil mi ("15.30.1", "0.15.3" ya da "15.3000").
func standsAlone(line string, s, e int) bool {
	if s > 0 && (line[s-1] == '.' || line[s-1] >= '0' && line[s-1] <= '9') {
		return false
	}
	if e < len(line) && (line[e] >= '0' && line[e] <= '9' || line[e] == '.' && e+1 < len(line) && line[e+1] >= '0' && line[e+1] <= '9') {
		return false
	}
	return true
}

// hasNumberDotNumber satırda "rakam.rakam" geçiyor mu: her madde numarası (N.M) bunu içerir; ucuz bir ön süzgeçtir.
func hasNumberDotNumber(s string) bool {
	for i := 1; i+1 < len(s); i++ {
		if s[i] == '.' && s[i-1] >= '0' && s[i-1] <= '9' && s[i+1] >= '0' && s[i+1] <= '9' {
			return true
		}
	}
	return false
}
