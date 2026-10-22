package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func itoa(n int) string { return strconv.Itoa(n) }

// dupDiff bir kümenin konumları arasındaki farktır: ilk konumla öbürleri aynı sırayla karşılaştırılır,
// ad ya da değeri farklı olan düğümler değişim olarak sayılır.
type dupDiff struct {
	Identical bool        `json:"identical"` // adlar ve değerler de aynı: birebir kopya
	Differing int         `json:"differing"` // farklı ad/değer sayısı (tüm konumlar toplamı)
	Changes   []dupChange `json:"changes,omitempty"`
	Shorter   bool        `json:"unequal_length,omitempty"`
}

// dupChange "a → b" değişimi ve kaç kez geçtiği.
type dupChange struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

const maxChanges = 8

// diffOf kümenin fark özetini çıkarır.
func (co *dupCorpus) diffOf(c *dupCluster) dupDiff {
	d := dupDiff{}
	counts := map[[2]string]int{}
	first := c.Locs[0]
	for _, l := range c.Locs[1:] {
		n := min(first.Nodes, l.Nodes)
		if first.Nodes != l.Nodes {
			d.Shorter = true
		}
		for k := 0; k < n; k++ {
			a, b := co.atom[first.start+k], co.atom[l.start+k]
			if a != b {
				d.Differing++
				counts[[2]string{a, b}]++
			}
		}
	}
	d.Identical = d.Differing == 0
	for k, n := range counts {
		d.Changes = append(d.Changes, dupChange{From: k[0], To: k[1], Count: n})
	}
	sort.Slice(d.Changes, func(i, j int) bool {
		if d.Changes[i].Count != d.Changes[j].Count {
			return d.Changes[i].Count > d.Changes[j].Count
		}
		return d.Changes[i].From+d.Changes[i].To < d.Changes[j].From+d.Changes[j].To
	})
	if len(d.Changes) > maxChanges {
		d.Changes = d.Changes[:maxChanges]
	}
	return d
}

// dupResult dups komutunun sonucudur (JSON şeması codegraph-dups/1).
type dupResult struct {
	Schema        string         `json:"schema"`
	Min           int            `json:"min_nodes"`
	Files         int            `json:"files"`
	Nodes         int            `json:"nodes"`
	Changed       string         `json:"changed,omitempty"`
	Total         int            `json:"total"`
	Clusters      []*dupCluster  `json:"clusters"`
	Silenced      map[string]int `json:"silenced,omitempty"`
	SkippedGroups int            `json:"skipped_boilerplate_groups,omitempty"`
	Unparsed      []string       `json:"unparsed_files,omitempty"`
}

const dupSchema = "codegraph-dups/1"

// dupOpts dups ve similar komutlarının ayarlarıdır.
type dupOpts struct {
	root, in, changed, base string
	min, limit              int
	asJSON, noExclude       bool
	listRules               bool
}

func parseDupFlags(cmd string, args []string, stderr io.Writer) (*dupOpts, []string, int) {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	o := &dupOpts{}
	fs.StringVar(&o.root, "root", "", "repository root (default: found from the current directory)")
	fs.IntVar(&o.min, "min", 100, "dups: the smallest clone, in syntax-tree nodes (dupl's default is 100)")
	fs.StringVar(&o.in, "in", "", "dups: only clones with a location under this path (a package directory)")
	fs.StringVar(&o.changed, "changed", "", "dups: only clones that touch the lines a branch or range changed (git diff base...branch); the branch must be checked out")
	fs.StringVar(&o.base, "base", "main", "dups --changed: the branch a branch name is compared with")
	fs.IntVar(&o.limit, "limit", 20, "lines per list")
	all := fs.Bool("all", false, "no limit")
	fs.BoolVar(&o.asJSON, "json", false, "machine-readable output")
	fs.BoolVar(&o.noExclude, "no-exclude", false, "dups: also report the deliberately separate places (see --list-exclusions)")
	fs.BoolVar(&o.listRules, "list-exclusions", false, "dups: print the exclusions with their reasons and exit")
	if err := fs.Parse(reorderDupFlags(args)); err != nil {
		return nil, nil, exitUsage
	}
	if *all {
		o.limit = 0
	}
	return o, fs.Args(), exitOK
}

// reorderDupFlags bayrakları adların önüne alır (reorderFlags'ın dups bayrakları için sürümü).
func reorderDupFlags(args []string) []string {
	valued := map[string]bool{"root": true, "min": true, "in": true, "changed": true, "base": true, "limit": true}
	var flags, names []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			names = append(names, a)
			continue
		}
		flags = append(flags, a)
		if name := strings.TrimLeft(a, "-"); valued[name] && !strings.Contains(a, "=") && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, names...)
}

// runDups `dups` komutudur.
func runDups(args []string, stdout, stderr io.Writer) int {
	o, rest, code := parseDupFlags("dups", args, stderr)
	if o == nil {
		return code
	}
	if o.listRules {
		printRules(stdout)
		return exitOK
	}
	if len(rest) != 0 {
		fmt.Fprintf(stderr, "codegraph dups: unexpected argument %q\n%s", rest[0], usage)
		return exitUsage
	}
	root, err := resolveRoot(o.root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	start := time.Now()
	co, unparsed, err := loadCorpus(root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	res := &dupResult{Schema: dupSchema, Min: o.min, Files: len(co.files), Nodes: co.nodes, Unparsed: unparsed}
	clusters, skipped := co.findDups(o.min)
	res.SkippedGroups = skipped
	if !o.noExclude {
		clusters, res.Silenced = applyRules(clusters, co)
	}
	if o.in != "" {
		clusters = filterIn(clusters, o.in)
	}
	if o.changed != "" {
		var cerr error
		if clusters, cerr = filterChanged(clusters, root, o); cerr != nil {
			fmt.Fprintln(stderr, "codegraph:", cerr)
			return exitFail
		}
		res.Changed = o.changed
	}
	res.Total = len(clusters)
	if o.limit > 0 && len(clusters) > o.limit {
		clusters = clusters[:o.limit]
	}
	res.Clusters = clusters
	if o.asJSON {
		return writeJSON(stdout, stderr, res)
	}
	printDups(stdout, res, time.Since(start))
	return exitOK
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	fmt.Fprintln(stdout, string(data))
	return exitOK
}

func printRules(w io.Writer) {
	for _, r := range append(append([]dupRule{}, dupRules...), dupLoaderExclusions...) {
		fmt.Fprintf(w, "%s\n  %s\n", r.Name, r.Reason)
	}
}

// filterIn yalnızca yolu pkg ile başlayan bir konumu olan kümeleri bırakır.
func filterIn(clusters []*dupCluster, pkg string) []*dupCluster {
	pkg = strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(pkg), "./"), "/")
	var out []*dupCluster
	for _, c := range clusters {
		for _, l := range c.Locs {
			if l.File == pkg || strings.HasPrefix(l.File, pkg+"/") {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func printDups(w io.Writer, res *dupResult, took time.Duration) {
	scope := ""
	if res.Changed != "" {
		scope = " touching " + res.Changed
	}
	fmt.Fprintf(w, "%d duplicate candidate(s)%s (%d shown; >= %d nodes; %d files, %d nodes, %.1fs)\n",
		res.Total, scope, len(res.Clusters), res.Min, res.Files, res.Nodes, took.Seconds())
	for i, c := range res.Clusters {
		fmt.Fprintf(w, "\n#%d  %d nodes, %d locations, %s\n", i+1, c.Nodes, len(c.Locs), diffSummary(c.Diff))
		for _, l := range c.Locs {
			mark := ""
			if l.Changed {
				mark = "  [changed]"
			}
			fmt.Fprintf(w, "  %s:%d-%d  %s%s\n", l.File, l.From, l.To, l.Func, mark)
		}
		for _, ch := range c.Diff.Changes {
			fmt.Fprintf(w, "    %s -> %s (x%d)\n", ch.From, ch.To, ch.Count)
		}
	}
	for name, n := range res.Silenced {
		fmt.Fprintf(w, "\nsilenced by exclusion %q: %d (see --list-exclusions)\n", name, n)
	}
	if res.SkippedGroups > 0 {
		fmt.Fprintf(w, "%d boilerplate group(s) of more than %d identical places skipped\n", res.SkippedGroups, maxGroup)
	}
	for _, f := range res.Unparsed {
		fmt.Fprintf(w, "not parsed: %s\n", f)
	}
}

func diffSummary(d dupDiff) string {
	switch {
	case d.Identical && !d.Shorter:
		return "exact copy (names and values too)"
	case d.Identical:
		return "same shape, locations differ in length"
	}
	return fmt.Sprintf("same shape, %d name/value differences", d.Differing)
}
