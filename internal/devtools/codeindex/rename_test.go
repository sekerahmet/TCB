package codeindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renameProject bindSource'u ve bir belgeyi geçici bir klasöre yazıp dizinler.
func renameProject(t *testing.T) (*Reader, string) {
	t.Helper()
	dir := writeTree(t, map[string]string{
		"m.tr":   bindSource + "yazdır(1)\n",
		"doc.md": "Dikkatin başı ve başları; başka bir şey. kv_baş da.\n",
	})
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return NewReader(idx), dir
}

func changeAt(list []Change, pos string) *Change {
	for i := range list {
		if list[i].Pos.String() == pos {
			return &list[i]
		}
	}
	return nil
}

// Çıplak ad o adı taşıyan bütün bildirimlerdir; kesin bölüm anlamsal yerleri, olası bölüm tahminleri
// ve metni taşır; hiçbir dosya değişmez.
func TestRenameImpactSplitsCertainAndLikely(t *testing.T) {
	r, dir := renameProject(t)
	before, _ := os.ReadFile(filepath.Join(dir, "m.tr"))
	res := r.RenameImpact("baş", "bakış_sayısı", RenameOptions{Root: dir})
	if res.Status != StatusOK || res.Reason != "" {
		t.Fatalf("%+v", res)
	}
	kinds := map[Kind]int{}
	for _, n := range res.Targets {
		kinds[n.Kind]++
	}
	if kinds[KindField] != 1 || kinds[KindParameter] != 3 || kinds[KindOption] != 1 {
		t.Errorf("targets: the field, three parameters (kurucu, yap, gölge) and the option: %v", kinds)
	}
	for pos, role := range map[string]Role{
		"m.tr:2:5":   RoleDefinition, // alanın tanımı
		"m.tr:5:15":  RoleWrite,      // kendi.baş = baş
		"m.tr:9:21":  RoleRead,       // sonuç kendi.baş
		"m.tr:4:29":  RoleDefinition, // kurucu(boyut, baş
		"m.tr:28:18": RoleArgument,   // yap(baş: 4)
		"m.tr:29:31": RoleOptionKey,  // komut_satırı({baş: 8
		"m.tr:29:63": RoleOptionKey,  // açıklamalar: {baş: ...
		"m.tr:15:13": RoleRead,       // d.baş: yapıcıdan tek atamalı yerel (çıkarımla)
	} {
		c := changeAt(res.Certain, pos)
		if c == nil || c.Role != role || c.Replacement != "bakış_sayısı" {
			t.Errorf("certain %s: %+v", pos, c)
		}
	}
	if c := changeAt(res.Certain, "m.tr:15:13"); c == nil || c.Basis != BasisLocalType || c.Confidence != Inferred {
		t.Errorf("the typed local is certain but labelled: %+v", c)
	}
	if c := changeAt(res.Likely, "m.tr:19:13"); c == nil || c.Match != MatchMemberGuess {
		t.Errorf("x.baş with an unknown receiver is a guess: %+v", c)
	}
	if changeAt(res.Certain, "m.tr:19:13") != nil {
		t.Error("a guess is never certain")
	}
	if c := changeAt(res.Likely, "doc.md:1:10"); c == nil || c.Match != MatchSuffixed || c.Replacement != "bakış_sayısıı" {
		t.Errorf("başı in the document: %+v", c)
	}
	if res.SkippedWords != 1 {
		t.Errorf("başka is another word: %d", res.SkippedWords)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "m.tr"))
	if string(before) != string(after) {
		t.Error("listing never writes")
	}
}

func TestRenameImpactNarrowedByAQualifiedName(t *testing.T) {
	r, dir := renameProject(t)
	res := r.RenameImpact("Dikkat.baş", "bakış", RenameOptions{Root: dir, NoText: true})
	if len(res.Targets) != 1 || res.Targets[0].Kind != KindField {
		t.Fatalf("%+v", res.Targets)
	}
	for _, c := range res.Certain {
		if c.Node != res.Targets[0].ID {
			t.Errorf("only the field's own places: %+v", c)
		}
	}
	for _, c := range res.Likely {
		if c.Where != WhereCode || c.Match != MatchMemberGuess {
			t.Errorf("no text scan with NoText, and only member guesses of this field: %+v", c)
		}
	}
}

func TestRenameImpactRejections(t *testing.T) {
	r, dir := renameProject(t)
	cases := []struct{ q, name, reason string }{
		{"baş", "yazdır", ReasonLanguageWord},
		{"baş", "döngü", ReasonLanguageWord},
		{"baş", "9x", ReasonInvalidName},
		{"baş", "a-b", ReasonInvalidName},
		{"baş", "BAŞ", ReasonSameName},
		{"yazdır", "yaz", ReasonBuiltin},
		{"m.tr", "n", ReasonModule},
	}
	for _, c := range cases {
		res := r.RenameImpact(c.q, c.name, RenameOptions{Root: dir})
		if res.Status != StatusRejected || res.Reason != c.reason || ExitCode(res.Status) != ExitUsage {
			t.Errorf("%s -> %s: %s %s", c.q, c.name, res.Status, res.Reason)
		}
	}
	miss := r.RenameImpact("bulunmayan", "x", RenameOptions{Root: dir})
	if miss.Status != StatusNotFound || ExitCode(miss.Status) != ExitNotFound {
		t.Errorf("%+v", miss)
	}
}

func TestRenameImpactReportsConflicts(t *testing.T) {
	r, dir := renameProject(t)
	res := r.RenameImpact("Dikkat.baş", "kv_baş", RenameOptions{Root: dir, NoText: true})
	if res.Status != StatusOK || len(res.Conflicts) != 1 || res.Conflicts[0].With.Name != "Dikkat.kv_baş" || ExitCode(res.Status) != ExitOK {
		t.Errorf("a clash is a warning, not an error: %+v", res.Conflicts)
	}
	if res := r.RenameImpact("Dikkat.baş", "bakış", RenameOptions{Root: dir, NoText: true}); len(res.Conflicts) != 0 {
		t.Errorf("a free name has no conflicts: %+v", res.Conflicts)
	}
}

func TestPrepareRename(t *testing.T) {
	r, _ := renameProject(t)
	got, reason := r.PrepareRename("m.tr", 5, 16)
	if reason != "" || got.Placeholder != "baş" || got.Node.Kind != KindField || got.Range.Line != 5 {
		t.Errorf("kendi.baş: %+v %q", got, reason)
	}
	if got, reason := r.PrepareRename("m.tr", 4, 30); reason != "" || got.Node.Kind != KindParameter {
		t.Errorf("a parameter in the header: %+v %q", got, reason)
	}
	if _, reason := r.PrepareRename("m.tr", 1, 1); reason != ReasonNoSymbol {
		t.Errorf("a keyword is not a symbol: %q", reason)
	}
	if _, reason := r.PrepareRename("m.tr", 30, 1); reason != ReasonBuiltin {
		t.Errorf("a built-in cannot be renamed: %q", reason)
	}
}

// Çıktı örnekle sabittir (-update ile yeniden yazılır).
func TestRenameImpactGolden(t *testing.T) {
	r, dir := renameProject(t)
	got, err := EncodeJSON(r.RenameImpact("baş", "bakış_sayısı", RenameOptions{Root: dir}))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "golden", "rename_bas.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != string(got) {
		t.Errorf("rename_bas.json changed (go test -update to rewrite):\n%s", got)
	}
}
