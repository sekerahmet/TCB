package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// folderRel ölçüler klasörünün, readmePath README'sinin depo köküne göre yoludur.
const (
	folderRel  = "docs/measurements"
	readmePath = "docs/measurements/README.md"
)

// measureFiles klasörün beş ölçü dosyası ve kimliğin ilk parçası olan kısaltmalarıdır.
var measureFiles = []struct{ path, prefix string }{
	{"docs/measurements/text-generation.md", "TG"},
	{"docs/measurements/speech-to-text.md", "STT"},
	{"docs/measurements/text-to-speech.md", "TTS"},
	{"docs/measurements/assistant.md", "AS"},
	{"docs/measurements/tensor-and-runtime.md", "RT"},
}

// rowHeader her bölümün tablosunun başlığıdır; aynen yazılır.
var rowHeader = raw("rowHeader")

// Satırın hücreleri, tablo başlığının sırasıyla.
const (
	cID = iota
	cDate
	cSession
	cDevice
	cCommit
	cPaths
	cOutOfRepo
	cSettings
	cModel
	cRuns
	cResult
	cSource
	cReplacedBy
	cellCount
)

// idPattern kimliğin biçimidir: <kısaltma>-<YYYY-AA-GG>-<makine>-<oturum>-<sıra>.
var idPattern = regexp.MustCompile(`^(TG|STT|TTS|AS|RT)-([0-9]{4}-[0-9]{2}-[0-9]{2})-([A-Z0-9]+)-([A-Z0-9]+)-([1-9][0-9]*)$`)

// idMention metnin içindeki kimlik sözüdür (göndermeler).
var idMention = regexp.MustCompile(`\b(TG|STT|TTS|AS|RT)-[0-9]{4}-[0-9]{2}-[0-9]{2}-[A-Z0-9]+-[A-Z0-9]+-[0-9]+\b`)

// separatorLine tablo başlığının altındaki hizalama satırıdır.
var separatorLine = regexp.MustCompile(`^\|( *:?-+:? *\|){13}$`)

// Row ölçü dosyasındaki bir satırdır.
type Row struct {
	File    string // depo köküne göre, '/' ile
	Line    int
	Section string // makine başlığı
	Cells   []string
}

// ID satırın kimliğidir.
func (r Row) ID() string { return r.Cells[cID] }

// Machine README'nin makine tablosundaki bir satırdır.
type Machine struct{ Code, Title, Kind string }

// Problem klasörde bulunan bir biçim yanlışıdır.
type Problem struct {
	File string
	Line int
	Msg  string
}

func (p Problem) String() string { return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Msg) }

// Folder ölçüler klasörünün okunmuş hâlidir.
type Folder struct {
	Root     string // depo kökü
	Machines []Machine
	Sessions map[string]bool
	Sets     map[string][]string
	Versions map[[2]string]string // (ad, makine kodu) → güncel sürüm
	Rows     []Row
	Lines    map[string][]string // klasördeki her .md dosyasının satırları
	Problems []Problem           // okurken bulunan biçim yanlışları
}

// machineByTitle başlığı verilen makineyi verir.
func (f *Folder) machineByTitle(title string) (Machine, bool) {
	for _, m := range f.Machines {
		if m.Title == title {
			return m, true
		}
	}
	return Machine{}, false
}

// rowByID kimliği verilen satırı verir.
func (f *Folder) rowByID(id string) (Row, bool) {
	for _, r := range f.Rows {
		if r.ID() == id {
			return r, true
		}
	}
	return Row{}, false
}

// problem key iletisini (texts.json) args ile Problems'a ekler.
func (f *Folder) problem(file string, line int, key string, args ...any) {
	f.Problems = append(f.Problems, Problem{file, line, text(key, args...)})
}

// Load root deposunun ölçüler klasörünü okur. Okunamayan dosya hatadır; biçim yanlışları
// Problems'a yazılır.
func Load(root string) (*Folder, error) {
	f := &Folder{Root: root, Sessions: map[string]bool{}, Sets: map[string][]string{},
		Versions: map[[2]string]string{}, Lines: map[string][]string{}}
	entries, err := reporoot.ReadDir(filepath.Join(root, filepath.FromSlash(folderRel))) // izlenmeyen dosya sayılmaz
	if err != nil {
		return nil, err
	}
	known := map[string]bool{"README.md": true}
	for _, m := range measureFiles {
		known[path.Base(m.path)] = true
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !known[e.Name()] {
			f.problem(folderRel+"/"+e.Name(), 1, "unexpectedFile", fileNames())
		}
	}
	readme, err := f.read(readmePath)
	if err != nil {
		return nil, err
	}
	f.readTables(readmePath, readme)
	for _, m := range measureFiles {
		lines, err := f.read(m.path)
		if err != nil {
			f.problem(m.path, 1, "missingFile")
			continue
		}
		f.readRows(m.path, lines)
	}
	return f, nil
}

func fileNames() string {
	var names []string
	for _, m := range measureFiles {
		names = append(names, path.Base(m.path))
	}
	return strings.Join(names, ", ")
}

func (f *Folder) read(rel string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(f.Root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	f.Lines[rel] = lines
	return lines, nil
}

// splitCells tablo satırını hücrelerine böler.
func splitCells(line string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	cells := strings.Split(inner, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// readmeTables README'nin tablolarıdır: bölüm başlığı → tablo başlığı.
var readmeTables = map[string]string{
	raw("titleMachines"): raw("headerMachines"),
	raw("titleSessions"): raw("headerSessions"),
	raw("titleSets"):     raw("headerSets"),
	raw("titleVersions"): raw("headerVersions"),
}

var backticked = regexp.MustCompile("`([^`]+)`")

// readTables README'nin dört tablosunu okur.
func (f *Folder) readTables(file string, lines []string) {
	seen := map[string]bool{}
	for i := 0; i < len(lines); i++ {
		title, ok := strings.CutPrefix(lines[i], "## ")
		header, wanted := readmeTables[title]
		if !ok || !wanted {
			continue
		}
		seen[title] = true
		j := i + 1
		for j < len(lines) && !strings.HasPrefix(lines[j], "|") {
			j++
		}
		if j >= len(lines) || lines[j] != header || j+1 >= len(lines) || !strings.HasPrefix(lines[j+1], "| :--") {
			f.problem(file, i+1, "tableHeader", title, header)
			continue
		}
		for j += 2; j < len(lines) && strings.HasPrefix(lines[j], "|"); j++ {
			f.tableRow(file, j+1, title, splitCells(lines[j]))
		}
	}
	for title := range readmeTables {
		if !seen[title] {
			f.problem(file, 1, "missingSection", title)
		}
	}
}

func (f *Folder) tableRow(file string, line int, title string, c []string) {
	want := len(splitCells(readmeTables[title]))
	if len(c) != want {
		f.problem(file, line, "tableCells", title, want, len(c))
		return
	}
	switch title {
	case raw("titleMachines"):
		if c[2] != raw("cpu") && c[2] != raw("gpu") {
			f.problem(file, line, "machineKind", c[2])
		}
		f.Machines = append(f.Machines, Machine{c[0], c[1], c[2]})
	case raw("titleSessions"):
		f.Sessions[c[0]] = true
	case raw("titleSets"):
		for _, m := range backticked.FindAllStringSubmatch(c[1], -1) {
			f.Sets[c[0]] = append(f.Sets[c[0]], m[1])
		}
	case raw("titleVersions"):
		f.Versions[[2]string{c[0], c[1]}] = c[2]
	}
}

// readRows bir ölçü dosyasının bölümlerini ve satırlarını okur.
func (f *Folder) readRows(file string, lines []string) {
	section := ""
	state := 0 // 0: bölüm dışı, 1: başlık bekleniyor, 2: hizalama bekleniyor, 3: satırlar
	for i, line := range lines {
		n := i + 1
		if title, ok := strings.CutPrefix(line, "## "); ok {
			section, state = title, 1
			continue
		}
		switch {
		case state == 0:
		case strings.TrimSpace(line) == "":
			if state == 3 {
				state = 4 // tablo bitti; bölümde başka metin olmaz
			}
		case state == 1 && line == rowHeader:
			state = 2
		case state == 2 && separatorLine.MatchString(line):
			state = 3
		case state == 3 && strings.HasPrefix(line, "|"):
			cells := splitCells(line)
			if len(cells) != cellCount {
				f.problem(file, n, "rowCells", cellCount, len(cells))
				continue
			}
			f.Rows = append(f.Rows, Row{file, n, section, cells})
		case state == 1:
			f.problem(file, n, "firstLineHeader", rowHeader)
		default:
			f.problem(file, n, "textInSection")
		}
	}
	if state == 1 || state == 2 {
		f.problem(file, len(lines), "tableMissing", section)
	}
}
