package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Bu dosya depodaki <alıcı>.TR_<ad> yazımlarının altın dosyasıdır (testdata/member_spellings.txt): iki aracın
// (codegraph ve trlang index) her yazıma verdiği cevap ve ikisinin eşitliği.

const memberSpellingsHeader = "# İzlenen .tr, .go ve .md dosyalarındaki <alıcı>.TR_<ad> yazımlarının iki araçtaki cevabı.\n" +
	"# Biçim: <yazım>\\t<durum>\\t<çıkış>\\t<adaylar> (yalnız codegraph'ta spec: adayı varsa sonunda \\ttr_index:not_found:4).\n" +
	"# Yeniden yazmak için: go test ./tools/codegraph -run TestDottedMemberSpellingsGolden -update\n"

var goldenClassedIdentity = regexp.MustCompile(`(^|[^\p{L}\p{N}_])(TR_(?:kw|ctx|val|type|fn|mem|arg|const|cmd|sub|opt|repl|dbg|env|mod)_[A-Za-z0-9_]+)`)

type memberAnswer struct {
	status string
	exit   int
	cands  []string
}

func (a memberAnswer) equal(b memberAnswer) bool {
	return a.status == b.status && a.exit == b.exit && slices.Equal(a.cands, b.cands)
}

func (a memberAnswer) format() string {
	c := "-"
	if len(a.cands) > 0 {
		c = strings.Join(a.cands, ",")
	}
	return a.status + "\t" + strconv.Itoa(a.exit) + "\t" + c
}

func buildMemberAnswer(cands []string) memberAnswer {
	slices.Sort(cands)
	cands = slices.Compact(cands)
	switch len(cands) {
	case 0:
		return memberAnswer{status: string(codeindex.StatusNotFound), exit: exitNotFound}
	case 1:
		return memberAnswer{status: string(codeindex.StatusOK), exit: exitOK, cands: cands}
	default:
		return memberAnswer{status: string(codeindex.StatusAmbiguous), exit: exitAmbiguous, cands: cands}
	}
}

func candLabel(id, noteCode string) string {
	if noteCode != "" {
		return id + "[" + noteCode + "]"
	}
	return id
}

func collectRepoMemberSpellings(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	files := codeindex.ListFiles(root, nil, codeindex.DefaultSkipDirs, codeindex.TrackedFiles, func(rel string) bool {
		ext := filepath.Ext(rel)
		return ext == ".tr" || ext == ".go" || ext == ".md"
	})
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		addRepoMemberSpellings(string(data), seen)
	}
	return slices.Sorted(maps.Keys(seen))
}

func hasDeadClassedIdentity(s string) bool {
	for _, m := range goldenClassedIdentity.FindAllStringSubmatch(s, -1) {
		if _, err := codes.Parse(m[2]); err != nil {
			return true
		}
	}
	return false
}

// diffGoldenLines altın dosyayla bugünkü cevapları karşılaştırır. Altın dosyada olmayan ve cevabı `ok` olan yazım fark
// değildir, accepted'a girer (K3): yeni bir satırın çözülen üye yazımı testi düşürmez, -update onu kaydeder. Fark
// kalanlar: var olan yazımın değişen cevabı, kalkan yazım, yeni yazımın `ok` olmayan cevabı (ambiguous, not_found).
// Kabul edilen satırın adayı ancak bir sonraki -update'te altın dosyaya girer; o zamana kadar onu yalnız iki aracın
// eşitliği korur.
func diffGoldenLines(wantText string, gotLines []string) (diffs, accepted []string) {
	wantMap, gotMap := map[string]string{}, map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(wantText, "\r\n", "\n"), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			q, rest, _ := strings.Cut(line, "\t")
			wantMap[q] = rest
		}
	}
	for _, line := range gotLines {
		q, rest, _ := strings.Cut(line, "\t")
		gotMap[q] = rest
	}
	keys := map[string]bool{}
	maps.Copy(keys, mapKeysBool(wantMap))
	maps.Copy(keys, mapKeysBool(gotMap))
	for _, q := range slices.Sorted(maps.Keys(keys)) {
		w, okW := wantMap[q]
		g, okG := gotMap[q]
		switch {
		case !okW && strings.HasPrefix(g, string(codeindex.StatusOK)+"\t"):
			accepted = append(accepted, q)
		case !okW:
			diffs = append(diffs, "+ "+q+"\t"+g)
		case !okG:
			diffs = append(diffs, "- "+q+"\t"+w)
		case w != g:
			diffs = append(diffs, q+":\n    want: "+w+"\n     got: "+g)
		}
	}
	return diffs, accepted
}

// acceptedNote kabul edilen yeni `ok` yazımlarının günlük satırıdır: sayısı ve ilk beşi.
func acceptedNote(accepted []string, path string) string {
	return fmt.Sprintf("%d new spelling(s) answered ok are not in %s yet; accepted (-update records them): %s",
		len(accepted), path, strings.Join(accepted[:min(len(accepted), 5)], ", "))
}

func mapKeysBool(m map[string]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// Kural (DENETİM istek 58): depoda izlenen .tr, .go ve .md dosyalarındaki her `<alıcı>.TR_<ad>` yazımı iki araca da
// sorulur; spec: dil tanımı düğümleri dışındaki adaylarda iki araç birebir aynı cevabı (durum, çıkış, aday kimlikleri,
// etiket kodları) verir ve bütün cevaplar testdata/member_spellings.txt altın dosyasıyla eşleşir.
func TestDottedMemberSpellingsGolden(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	idx, err := repoIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	reader := codeindex.NewReader(idx)
	start := time.Now()
	spellings := collectRepoMemberSpellings(t, root)
	if len(spellings) < 1000 {
		t.Fatalf("only %d member spellings found across .tr, .go, .md files", len(spellings))
	}
	var gotLines, toolDiffs []string
	for _, q := range spellings {
		var cgAll, cgTR, trAll []string
		hasSpec := false
		for _, n := range ix.lookup(q) {
			lbl := candLabel(n.ID, n.NoteCode)
			cgAll = append(cgAll, lbl)
			if n.Kind == kindSpec {
				hasSpec = true
			} else {
				cgTR = append(cgTR, lbl)
			}
		}
		for _, n := range reader.Resolve(q, "") {
			trAll = append(trAll, candLabel(graphID(*n), n.NoteCode))
		}
		cgAns, cgTRAns, trAns := buildMemberAnswer(cgAll), buildMemberAnswer(cgTR), buildMemberAnswer(trAll)
		if !cgTRAns.equal(trAns) || (hasSpec && len(cgTR) > 0) {
			toolDiffs = append(toolDiffs, q+": codegraph="+cgAns.format()+" vs tr_index="+trAns.format())
		}
		if hasDeadClassedIdentity(q) {
			// Kural 17.4, 17.10 (TestClassedIdentitiesAreLive): bilerek uydurma sınıflı kimlikler (lib/optimizers.tr'deki
			// negatif desen örneği gibi) başka izlenen dosyaya kopyalanmaz; iki aracın da "bulunamadı" döndüğü denetlenir.
			if cgAns.exit != exitNotFound || trAns.exit != exitNotFound {
				toolDiffs = append(toolDiffs, q+": dead classed identity resolved: "+cgAns.format())
			}
			continue
		}
		line := q + "\t" + cgAns.format()
		if hasSpec {
			line += "\ttr_index:" + trAns.status + ":" + strconv.Itoa(trAns.exit)
		}
		gotLines = append(gotLines, line)
	}
	t.Logf("%d spellings checked in %s (%d in golden file)", len(spellings), time.Since(start).Round(time.Millisecond), len(gotLines))
	if len(toolDiffs) > 0 {
		t.Fatalf("%d spellings differ between codegraph and trlang index:\n  %s", len(toolDiffs), strings.Join(toolDiffs[:min(len(toolDiffs), 30)], "\n  "))
	}
	path := filepath.Join("testdata", "member_spellings.txt")
	content := memberSpellingsHeader + strings.Join(gotLines, "\n") + "\n"
	if *updateGolden {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	diffs, accepted := diffGoldenLines(string(wantBytes), gotLines)
	if len(accepted) > 0 {
		t.Log(acceptedNote(accepted, filepath.ToSlash(path)))
	}
	if len(diffs) > 0 {
		t.Errorf("%d member spelling answers changed in %s (run with -update to rewrite):\n  %s", len(diffs), path, strings.Join(diffs[:min(len(diffs), 30)], "\n  "))
	}
}

// Kural (K3): altın dosyayla karşılaştırmada altın dosyada olmayan ve cevabı `ok` olan yazım fark değildir, kabul edilir
// ve günlüğe yazılır; fark kalanlar: var olan yazımın değişen cevabı, kalkan yazım, yeni yazımın `ok` olmayan cevabı.
// Kabul edilenlerin sayısı ve ilk beşi günlük satırındadır.
func TestGoldenDiffAcceptsOnlyNewOkSpellings(t *testing.T) {
	// Örnek yazımlar kaynakta `~` ile yazılır: bu dosya da depodaki yazımların taramasına girer.
	sp := func(s string) string { return strings.ReplaceAll(s, "~", ".") }
	want := "# başlık\n" +
		"a~TR_x\tok\t0\tid1\n" +
		"b~TR_y\tok\t0\tid2\n" +
		"c~TR_z\tok\t0\tid3\n"
	got := []string{
		"a~TR_x\tok\t0\tid1",
		"b~TR_y\tambiguous\t3\tid2,id9",
		"d~TR_n\tok\t0\tid4",
		"e~TR_n\tnot_found\t4\t-",
		"f~TR_n\tambiguous\t3\tid5,id6",
	}
	for i := range got {
		got[i] = sp(got[i])
	}
	diffs, accepted := diffGoldenLines(sp(want), got)
	if !slices.Equal(accepted, []string{sp("d~TR_n")}) {
		t.Errorf("accepted %v, want only the new ok spelling d~TR_n", accepted)
	}
	wantDiffs := []string{
		"b~TR_y:\n    want: ok\t0\tid2\n     got: ambiguous\t3\tid2,id9",
		"- c~TR_z\tok\t0\tid3",
		"+ e~TR_n\tnot_found\t4\t-",
		"+ f~TR_n\tambiguous\t3\tid5,id6",
	}
	for i := range wantDiffs {
		wantDiffs[i] = sp(wantDiffs[i])
	}
	if !slices.Equal(diffs, wantDiffs) {
		t.Errorf("diffs:\n%q\nwant:\n%q", diffs, wantDiffs)
	}
	const path = "testdata/member_spellings.txt"
	if got, want := acceptedNote([]string{"d"}, path), "1 new spelling(s) answered ok are not in testdata/member_spellings.txt yet; accepted (-update records them): d"; got != want {
		t.Errorf("note:\n%s\nwant:\n%s", got, want)
	}
	seven := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"}
	if got, want := acceptedNote(seven, path), "7 new spelling(s) answered ok are not in testdata/member_spellings.txt yet; accepted (-update records them): a1, a2, a3, a4, a5"; got != want {
		t.Errorf("note lists the first five:\n%s\nwant:\n%s", got, want)
	}
}
