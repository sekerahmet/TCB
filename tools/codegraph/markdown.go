package main

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// mdHeading bir Markdown başlığıdır; owner üst başlığın sırasıdır (-1: yok).
type mdHeading struct {
	level int
	text  string
	line  int
	slug  string
	owner int
}

// mdLink bir bağlantıdır; owner onu içeren başlığın sırasıdır (-1: başlıktan önce).
type mdLink struct {
	target string
	line   int
	owner  int
}

// mdRule içinde "Denetim:" geçen bir madde ya da "Kural | Test" tablosunun bir satırıdır.
// guards, kuralı koruyan testlerin yazıldığı metindir.
type mdRule struct {
	line   int
	owner  int
	text   string
	guards string
}

// mdDoc ayrıştırılmış bir Markdown dosyasıdır.
type mdDoc struct {
	rel      string
	headings []mdHeading
	links    []mdLink
	rules    []mdRule
	lines    int // dosyanın satır sayısı
}

var (
	headingLine = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	linkTarget  = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	linkInText  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	codeSpan    = regexp.MustCompile("`[^`]*`")
	listItem    = regexp.MustCompile(`^\s*([-*+]|\d+\.)\s`)
	guardMarker = "Denetim:"
)

// slugify GitHub'ın başlık bağlantısı kuralıdır: küçük harf, harf-rakam-tire-alt çizgi kalır,
// boşluk tire olur, diğer işaretler düşer.
func slugify(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			sb.WriteRune(r)
		case r == ' ':
			sb.WriteByte('-')
		}
	}
	return sb.String()
}

// headingText başlığın düz metnidir: bağlantılar metnine, kod aralıkları içeriğine iner.
func headingText(s string) string {
	s = linkInText.ReplaceAllString(s, "$1")
	s = strings.NewReplacer("`", "", "*", "").Replace(s)
	return strings.TrimSpace(s)
}

// parseMarkdown başlıkları, bağlantıları ve denetim kurallarını çıkarır; kod çitlerinin içine
// bakılmaz.
func parseMarkdown(rel, text string) *mdDoc {
	d := &mdDoc{rel: rel}
	p := &mdParser{doc: d, owner: -1, seen: map[string]int{}}
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		n := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			p.flush()
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		p.line(n, line, trimmed)
	}
	p.flush()
	d.lines = strings.Count(text, "\n") + 1
	return d
}

// mdParser satır satır ayrıştırma durumudur.
type mdParser struct {
	doc      *mdDoc
	owner    int
	seen     map[string]int
	stack    []int // açık başlıkların sıraları
	blockAt  int
	block    []string
	tableHdr []string // geçerli "Kural | Test" tablosunun başlık hücreleri
	tblState int      // 0: tablo dışı, 1: başlık görüldü, 2: gövde
}

func (p *mdParser) line(n int, raw, trimmed string) {
	if m := headingLine.FindStringSubmatch(raw); m != nil {
		p.flush()
		p.endTable()
		p.heading(n, len(m[1]), headingText(m[2]))
		return
	}
	for _, m := range linkTarget.FindAllStringSubmatch(codeSpan.ReplaceAllString(raw, ""), -1) {
		p.doc.links = append(p.doc.links, mdLink{target: m[1], line: n, owner: p.owner})
	}
	switch {
	case strings.HasPrefix(trimmed, "|"):
		p.flush()
		p.tableLine(n, trimmed)
		return
	case trimmed == "":
		p.flush()
		p.endTable()
		return
	}
	p.endTable()
	if listItem.MatchString(raw) {
		p.flush()
	}
	if len(p.block) == 0 {
		p.blockAt = n
	}
	p.block = append(p.block, trimmed)
}

func (p *mdParser) endTable() { p.tblState, p.tableHdr = 0, nil }

func (p *mdParser) heading(n, level int, text string) {
	for len(p.stack) > 0 && p.doc.headings[p.stack[len(p.stack)-1]].level >= level {
		p.stack = p.stack[:len(p.stack)-1]
	}
	owner := -1
	if len(p.stack) > 0 {
		owner = p.stack[len(p.stack)-1]
	}
	slug := slugify(text)
	if c := p.seen[slug]; c > 0 {
		p.seen[slug] = c + 1
		slug += "-" + strconv.Itoa(c)
	} else {
		p.seen[slug] = 1
	}
	p.doc.headings = append(p.doc.headings, mdHeading{level: level, text: text, line: n, slug: slug, owner: owner})
	p.stack = append(p.stack, len(p.doc.headings)-1)
	p.owner = len(p.doc.headings) - 1
}

// flush biriken paragraf ya da liste maddesini kapatır; "Denetim:" geçiyorsa kural olur.
func (p *mdParser) flush() {
	if len(p.block) == 0 {
		return
	}
	text := strings.Join(p.block, " ")
	if i := strings.Index(text, guardMarker); i >= 0 {
		p.doc.rules = append(p.doc.rules, mdRule{line: p.blockAt, owner: p.owner, text: text, guards: text[i+len(guardMarker):]})
	}
	p.block = nil
}

// tableLine tablo satırını işler: başlıkta "Kural" ve "Test" sütunu olan tabloların her
// gövde satırı bir kuraldır.
func (p *mdParser) tableLine(n int, line string) {
	cells := splitCells(line)
	switch p.tblState {
	case 0:
		p.tblState, p.tableHdr = 1, cells
	case 1:
		p.tblState = 2
	default:
		rule, test := -1, -1
		for i, h := range p.tableHdr {
			h = strings.ToLower(h)
			if strings.HasPrefix(h, "kural") && rule < 0 {
				rule = i
			}
			if strings.HasPrefix(h, "test") && test < 0 {
				test = i
			}
		}
		if rule >= 0 && test >= 0 && rule < len(cells) && test < len(cells) {
			p.doc.rules = append(p.doc.rules, mdRule{line: n, owner: p.owner, text: cells[rule], guards: cells[test]})
		}
	}
}

func splitCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
