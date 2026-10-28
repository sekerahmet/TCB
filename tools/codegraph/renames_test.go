package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// renameIn bir ad kodunun anahtarını iki dil dosyasında değiştirir, eşleme belgesine satırını
// ekler ve commit'ler (madde 15 adım 0'ın yaptığı gibi).
func renameIn(t *testing.T, root, old, new, also string) {
	t.Helper()
	dir := filepath.Join(root, "internal", "lang", "codes")
	for _, f := range []string{"tr.json", "en.json"} {
		replaceIn(t, filepath.Join(dir, f), `"`+old+`":`, `"`+new+`":`)
	}
	doc := filepath.Join(root, filepath.FromSlash(identityMaps[0]))
	row := "| " + old + " | `" + new + "` | x |\n"
	if old == "TR_print" {
		testkit.WritePath(t, doc, "| Eski kod | Kimlik | Go sabiti |\n| :-- | :-- | :-- |\n"+row)
	} else {
		replaceIn(t, doc, "| :-- | :-- | :-- |\n", "| :-- | :-- | :-- |\n"+row)
	}
	if also != "" {
		replaceIn(t, filepath.Join(dir, "tr.json"), also, "")
		replaceIn(t, filepath.Join(dir, "en.json"), strings.ReplaceAll(also, "eger", "if"), "")
	}
	commitAll(t, root, "codes: a key is renamed")
}

// Kural: eşleme belgesindeki her "eski → yeni" çifti bir yeniden adlandırma olayıdır: deleted'a
// girmez, history eski anahtarın geçmişiyle sürer (zincir de: print → write → output), eski
// anahtar sorgusu bugünkü koda gider. Gerçekten silinen kod deleted'da kalır.
func TestARenamedCodeIsNotDeletedAndKeepsItsHistory(t *testing.T) {
	slowFixture(t)
	root := gitFixture(t)
	renameIn(t, root, "TR_print", "TR_fn_write", "")
	renameIn(t, root, "TR_fn_write", "TR_fn_output", "\n  \"TR_kw_if\": { \"name\": \"eger\", \"family\": \"keyword\" },")

	_, out, _ := runDeletedIn(root)
	testkit.MustContain(t, out, "names: 1 deleted codes", "TR_if  removed ")
	if strings.Contains(out, "TR_print") || strings.Contains(out, "TR_fn_write") {
		t.Errorf("a renamed code is not deleted:\n%s", out)
	}
	graph := buildInto(t, root)
	code, out, errOut := commandIn("history", root, graph, "TR_fn_output")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "renamed TR_fn_write → TR_fn_output", "renamed TR_print → TR_fn_write", "added    ")
	if i, j, k := strings.Index(out, "→ TR_fn_output"), strings.Index(out, "→ TR_fn_write"), strings.Index(out, "added"); !(i < j && j < k) {
		t.Errorf("events newest first, the first add last:\n%s", out)
	}
	if strings.Contains(out, "removed") {
		t.Errorf("a rename is not a removal:\n%s", out)
	}
	code, out, _ = commandIn("query", root, graph, "TR_print")
	if code != exitOK || !strings.HasPrefix(out, "TR_fn_output [name]") {
		t.Errorf("the old key leads to the code: exit %d\n%s", code, out)
	}
}

// Kural: hiçbir canlı kimliğin eski anahtarı deleted'da yoktur ve her kimliğin geçmişi eski
// anahtarının ilk eklenişiyle başlar. Örnekle değil, eşleme belgesinin her satırıyla ve deponun
// kendi geçmişiyle sınanır; yeni satırlar (bir kimliğin yeniden adlandırılması, kitaplık kodları)
// aynı testten geçer.
func TestEveryRenameInTheMappingKeepsItsHistory(t *testing.T) {
	slowFixture(t)
	root := reporoot.Root()
	if out, err := exec.Command("git", "-C", root, "rev-parse", "--is-shallow-repository").Output(); err != nil || strings.TrimSpace(string(out)) != "false" {
		t.Skip("tam git geçmişi yok")
	}
	renames := readRenames(root)
	doc, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(identityMaps[0])))
	if err != nil {
		t.Fatal(err)
	}
	rows := len(regexp.MustCompile("(?m)^\\| [A-Z]\\d{3} \\| `TR_").FindAll(doc, -1)) // bağımsız sayım: eski kodla başlayan satırlar
	if rows == 0 || len(renames) != rows {
		t.Fatalf("%d renames read, the mapping has %d rows", len(renames), rows)
	}
	cur := currentCodes(root, nameFiles)
	gone, redefined, err := deletedCodes(root, codeFamily{"names", nameFiles})
	if err != nil {
		t.Fatal(err)
	}
	former := formerKeys(renames)
	for _, g := range gone {
		if id, renamed := renames[g.code]; renamed {
			t.Errorf("%s is listed as deleted, but it was renamed to %s", g.code, id)
		}
	}
	for _, g := range redefined {
		if len(former[g.code]) > 0 || renames[g.code] != "" {
			t.Errorf("%s is listed as defined again after a deletion, but it was a rename", g.code)
		}
	}
	events, err := scanCodeHistory(root, nameFiles)
	if err != nil {
		t.Fatal(err)
	}
	merged := followRenames(events, renames)
	for old, id := range renames {
		if !cur[id] {
			continue // kimlik sonradan silindi: geçmişi deleted'dadır
		}
		list := merged[id]
		if len(list) == 0 || !list[len(list)-1].added || list[len(list)-1].key != old {
			t.Errorf("%s: its history does not start with the first add of %s: %+v", id, old, list)
		}
		if !slices.ContainsFunc(list, func(e codeEvent) bool { return e.from == old }) {
			t.Errorf("%s: no rename event from %s: %+v", id, old, list)
		}
		if _, left := merged[old]; left {
			t.Errorf("%s still has a history of its own after the rename to %s", old, id)
		}
	}
	if list := merged["TR_sum"]; len(list) == 0 || list[len(list)-1].commit.date != "2026-10-19" {
		t.Errorf("TR_sum was first added on 2026-10-19, under its old key: %+v", list)
	}
	t.Logf("%d renames, %d codes really deleted", len(renames), len(gone))
}
