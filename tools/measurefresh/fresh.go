package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// compareRef tazeliğin kıyaslandığı daldır.
const compareRef = "origin/main"

// repo bir git deposunda komut koşturur.
type repo struct{ dir string }

func (g repo) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// readAt dosyanın compareRef'teki metnini, orada yoksa çalışılan ağaçtakini verir.
func (g repo) readAt(rel string) ([]byte, error) {
	if out, err := exec.Command("git", "-C", g.dir, "show", compareRef+":"+rel).Output(); err == nil {
		return out, nil
	}
	return os.ReadFile(filepath.Join(g.dir, filepath.FromSlash(rel)))
}

var shortSum = regexp.MustCompile(`^[0-9a-f]{12}$`)
var toolchainLine = regexp.MustCompile(`(?m)^toolchain[ \t]+(\S+)`)
var goLine = regexp.MustCompile(`(?m)^go[ \t]+(\S+)`)

// goVersion compareRef'teki go.mod'un Go sürümüdür: toolchain satırı, yoksa go satırı.
func (g repo) goVersion() (string, error) {
	data, err := g.readAt("go.mod")
	if err != nil {
		return "", err
	}
	if m := toolchainLine.FindSubmatch(data); m != nil {
		return string(m[1]), nil
	}
	if m := goLine.FindSubmatch(data); m != nil {
		return "go" + string(m[1]), nil
	}
	return "", errors.New(raw("noGoVersion"))
}

// rowPaths satırın ölçtüğü yollardır: kümeler, programlar ve aktardıkları; programsız ya da
// programı bilinmeyen satırda bütün lib/. Motordan bağımsız satırın yolu yoktur.
func rowPaths(f *Folder, g repo, r Row) ([]string, error) {
	cell := r.Cells[cPaths]
	if cell == engineFree {
		return nil, nil
	}
	set := map[string]bool{}
	marker := programMarker.FindString(cell)
	if marker != "" {
		set["lib/"] = true
	}
	for _, item := range strings.Split(strings.Replace(cell, marker, "", 1), ",") {
		item = strings.TrimSpace(item)
		switch {
		case item == "":
		case strings.HasSuffix(item, ".tr"):
			files, err := programImports(item, g.readAt)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", r.ID(), err)
			}
			for _, p := range files {
				set[p] = true
			}
		case f.Sets[item] != nil:
			for _, p := range f.Sets[item] {
				set[p] = true
			}
		default:
			return nil, errors.New(text("unknownRowItem", r.ID(), item))
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

// versionItems Depo dışı hücresinin (ad, sürüm) çiftleridir; sürüm son sözcüktür.
func versionItems(cell string) [][2]string {
	if dashReason.MatchString(cell) {
		return nil
	}
	var out [][2]string
	for _, item := range strings.Split(cell, "; ") {
		item = strings.TrimSpace(item)
		if i := strings.LastIndex(item, " "); i > 0 {
			out = append(out, [2]string{item[:i], item[i+1:]})
		}
	}
	return out
}

// freshness satırın tazelik satırını verir (söz bölümü S1).
func freshness(f *Folder, g repo, goVer string, r Row) (string, error) {
	day, _, _ := rowDate(r)
	where := fmt.Sprintf("%s, %s", day, r.Section)
	line := text("outCurrent", r.ID())
	paths, err := rowPaths(f, g, r)
	if err != nil {
		return "", err
	}
	commit := r.Cells[cCommit]
	stale := false
	if len(paths) > 0 {
		if _, err := g.git("cat-file", "-e", commit+"^{commit}"); err != nil {
			line, stale = text("outMissingCommit", r.ID(), commit, where), true
		} else {
			args := append([]string{"rev-list", "--count", "--no-merges", commit + ".." + compareRef, "--"}, paths...)
			n, err := g.git(args...)
			if err != nil {
				return "", err
			}
			if n != "0" {
				line, stale = text("outCommits", r.ID(), n, where), true
			}
		}
	}
	if !stale {
		machine, _ := f.machineByTitle(r.Section)
		for _, v := range versionItems(r.Cells[cOutOfRepo]) {
			current, known := f.Versions[[2]string{v[0], machine.Code}]
			if v[0] == "Go" {
				current, known = goVer, true
			}
			if known && current != v[1] {
				line = text("outVersion", r.ID(), v[0], v[1], current, where)
				break
			}
		}
	}
	if id := r.Cells[cReplacedBy]; id != "—" {
		line += text("outReplacedBy", id)
	}
	return line, nil
}

// possibleReplacement Yerini alan'ı boş satırın, aynı dosyada, aynı bölümde, aynı modelle ve
// aynı programla ölçülmüş en yeni satırının kimliğini verir (yoksa "").
func possibleReplacement(f *Folder, r Row) string {
	if r.Cells[cReplacedBy] != "—" {
		return ""
	}
	for _, s := range f.Rows { // bölümde en yeni satır üstte: ilk eşleşen en yenisidir
		if s.File == r.File && s.Section == r.Section && s.ID() != r.ID() && newer(s, r) &&
			sameModels(r.Cells[cModel], s.Cells[cModel]) && samePrograms(r.Cells[cPaths], s.Cells[cPaths]) {
			return s.ID()
		}
	}
	return ""
}

// models Model hücresinin dosya adı → sha256 önekidir ("" bilinmiyor).
func models(cell string) map[string]string {
	out := map[string]string{}
	if dashReason.MatchString(cell) {
		return out
	}
	for _, item := range strings.Split(cell, "; ") {
		if m := modelItem.FindStringSubmatch(item); m != nil {
			sum := m[2]
			if !shortSum.MatchString(sum) {
				sum = "" // sha256 yazılmamış (taşındı)
			}
			out[m[1]] = sum
		}
	}
	return out
}

// sameModels iki satırın aynı modelleri ölçtüğünü söyler: dosya adları aynı, sha256 yalnız iki
// satırda da yazılıysa karşılaştırılır.
func sameModels(a, b string) bool {
	ma, mb := models(a), models(b)
	if len(ma) != len(mb) {
		return false
	}
	for name, sa := range ma {
		sb, ok := mb[name]
		if !ok || sa != "" && sb != "" && sa != sb {
			return false
		}
	}
	return true
}

// samePrograms iki satırın aynı .tr programlarını ölçtüğünü söyler; programı bilinmeyen taşınan
// satır her programla eşleşir.
func samePrograms(a, b string) bool {
	if strings.Contains(a, programUnknown) || strings.Contains(b, programUnknown) {
		return true
	}
	return strings.Join(programsOf(a), ",") == strings.Join(programsOf(b), ",")
}

func programsOf(cell string) []string {
	var out []string
	for _, item := range strings.Split(cell, ",") {
		if item = strings.TrimSpace(item); strings.HasSuffix(item, ".tr") {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}
