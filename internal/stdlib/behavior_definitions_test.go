package stdlib

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Kod listesinin (internal/lang/codes/definitions.json, tr.json) gerçek davranışla aynı olduğunu sınayan
// testler: örnekler denetlenir ve çalışır, tanımdaki adlı argümanlar yerleşiklerin ve
// metotların gerçekten kabul ettikleridir, üye tanımlarındaki tip listeleri üyeyi gerçekten
// destekleyen tiplerdir.

// chdirTemp çalışma klasörünü test bitene kadar geçici bir klasör yapar (örneklerin
// yazdığı dosyalar orada kalır). Bu paketin testleri paralel çalışmaz.
func chdirTemp(t *testing.T) {
	t.Helper()
	// Depo kökü çalışma klasöründen yukarı aranarak bulunur: klasör değişmeden önce bulunmalı
	// (test tek başına koşunca kökü ilk kez bu test arar).
	reporoot.Root()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
	// Örneklerin açtığı günlük ve satır okuyucusu dosyaları kapansın (Windows'ta açık dosya
	// silinemez), geçici_klasör()'ün klasörleri silinsin.
	t.Cleanup(Close)
}

func repoRoot(*testing.T) string { return reporoot.Root() }

func exampleProgram(t *testing.T, k codes.Code) (*ast.Program, string) {
	t.Helper()
	g, _ := codes.Source().Entry(k)
	p := parser.New(source.TextLexer(g.Example))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Errorf("%s (%s) örneği ayrıştırılamadı: %v\n  %s", k, g.Name, p.Errors(), g.Example)
		return nil, g.Example
	}
	return prog, g.Example
}

// runExample programı depo kökünde duran bir program gibi (göreli yollar depo köküne
// göre de çözülür), boş girdiyle ve çıktısı atılarak çalıştırır.
func runExample(t *testing.T, prog *ast.Program) error {
	t.Helper()
	m := interp.New()
	m.Out = io.Discard
	m.Err = io.Discard
	m.Stdin = strings.NewReader("Ada\n")
	m.SourceFile = filepath.Join(repoRoot(t), "ornek.tr")
	result := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		m.WaitAsync()
		result <- err
	}()
	select {
	case err := <-result:
		return err
	case <-time.After(20 * time.Second):
		t.Fatal("örnek 20 saniyede bitmedi")
		return nil
	}
}

// Her örnek `tr kontrol`'den (ayrıştırma ve tip denetimi) geçer ve hatasız çalışır
// (ağ gerektirenler yalnızca denetlenir). Örnekler geçici bir çalışma klasöründe çalışır;
// yazdıkları dosyalar orada kalır.
func TestExamplesRun(t *testing.T) {
	chdirTemp(t)
	for _, k := range codes.Codes() {
		g, _ := codes.Source().Entry(k)
		// Komutların örneği bir komut satırıdır, TR programı değil; komut satırı testleri sınar.
		// Kitaplık kodunun örneği aktarım ister (madde 15 adım 3c).
		if g.Example == "" || k.ExpectedKind() == codes.KindCommand || k.IsLibrary() {
			continue
		}
		prog, example := exampleProgram(t, k)
		if prog == nil {
			continue
		}
		if errs := checker.New().CheckProgram(prog); len(errs) > 0 {
			t.Errorf("%s (%s) örneği tip denetiminden geçmedi: %v\n  %s", k, g.Name, errs, example)
			continue
		}
		if err := runExample(t, prog); err != nil {
			t.Errorf("%s (%s) örneği çalışmadı: %v\n  %s", k, g.Name, err, example)
		}
	}
}

// calls programdaki çağrılardan uygun olanları verir.
func calls(node any, match func(*ast.CallExpression) bool) []*ast.CallExpression {
	var out []*ast.CallExpression
	seen := map[uintptr]bool{}
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() || seen[v.Pointer()] {
				return
			}
			seen[v.Pointer()] = true
			if c, ok := v.Interface().(*ast.CallExpression); ok && match(c) {
				out = append(out, c)
			}
			walk(v.Elem())
		case reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(node))
	return out
}

// unknownNamed denemede kullanılan, hiçbir adlı argümana yakın olmayan addır.
const unknownNamed = "zzq_bilinmeyen"

// validNames bilinmeyen adlı argüman hatasının lang dilindeki iletisinden geçerli adlar listesini
// çıkarır: iletinin son ayracı geçerli adları sayar ("(geçerli adlar: a, b)", "(valid names: a, b)")
// ya da adlı argüman almadığını söyler (iki noktasız ayraç). Çalışma anı hatası ileti kodunu
// sarmadığı için (messages.Has görmez) ileti metninden okunur; hata bilinmeyen adı anmıyorsa ok yanlış.
func validNames(err error, lang string) ([]string, bool) {
	if err == nil {
		return nil, false
	}
	s := messages.ErrorIn(err, lang)
	open, end := strings.LastIndex(s, "("), strings.LastIndex(s, ")")
	if !strings.Contains(s, "'"+unknownNamed+"'") || open < 0 || end < open {
		return nil, false
	}
	inner := s[open+1 : end]
	i := strings.Index(inner, ": ")
	if i < 0 {
		return []string{}, true
	}
	names := strings.Split(inner[i+2:], ", ")
	sort.Strings(names)
	return names, true
}

// namedArgNames imzadaki adlı argümanların lang dilindeki adlarıdır (sıralı).
func namedArgNames(im codes.Signature, lang string) []string {
	out := []string{}
	for _, a := range im.Named {
		out = append(out, codes.NameRef(a.Code).MessageText(lang))
	}
	sort.Strings(out)
	return out
}

// Kural (13.5): tanımdaki adlı argümanlar, yerleşiğin ya da metodun gerçekten kabul ettikleridir:
// her fonksiyonun ve metodun örneğindeki çağrısına bilinmeyen bir adlı argüman eklenir; çağrı,
// geçerli adları sayan (ya da adlı argüman almadığını söyleyen) hatayla durmalı ve bu adlar her
// yazım dilinin iletisinde tanımdakilerin o dildeki adlarıyla aynı olmalıdır.
func TestNamedArgsMatchDefinitions(t *testing.T) {
	chdirTemp(t)
	tried := 0
	for _, k := range codes.Codes() {
		tn, _ := codes.DefinitionOf(k)
		// Kitaplık kodunun imzası yalnız kitaplığın kaynağındadır (madde 15, Ç7).
		if tn.Kind != codes.KindFunction && tn.Kind != codes.KindMethod || k.IsLibrary() {
			continue
		}
		prog, example := exampleProgram(t, k)
		if prog == nil {
			continue
		}
		// Ayrıştırılan program içeride kimliklerle durur: çağrı kimlikle aranır.
		id := k.String()
		cs := calls(prog, func(c *ast.CallExpression) bool {
			switch f := c.Function.(type) {
			case *ast.Identifier:
				return tn.Kind == codes.KindFunction && f.Value == id
			case *ast.MemberExpression:
				return tn.Kind == codes.KindMethod && f.Property == id
			}
			return false
		})
		if len(cs) == 0 {
			t.Errorf("%s: örnek bu kodu çağırmıyor; adlı argümanlar sınanamaz\n  %s", k, example)
			continue
		}
		for _, c := range cs {
			c.NamedArgs = append(c.NamedArgs, ast.NamedArg{Name: unknownNamed, Value: &ast.IntegerLiteral{Value: 1}})
		}
		err := runExample(t, prog)
		tried++
		for _, lang := range []string{"tr", "en"} {
			received, ok := validNames(err, lang)
			if !ok {
				t.Errorf("%s: bilinmeyen adlı argüman geçerli adları sayan bir hata vermedi: %v\n  %s", k, err, example)
				break
			}
			var wants [][]string
			if tn.Kind == codes.KindFunction {
				wants = append(wants, namedArgNames(tn.Signature, lang))
			}
			for _, u := range tn.Members {
				wants = append(wants, namedArgNames(u.Signature, lang))
			}
			if !slices.ContainsFunc(wants, func(b []string) bool { return slices.Equal(b, received) }) {
				t.Errorf("%s (%s): yerleşik %v adlı argümanlarını kabul ediyor, tanımda %v", k, lang, received, wants)
			}
		}
	}
	if tried < 70 {
		t.Errorf("yalnızca %d kod denendi", tried)
	}
}

// Kural: üye tanımlarındaki tip listeleri, üyeyi gerçekten destekleyen tiplerle aynıdır: her
// tipten bir örnek değer üzerinde her üye kimliğiyle okunur (kodlu gövde; metotlar çağrılmaz, bağlı
// değer olarak döner); okuma hatasızsa (sözlükte: yok dönmezse) tip üyeyi destekler.
func TestMemberTypesMatchDefinitions(t *testing.T) {
	m := interp.New()
	m.Out, m.Err = io.Discard, io.Discard
	m.SourceFile = filepath.Join(repoRoot(t), "ornek.tr")
	examples := []struct {
		typ   codes.Code
		setup string
	}{
		{codes.TypeInt, `5`}, {codes.TypeFloat, `2.5`}, {codes.TypeString, `"abc"`}, {codes.TypeBool, `doğru`},
		{codes.TypeList, `[1, 2]`}, {codes.TypeDict, `{}`}, {codes.TypeRange, `1..3`}, {codes.TypeFunction, `(x) => x`},
		{codes.TypeBuiltinFunction, `yazdır`}, {codes.TypeTensor, `TR_tensor([[1.0, 2.0]], TR_requires_grad: TR_true)`},
		{codes.TypeOptimizer, `TR_AdamW([TR_tensor([1.0], TR_requires_grad: TR_true)])`}, {codes.TypeOptimizer, `TR_SGD([TR_tensor([1.0], TR_requires_grad: TR_true)])`},
		{codes.TypeDataset, `TR_dataset("tests/data/numbers")`},
		{codes.TypeSampler, `TR_dataset("tests/data/numbers").TR_sampler(TR_batch_size: 1, TR_context: 2)`},
		{codes.TypeSequentialReader, `TR_dataset("tests/data/numbers").TR_sequential(TR_batch_size: 1, TR_context: 2)`},
		{codes.TypeTokenizer, `TR_load_tokenizer("tests/data/tokenizer.json")`},
		{codes.TypePath, `TR_path("tests/yerlesikler_test.tr")`},
		{codes.TypeLineReader, `TR_read_lines("tests/data/tokenizer.json")`},
		{codes.TypeProgress, `TR_progress(3)`},
		{codes.TypeLogger, `TR_logger("` + filepath.ToSlash(filepath.Join(t.TempDir(), "u.jsonl")) + `")`},
		{codes.TypeCheckpoint, `TR_checkpoint("` + filepath.ToSlash(t.TempDir()) + `", {w: TR_tensor([1.0], TR_requires_grad: TR_true)})`},
	}
	t.Cleanup(logging.CloseAll)
	var setup []string
	for i, o := range examples {
		setup = append(setup, "TR_var u_"+string(rune('a'+i))+" = "+o.setup)
	}
	if _, err := runSame(t, m, strings.Join(setup, "\n")); err != nil {
		t.Fatal(err)
	}
	for _, k := range codes.Codes() {
		tn, _ := codes.DefinitionOf(k)
		// Kitaplık üyesinin tipleri kitaplığın yapılarıdır, tanımda yazılmaz (madde 15, Ç8).
		if tn.Kind != codes.KindMethod && tn.Kind != codes.KindProperty || k.IsLibrary() {
			continue
		}
		var real []codes.Code
		for i, o := range examples {
			v, err := runCoded(t, m, "u_"+string(rune('a'+i))+"."+k.String())
			if err == nil && !(o.typ == codes.TypeDict && v == interp.Null) {
				real = append(real, o.typ)
			}
		}
		slices.Sort(real)
		real = slices.Compact(real) // aynı tipten birden çok örnek (AdamW, SGD: İyileştirici)
		def := tn.Types()
		slices.Sort(def)
		if !slices.Equal(real, def) {
			t.Errorf("%s: üyeyi destekleyen tipler %v, tanımda %v", k, real, def)
		}
	}
}
