package main

import (
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: gömülü dil listesi kurulamayınca (bozuk ya da yarım değişmiş tr.json) codegraph düşmez:
// Go sorusu cevaplanır ve cevap .tr tarafının eksik olduğunu söyler. Dil paketleri codegraph'ın
// içine derlendiği için ikili bozuk listeyle derlenir; kökün kendi tr.json'u da bozuktur (ad
// kodlarını codegraph oradan okur).
func TestABrokenLanguageFileLeavesTheGoAnswers(t *testing.T) {
	if testing.Short() {
		t.Skip("codegraph'ı yeniden derler")
	}
	const broken = `{ "TR_kw_if": `
	exe := testkit.BuildWithFile(t, "./tools/codegraph", "internal/lang/codes/tr.json", broken)
	root := t.TempDir()
	testkit.CopyDir(t, filepath.Join("testdata", "mod"), root)
	testkit.WritePath(t, filepath.Join(root, "internal", "lang", "codes", "tr.json"), broken)

	built := testkit.RunBinary(t, exe, root, "", nil, "build", "--root", root)
	testkit.Wait(t, built, exitOK, []string{"warnings:", trSkipped}, nil)
	answer := testkit.RunBinary(t, exe, root, "", nil, "tests-for", "--root", root, "shape/shape.go")
	testkit.Wait(t, answer, exitOK, []string{"shape.TestDescribe"}, []string{"answers about .tr code are missing", "language list tr"})
}
