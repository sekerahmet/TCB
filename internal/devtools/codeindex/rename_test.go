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
		"m.tr":   bindSource + "TR_print(1)\n",
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

// Çıplak ad birden çok bildirime uyuyorsa hata (çıkış 2) ve adaylar; her aday kimliğiyle tek tek
// sorulur. Kesin bölüm anlamsal yerleri, olası bölüm tahminleri ve metni taşır; hiçbir dosya değişmez.
func TestRenameImpactSplitsCertainAndLikely(t *testing.T) {
	r, dir := renameProject(t)
	before, _ := os.ReadFile(filepath.Join(dir, "m.tr"))
	amb := r.RenameImpact("baş", "bakış_sayısı", RenameOptions{Root: dir})
	if amb.Status != StatusRejected || amb.Reason != ReasonAmbiguous || ExitCode(amb.Status) != ExitUsage {
		t.Fatalf("a bare name with several meanings is an error: %+v", amb)
	}
	if text := RenameText(amb); !strings.Contains(text, "parameter:m.tr#yap.baş") || !strings.Contains(text, "field:m.tr#Dikkat.baş") {
		t.Errorf("the candidates are listed with their ids:\n%s", text)
	}
	kinds := map[Kind]int{}
	res := &Result{Status: StatusOK}
	for _, n := range amb.Candidates {
		kinds[n.Kind]++
		one := r.RenameImpact(n.ID, "bakış_sayısı", RenameOptions{Root: dir})
		if one.Status != StatusOK || len(one.Targets) != 1 {
			t.Fatalf("%s: %+v", n.ID, one)
		}
		res.Certain = append(res.Certain, one.Certain...)
		res.Likely = append(res.Likely, one.Likely...)
		res.SkippedWords += one.SkippedWords
	}
	if kinds[KindField] != 1 || kinds[KindParameter] != 3 || kinds[KindOption] != 1 {
		t.Errorf("candidates: the field, three parameters (kurucu, yap, gölge) and the option: %v", kinds)
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
	if res.SkippedWords < 1 {
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
		{"Dikkat.baş", "yazdır", ReasonLanguageWord},
		{"Dikkat.baş", "döngü", ReasonLanguageWord},
		{"Dikkat.baş", "9x", ReasonInvalidName},
		{"Dikkat.baş", "a-b", ReasonInvalidName},
		{"Dikkat.baş", "BAŞ", ReasonSameName},
		{"yazdır", "yaz", ReasonBuiltin},
		{"m.tr", "n", ReasonModule},
	}
	for _, c := range cases {
		res := r.RenameImpact(c.q, c.name, RenameOptions{Root: dir})
		if res.Status != StatusRejected || res.Reason != c.reason || ExitCode(res.Status) != ExitUsage {
			t.Errorf("%s -> %s: %s %s", c.q, c.name, res.Status, res.Reason)
		}
	}
	miss := r.RenameImpact("bulunmayan", "x", RenameOptions{Root: dir, NoText: true})
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
	got, err := EncodeJSON(r.RenameImpact("Dikkat.baş", "bakış_sayısı", RenameOptions{Root: dir}))
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

// Terim kipi sembol aramaz: yalnızca metin eşleşmeleri (belge, yorum, metin), Go'da kod adları hariç.
func TestRenameTermMode(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"m.tr":   "# çekirdek derlenir\nTR_func çekirdek_ekle() {\n    TR_return 1\n}\n",
		"doc.md": "GPU çekirdeği ve işlemci çekirdeği.\n",
		"k.go":   "package k\n\n// çekirdek başlatılır\nvar çekirdek = 1\nvar s = \"çekirdek\"\n",
	})
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	for _, term := range []bool{true, false} { // sembol yoksa kip kendiliğinden terim olur
		res := r.RenameImpact("çekirdek", "kernel", RenameOptions{Root: dir, Term: term})
		if res.Status != StatusOK || res.Mode != ModeTerm || len(res.Certain) != 0 || len(res.Targets) != 0 {
			t.Fatalf("term=%v: %+v", term, res)
		}
		got := strings.Join(hitSummaryOfChanges(res.Likely), "\n")
		want := strings.Join([]string{"doc.md:1:5 doc/suffixed", "doc.md:1:26 doc/suffixed", "k.go:3:4 comment/exact", "k.go:5:10 string/exact",
			"m.tr:1:4 comment/exact", "m.tr:2:11 code/compound"}, "\n")
		if got != want {
			t.Errorf("term=%v hits:\n%s\nwant:\n%s", term, got, want)
		}
	}
	if res := r.RenameImpact("çekirdek", "9", RenameOptions{Root: dir, Term: true}); res.Reason != ReasonInvalidName {
		t.Errorf("%+v", res)
	}
	if res := r.RenameImpact("yok_boyle", "x", RenameOptions{Root: dir, NoText: true}); res.Status != StatusNotFound {
		t.Errorf("without text there is nothing to list: %+v", res)
	}
}

func hitSummaryOfChanges(list []Change) []string {
	var out []string
	for _, c := range list {
		out = append(out, hitLine(c.Pos, c.Where, c.Match))
	}
	return out
}
