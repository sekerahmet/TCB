// trlang_migrate migrates CLI command references from tr to trlang across tracked
// repository files using the ClassCommand list from internal/lang/codes (S3-S6).
// Running it a second time produces zero changes.
//
// Layer: tools (tools); temporary migration tool, not shipped to users.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

type catStat struct {
	lines int
	files map[string]bool
}

func newCatStat() *catStat {
	return &catStat{files: map[string]bool{}}
}

func (c *catStat) add(rel string) {
	c.lines++
	c.files[rel] = true
}

type stats struct {
	cats        map[string]*catStat
	perCmd      map[string]int
	exampleFile map[string]bool
	msgLines    map[string]int
	changedFile int
	changedLine int
	codesDirty  bool
	msgsDirty   bool
}

func newStats(cmds []string) *stats {
	cats := map[string]*catStat{}
	for _, k := range []string{"1_cmd", "2_file", "3_template", "4_exe", "5_path", "s3_union", "6_cmd_pkg", "7_extra"} {
		cats[k] = newCatStat()
	}
	perCmd := make(map[string]int, len(cmds))
	for _, c := range cmds {
		perCmd[c] = 0
	}
	return &stats{
		cats:        cats,
		perCmd:      perCmd,
		exampleFile: map[string]bool{},
		msgLines:    map[string]int{},
	}
}

func replaceBounded(s string, re *regexp.Regexp, leftOK func(string, int) bool, rightOK func(string, int) bool, repl func(string) string) (string, int) {
	idxs := re.FindAllStringIndex(s, -1)
	if len(idxs) == 0 {
		return s, 0
	}
	var b strings.Builder
	last := 0
	hits := 0
	for _, loc := range idxs {
		st, en := loc[0], loc[1]
		if !leftOK(s, st) || !rightOK(s, en) {
			continue
		}
		b.WriteString(s[last:st])
		b.WriteString(repl(s[st:en]))
		last = en
		hits++
	}
	if hits == 0 {
		return s, 0
	}
	b.WriteString(s[last:])
	return b.String(), hits
}

func leftPlain(s string, st int) bool {
	r, ok := prevRune(s, st)
	return !ok || !isPathOrIdentRune(r)
}

func leftNoOnly(s string, st int) bool {
	return leftPlain(s, st) && !strings.HasSuffix(s[:st], "--only ")
}

func rightIdent(s string, en int) bool {
	r, ok := nextRune(s, en)
	return !ok || !isIdentOrDashRune(r)
}

func rightPath(s string, en int) bool {
	r, ok := nextRune(s, en)
	return !ok || !isPathOrIdentRune(r)
}

func applyS3Line(rel, line string, r *compiledRules, st *stats) (string, bool) {
	cur := line
	s3Hit := false
	for _, c := range r.cmds {
		_, n := replaceBounded(line, r.cmdEachRe[c], leftPlain, rightIdent, func(m string) string { return m })
		if n > 0 {
			st.perCmd[c]++
		}
	}
	if next, n := replaceBounded(cur, r.cmdRe, leftPlain, rightIdent, func(m string) string {
		return newProg + m[len(oldProg):]
	}); n > 0 {
		cur = next
		st.cats["1_cmd"].add(rel)
		s3Hit = true
	}
	if next, n := replaceBounded(cur, r.fileRe, leftPlain, rightPath, func(m string) string {
		return newProg + m[len(oldProg):]
	}); n > 0 {
		cur = next
		st.cats["2_file"].add(rel)
		if strings.HasPrefix(rel, "examples/") {
			st.exampleFile[rel] = true
		}
		s3Hit = true
	}
	cur, tmplHit := applyTemplates(rel, cur, r)
	if tmplHit {
		st.cats["3_template"].add(rel)
		s3Hit = true
	}
	if next, n := replaceBounded(cur, r.exeRe, func(s string, i int) bool {
		r, ok := prevRune(s, i)
		return !ok || (!isIdentOrDashRune(r) && r != '.')
	}, rightIdent, func(string) string {
		return newProg + ".exe"
	}); n > 0 {
		cur = next
		st.cats["4_exe"].add(rel)
		s3Hit = true
	}
	if next, hit := applyBinaryPaths(cur, r); hit {
		cur = next
		st.cats["5_path"].add(rel)
		s3Hit = true
	}
	if s3Hit {
		st.cats["s3_union"].add(rel)
		if rel == "internal/lang/messages/tr.json" || rel == "internal/lang/messages/en.json" {
			st.msgLines[rel]++
		}
	}
	return cur, cur != line
}

func applyTemplates(rel, cur string, r *compiledRules) (string, bool) {
	hit := false
	if next, n := replaceBounded(cur, r.tmplBrace, leftPlain, func(string, int) bool { return true }, func(m string) string {
		return newProg + m[len(oldProg):]
	}); n > 0 {
		cur, hit = next, true
	}
	if next, n := replaceBounded(cur, r.tmplAngle, leftPlain, func(s string, en int) bool {
		return en == len(s) || s[en] != '/'
	}, func(m string) string {
		return newProg + m[len(oldProg):]
	}); n > 0 {
		cur, hit = next, true
	}
	if next, n := replaceBounded(cur, r.tmplDash, leftNoOnly, func(string, int) bool { return true }, func(m string) string {
		return newProg + m[len(oldProg):]
	}); n > 0 {
		cur, hit = next, true
	}
	if (rel == "internal/lang/messages/tr.json" || rel == "internal/lang/messages/en.json") && strings.Contains(cur, `"`+oldProg+`\t`) {
		cur = strings.ReplaceAll(cur, `"`+oldProg+`\t`, `"`+newProg+`\t`)
		hit = true
	}
	return cur, hit
}

func applyBinaryPaths(cur string, r *compiledRules) (string, bool) {
	hit := false
	for _, re := range []*regexp.Regexp{r.pathOutRe, r.pathBinRe} {
		next, n := replaceBounded(cur, re, func(s string, st int) bool {
			if st == 0 || s[st] == '-' || s[st] == '$' || s[st] == '/' {
				return true
			}
			pr, ok := prevRune(s, st)
			return !ok || !isIdentOrDashRune(pr)
		}, validBinaryEnd, func(m string) string {
			return m[:len(m)-len(oldProg)] + newProg
		})
		if n > 0 {
			cur, hit = next, true
		}
	}
	return cur, hit
}

func applyExtraLine(rel, cur string, r *compiledRules, st *stats) string {
	// Never touch Go code lines in non-comment lines with broad prose rules; only comments/docs/JSON/testdata.
	if next, n := replaceBounded(cur, r.cmdPkgRe, func(s string, i int) bool {
		pr, ok := prevRune(s, i)
		return !ok || !isIdentOrDashRune(pr)
	}, rightIdent, func(string) string {
		return "cmd/" + newProg
	}); n > 0 {
		cur = next
		st.cats["6_cmd_pkg"].add(rel)
	}
	before := cur
	cur = applyDocAndTestExtras(rel, cur, r)
	if cur != before {
		st.cats["7_extra"].add(rel)
	}
	return cur
}

func applyDocAndTestExtras(rel, cur string, r *compiledRules) string {
	switch rel {
	case "docs/spec/17_kodlar.md":
		cur = strings.ReplaceAll(cur, "`"+oldProg+"` komutlar", "`"+newProg+"` komutlar")
		cur = strings.ReplaceAll(cur, r.specRunOld, r.specRunNew)
		return cur
	case "tools/gen/codesdoc/text.json":
		cur = strings.ReplaceAll(cur, "Komutlar ("+oldProg+", ", "Komutlar ("+newProg+", ")
		cur = strings.ReplaceAll(cur, `"`+oldProg+` komutlar`, `"`+newProg+` komutlar`)
		return cur
	case "tools/gpu/COLAB.md":
		cur = strings.ReplaceAll(cur, "echo '"+oldProg+" derlendi'", "echo '"+newProg+" derlendi'")
		cur = strings.ReplaceAll(cur, "for ad in "+oldProg+" "+oldProg+"_deney", "for ad in "+newProg+" "+oldProg+"_deney")
	}
	if strings.HasPrefix(rel, "docs/spec/") {
		return cur
	}
	if strings.HasPrefix(rel, "tools/codegraph/") {
		if next, n := replaceBounded(cur, r.mainSymRe, leftPlain, rightIdent, func(m string) string {
			return newProg + m[len(oldProg):]
		}); n > 0 {
			cur = next
		}
	}
	if strings.HasPrefix(rel, "internal/lang/codes/") && strings.HasSuffix(rel, ".go") {
		cur, _ = replaceBounded(cur, r.cmdsOfRe, func(string, int) bool { return true }, rightIdent, func(string) string {
			return "commands of " + newProg
		})
		cur = strings.ReplaceAll(cur, "'"+oldProg+" definition'", "'"+newProg+" definition'")
		cur = strings.ReplaceAll(cur, "'"+oldProg+" tan", "'"+newProg+" tan")
	}
	if canApplyProseCmd(rel, cur) {
		cur, _ = replaceBounded(cur, r.proseCmdRe, leftPlain, func(string, int) bool { return true }, func(m string) string {
			if strings.HasPrefix(m, "`"+oldProg+"`") {
				return "`" + newProg + "`" + m[len(oldProg)+2:]
			}
			if strings.HasPrefix(m, "'"+oldProg+"'") {
				return "'" + newProg + "'" + m[len(oldProg)+2:]
			}
			return newProg + m[len(oldProg):]
		})
	}
	return cur
}

func canApplyProseCmd(rel, line string) bool {
	if rel == "internal/lang/codes/tr.json" {
		return true
	}
	if strings.HasSuffix(rel, ".md") {
		return rel != "docs/status.md" && rel != "docs/roadmap.md"
	}
	if strings.HasSuffix(rel, ".go") {
		trimmed := strings.TrimSpace(line)
		return strings.HasPrefix(trimmed, "//")
	}
	return false
}

func trackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	raw := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n")), "\n")
	var list []string
	for _, f := range raw {
		if f != "" && !shouldSkipFile(f) {
			list = append(list, f)
		}
	}
	return list, nil
}

func processFile(root, rel string, dry bool, r *compiledRules, st *stats) error {
	full := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil
	}
	crlf := bytes.Contains(data, []byte("\r\n"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	fileChanged := false
	for i, line := range lines {
		if shouldSkipLine(rel, line) {
			continue
		}
		next, _ := applyS3Line(rel, line, r, st)
		next = applyExtraLine(rel, next, r, st)
		if next != line {
			lines[i] = next
			fileChanged = true
			st.changedLine++
		}
	}
	if !fileChanged {
		return nil
	}
	st.changedFile++
	if strings.HasPrefix(rel, "internal/lang/codes/") || strings.HasPrefix(rel, "tools/gen/codesdoc/") {
		st.codesDirty = true
	}
	if strings.HasPrefix(rel, "internal/lang/messages/") {
		st.msgsDirty = true
	}
	if dry {
		return nil
	}
	sep := "\n"
	if crlf {
		sep = "\r\n"
	}
	return os.WriteFile(full, []byte(strings.Join(lines, sep)), 0o644)
}

func runGenerators(root string, st *stats) error {
	var gens []string
	if st.codesDirty {
		gens = append(gens, "./tools/gen/codesdoc")
	}
	if st.msgsDirty {
		gens = append(gens, "./tools/gen/msgconsts")
	}
	for _, g := range gens {
		cmd := exec.Command("go", "run", g)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go run %s: %w\n%s", g, err, out)
		}
	}
	return nil
}

func printDryStats(r *compiledRules, st *stats) {
	order := []string{"1_cmd", "2_file", "3_template", "4_exe", "5_path", "s3_union", "6_cmd_pkg", "7_extra"}
	for _, k := range order {
		c := st.cats[k]
		fmt.Printf("%-12s: %4d lines / %3d files\n", k, c.lines, len(c.files))
	}
	fmt.Printf("examples/ (2_file): %d files\n", len(st.exampleFile))
	fmt.Printf("messages: tr.json=%d, en.json=%d\n", st.msgLines["internal/lang/messages/tr.json"], st.msgLines["internal/lang/messages/en.json"])
	var parts []string
	for _, c := range r.cmds {
		parts = append(parts, fmt.Sprintf("%s=%d", c, st.perCmd[c]))
	}
	fmt.Printf("per-command : %s\n", strings.Join(parts, ", "))
	fmt.Printf("total target: %d files (%d lines)\n", st.changedFile, st.changedLine)
}

func main() {
	dry := flag.Bool("dry", false, "count matches without modifying files")
	flag.Parse()
	root := reporoot.Root()
	rules := buildRules()
	st := newStats(rules.cmds)
	files, err := trackedFiles(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, rel := range files {
		if err := processFile(root, rel, *dry, rules, st); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", rel, err)
			os.Exit(1)
		}
	}
	if *dry {
		printDryStats(rules, st)
		return
	}
	if err := runGenerators(root, st); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d files changed (%d lines)\n", st.changedFile, st.changedLine)
}
