package codeindex

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Kitaplığın kodlu bir adının (Node.Code taşıyan lib/ bildirimi: parametre, yapı, işlev, alan, metot) yeniden
// adlandırılması dil dosyasında ad değişikliğidir: kodlu dosyalar kimliği taşır, adı değil; onlar olduğu gibi kalır.
// Değişen okuyanın dil dosyasındaki satırdır (internal/lang/codes/<dil>.json) ve, ad Türkçeyse, başlıksız (Türkçe yazılmış)
// dosyalardaki yazımı. Aynı kodu taşıyan öbür bildirimler de aynı adı alır: bağlı listede. Yeni ad kullanıcıya görünen
// bir addır, baş geliştiricinin onayıyla gelir (CLAUDE.md); önizleme bunu söyler (H8871). Kullanıcı yapısının aynı kodlu
// üyesi (kitaplığın üye adıyla yazılmış `ileri`) kullanıcının adıdır: eski plan geçerlidir, kimlik yeni adla değişir.

// languageFileDir dil dosyalarının depodaki klasörüdür.
const languageFileDir = "internal/lang/codes"

// renameCode kodlu hedefin (n.Code) yeniden adlandırma planıdır (RenameImpact'in ModeLanguage kipi).
func (r *Reader) renameCode(res *Result, n *Node, newName string, o RenameOptions) *Result {
	k, _ := codes.Parse(n.Code)
	lang := codes.Display()
	old := lang.Name(k)
	if reason := validateNewName(old, newName); reason != "" {
		res.Status, res.Reason = StatusRejected, reason
		return res
	}
	res.Mode = ModeLanguage
	res.Targets = append(res.Targets, r.display(*n))
	lines := &lineSource{root: o.Root}
	if row := languageRow(o.Root, lang.Code, n.Code); row.Pos.File != "" {
		row.Word, row.Replacement, row.Context = old, newName, lines.at(row.Pos)
		res.Certain = append(res.Certain, row)
	}
	skip := map[string]bool{}
	for i := range r.Index.Nodes {
		other := &r.Index.Nodes[i]
		if other.Code != n.Code {
			continue
		}
		if other.ID != n.ID {
			res.Linked = append(res.Linked, r.display(*other))
		}
		if lang.Code != codes.SourceLanguage {
			continue // başlıksız dosyalar Türkçe yazılmıştır, başka dildeki ad onlarda geçmez
		}
		for _, oc := range r.Occurrences(other.ID) {
			if m := r.byID[NodeID(KindModule, oc.Pos.File, "")]; m == nil || m.Coded {
				continue // kodlu dosya kimliği taşır
			}
			skip[oc.Pos.File+":"+strconv.Itoa(oc.Pos.Line)+":"+strconv.Itoa(oc.Pos.Col)] = true
			res.Certain = append(res.Certain, Change{Pos: oc.Pos, Node: other.ID, Role: oc.Role, Confidence: oc.Confidence,
				Basis: oc.Basis, Replacement: newName, Context: lines.at(oc.Pos)})
		}
	}
	return finishRename(res, []string{old}, newName, o, skip, lines)
}

// languageRow kodun dil dosyasındaki satırıdır (`"TR_arg_num_steps": { "name": ... }`); dosya kökte yoksa (TR'nin kendi
// deposu dışında bir proje) konum dosyanın yoludur, satırsız.
func languageRow(root, lang, code string) Change {
	rel := path.Join(languageFileDir, lang+".json")
	ch := Change{Pos: Pos{File: rel}, Where: WhereData, Match: MatchLanguageFile}
	if root == "" {
		return ch
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return ch
	}
	for i, line := range strings.Split(string(data), "\n") {
		if j := strings.Index(line, `"`+code+`"`); j >= 0 {
			ch.Pos.Line, ch.Pos.Col = i+1, 1+utf8.RuneCountInString(line[:j])
			return ch
		}
	}
	return ch
}
