package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	datePattern = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2}) ([0-9]{2}:[0-9]{2}) \+0300$`)
	movedDate   = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2}) ` + regexp.QuoteMeta(raw("noTime")) + `$`)
	fullHash    = regexp.MustCompile(`^[0-9a-f]{40}$`)
	dashReason  = regexp.MustCompile(`^— \(.+\)$`)
	modelItem   = regexp.MustCompile(`^(\S+) ([0-9a-f]{12}|` + regexp.QuoteMeta(raw("noSum")) + `)$`)
	movedSource = regexp.MustCompile(regexp.QuoteMeta(raw("movedFrom")) + ` \S+@[0-9a-f]{7,40}:[0-9]+`)
	// programMarker programsız ya da programı bilinmeyen satırın Yollar işaretidir.
	programMarker = regexp.MustCompile(`— \(` + regexp.QuoteMeta(raw("programNone")) + ` [^)]+\)|` + regexp.QuoteMeta(raw("programUnknown")))
)

var (
	engineFree     = raw("engineFree")
	programUnknown = raw("programUnknown")
)

// localPaths klasörde yasak olan yerel yol ve kişi biçimleridir: sürücü harfli yol (harfin
// önünde harf ya da rakam olmaz, yoksa https:// da yakalanırdı), Git Bash'in /c/'si, kullanıcı
// klasörleri, Colab'ın ve Linux'un ev klasörleri, scratchpad ve "<Ad>'ın makinesi".
var localPaths = []*regexp.Regexp{
	regexp.MustCompile(`(^|[^A-Za-z0-9])[A-Za-z]:\\`),
	regexp.MustCompile(`(^|[^A-Za-z0-9])[A-Za-z]:/`),
	regexp.MustCompile(`(^|[^A-Za-z0-9_./-])/[A-Za-z]/`),
	regexp.MustCompile(`%USERPROFILE%|%LOCALAPPDATA%|Users\\|Users/|~/\.claude|/content/|/home/`),
	regexp.MustCompile(`(?i)scratchpad`),
	regexp.MustCompile(raw("personPattern")),
}

// Check klasörün biçim yanlışlarını verir: okurken bulunanlar, satırların alanları, bölüm
// sırası, yerel yollar.
func Check(f *Folder) []Problem {
	c := &checker{f: f, ids: map[string]Row{}}
	for _, r := range f.Rows {
		if prev, dup := c.ids[r.ID()]; dup {
			c.add(r, "duplicateID", r.ID(), prev.File, prev.Line)
		}
		c.ids[r.ID()] = r
	}
	sections := map[[2]string]bool{}
	for _, r := range f.Rows {
		key := [2]string{r.File, r.Section}
		if !sections[key] {
			sections[key] = true
			if _, ok := f.machineByTitle(r.Section); !ok {
				c.add(r, "unknownSection", r.Section)
			}
		}
		c.row(r)
	}
	c.order()
	for file, lines := range f.Lines {
		for i, line := range lines {
			for _, p := range localPaths {
				if m := p.FindString(line); m != "" {
					f.problem(file, i+1, "localPath", strings.TrimSpace(m))
				}
			}
		}
	}
	return f.Problems
}

type checker struct {
	f   *Folder
	ids map[string]Row
}

func (c *checker) add(r Row, key string, args ...any) {
	c.f.problem(r.File, r.Line, key, args...)
}

// row bir satırın alanlarını denetler.
func (c *checker) row(r Row) {
	for i, cell := range r.Cells {
		if cell == "" || cell == "—" && i != cReplacedBy {
			c.add(r, "emptyCell", i+1)
		}
	}
	c.identity(r)
	machine, _ := c.f.machineByTitle(r.Section)
	device := r.Cells[cDevice]
	kind, name, ok := strings.Cut(device, ", ")
	if !ok || kind != raw("cpu") && kind != raw("gpu") || strings.TrimSpace(name) == "" {
		c.add(r, "deviceForm", device)
	} else if machine.Kind != "" && kind != machine.Kind {
		c.add(r, "deviceKind", kind, machine.Kind)
	}
	free := r.Cells[cPaths] == engineFree
	commit := r.Cells[cCommit]
	if !fullHash.MatchString(commit) && !(free && dashReason.MatchString(commit)) {
		c.add(r, "commitForm", commit)
	}
	c.paths(r)
	if free && strings.HasPrefix(r.Cells[cOutOfRepo], "—") {
		c.add(r, "freeNeedsVersions")
	}
	if !dashReason.MatchString(r.Cells[cOutOfRepo]) {
		for _, item := range strings.Split(r.Cells[cOutOfRepo], "; ") {
			if !strings.Contains(strings.TrimSpace(item), " ") {
				c.add(r, "versionForm", item)
			}
		}
	}
	c.model(r)
	c.moved(r)
	c.replacedBy(r)
}

// identity kimliği ve onunla uyuşması gereken Tarih ve Oturum'u denetler.
func (c *checker) identity(r Row) {
	m := idPattern.FindStringSubmatch(r.ID())
	if m == nil {
		c.add(r, "idForm", r.ID())
		return
	}
	if prefix := filePrefix(r.File); m[1] != prefix {
		c.add(r, "idPrefix", m[1], prefix)
	}
	day, _, ok := rowDate(r)
	if !ok {
		c.add(r, "dateForm", r.Cells[cDate])
	} else if day != m[2] {
		c.add(r, "idDay", m[2], day)
	}
	if machine, ok := c.f.machineByTitle(r.Section); ok && m[3] != machine.Code {
		c.add(r, "idMachine", m[3], machine.Code)
	}
	if !c.f.Sessions[m[4]] {
		c.add(r, "unknownSession", m[4])
	}
	if r.Cells[cSession] != m[4] {
		c.add(r, "sessionMismatch", r.Cells[cSession], m[4])
	}
}

func filePrefix(file string) string {
	for _, m := range measureFiles {
		if file == m.path {
			return m.prefix
		}
	}
	return ""
}

// rowDate satırın gününü ve saatini verir (saat yazılmamışsa "").
func rowDate(r Row) (day, clock string, ok bool) {
	if m := datePattern.FindStringSubmatch(r.Cells[cDate]); m != nil {
		return m[1], m[2], true
	}
	if m := movedDate.FindStringSubmatch(r.Cells[cDate]); m != nil {
		return m[1], "", true
	}
	return "", "", false
}

// newer a'nın b'den kesin olarak yeni olup olmadığını söyler; aynı gün saati bilinmeyen satır
// için sıra bilinmez (false).
func newer(a, b Row) bool {
	ad, at, _ := rowDate(a)
	bd, bt, _ := rowDate(b)
	if ad != bd {
		return ad > bd
	}
	return at != "" && bt != "" && at > bt
}

// paths Yollar hücresini denetler: kümeler, programlar, engine ve program zorunluluğu.
func (c *checker) paths(r Row) {
	cell := r.Cells[cPaths]
	if cell == engineFree {
		return
	}
	marker := programMarker.FindString(cell)
	rest := strings.TrimSpace(strings.Trim(strings.Replace(cell, marker, "", 1), ", "))
	sets, programs := 0, 0
	hasEngine := false
	for _, item := range strings.Split(rest, ", ") {
		item = strings.TrimSpace(item)
		switch {
		case item == "":
		case strings.HasSuffix(item, ".tr"):
			programs++
			if !c.exists(item) {
				c.add(r, "programMissing", item)
			}
		case c.f.Sets[item] != nil:
			sets++
			hasEngine = hasEngine || item == "engine"
			for _, p := range c.f.Sets[item] {
				if !c.exists(p) {
					c.add(r, "setPathMissing", item, p)
				}
			}
		default:
			c.add(r, "unknownItem", item)
		}
	}
	if !hasEngine {
		c.add(r, "needEngine", engineFree)
	}
	if programs == 0 && marker == "" {
		c.add(r, "needProgram", programUnknown)
	}
}

func (c *checker) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(c.f.Root, filepath.FromSlash(strings.TrimSuffix(rel, "/"))))
	return err == nil
}

// model Model hücresini denetler.
func (c *checker) model(r Row) {
	cell := r.Cells[cModel]
	if dashReason.MatchString(cell) {
		return
	}
	for _, item := range strings.Split(cell, "; ") {
		if !modelItem.MatchString(item) {
			c.add(r, "modelForm", item)
		}
	}
}

// moved taşınan satırın Kaynak'ını denetler.
func (c *checker) moved(r Row) {
	moved := r.Cells[cSession] == raw("movedSession") || movedDate.MatchString(r.Cells[cDate]) ||
		strings.Contains(r.Cells[cModel], raw("noSum")) || strings.Contains(r.Cells[cPaths], programUnknown)
	if !moved {
		return
	}
	source := r.Cells[cSource]
	if !movedSource.MatchString(source) {
		c.add(r, "movedSource")
	}
	if strings.HasPrefix(r.Section, raw("colab")) && !strings.Contains(source, raw("timeZone")) {
		c.add(r, "movedTimeZone")
	}
}

// replacedBy Yerini alan'ın aynı dosyada daha yeni bir satıra gittiğini denetler.
func (c *checker) replacedBy(r Row) {
	id := r.Cells[cReplacedBy]
	if id == "—" {
		return
	}
	other, ok := c.ids[id]
	switch {
	case !ok:
		c.add(r, "replacedMissing", id)
	case other.File != r.File:
		c.add(r, "replacedOtherFile", id, other.File)
	case id == r.ID() || newer(r, other) || sameClock(r, other):
		c.add(r, "replacedNotNewer", id)
	}
}

// sameClock iki satırın aynı gün ve aynı saatte yazıldığını söyler (ikisinde de saat yazılı).
// Yerini alan "daha yeni" ister; aynı saat yeni değildir (DENETİM 842 N3). Saati bilinmeyen
// aynı günlü satırda sıra bilinmez, kabul edilir.
func sameClock(a, b Row) bool {
	ad, at, _ := rowDate(a)
	bd, bt, _ := rowDate(b)
	return ad == bd && at != "" && at == bt
}

// order her bölümde en yeni satırın üstte olduğunu denetler.
func (c *checker) order() {
	rows := c.f.Rows
	for i := 1; i < len(rows); i++ {
		a, b := rows[i-1], rows[i]
		if a.File == b.File && a.Section == b.Section && newer(b, a) {
			c.add(b, "order", a.ID())
		}
	}
}

// References metindeki kimlik sözlerinden klasörde olmayanları verir: satır numarası → kimlik.
func References(f *Folder, text string) []Problem {
	var out []Problem
	for i, line := range strings.Split(text, "\n") {
		for _, id := range idMention.FindAllString(line, -1) {
			if _, ok := f.rowByID(id); !ok {
				out = append(out, Problem{"", i + 1, id})
			}
		}
	}
	return out
}
