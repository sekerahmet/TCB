package internal_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Go metninde kullanıcıya görünen Türkçe yoktur: adlar ad kodudur (internal/lang/codes), iletiler
// ileti kodudur (internal/lang/messages); metinleri dil dosyalarındadır. Bu test, test dışı Go
// dosyalarında Türkçe harf (çğıöşüâîû ÇĞİÖŞÜÂÎÛ) taşıyan metin sabitini arar. Bilinçli yerler
// turkishLiteralAllowed'da gerekçesiyle durur; liste yalnızca küçülür.
//
// Başka projelerin karşılığı: Android lint "HardcodedText", Kubernetes'in i18n denetimi ve
// go-i18n'in çıkarıcısı kaynak dilin metnini koddan dil dosyasına taşımayı ister. Türkçe harf
// ölçütü kaba ama kesindir (yanlış alarm vermez); Türkçe harfsiz sözcükleri ("bitti") ise
// ileti denetimi (msgconsts: fmt.Errorf/errors.New yasağı) ve çıktı yerlerinin taraması yakalar.

const turkishLetters = "çğıöşüâîûÇĞİÖŞÜÂÎÛ"

// turkishLiteralAllowed "dosya" ya da "dosya:metin" anahtarlı izinlerdir → gerekçe. Kalıcı izin
// yalnızca "dosya:metin" biçimindedir (veri biçimi anahtarı, Türkçe belge üreten aracın içeriği,
// Go yorumu üreten aracın yorum metni). "geçici" kayıtlar testin yazıldığı gün (2026-10-20)
// bulunanlardır; düzeltilince silinir.
var turkishLiteralAllowed = map[string]string{
	"cmd/tr/build.go":                               "geçici (ana oturum: lang/cmd/debugger)",
	"cmd/tr/coded.go":                               "geçici (ana oturum: lang/cmd/debugger)",
	"cmd/tr/main.go":                                "geçici (ana oturum: lang/cmd/debugger)",
	"cmd/tr/prepare.go":                             "geçici (ana oturum: lang/cmd/debugger)",
	"internal/compute/tensor/counter.go":            "geçici (B: compute/stdlib/profiler)",
	"internal/compute/tensor/device.go":             "geçici (B: compute/stdlib/profiler)",
	"internal/compute/tensor/elementwise_wide.go":   "geçici (B: compute/stdlib/profiler)",
	"internal/compute/tensor/tensor.go":             "geçici (B: compute/stdlib/profiler)",
	"internal/devtools/debugger/command_guide.go":   "geçici (ana oturum: lang/cmd/debugger)",
	"internal/devtools/profiler/setup.go":           "geçici (B: compute/stdlib/profiler)",
	"internal/lang/codes/code.go":                   "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/codes/definition.go":             "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/codes/json_parse.go":             "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/codes/language.go":               "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/interp/code_names.go":            "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/interp/compiler_stmt.go":         "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/interp/environment.go":           "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/interp/methods.go":               "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/interp/moved_names.go":           "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/interp/type_value.go":            "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/interp/values.go":                "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/lexer/keywords.go":               "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/lexer/raw_string.go":             "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/lexer/token.go":                  "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/messages/catalog.go":             "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/messages/code.go":                "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/messages/template.go":            "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/parser/statements.go":            "geçici (ana oturum: lang/cmd/debugger)",
	"internal/lang/source/prepare.go":               "geçici (ana oturum: lang/cmd/debugger)",
	"internal/reporoot/root.go":                     "geçici (B: compute/stdlib/profiler)",
	"internal/stdlib/checkpoint/format.go":          "geçici (B: compute/stdlib/profiler)",
	"internal/stdlib/checkpoint/plain.go":           "geçici (B: compute/stdlib/profiler)",
	"internal/stdlib/common/jsonwrite/jsonwrite.go": "geçici (B: compute/stdlib/profiler)",
	"internal/stdlib/dataset/state.go":              "geçici (B: compute/stdlib/profiler)",
	"internal/stdlib/logging/log.go":                "geçici (B: compute/stdlib/profiler)",
}

// turkishLiteralSkipDirs taranmayan klasörlerdir: Türkçenin dilbilgisi paketi (ek ve harf
// tabloları dilin kendisidir) ve depo dışı klasörler.
var turkishLiteralSkipDirs = []string{"internal/lang/turkish", ".git", ".claude", ".codegraph", "node_modules", "testdata"}

func TestNoTurkishLiterals(t *testing.T) {
	root := reporoot.Root()
	found := map[string][]string{}
	used := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if d.IsDir() {
			for _, s := range turkishLiteralSkipDirs {
				if rel == s || strings.HasSuffix(rel, "/"+s) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		for _, lit := range turkishLiterals(t, path) {
			if _, ok := turkishLiteralAllowed[rel]; ok {
				used[rel] = true
				continue
			}
			if _, ok := turkishLiteralAllowed[rel+":"+lit]; ok {
				used[rel+":"+lit] = true
				continue
			}
			found[rel] = append(found[rel], lit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	total := 0
	for f, l := range found {
		files = append(files, f)
		total += len(l)
	}
	sort.Slice(files, func(i, j int) bool { return len(found[files[i]]) > len(found[files[j]]) })
	for _, f := range files {
		t.Errorf("%s: %d metin: %s", f, len(found[f]), strings.Join(quoteAll(found[f], 6), ", "))
	}
	if total > 0 {
		t.Logf("%d dosyada %d Türkçe metin sabiti: adı ad koduna, iletiyi ileti koduna taşıyın; bilinçli bir yerse gerekçesiyle turkishLiteralAllowed'a yazın", len(files), total)
	}
	for k := range turkishLiteralAllowed {
		if !used[k] {
			t.Errorf("izin listesinde kullanılmayan kayıt: %q; silin (liste yalnızca küçülür)", k)
		}
	}
}

// turkishLiterals dosyadaki Türkçe harfli metin sabitleridir; yapı alanı etiketleri (veri biçimi
// anahtarları, `json:"adım"`) sayılmaz. Derleme etiketine bakılmaz: her işletim sisteminin
// dosyası taranır.
func turkishLiterals(t *testing.T, path string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	tags := map[*ast.BasicLit]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if fld, ok := n.(*ast.Field); ok && fld.Tag != nil {
			tags[fld.Tag] = true
		}
		return true
	})
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || tags[lit] {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			s = lit.Value
		}
		if strings.ContainsAny(s, turkishLetters) {
			out = append(out, s)
		}
		return true
	})
	return out
}

func quoteAll(list []string, max int) []string {
	var out []string
	for i, s := range list {
		if i == max {
			out = append(out, fmt.Sprintf("… (+%d)", len(list)-max))
			break
		}
		if len([]rune(s)) > 40 {
			s = string([]rune(s)[:40]) + "…"
		}
		out = append(out, strconv.Quote(s))
	}
	return out
}
