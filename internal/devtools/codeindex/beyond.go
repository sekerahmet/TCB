package codeindex

import (
	"cmp"
	"path"
	"slices"
)

// BeyondEntry adım sınırının ötesindeki testlerin bir girişidir (codegraph B1a, kademe 2): Entry'den sorulan liste bu
// testleri sınır içinde bulur (Reached.Entry). Tests girişten gelen test sayısı, Steps hedeften en kısa zincirleri.
type BeyondEntry struct {
	Entry Node `json:"entry"`
	Tests int  `json:"tests"`
	Steps int  `json:"steps"`
}

// pkgOf düğümün klasörüdür (WalkCallers'ın paket sınırı).
func (r *Reader) pkgOf(id string) string {
	if n := r.byID[id]; n != nil && n.Pos != nil {
		return path.Dir(n.Pos.File)
	}
	return ""
}

func (r *Reader) callersOf(id string) []string { return r.callers[id] }

// beyondLimit id'nin listede (listed) olmayan, yalnız paket içi zincirle TestMaxSteps'in ötesinde ulaşılan test
// bloklarını sayar ve girişlerine göre toplar: en çok testli giriş önce (eşitlikte kimliğe göre).
func (r *Reader) beyondLimit(id string, listed map[string]bool) (int, []BeyondEntry) {
	byEntry := map[string]*BeyondEntry{}
	count := 0
	WalkCallersBeyond(id, r.callersOf, r.pkgOf, func(rc Reached) bool {
		c := r.byID[rc.ID]
		if c == nil || c.Kind != KindTest {
			return c != nil
		}
		if rc.Steps > TestMaxSteps && !listed[rc.ID] {
			count++
			e := byEntry[rc.Entry]
			if e == nil {
				e = &BeyondEntry{Entry: r.display(*r.byID[rc.Entry]), Steps: rc.Steps}
				byEntry[rc.Entry] = e
			}
			e.Tests++
		}
		return false
	})
	var out []BeyondEntry
	for _, e := range byEntry {
		out = append(out, *e)
	}
	slices.SortFunc(out, func(a, b BeyondEntry) int {
		if c := cmp.Compare(b.Tests, a.Tests); c != 0 {
			return c
		}
		return cmp.Compare(a.Entry.ID, b.Entry.ID)
	})
	return count, out
}
