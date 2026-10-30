package checker

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// gateCodes kapının ileti kodlarıdır (signatures.go).
var gateCodes = []messages.Code{messages.H1109, messages.H1110, messages.H1111, messages.H1112, messages.H1113}

// gateErrors c'nin kapı hatalarıdır.
func gateErrors(c *Checker) []error {
	var out []error
	for _, err := range c.Errs() {
		for _, code := range gateCodes {
			if messages.Has(err, code) {
				out = append(out, err)
				break
			}
		}
	}
	return out
}

// checkLibraryFile kitaplığın dosyasını (modül ya da test dosyası) denetler; library boş değilse
// dosya gömülü kitaplığın o adlı dosyası sayılır.
func checkLibraryFile(t *testing.T, file, library string) *Checker {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimPrefix(string(data), "\uFEFF")
	p := parser.New(lexer.NewBody(text, true))
	var prog *ast.Program
	kind := ModuleFile
	if strings.HasSuffix(file, "_test.tr") {
		prog, kind = p.ParseProgram(), TestFile
	} else {
		prog = p.ParseModule()
	}
	if len(p.Errs()) > 0 {
		t.Fatalf("%s: %v", file, p.Errs())
	}
	c := New()
	c.SetFile(kind, text)
	c.SetLibraryFile(library)
	c.CheckProgram(prog)
	return c
}

// Kural (6.1, 7.1): gömülü kitaplığın 4c'yi bekleyen yerleri kitaplık dosyası denetlenirken tanı
// vermez (ne hata ne uyarı); aynı dosya kullanıcının programı olarak denetlenince aynı yerler
// kapının hatasıdır. İstisna adla tutulur ve her kaydı kullanılır (liste yalnız küçülür, 4c'de
// kalkar); kitaplığın başka hiçbir yeri tipsiz değildir.
func TestLibraryExceptionsAreUsed(t *testing.T) {
	files := testkit.GlobFiles(t, "lib/*.tr")
	if len(files) < 6 {
		t.Fatalf("lib/*.tr: yalnız %d dosya", len(files))
	}
	used := map[string]bool{}
	asUser := 0
	for _, file := range files {
		name := filepath.Base(file)
		c := checkLibraryFile(t, file, name)
		if errs := gateErrors(c); len(errs) > 0 {
			t.Errorf("%s: kitaplıkta istisnası olmayan tipsiz yer: %v", name, messages.Texts(errs))
		}
		for _, w := range c.Warnings() {
			for _, code := range gateCodes {
				if messages.Has(w, code) {
					t.Errorf("%s: istisnalı yer uyarı da vermez: %v", name, w)
				}
			}
		}
		for k := range c.exceptionsUsed {
			used[k] = true
		}
		asUser += len(gateErrors(checkLibraryFile(t, file, "")))
	}
	var stale []string
	for k := range libraryExceptions {
		if !used[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("kullanılmayan istisnalar (yer tiplendi ya da adı değişti; listeden silin): %v", stale)
	}
	if asUser != len(libraryExceptions) {
		t.Errorf("kullanıcı programı olarak denetlenen kitaplık dosyalarında %d istisnalı yerin her biri hata vermeli; %d hata", len(libraryExceptions), asUser)
	}
}

// İstisna yalnız gömülü kitaplığın adlı dosyasınındır: aynı adlı fonksiyon başka dosyada, ya da
// kitaplığın başka bir dosyasında hata verir.
func TestLibraryExceptionIsOnlyTheLibraryFiles(t *testing.T) {
	src := "TR_func TR_fn_config_value(TR_arg_config, TR_arg_key: TR_String, TR_arg_fallback) {\n    TR_return TR_arg_fallback\n}\n"
	for _, library := range []string{"", "layers.tr", "pretrained.tr"} {
		p := parser.New(lexer.NewBody(src, true))
		prog := p.ParseModule()
		c := New()
		c.SetFile(ModuleFile, src)
		c.SetLibraryFile(library)
		c.CheckProgram(prog)
		got := len(gateErrors(c))
		if want := map[string]int{"": 3, "layers.tr": 3, "pretrained.tr": 0}[library]; got != want {
			t.Errorf("kitaplık dosyası %q: %d kapı hatası bekleniyordu, %d: %v", library, want, got, c.Errors())
		}
	}
}
