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

// Kural: .tr kaynağında metin olarak yazılan her ileti kodu (`TR_Message("H6932", x)`, `TR_error_code(h) == "H6932"`) ileti düğümünün bir
// başvurusudur: düğüm kodun kendisiyle bulunur, başvurular onu yazan bildirimden çıkar (`tr dizin kullananlar H6932`).
// Kapsam: depodaki her .tr dosyasının sözcük çözümleyicisinin verdiği her ileti kodu belirteci bir başvurudur.
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
	tokens := 0
	for _, n := range idx.Nodes {
		if n.Kind != KindModule || skipped[n.Pos.File] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.Pos.File)))
		if err != nil {
			t.Fatal(err)
		}
		b, err := source.Split(string(data))
		if err != nil {
			continue
		}
		l := b.IdentityLexer() // gövde başlık satırını içerir: satırlar dosyanınkidir (readIn gibi)
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
	if tokens < 20 {
		t.Fatalf("only %d message code tokens in the repository's .tr files; the scan is broken", tokens)
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
	for _, want := range []string{"method:lib/egitim.tr#TR_type_Run.TR_ctx_init", "test_block:lib/egitim_test.tr#"} {
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
