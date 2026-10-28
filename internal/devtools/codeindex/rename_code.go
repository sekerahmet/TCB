package codeindex

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Kitaplığın kodlu bir adının (Node.Code taşıyan lib/ bildirimi: parametre, yapı, işlev, alan, metot) yeniden
// adlandırılması dil dosyasında ad değişikliğidir: kodlu dosyalar kimliği taşır, adı değil; onlar olduğu gibi kalır.
// Değişen okuyanın dil dosyasındaki satırdır (internal/lang/codes/<dil>.json); dosyalar kimliğin yazımını taşır
// (TR_num_steps, dil tanımı 2.3a), dilin adını değil. Aynı kodu taşıyan öbür bildirimler de aynı adı alır: bağlı
// listede. Yeni ad kullanıcıya görünen bir addır, baş geliştiricinin onayıyla gelir (CLAUDE.md); önizleme bunu söyler
// (H8871). Kullanıcı yapısının aynı kodlu
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
	for i := range r.Index.Nodes {
		if other := &r.Index.Nodes[i]; other.Code == n.Code && other.ID != n.ID {
			res.Linked = append(res.Linked, r.display(*other))
		}
	}
	return finishRename(res, []string{old}, newName, o, map[string]bool{}, lines)
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
