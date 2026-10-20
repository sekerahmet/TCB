package internal_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Go adları Go biçimindedir (MixedCaps): C tarzı BÜYÜK_HARFLİ ad (`TOKEN_IDENT`, `PREC_AND`) ya da
// tamamı büyük harfli uzun ad (`TOKEN`) tanımlanmaz. Kısaltmalar MixedCaps içinde serbesttir
// (`ID`, `URL`, `GPU`, `JSON`, `HTTPServer`); tek başına kısaltma olarak izin verilenler
// goNameAcronyms'tadır. Dış API'lerin adları goNameExceptions'ta gerekçesiyle durur.

var (
	goNameUnderscored = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$`)
	goNameShouting    = regexp.MustCompile(`^[A-Z]{4,}$`)
)

// goNameAcronyms tek başına büyük harfle yazılabilen kısaltmalardır.
var goNameAcronyms = set("JSON HTTP HTML YAML UTF ASCII CUDA")

// goNameExceptions "dosya:Ad" anahtarlı istisnalardır; yalnızca bir dış API'den (CUDA, Windows,
// cgo/purego bağı) gelen ad yazılır, gerekçesiyle. Bugün boştur (CUDA sabitleri CuXxx
// biçimindedir). Liste yalnızca küçülür.
var goNameExceptions = map[string]string{}

// goNameSkipDirs taranmayan klasörlerdir.
var goNameSkipDirs = set(".git .claude .codegraph node_modules")

// goNameDeclared dosyada tanımlanan sabit, değişken, tür, işlev, metot ve alan adlarını verir.
func goNameDeclared(f *ast.File) []*ast.Ident {
	var ids []*ast.Ident
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			ids = append(ids, n.Name)
		case *ast.ValueSpec:
			ids = append(ids, n.Names...)
		case *ast.TypeSpec:
			ids = append(ids, n.Name)
		case *ast.Field:
			ids = append(ids, n.Names...)
		}
		return true
	})
	return ids
}

// goNameIsCStyle ad C tarzı BÜYÜK_HARFLİ ise doğrudur.
func goNameIsCStyle(name string) bool {
	if goNameUnderscored.MatchString(name) {
		return true
	}
	return goNameShouting.MatchString(name) && !goNameAcronyms[name]
}

// TestGoNamesMixedCaps bütün modülde (her GOOS'un dosyası, yapı etiketine bakmadan) C tarzı ad
// tanımlanmadığını denetler.
func TestGoNamesMixedCaps(t *testing.T) {
	root := reporoot.Root()
	var bad []string
	used := map[string]bool{}
	scanned := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && goNameSkipDirs[d.Name()] {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		scanned++
		for _, id := range goNameDeclared(f) {
			if key := rel + ":" + id.Name; goNameIsCStyle(id.Name) && goNameExceptions[key] != "" {
				used[key] = true
			} else if goNameIsCStyle(id.Name) {
				bad = append(bad, rel+":"+strconv.Itoa(fset.Position(id.Pos()).Line)+": "+id.Name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 100 {
		t.Fatalf("yalnızca %d Go dosyası tarandı", scanned)
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("C tarzı ad (MixedCaps olmalı ya da goNameExceptions'a gerekçesiyle girmeli): %s", b)
	}
	for key := range goNameExceptions {
		if !used[key] {
			t.Errorf("istisna %q artık gerekmiyor; kaydı silin", key)
		}
	}
}

// TestGoNamesRuleDetects ad kuralının neyi yakaladığını sabitler.
func TestGoNamesRuleDetects(t *testing.T) {
	for _, name := range []string{"TOKEN_IDENT", "VAL_INT", "A_B", "CU_DEVICE_ATTRIBUTE_X", "TOKEN", "LEXER"} {
		if !goNameIsCStyle(name) {
			t.Errorf("%s yakalanmalı", name)
		}
	}
	for _, name := range []string{"TokIdent", "ID", "URL", "GPU", "CPU", "JSON", "HTTPServer", "parseURL", "prec", "X", "AB", "ValInt", "_", "k2"} {
		if goNameIsCStyle(name) {
			t.Errorf("%s yakalanmamalı", name)
		}
	}
}
