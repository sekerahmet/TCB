package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// dropH0002 H0002'yi hem ileti kataloğundan hem tanımlardan siler ve commit'ler.
func dropH0002(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "internal", "lang", "messages")
	replaceIn(t, filepath.Join(dir, "tr.json"), ",\n\"H0002\": \"second message\"", "")
	replaceIn(t, filepath.Join(dir, "definitions.json"), ",\n\"H0002\": { \"param\": [] }", "")
	commitAll(t, root, "messages: drop H0002")
}

func runDeletedIn(root string, args ...string) (int, string, string) {
	return runCLI(append([]string{"deleted", "--root", root}, args...)...)
}

func TestDeletedListsCodesGoneFromTheDictionary(t *testing.T) {
	root := gitFixture(t)
	code, out, errOut := runDeletedIn(root)
	if code != exitOK || !strings.Contains(out, "messages: 0 deleted codes") {
		t.Fatalf("nothing is deleted yet: exit %d\n%s%s", code, out, errOut)
	}
	dropH0002(t, root)
	code, out, errOut = runDeletedIn(root)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	testkit.MustContain(t, out, "messages: 1 deleted codes", "H0002  removed ", "messages: drop H0002", "(added ", "fixture")
	if strings.Contains(out, "defined again") {
		t.Errorf("H0002 is not defined again:\n%s", out)
	}
}

func TestDeletedFlagsRedefinedCodes(t *testing.T) {
	root := gitFixture(t)
	dropH0002(t, root)
	dir := filepath.Join(root, "internal", "lang", "messages")
	replaceIn(t, filepath.Join(dir, "tr.json"), "\"H0001\": \"first message {name}\"", "\"H0001\": \"first message {name}\",\n\"H0002\": \"back\"")
	commitAll(t, root, "messages: reuse H0002 (wrong)")
	_, out, _ := runDeletedIn(root)
	testkit.MustContain(t, out, "messages: 0 deleted codes", "1 codes defined again after being deleted", "H0002  removed ")
}

func TestDeletedRetiredJSON(t *testing.T) {
	root := gitFixture(t)
	dropH0002(t, root)
	code, out, errOut := runDeletedIn(root, "--retired-json")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var got map[string]struct{ Commit, Reason string }
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	e, ok := got["H0002"]
	if !ok || len(got) != 1 || e.Commit == "" || !strings.Contains(e.Reason, "drop H0002") {
		t.Errorf("retired.json content: %+v", got)
	}
	if code, _, _ := runDeletedIn(root, "--retired-json", "--family", "nope"); code != exitUsage {
		t.Errorf("unknown family: exit %d", code)
	}
}

// reusedBeforeTheRule silindikten sonra yeniden tanımlanmış dört ileti kodudur: ikisi retired.json'dan (0898aa1e)
// önce silindi, ikisi sonra ama dosyaya yazılmadı. Kural öncesi yeniden kullanımdır; canlı kalırlar, yeniden
// numaralanmazlar (TCB, 2026-11-14). Listeye yeni kod girmez: giren kod TestEveryDeletedMessageCodeIsRetired'i düşürür.
var reusedBeforeTheRule = map[string]bool{"H2487": true, "H3088": true, "H5291": true, "H8604": true}

// retiredCheck silinmiş ileti kodlarının retired.json'a göre eksikleridir.
type retiredCheck struct {
	missing     []string // geçmişte silinmiş, bugün canlı değil, retired.json'da yok
	again       []string // silindikten sonra yeniden tanımlanmış, reusedBeforeTheRule'da değil
	uncommitted []string // HEAD'in ileti dosyalarında var, çalışma ağacındakilerde ve retired.json'da yok
	entries     string   // uncommitted için retired.json'a önerilen girdiler
}

// checkRetiredCodes root'taki ileti kodlarını denetler. deletedCodes silmeyi git geçmişinden okur; commit'e girmemiş
// silmeyi HEAD'deki dosyalarla çalışma ağacındakilerin farkı verir (commit kipi testi commit'ten önce koşar).
func checkRetiredCodes(root string) (retiredCheck, error) {
	var c retiredCheck
	retired := currentCodes(root, []string{messagesDir + "/retired.json"})
	gone, redefined, err := deletedCodes(root, codeFamily{"messages", messageFiles})
	if err != nil {
		return c, err
	}
	listed := map[string]bool{}
	for _, g := range gone {
		if !retired[g.code] {
			c.missing = append(c.missing, g.code)
			listed[g.code] = true
		}
	}
	for _, g := range redefined {
		if !reusedBeforeTheRule[g.code] {
			c.again = append(c.again, g.code)
		}
	}
	where, err := deletingCommit(root)
	if err != nil {
		return c, err
	}
	cur := currentCodes(root, messageFiles)
	var entries []string
	for _, k := range headCodes(root, messageFiles) {
		if !cur[k] && !retired[k] && !listed[k] {
			c.uncommitted = append(c.uncommitted, k)
			entries = append(entries, fmt.Sprintf(`"%s": {"commit": "%s", "reason": "deleted: <why>"}`, k, where))
		}
	}
	c.entries = strings.Join(entries, ", ")
	return c, nil
}

// headCodes dosyaların HEAD'deki kodlarını sıralı verir (HEAD'de olmayan dosya atlanır).
func headCodes(root string, files []string) []string {
	seen := map[string]bool{}
	for _, f := range files {
		data, err := gitOut(root, "show", "HEAD:"+f)
		if err != nil {
			continue
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(data, &m) == nil {
			for k := range m {
				seen[k] = true
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// deletingCommit commit'e girmemiş bir silmenin retired.json'daki commit alanıdır: dalın adı; main'de ve dal
// dışında dal adı iz vermez, silen commit de HEAD değil ondan sonrakidir: "after <HEAD'in 8 karakteri>".
func deletingCommit(root string) (string, error) {
	out, err := gitOut(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if b := strings.TrimSpace(string(out)); b != "main" && b != "HEAD" {
		return b, nil
	}
	out, err = gitOut(root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return "after " + strings.TrimSpace(string(out))[:8], nil
}

// Kural: silinmiş bir ileti numarası yeniden kullanılmaz. msgconsts yalnız retired.json'daki kodun yeniden tanımını
// reddeder (tools/gen/msgconsts/main.go, checkRetired); bu test silinen her kodun orada olduğunu deponun kendi
// geçmişiyle sınar. Geçici depoda her yolun iletisi izlenir: TestRetiredCheck* testleri.
func TestEveryDeletedMessageCodeIsRetired(t *testing.T) {
	root := reporoot.Root()
	out, err := gitOut(root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "false" {
		t.Skip("shallow clone: the history of the message files is incomplete, so deleted codes cannot be listed")
	}
	if len(currentCodes(root, []string{messagesDir + "/retired.json"})) == 0 {
		t.Fatalf("%s/retired.json has no codes", messagesDir)
	}
	c, err := checkRetiredCodes(root)
	if err != nil {
		t.Fatal(err)
	}
	reportRetiredCheck(t, c)
}

// reportRetiredCheck eksiklerin iletilerini yazar.
func reportRetiredCheck(t *testing.T, c retiredCheck) {
	t.Helper()
	if len(c.missing) > 0 {
		t.Errorf("%d deleted message codes are not in internal/lang/messages/retired.json: %s; a deleted number must not be reused: add each one from go run ./tools/codegraph deleted --retired-json and keep the existing entries as they are", len(c.missing), strings.Join(c.missing, ", "))
	}
	if len(c.again) > 0 {
		t.Errorf("%d deleted message codes are defined again: %s; a deleted number must not be reused: give each new message a new code, then add the old code from go run ./tools/codegraph deleted --retired-json to internal/lang/messages/retired.json (only H2487, H3088, H5291 and H8604 were reused before the rule and stay)", len(c.again), strings.Join(c.again, ", "))
	}
	if len(c.uncommitted) > 0 {
		t.Errorf("%d message codes are in HEAD but not in the working tree, and not in internal/lang/messages/retired.json: %s; a deleted number must not be reused: add these entries to retired.json: %s", len(c.uncommitted), strings.Join(c.uncommitted, ", "), c.entries)
	}
}

// mustCheckRetired geçici depoyu denetler.
func mustCheckRetired(t *testing.T, root string) retiredCheck {
	t.Helper()
	c, err := checkRetiredCodes(root)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// writeRetired retired.json'ı yazar.
func writeRetired(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(messagesDir), "retired.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// S1'in önerisi: deleted --retired-json'ın çıktısı eksik kodu kapatır.
func TestRetiredCheckAsksForEachDeletedCode(t *testing.T) {
	root := gitFixture(t)
	if c := mustCheckRetired(t, root); c.missing != nil || c.again != nil || c.uncommitted != nil {
		t.Fatalf("nothing is deleted yet: %+v", c)
	}
	dropH0002(t, root)
	if c := mustCheckRetired(t, root); strings.Join(c.missing, ",") != "H0002" || c.uncommitted != nil {
		t.Fatalf("H0002 is deleted and not retired: %+v", c)
	}
	_, out, _ := runDeletedIn(root, "--retired-json")
	writeRetired(t, root, out)
	if c := mustCheckRetired(t, root); c.missing != nil || c.again != nil || c.uncommitted != nil {
		t.Errorf("retired.json from deleted --retired-json: %+v", c)
	}
}

// S5'in önerisi: yeni iletiye yeni kod verilir, eski kod retired.json'a girer.
func TestRetiredCheckRejectsAReusedCode(t *testing.T) {
	root := gitFixture(t)
	dropH0002(t, root)
	dir := filepath.Join(root, "internal", "lang", "messages")
	replaceIn(t, filepath.Join(dir, "tr.json"), "\"H0001\": \"first message {name}\"", "\"H0001\": \"first message {name}\",\n\"H0002\": \"back\"")
	commitAll(t, root, "messages: reuse H0002 (wrong)")
	if c := mustCheckRetired(t, root); strings.Join(c.again, ",") != "H0002" {
		t.Fatalf("H0002 is defined again: %+v", c)
	}
	replaceIn(t, filepath.Join(dir, "tr.json"), "\"H0002\": \"back\"", "\"H0003\": \"back\"")
	commitAll(t, root, "messages: the new message is H0003")
	_, out, _ := runDeletedIn(root, "--retired-json")
	writeRetired(t, root, out)
	if c := mustCheckRetired(t, root); c.missing != nil || c.again != nil || c.uncommitted != nil {
		t.Errorf("after the advice: %+v", c)
	}
}

// S6'nın önerisi: commit'e girmemiş silme; girdinin commit alanı dalda dalın adı, main'de ve dal dışında
// "after <HEAD'in 8 karakteri>". Önerilen girdiler, gerekçe yazılınca, retired.json olarak geçerlidir.
func TestRetiredCheckSeesAnUncommittedDeletion(t *testing.T) {
	root := gitFixture(t)
	dir := filepath.Join(root, "internal", "lang", "messages")
	replaceIn(t, filepath.Join(dir, "tr.json"), ",\n\"H0002\": \"second message\"", "")
	replaceIn(t, filepath.Join(dir, "definitions.json"), ",\n\"H0002\": { \"param\": [] }", "")
	head, err := gitOut(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	after := `"H0002": {"commit": "after ` + string(head[:8]) + `", "reason": "deleted: <why>"}`
	for _, step := range []struct {
		git  []string
		want string
	}{
		{nil, after},
		{[]string{"switch", "-q", "-c", "topic"}, `"H0002": {"commit": "topic", "reason": "deleted: <why>"}`},
		{[]string{"switch", "-q", "--detach"}, after},
	} {
		if step.git != nil {
			gitMust(t, root, step.git...)
		}
		c := mustCheckRetired(t, root)
		if strings.Join(c.uncommitted, ",") != "H0002" || c.missing != nil || c.entries != step.want {
			t.Errorf("after git %v: %+v, want entries %s", step.git, c, step.want)
		}
	}
	writeRetired(t, root, "{"+strings.ReplaceAll(mustCheckRetired(t, root).entries, "<why>", "the second message went")+"}")
	if c := mustCheckRetired(t, root); c.missing != nil || c.again != nil || c.uncommitted != nil {
		t.Errorf("after the advice: %+v", c)
	}
}
