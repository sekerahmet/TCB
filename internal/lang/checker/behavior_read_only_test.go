package checker

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Denetçi kod çalıştırmaz: yalnızca okur ve raporlar. Yorumlayıcıyı (ve ayrıştırıcıyı) içe
// aktaramaz; dosya, süreç, ağ ya da yansıma paketlerini de kullanamaz. Test, test dışı bütün
// dosyaların içe aktarmalarını izin listesiyle karşılaştırır. Yeni bir içe aktarma gerekirse
// listeye gerekçesiyle eklenir.
var allowedImports = map[string]string{
	"fmt":     "ileti kurma",
	"sort":    "ad listelerini sıralama",
	"strings": "metin işleme",
	"sync":    "ortak tabloların bir kez kurulması",
	"slices":  "dilim işlemleri",
	"maps":    "harita işlemleri",
	"unicode": "ad yazılışı denetimi (harf sınıfları, Türkçe büyük/küçük harf)",
}

const langPrefix = "github.com/sekerahmet/TCB/internal/lang/"

// langAllowed denetçinin içe aktarabildiği dil çekirdeği paketleridir (yorumlayıcı ve ayrıştırıcı yok).
var langAllowed = map[string]bool{"ast": true, "codes": true, "hint": true, "lexer": true, "messages": true, "turkish": true}

func TestCheckerOnlyReads(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if rest, ok := strings.CutPrefix(p, langPrefix); ok {
				if !langAllowed[rest] {
					t.Errorf("%s: %s içe aktarılıyor; denetçi kod çalıştırmaz (yalnızca ast, codes, hint, lexer, messages, turkish)", name, p)
				}
				continue
			}
			if _, ok := allowedImports[p]; !ok {
				t.Errorf("%s: %q içe aktarılıyor; denetçi dosya, süreç ya da ağ kullanmaz (gerekiyorsa allowedImports'a gerekçesiyle ekleyin)", name, p)
			}
		}
	}
	if checked == 0 {
		t.Fatal("denetçi dosyası bulunamadı")
	}
}
