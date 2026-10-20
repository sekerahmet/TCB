package main

import (
	"fmt"
	"regexp"
	"strings"
)

// codeKey bir JSON satırındaki kod anahtarıdır (+/- önekiyle): ileti H0000, ad A000.
var codeKeyDiff = regexp.MustCompile(`^([+-])\s*"([A-Z]\d{3,4})"\s*:`)

// Kod sözlüğü dosyaları (modül köküne göre): ileti kodları ve ad kodları.
var (
	messageFiles = []string{messagesDir + "/definitions.json", messagesDir + "/tr.json"}
	nameFiles    = []string{codesDir + "/definitions.json", codesDir + "/tr.json", codesDir + "/en.json"}
)

// commitInfo bir commit'in özetidir.
type commitInfo struct{ hash, date, subject string }

func (c commitInfo) String() string { return c.hash + " " + c.date + " " + c.subject }

// codeEvent bir commit'te bir kodun bir dosyada eklenmesi (added) ya da silinmesidir (removed).
type codeEvent struct {
	commit  commitInfo
	file    string
	added   bool
	removed bool
}

// scanCodeHistory sözlük dosyalarının geçmişini `git log -p -U0` ile tarar ve kod başına
// olayları yeniden eskiye doğru verir. only boş değilse yalnızca o kodun geçtiği commit'ler
// taranır (pickaxe). Aynı commit'te hem eklenip hem silinen satır (taşıma) olay sayılmaz.
func scanCodeHistory(root string, files []string, only string) (map[string][]codeEvent, error) {
	args := []string{"log", "--no-renames", "-p", "-U0", "--no-color", "--no-ext-diff", "--format=@@@%h%x09%as%x09%s"}
	if only != "" {
		args = append(args, "-S\""+only+"\"")
	}
	args = append(args, "--")
	args = append(args, files...)
	out, err := gitOut(root, args...)
	if err != nil {
		return nil, err
	}
	type counts struct{ plus, minus int }
	events := map[string][]codeEvent{}
	var commit commitInfo
	var file string
	pending := map[[2]string]*counts{} // (kod, dosya) -> sayılar, geçerli commit için
	flush := func() {
		for k, c := range pending {
			if (c.plus > 0) != (c.minus > 0) {
				events[k[0]] = append(events[k[0]], codeEvent{commit: commit, file: k[1], added: c.plus > 0, removed: c.minus > 0})
			}
		}
		pending = map[[2]string]*counts{}
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "@@@"):
			flush()
			f := strings.SplitN(line[3:], "\t", 3)
			commit = commitInfo{f[0], f[1], f[2]}
		case strings.HasPrefix(line, "diff --git "):
			file = line[strings.LastIndex(line, " b/")+3:]
		default:
			if m := codeKeyDiff.FindStringSubmatch(line); m != nil {
				c := pending[[2]string{m[2], file}]
				if c == nil {
					c = new(counts)
					pending[[2]string{m[2], file}] = c
				}
				if m[1] == "+" {
					c.plus++
				} else {
					c.minus++
				}
			}
		}
	}
	flush()
	sortEvents(events)
	return events, nil
}

// sortEvents her kodun olaylarını yeniden eskiye diziler (git log sırası korunur; burada
// yalnızca aynı commit'teki dosyalar ada göre sıralanır).
func sortEvents(events map[string][]codeEvent) {
	for _, list := range events {
		for i := 1; i < len(list); i++ {
			for j := i; j > 0 && list[j].commit == list[j-1].commit && list[j].file < list[j-1].file; j-- {
				list[j], list[j-1] = list[j-1], list[j]
			}
		}
	}
}

// formatEvents bir kodun geçmişini satırlara döker: "added"/"removed" işaretli commit'ler.
func formatEvents(list []codeEvent) []string {
	var out []string
	for _, e := range list {
		verb := "removed"
		if e.added {
			verb = "added  "
		}
		out = append(out, fmt.Sprintf("%s  %s  (%s)", verb, e.commit, e.file))
	}
	return out
}
