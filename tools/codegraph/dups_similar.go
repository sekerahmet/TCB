package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Bu dosya `similar <fonksiyon>`: bir işleve en çok benzeyen işlevler, benzerlik puanıyla. Her
// işlevin düğüm dizisinden ardışık shingleSize'lık parçalar (n-gram) çıkarılır; iki işlevin puanı
// ortak farklı parçaların Dice katsayısıdır (2·|A∩B| / (|A|+|B|), 0..1). Puan sıra ve yapı
// benzerliğini ölçer, adları ve değerleri saymaz (dups ile aynı düğüm dizisi). Küçük işlevler
// (minFuncNodes'tan az düğüm) her şeye benzeyeceğinden listelenmez.

const (
	shingleSize  = 4
	minFuncNodes = 12
	minSimilar   = 0.30
)

// funcGrams bir işlevin farklı parçalarıdır.
type funcGrams struct {
	fn    *dupFunc
	grams map[uint64]struct{}
}

// shingles işlevin parça kümesini verir.
func (co *dupCorpus) shingles(fn *dupFunc) funcGrams {
	g := funcGrams{fn: fn, grams: map[uint64]struct{}{}}
	for i := fn.from; i+shingleSize <= fn.to; i++ {
		var h uint64 = 14695981039346656037
		for k := 0; k < shingleSize; k++ {
			h = (h ^ uint64(co.seq[i+k])) * 1099511628211
		}
		g.grams[h] = struct{}{}
	}
	return g
}

// similarity iki işlevin Dice puanıdır.
func similarity(a, b funcGrams) (score float64, shared int) {
	small, large := a.grams, b.grams
	if len(small) > len(large) {
		small, large = large, small
	}
	for h := range small {
		if _, ok := large[h]; ok {
			shared++
		}
	}
	if len(a.grams)+len(b.grams) == 0 {
		return 0, 0
	}
	return 2 * float64(shared) / float64(len(a.grams)+len(b.grams)), shared
}

// simMatch bir benzer işlevdir.
type simMatch struct {
	File   string  `json:"file"`
	From   int     `json:"from"`
	To     int     `json:"to"`
	Func   string  `json:"func"`
	Score  float64 `json:"score"`
	Shared int     `json:"shared_grams"`
}

// similarResult similar komutunun sonucudur (JSON şeması codegraph-similar/1).
type similarResult struct {
	Schema  string     `json:"schema"`
	Target  simMatch   `json:"target"`
	Matches []simMatch `json:"matches"`
	Total   int        `json:"total"`
}

// findFunc ad işlevi bulur: "Ad", "Tür.Ad" ya da başına paket klasörü eklenmiş ("tokenize.parallel",
// "internal/compute/tokenize.parallel"). Birden çok eşleşmede hepsini döner.
func (co *dupCorpus) findFunc(q string) []*dupFunc {
	var out []*dupFunc
	for i := range co.funcs {
		fn := &co.funcs[i]
		file := co.files[fn.file]
		dir := file[:max(strings.LastIndex(file, "/"), 0)]
		full := dir + "." + fn.name
		if fn.name == q || full == q || strings.HasSuffix(full, "/"+q) || strings.HasSuffix(dir, "/"+strings.SplitN(q, ".", 2)[0]) && strings.HasSuffix(full, "."+afterDot(q)) {
			out = append(out, fn)
		}
	}
	return out
}

func afterDot(q string) string {
	if i := strings.Index(q, "."); i >= 0 {
		return q[i+1:]
	}
	return q
}

func (co *dupCorpus) matchOf(fn *dupFunc, score float64, shared int) simMatch {
	return simMatch{File: co.files[fn.file], From: fn.line, To: fn.last, Func: fn.name, Score: score, Shared: shared}
}

// runSimilar `similar` komutudur.
func runSimilar(args []string, stdout, stderr io.Writer) int {
	o, rest, code := parseDupFlags("similar", args, stderr)
	if o == nil {
		return code
	}
	if len(rest) != 1 {
		fmt.Fprintf(stderr, "codegraph similar: expected 1 function name\n%s", usage)
		return exitUsage
	}
	root, err := resolveRoot(o.root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	co, _, err := loadCorpus(root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	found := co.findFunc(rest[0])
	switch len(found) {
	case 0:
		fmt.Fprintf(stderr, "%q not found\n", rest[0])
		return exitNotFound
	case 1:
	default:
		fmt.Fprintf(stdout, "%q is ambiguous: %d candidates (repeat with a package directory or a type)\n", rest[0], len(found))
		for _, fn := range found {
			fmt.Fprintf(stdout, "  %s:%d  %s\n", co.files[fn.file], fn.line, fn.name)
		}
		return exitAmbiguous
	}
	res := co.similarTo(found[0], o.limit)
	if o.asJSON {
		return writeJSON(stdout, stderr, res)
	}
	printSimilar(stdout, res)
	return exitOK
}

// similarTo hedef işleve benzeyen işlevleri puan sırasıyla verir.
func (co *dupCorpus) similarTo(target *dupFunc, limit int) *similarResult {
	tg := co.shingles(target)
	res := &similarResult{Schema: "codegraph-similar/1", Target: co.matchOf(target, 1, len(tg.grams))}
	for i := range co.funcs {
		fn := &co.funcs[i]
		if fn == target || fn.to-fn.from < minFuncNodes {
			continue
		}
		if score, shared := similarity(tg, co.shingles(fn)); score >= minSimilar {
			res.Matches = append(res.Matches, co.matchOf(fn, score, shared))
		}
	}
	sort.SliceStable(res.Matches, func(i, j int) bool {
		if res.Matches[i].Score != res.Matches[j].Score {
			return res.Matches[i].Score > res.Matches[j].Score
		}
		return res.Matches[i].File+itoa(res.Matches[i].From) < res.Matches[j].File+itoa(res.Matches[j].From)
	})
	res.Total = len(res.Matches)
	if limit > 0 && len(res.Matches) > limit {
		res.Matches = res.Matches[:limit]
	}
	return res
}

func printSimilar(w io.Writer, r *similarResult) {
	t := r.Target
	fmt.Fprintf(w, "%s:%d-%d  %s  (%d distinct %d-grams)\n", t.File, t.From, t.To, t.Func, t.Shared, shingleSize)
	fmt.Fprintf(w, "%d similar function(s) with score >= %.2f (%d shown):\n", r.Total, minSimilar, len(r.Matches))
	for _, m := range r.Matches {
		fmt.Fprintf(w, "  %.2f  %s:%d-%d  %s\n", m.Score, m.File, m.From, m.To, m.Func)
	}
}
