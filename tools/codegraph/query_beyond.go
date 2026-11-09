package main

import (
	"cmp"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Adım sınırının ötesindeki testler (B1a, K1; iki kademe, codeindex/reach.go'nun baş yorumu). Listeler (tests-for,
// changed --go-tests, trlang index tested_by ve impact) WalkCallers'ın sınırında kalır. codeindex.WalkCallersBeyond aynı
// yürüyüşü hedefin klasöründe adım sınırı olmadan sürdürür:
//   - kademe 1: `untested` bir işlevi bu testlerle de "testli" sayar (testsForNode);
//   - kademe 2: tests-for önce hedefin kendi test dosyasındakileri adıyla, kalanını tek satırda girişiyle yazar.

// beyondTests hedefin listede olmayan, yalnız klasör içi zincirle sınırın ötesinde ulaşılan testleridir.
type beyondTests struct {
	own     []indirectTest // hedefin kendi test dosyasındakiler (<dosya>_test.go): adıyla yazılır
	rest    int            // öbürleri
	entries []beyondEntry  // öbürlerinin girişleri, en çok testli önce
}

// beyondEntry bir giriştir: ondan sorulan tests-for bu testleri sınır içinde bulur (codeindex.Reached.Entry).
type beyondEntry struct {
	entry        *Node
	tests, steps int // girişten gelen test sayısı; hedeften en kısa zincirleri
}

func (b beyondTests) total() int { return len(b.own) + b.rest }

// beyondLimit n'nin sınır ötesi testleridir; listed (cevabın listelediği testler) sayılmaz, ölçümlerin ötesine geçilmez.
func (ix *index) beyondLimit(n *Node, listed []*Node) beyondTests {
	skip := map[string]bool{}
	for _, t := range listed {
		skip[t.ID] = true
	}
	stem := strings.TrimSuffix(n.File, path.Ext(n.File)) + "_test"
	var out beyondTests
	byEntry := map[string]*beyondEntry{}
	codeindex.WalkCallersBeyond(n.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
		t := ix.byID[r.ID]
		if !isTestNode(t) && !isMeasurement(t) {
			return t != nil
		}
		if r.Steps <= testMaxSteps || skip[r.ID] || isMeasurement(t) {
			return false
		}
		if strings.TrimSuffix(t.File, path.Ext(t.File)) == stem {
			out.own = append(out.own, indirectTest{t, r.Steps, ix.byID[r.Via]})
			return false
		}
		out.rest++
		e := byEntry[r.Entry]
		if e == nil {
			e = &beyondEntry{entry: ix.byID[r.Entry], steps: r.Steps}
			byEntry[r.Entry] = e
		}
		e.tests++
		return false
	})
	for _, e := range byEntry {
		out.entries = append(out.entries, *e)
	}
	slices.SortFunc(out.entries, func(a, b beyondEntry) int {
		if c := cmp.Compare(b.tests, a.tests); c != 0 {
			return c
		}
		return cmp.Compare(a.entry.ID, b.entry.ID)
	})
	return out
}

// printBeyond sınır ötesi testleri yazar: kendi test dosyasındakiler adıyla (koşu satırı onları da adlandırır), kalanı
// tek satırda en çok testli girişle; girişten sorulan tests-for onları sınır içinde listeler.
func printBeyond(w io.Writer, b beyondTests, limit int) {
	if len(b.own) > 0 {
		fmt.Fprintf(w, "  <- this file's tests beyond the %d-step limit (%d; a chain inside its folder):\n", testMaxSteps, len(b.own))
		forEachLimited(w, b.own, limit, "    ", moreHint, func(it indirectTest) string {
			return fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name)
		})
	}
	if b.rest == 0 {
		return
	}
	e := b.entries[0]
	more := ""
	if len(b.own) > 0 {
		more = " more"
	}
	if len(b.entries) == 1 {
		fmt.Fprintf(w, "  beyond the %d-step limit, inside its folder: %d%s tests reach it through %s (from %d steps); `tests-for %s` lists them\n",
			testMaxSteps, b.rest, more, e.entry.Name, e.steps, e.entry.Name)
		return
	}
	fmt.Fprintf(w, "  beyond the %d-step limit, inside its folder: %d%s tests reach it, %d of them through %s (from %d steps) and the rest through %d more entries; `tests-for %s` lists those %d\n",
		testMaxSteps, b.rest, more, e.tests, e.entry.Name, e.steps, len(b.entries)-1, e.entry.Name, e.tests)
}
