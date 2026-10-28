package codeindex

import (
	"os"
	"path/filepath"
	"testing"
)

// Kural: bir dosyada görünen ad önce kendi tanımıdır, sonra takma adsız aktarılan modüllerinki, dolaylı olanlar dahil;
// aktarma sırasıyla, derinlik önce, bir modülün kendi tanımları aktardıklarından önce; ilk bulunan kazanır
// (lookupVisible). ana.tr b'yi sonra c'yi aktarır, b d'yi aktarır: x hem d'de hem c'de vardır, ana.tr'deki x d'ninkidir
// (derinlik önce); y hem b'de hem d'de vardır, b'ninkidir (modül aktardığından önce). Takma adlı aktarmanın adları
// görünür kılınmaz.
func TestAVisibleNameIsTheFirstInImportOrderDepthFirst(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"ana.tr": "TR_import \"b\"\nTR_import \"c\"\nTR_import \"e\" TR_as e\nTR_print(x, y, z)\n",
		"b.tr":   "TR_import \"d\"\nTR_const y = 2\n",
		"c.tr":   "TR_const x = 3\n",
		"d.tr":   "TR_const x = 4\nTR_const y = 4\n",
		"e.tr":   "TR_const z = 5\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), trText(filepath.Join(dir, name), text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	used := func(id string) bool { // ana.tr id'ye başvuruyor mu
		for _, o := range idx.Occurrences {
			if o.Node == id && o.Pos.File == "ana.tr" {
				return true
			}
		}
		return false
	}
	if !used("constant:d.tr#x") || used("constant:c.tr#x") || !used("constant:b.tr#y") || used("constant:d.tr#y") || used("constant:e.tr#z") {
		t.Error("ana.tr's x is d's (depth first), y is b's (a module before its imports), z of an aliased import is not visible")
	}
}
