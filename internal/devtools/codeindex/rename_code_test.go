package codeindex

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural: kodlu bir adı (kitaplığın parametresi, yapısı, işlevi; Node.Code) yeniden adlandırmak dil dosyasında ad
// değişikliğidir: kodlu dosyalar kimliği taşır ve olduğu gibi kalır, kesin listede yer almaz; kesin listede okuyanın dil
// dosyasındaki satır vardır; aynı kodu taşıyan öbür bildirimler bağlı listede, kip ModeLanguage (yeni ad baş
// geliştiricinin onayıyla gelir). Depo üzerinde koşar: dil dosyaları ve kodlu kitaplık oradadır.
func TestACodedNameIsRenamedInTheLanguageFile(t *testing.T) {
	root := reporoot.Root()
	idx, err := Build(root, Options{Dirs: []string{"lib", "examples", "tools"}, Tracked: true})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	coded := map[string]bool{}
	for _, n := range idx.Nodes {
		if n.Kind == KindModule && n.Coded {
			coded[n.Pos.File] = true
		}
	}
	for _, c := range []struct{ id, code, linked string }{
		{"parameter:lib/egitim.tr#TR_type_Run.TR_ctx_init.TR_arg_num_steps", "TR_num_steps", "parameter:lib/iyilestirici.tr#TR_fn_lr_schedule.TR_arg_num_steps"},
		{"struct:lib/egitim.tr#TR_type_Run", "TR_Run", ""},
		{"function:lib/iyilestirici.tr#TR_fn_lr_schedule", "TR_lr_schedule", ""},
	} {
		res := r.RenameImpact(c.id, "yeni_bir_ad", RenameOptions{Root: root, NoText: true})
		if res.Status != StatusOK || res.Mode != ModeLanguage {
			t.Errorf("%s: status %s, mode %q, want ok in the language mode", c.id, res.Status, res.Mode)
			continue
		}
		langFile := "internal/lang/codes/" + codes.Display().Code + ".json"
		found := false
		for _, ch := range res.Certain {
			if coded[ch.Pos.File] {
				t.Errorf("%s: a coded file is in the certain list: %s", c.id, ch.Pos)
			}
			found = found || ch.Pos.File == langFile && ch.Pos.Line > 0 && strings.Contains(ch.Context, `"`+c.code+`"`)
		}
		if !found {
			t.Errorf("%s: the certain list lacks %s's line of %s: %+v", c.id, langFile, c.code, res.Certain)
		}
		if c.linked != "" {
			linked := false
			for _, n := range res.Linked {
				linked = linked || n.ID == c.linked
			}
			if !linked {
				t.Errorf("%s: the other declaration with the same code (%s) is not listed: %+v", c.id, c.linked, res.Linked)
			}
		}
	}
}
