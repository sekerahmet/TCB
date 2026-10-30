package internal_test

import (
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (1.2, 6.1, 7.1): depodaki TR kodu tiplidir. Her parametre ve her alan tipini yazar, liste ve
// sözlük öğe tipini yazar (TR_List[T], TR_Dict[K, V]; 3.6, 3.7), değer döndüren fonksiyon dönüş
// tipini '->' ile yazar; genel tipler (TR_Any, TR_Object, TR_StructType, çıplak TR_Function,
// TR_BuiltinFunction; 3.11) tip yerine yazılmaz. Kod ayrıştırıcıyla okunur, metin aramasıyla değil.
// Sayılmayanlar: fonksiyon ifadesinin parametresi ((x) => ..., 6.6: tipi beklenen tipten çıkar) ve
// TR_self (parametre değildir, 7.3).
//
// Bu test geçicidir: 4a1 tipsiz parametreyi ve alanı denetleyicinin hatası yapınca kalkar (o gün
// kural her programda denetlenir). O güne kadar kalan yerler izin listesindedir
// (testdata/typed_code_known.txt): her satırın nedeni yazılıdır, liste yalnız küçülür (listede olup
// artık tipli olan satır da testi düşürür; -update-typed siler).

var updateTyped = flag.Bool("update-typed", false, "testdata/typed_code_known.txt'ten artık tipli olan satırları sil")

const typedKnownFile = "testdata/typed_code_known.txt"

// typedSources taranan .tr dosyalarıdır. tools/codegraph ve internal/devtools/codeindex'in deneme
// projeleri o araçların sahibinindir; onlar kendi işlerinde tiplenir.
var typedSources = []string{"lib", "tests", "examples", "tools"}

var skippedTypedDirs = []string{"tools/codegraph/"}

// typedDocs ```tr bloklarına bakılan belgelerdir; docs/ altındaki bütün belgeler eklenir. Arşiv
// geçmişi olduğu gibi saklar, sayılmaz. docs/codes.md tanım dosyasının örneklerinden üretilir:
// örnekler orada okunur.
var typedDocs = []string{"README.md", "SECURITY.md", "CONTRIBUTING.md", "lib/README.md", "examples/README.md",
	"editors/vscode/README.md", "tools/bench/README.md", ".github/ISSUE_TEMPLATE/name_request.md"}

var typedBlockRe = regexp.MustCompile("(?ms)^```tr( hata)?[ \t]*\r?\n(.*?)^```")

// trSource taranan bir TR metnidir: yolu, metnin dosyadaki ilk satırı ve başlıksız gövde olup olmadığı.
type trSource struct {
	path      string
	firstLine int
	text      string
}

func TestTRCodeIsTyped(t *testing.T) {
	sources := typedFiles(t)
	files := len(sources)
	sources = append(sources, typedDocBlocks(t)...)
	if files < 150 || len(sources)-files < 300 {
		t.Fatalf("yalnız %d dosya ve %d kod bloğu bulundu; depo düzeni değişmiş olabilir", files, len(sources)-files)
	}
	var found []string
	for _, s := range sources {
		found = append(found, untypedPlaces(s)...)
	}
	checkTypedKnown(t, found, *updateTyped)
}

func typedFiles(t *testing.T) []trSource {
	t.Helper()
	var out []trSource
	for _, dir := range typedSources {
		err := filepath.WalkDir(reporoot.Path(dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".tr") {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(p, reporoot.Root()+string(filepath.Separator)))
			for _, skip := range skippedTypedDirs {
				if strings.HasPrefix(rel, skip) {
					return nil
				}
			}
			data, err := os.ReadFile(p)
			out = append(out, trSource{path: rel, firstLine: 1, text: string(data)})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func typedDocBlocks(t *testing.T) []trSource {
	t.Helper()
	docs := slices.Clone(typedDocs)
	for _, pattern := range []string{"docs/*.md", "docs/*/*.md"} {
		found, err := filepath.Glob(reporoot.Path(pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range found {
			if rel, _ := filepath.Rel(reporoot.Root(), p); !strings.HasPrefix(filepath.ToSlash(rel), "docs/archive/") {
				docs = append(docs, filepath.ToSlash(rel))
			}
		}
	}
	var out []trSource
	for _, doc := range docs {
		data, err := os.ReadFile(reporoot.Path(doc))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, m := range typedBlockRe.FindAllStringSubmatchIndex(text, -1) {
			line := strings.Count(text[:m[4]], "\n") + 1
			out = append(out, trSource{path: doc, firstLine: line, text: text[m[4]:m[5]]})
		}
	}
	return out
}

// untypedPlaces kaynağın tipsiz yerlerini "yol:satır tür ad" diye verir. Ayrışmayan kaynak
// (bilerek hatalı fikstür) ayrıştırıcının kurabildiği kadarıyla sayılır.
func untypedPlaces(s trSource) []string {
	body := !strings.HasPrefix(strings.TrimPrefix(s.text, "\uFEFF"), source.Header)
	r := source.Prepare(s.text, source.Options{Body: body})
	// Dosyanın satırları başlık dahil sayılır; başlıksız gövdenin 1. satırı metnin ilk satırıdır.
	offset := s.firstLine - 1
	var out []string
	add := func(line int, kind, name string) {
		out = append(out, fmt.Sprintf("%s:%d %s %s", s.path, line+offset, kind, name))
	}
	ast.Walk(r.Program, func(n any) bool {
		switch n := n.(type) {
		case *ast.FunctionLiteral:
			if n.Token.Type == lexer.TokFunction {
				checkFunction(n, add)
			}
		case *ast.StructStatement:
			for _, f := range n.Fields {
				checkField(n.Name, f, add)
			}
			for _, v := range n.Variants {
				for _, f := range v.Fields {
					checkField(n.Name+"."+v.Name, f, add)
				}
			}
		case *ast.VarStatement:
			checkTypeRef(n.Type, n.Token.Line, add)
		}
		return true
	})
	return out
}

func checkFunction(f *ast.FunctionLiteral, add func(int, string, string)) {
	for _, p := range f.Parameters {
		if p.Type == nil {
			add(p.NameToken.Line, "parametre", f.Name+"."+p.Name)
		}
		checkTypeRef(p.Type, p.NameToken.Line, add)
	}
	checkTypeRef(f.ReturnType, f.Token.Line, add)
	if f.ReturnType == nil && returnsValue(f.Body) {
		add(f.Token.Line, "dönüş", f.Name)
	}
}

func checkField(owner string, f ast.StructField, add func(int, string, string)) {
	if f.Type == nil {
		add(f.NameToken.Line, "alan", owner+"."+f.Name)
	}
	checkTypeRef(f.Type, f.NameToken.Line, add)
}

// genericTypes tip yerine yazılmayan genel tiplerdir (3.11); çıplak TR_Function da öyledir: fonksiyon
// tipi (T) -> R diye yazılır (6.9).
var genericTypes = map[codes.Code]bool{codes.TypeAny: true, codes.TypeObject: true, codes.TypeStructType: true,
	codes.TypeFunction: true, codes.TypeBuiltinFunction: true}

// checkTypeRef yazılan tipte öğe tipi yazılmamış TR_List ve TR_Dict'i ve genel tipi bulur.
func checkTypeRef(t *ast.TypeRef, line int, add func(int, string, string)) {
	if t == nil {
		return
	}
	if t.Token.Line > 0 {
		line = t.Token.Line
	}
	switch {
	case (t.Token.Code == codes.TypeList || t.Token.Code == codes.TypeDict) && len(t.Args) == 0:
		add(line, "öğe-tipi", t.Name)
	case genericTypes[t.Token.Code] && t.Return == nil && len(t.Elems) == 0:
		add(line, "genel-tip", t.Name)
	}
	for _, inner := range [][]ast.TypeRef{t.Args, t.Elems, t.Params} {
		for i := range inner {
			checkTypeRef(&inner[i], line, add)
		}
	}
	checkTypeRef(t.Return, line, add)
}

// returnsValue gövde değerli bir TR_return taşıyor mu? İç fonksiyonların dönüşleri sayılmaz; değersiz
// TR_return'ü ayrıştırıcı kendi belirteciyle bir TR_none olarak kurar.
func returnsValue(body *ast.BlockStatement) bool {
	found := false
	ast.Walk(body, func(n any) bool {
		switch n := n.(type) {
		case *ast.FunctionLiteral:
			return false
		case *ast.ReturnStatement:
			if null, bare := n.ReturnValue.(*ast.NullLiteral); n.ReturnValue != nil && !(bare && null.Token == n.Token) {
				found = true
			}
		}
		return !found
	})
	return found
}

// checkTypedKnown bulunan yerleri izin listesiyle karşılaştırır. Listenin her satırı "yer — neden"
// diye yazılır; neden boş olamaz. Listede olmayan tipsiz yer de, listede olup artık tipli olan satır
// da testi düşürür; update doğruysa artık tipli olan satırlar silinir.
func checkTypedKnown(t *testing.T, found []string, update bool) {
	t.Helper()
	found = slices.Compact(slices.Sorted(slices.Values(found)))
	data, err := os.ReadFile(typedKnownFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	known := map[string]bool{}
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		place, reason, ok := strings.Cut(line, " — ")
		if !ok || strings.TrimSpace(reason) == "" {
			t.Errorf("%s: satırın nedeni yok: %q", typedKnownFile, line)
		}
		known[place] = true
	}
	var unexpected []string
	for _, f := range found {
		if !known[f] {
			unexpected = append(unexpected, f)
		}
		delete(known, f)
	}
	if len(unexpected) > 0 {
		t.Errorf("%d tipsiz yer (1.2, 6.1, 7.1) %s'te yok; tipini yazın (yalnız genel tip gerekiyorsa ya da bilerek hatalı fikstürse nedeniyle ekleyin):\n%s",
			len(unexpected), typedKnownFile, strings.Join(unexpected, "\n"))
	}
	stale := slices.Sorted(maps.Keys(known))
	if update && len(unexpected) == 0 && len(stale) > 0 {
		var kept []string
		for _, line := range lines {
			if place, _, _ := strings.Cut(line, " — "); !slices.Contains(stale, place) {
				kept = append(kept, line)
			}
		}
		if err := os.WriteFile(typedKnownFile, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if len(stale) > 0 {
		t.Errorf("%s'teki %d yer artık tipli; satırları silin (-update-typed):\n%s", typedKnownFile, len(stale), strings.Join(stale, "\n"))
	}
	t.Logf("%d tipsiz yer izin listesinde", len(found))
}
