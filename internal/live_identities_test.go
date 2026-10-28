package internal_test

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// classedIdentity sınıflı kimlik yazımıdır (TR_fn_print, TR_type_Int, TR_mem_length): belgede, yorumda ve iletide
// bir kodu anar. Program metnindeki yazım (TR_print) sınıfsızdır, burada aranmaz.
var classedIdentity = regexp.MustCompile(`(^|[^\p{L}\p{N}_])(TR_(?:kw|ctx|val|type|fn|mem|arg|const|cmd|sub|opt|repl|dbg|env|mod)_[A-Za-z0-9_]+)`)

// deadIdentityAllowed canlı olmayan sınıflı kimliği bilerek yazan yerlerdir: dosya → kimlik → gerekçe ("*" dosyanın
// bütün eşleşmeleri). Liste küçük tutulur; yeni bir kayıt gerekçesiyle girer.
var deadIdentityAllowed = map[string]map[string]string{
	"internal/live_identities_test.go":                            {"*": "bu testin kendisi: izin listesi bilerek yazılan kimlikleri anar"},
	"docs/spec/17_kodlar.md":                                      {"TR_fn_yok": "bilinmeyen kimlik iletisinin örneği (17.2)"},
	"docs/spec/rationale/02_lexical_structure.md":                 {"TR_fn_argv": "84ee01b8'deki ölçümün alıntısı: bugünkü TR'nin öneri vermediği ad"},
	"docs/code_identities.md":                                     {"*": "eski kodu kimliğe eşleyen donmuş tablo: silinen kimlikleri bilerek taşır"},
	"internal/lang/codes/language_test.go":                        {"*": "kod listesinin denetimi: geçersiz ve uydurma kimliklerin örnekleri"},
	"internal/lang/messages/template_test.go":                     {"*": "ileti kalıbının denetimi: uydurma kimlik örnekleri"},
	"tools/codegraph/renames_test.go":                             {"*": "yeniden adlandırma geçmişinin uydurma kodları"},
	"tools/migrate/files.go":                                      {"TR_kw_while": "eski biçimin sözcüğü: çevirici onu çevirir"},
	"tools/migrate/main.go":                                       {"TR_kw_while": "eski biçimin sözcüğü", "TR_kw_default": "eski biçimin sözcüğü"},
	"tools/migrate/removed.json":                                  {"*": "eski biçimden kalkan sözcükler"},
	"tools/migrate/syntax.go":                                     {"TR_kw_while": "eski biçimin sözcüğü", "TR_kw_default": "eski biçimin sözcüğü"},
	"internal/devtools/testrunner/behavior_language_spec_test.go": {"TR_fn_str": "program metninde kimlik yerine yazılan sınıflı eski yazımın örneği"},
	"lib/iyilestirici.tr":                                         {"TR_mem_ple_table": "parametre yolu deseninin örneği", "TR_mem_embedding_x": "parametre yolu deseninin uymayan örneği"},
	"lib/iyilestirici_test.tr":                                    {"TR_mem_ple_table": "parametre yolu deseninin testi", "TR_mem_embedding_x": "parametre yolu deseninin testi"},
}

// Kural (17.6): bir kimlik silinince satırı kod listesinden silinir; onu anan belge, yorum ve ileti de güncellenir.
// İzlenen metin dosyalarında (arşiv hariç) yazılan her sınıflı kimlik canlı bir koda gider. Neden: silinen kimliği
// anan metin okuyanı (ajanı) artık olmayan bir ada götürür ve kimse fark etmez (katman 2'de 33 dosyada 150 kez kaldı).
func TestClassedIdentitiesAreLive(t *testing.T) {
	live := map[string]bool{}
	for _, k := range codes.Codes() {
		live[k.String()] = true
	}
	var bad []string
	used := map[string]bool{}
	skip := func(rel string) bool { return strings.HasPrefix(rel, "docs/archive/") } // geçmiş: o günün kimlikleri
	scanTextLines(t, skip, func(rel string, line int, text string) {
		allowed := deadIdentityAllowed[rel]
		for _, m := range classedIdentity.FindAllStringSubmatch(text, -1) {
			id := m[2]
			if live[id] {
				continue
			}
			if allowed["*"] != "" {
				used[rel+" *"] = true
				continue
			}
			if _, ok := allowed[id]; ok {
				used[rel+" "+id] = true
				continue
			}
			bad = append(bad, rel+":"+strconv.Itoa(line)+": "+id)
		}
	})
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%d canlı olmayan kimlik (silinen kodun yeni adını yazın; bilerek yazılmışsa gerekçesiyle deadIdentityAllowed'a):\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
	for file, ids := range deadIdentityAllowed {
		for id := range ids {
			if !used[file+" "+id] {
				t.Errorf("deadIdentityAllowed: %s dosyasında %s artık geçmiyor; kaydı silin", file, id)
			}
		}
	}
}
