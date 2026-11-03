package codeindex

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural: .tr kaynağında metin olarak yazılan her ileti kodu (`TR_Message("H6932", x)`, `TR_error_code(h) == "H6932"`) ve
// negatif fikstürlerin (`negative/*.tr`) satır sonu etiketlerindeki her ileti kodu ileti düğümünün bir başvurusudur:
// düğüm kodun kendisiyle bulunur, başvurular onu yazan bildirimden ya da modülden çıkar (`trlang index references H6932`).
// Kapsam: depodaki her .tr dosyasının sözcük çözümleyicisinin verdiği her ileti kodu belirteci ve her negatif fikstür etiketi bir başvurudur.
func TestEveryMessageCodeTokenIsAUse(t *testing.T) {
	root := reporoot.Root()
	idx, err := Build(root, Options{Dirs: []string{"lib", "examples", "tools", "tests"}, Tracked: true})
	if err != nil {
		t.Fatal(err)
	}
	skipped := map[string]bool{}
	for _, s := range idx.Skipped {
		skipped[s.File] = true
	}
	r := NewReader(idx)
	uses := map[string]bool{} // "dosya:satır:sütun kod"
	for _, o := range idx.Occurrences {
		if n := r.byID[o.Node]; n != nil && n.Kind == KindMessage {
			uses[o.Pos.File+":"+strconv.Itoa(o.Pos.Line)+":"+strconv.Itoa(o.Pos.Col)+" "+n.Name] = true
		}
	}
	tokens, labels := 0, 0
	for _, n := range idx.Nodes {
		if n.Kind != KindModule {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.Pos.File)))
		if err != nil {
			t.Fatal(err)
		}
		if IsNegativeFixture(n.Pos.File) {
			want, _ := FixtureLabels(string(data))
			for _, d := range want {
				labels++
				key := n.Pos.File + ":" + strconv.Itoa(d.Line) + ":0 " + d.Code
				if !uses[key] {
					t.Errorf("negative fixture label %s is not a use of its node", key)
				}
			}
		}
		if skipped[n.Pos.File] {
			continue
		}
		b, err := source.Split(string(data))
		if err != nil {
			continue
		}
		l := b.Lexer() // gövde başlık satırını içerir: satırlar dosyanınkidir (readIn gibi)
		for tok := l.NextToken(); tok.Type != lexer.TokEOF; tok = l.NextToken() {
			// İleti kodu bir metin olarak yazılır ("H6903").
			if _, err := messages.Parse(tok.Literal); tok.Type != lexer.TokString || err != nil {
				continue
			}
			tokens++
			key := n.Pos.File + ":" + strconv.Itoa(tok.Line) + ":" + strconv.Itoa(tok.Column) + " " + tok.Literal
			if !uses[key] {
				t.Errorf("message code %s is not a use of its node", key)
			}
		}
	}
	if tokens < 20 || labels < 50 {
		t.Fatalf("only %d message code tokens and %d fixture labels in the repository's .tr files; the scan is broken", tokens, labels)
	}
}

// Kural: negatif fikstürün (`negative/*.tr`) üç etiket türü (`ERROR`, `WARNING`, `RUN-ERROR`) ve çok kodlu etiketleri
// (`# ERROR H1026 H1027`) modülden ileti düğümüne bağ kurar; etiket olmayan yorumda geçen H-kodu ve negatif olmayan
// dosyadaki yorum bağ kurmaz.
func TestNegativeFixtureLabelsAreMessageUses(t *testing.T) {
	dir := t.TempDir()
	negDir := filepath.Join(dir, "tests", "negative")
	if err := os.MkdirAll(negDir, 0o755); err != nil {
		t.Fatal(err)
	}
	negSrc := "# TR 4\n" +
		"# TEST 2.6 negatif etiket türleri (bu yorumda H1028 ve H1029 var ama etiket değil)\n" +
		"TR_var a = 1 # sıradan yorum H1030 bağ kurmamalı\n" +
		"TR_var b = 2 # ERROR H1026 H1027\n" +
		"TR_var c = 3 # WARNING H1012\n" +
		"TR_var d = 4 # RUN-ERROR H2478\n"
	if err := os.WriteFile(filepath.Join(negDir, "etiketler.tr"), []byte(negSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	normalSrc := "# TR 4\nTR_var x = 1 # ERROR H1031\n"
	if err := os.WriteFile(filepath.Join(dir, "tests", "normal.tr"), []byte(normalSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	for _, want := range []struct {
		code string
		line int
	}{
		{"H1026", 4},
		{"H1027", 4},
		{"H1012", 5},
		{"H2478", 6},
	} {
		found := r.Resolve(want.code, "")
		if len(found) != 1 || found[0].Kind != KindMessage {
			t.Fatalf("Resolve(%s) = %+v, want one message node", want.code, found)
		}
		occs := r.Occurrences(found[0].ID)
		if len(occs) != 1 || occs[0].In != "module:tests/negative/etiketler.tr" || occs[0].Pos.Line != want.line {
			t.Errorf("%s occurrences = %+v, want module:tests/negative/etiketler.tr at line %d", want.code, occs, want.line)
		}
	}
	for _, unwanted := range []string{"H1028", "H1029", "H1030", "H1031"} {
		if found := r.Resolve(unwanted, ""); len(found) != 0 {
			t.Errorf("non-label comment code %s resolved to %+v, want no message node", unwanted, found)
		}
	}
}

// Kural: ileti kodu bir ad gibi bulunur (çıplak kod), türü iletidir ve yeniden adlandırılamaz (kod değişmez, metni ileti
// kataloğundadır); kullananları kitaplıkta onu üreten bildirim ve kodunu bekleyen testtir.
func TestAMessageCodeIsFoundAsAName(t *testing.T) {
	root := reporoot.Root()
	idx, err := Build(root, Options{Dirs: []string{"lib"}, Tracked: true})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	found := r.Resolve("H6932", "")
	if len(found) != 1 || found[0].Kind != KindMessage {
		t.Fatalf("Resolve(H6932) = %+v, want one message node", found)
	}
	var in []string
	for _, oc := range r.Occurrences(found[0].ID) {
		in = append(in, oc.In)
	}
	for _, want := range []string{"method:lib/training.tr#TR_type_Run.TR_ctx_init", "test_block:lib/training_test.tr#"} {
		ok := false
		for _, id := range in {
			ok = ok || id == want || want[len(want)-1] == '#' && len(id) > len(want) && id[:len(want)] == want
		}
		if !ok {
			t.Errorf("H6932's uses lack %s: %v", want, in)
		}
	}
	if res := r.RenameImpact("H6932", "H6933", RenameOptions{}); res.Status != StatusRejected || res.Reason != ReasonMessage {
		t.Errorf("rename H6932: status %s, reason %q, want rejected as a message code", res.Status, res.Reason)
	}
}
