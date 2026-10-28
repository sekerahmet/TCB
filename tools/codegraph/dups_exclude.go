package main

import (
	"path"
	"strings"
)

// Bu dosya bilerek ayrı tutulan yerleri tekrar uyarısından çıkarır. Her dışlamanın adı ve gerekçesi
// vardır (`dups --list-exclusions` yazar); kural bir kümeyi tamamen susturur, susturulan küme
// sayısı çıktıda ad ad görünür. Yeni bir yer eklemek için bir satır ve gerekçe yazılır.

// dupRule bir dışlama kuralıdır.
type dupRule struct {
	Name   string
	Reason string
	Match  func(c *dupCluster, co *dupCorpus) bool
}

// dupRules küme düzeyindeki dışlamalardır.
var dupRules = []dupRule{
	{
		Name: "gpu-simulator-vs-tensor",
		Reason: "internal/compute/gpu/sim*.go is the PTX simulator: an independent re-implementation of the tensor " +
			"conversions and arithmetic that checks the real kernels against the CPU core; sharing the code would make " +
			"the check circular.",
		Match: pairOf("internal/compute/gpu/sim*.go", "internal/compute/tensor/**"),
	},
	{
		Name: "test-float64-reference",
		Reason: "tests keep deliberately naive float64 reference implementations of production algorithms; they " +
			"are the independent oracle the production code is compared with (an independent reference is not a " +
			"duplicate).",
		Match: testFloat64Reference,
	},
	{
		Name: "ast-node-boilerplate",
		Reason: "internal/lang/ast declares every syntax-tree node as a struct plus the same marker methods " +
			"(statementNode, expressionNode, TokenLiteral, Line): the Go idiom for closed node sets; merging them " +
			"would need generics or code generation for no gain.",
		Match: allIn("internal/lang/ast/*.go"),
	},
}

// dupLoaderExclusions yükleyicinin kendiliğinden dışladıkları ve Go dışındakiler (belge amaçlıdır).
var dupLoaderExclusions = []dupRule{
	{Name: "generated-files", Reason: "files with the `// Code generated ... DO NOT EDIT.` header (codes_generated.go, Unicode tables) are not read: they are produced from one source."},
	{Name: "tables", Reason: "composite literals count as one leaf in *_test.go files (test tables) and, in production code, when they have 8 or more elements (name tables, operator sets): similar table rows are not duplicates."},
	{Name: "sibling-runs", Reason: "3 or more consecutive same-shape units under one parent, each under 40 syntax-tree nodes (const block items, registration calls, kernel parameter declarations, one-statement test functions), count as one leaf: a list is a table of facts, not logic."},
	{Name: "imports", Reason: "import declarations are not read."},
	{Name: "tr-sources", Reason: "only Go is read; .tr files are tr dizin territory."},
}

// allIn kümenin bütün konumları desene uyuyorsa kuralı uygular.
func allIn(pattern string) func(*dupCluster, *dupCorpus) bool {
	return func(c *dupCluster, _ *dupCorpus) bool {
		for _, l := range c.Locs {
			if !matchGlob(pattern, l.File) {
				return false
			}
		}
		return true
	}
}

// pairOf iki desenden birine uyan konumlar aynı kümede ise kuralı uygular.
func pairOf(a, b string) func(*dupCluster, *dupCorpus) bool {
	return func(c *dupCluster, _ *dupCorpus) bool {
		hasA, hasB := false, false
		for _, l := range c.Locs {
			hasA = hasA || matchGlob(a, l.File)
			hasB = hasB || matchGlob(b, l.File)
		}
		return hasA && hasB
	}
}

// testFloat64Reference kümede float64 kullanan bir test konumu ve test olmayan bir konum varsa doğrudur.
func testFloat64Reference(c *dupCluster, co *dupCorpus) bool {
	test, prod := false, false
	for _, l := range c.Locs {
		if !co.isTest[l.fileIdx] {
			prod = true
			continue
		}
		for i := l.start; i < l.start+l.Nodes; i++ {
			if co.atom[i] == "float64" {
				test = true
				break
			}
		}
	}
	return test && prod
}

// applyRules kuralların susturduğu kümeleri atar; kural başına susturulan sayıyı verir.
func applyRules(clusters []*dupCluster, co *dupCorpus) (kept []*dupCluster, silenced map[string]int) {
	silenced = map[string]int{}
next:
	for _, c := range clusters {
		for _, r := range dupRules {
			if r.Match(c, co) {
				silenced[r.Name]++
				continue next
			}
		}
		kept = append(kept, c)
	}
	return kept, silenced
}

// matchGlob yol desenidir: `*` bir yol parçası içinde, `**` her derinlikte eşleşir.
func matchGlob(pattern, p string) bool {
	return globParts(strings.Split(pattern, "/"), strings.Split(p, "/"))
}

func globParts(pat, parts []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(parts); i++ {
				if globParts(pat[1:], parts[i:]) {
					return true
				}
			}
			return false
		}
		if len(parts) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], parts[0]); !ok {
			return false
		}
		pat, parts = pat[1:], parts[1:]
	}
	return len(parts) == 0
}
