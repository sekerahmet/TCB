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
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
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
	g, _ := codes.Active().Entry(k)
	p := parser.New(lexer.New(g.Example))
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
		g, _ := codes.Active().Entry(k)
		// Komutların (K) örneği bir komut satırıdır, TR programı değil; komut satırı testleri sınar.
		if g.Example == "" || k.Letter() == 'K' {
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

// validNames bilinmeyen adlı argüman hatasından geçerli adlar listesini çıkarır
// ("(geçerli adlar: a, b)" ya da "(adlı argüman almaz)"); hata bu biçimde değilse ok yanlış.
func validNames(err error) ([]string, bool) {
	if err == nil {
		return nil, false
	}
	s := err.Error()
	if !strings.Contains(s, "'"+unknownNamed+"' adında bir parametre almaz") {
		return nil, false
	}
	if strings.Contains(s, "(adlı argüman almaz)") {
		return []string{}, true
	}
	i := strings.Index(s, "(geçerli adlar: ")
	if i < 0 {
		return nil, false
	}
	s = s[i+len("(geçerli adlar: "):]
	j := strings.Index(s, ")")
	if j < 0 {
		return nil, false
	}
	names := strings.Split(s[:j], ", ")
	sort.Strings(names)
	return names, true
}

func namedArgNames(im codes.Signature) []string {
	out := []string{}
	for _, a := range im.Named {
		out = append(out, codes.Name(a.Code))
	}
	sort.Strings(out)
	return out
}

// Tanımdaki adlı argümanlar, yerleşiğin ya da metodun gerçekten kabul ettikleridir: her
// fonksiyonun ve metodun örneğindeki çağrısına bilinmeyen bir adlı argüman eklenir;
// çağrı, geçerli adları sayan (ya da adlı argüman almadığını söyleyen) hatayla durmalı ve
// bu adlar tanımdakilerle aynı olmalıdır.
func TestNamedArgsMatchDefinitions(t *testing.T) {
	chdirTemp(t)
	tried := 0
	for _, k := range codes.Codes() {
		tn, _ := codes.DefinitionOf(k)
		if tn.Kind != codes.KindFunction && tn.Kind != codes.KindMethod {
			continue
		}
		prog, example := exampleProgram(t, k)
		if prog == nil {
			continue
		}
		name := codes.Name(k)
		cs := calls(prog, func(c *ast.CallExpression) bool {
			switch f := c.Function.(type) {
			case *ast.Identifier:
				return tn.Kind == codes.KindFunction && f.Value == name
			case *ast.MemberExpression:
				return tn.Kind == codes.KindMethod && f.Property == name
			}
			return false
		})
		if len(cs) == 0 {
			t.Errorf("%s (%s): örnek bu kodu çağırmıyor; adlı argümanlar sınanamaz\n  %s", k, name, example)
			continue
		}
		for _, c := range cs {
			c.NamedArgs = append(c.NamedArgs, ast.NamedArg{Name: unknownNamed, Value: &ast.IntegerLiteral{Value: 1}})
		}
		err := runExample(t, prog)
		received, ok := validNames(err)
		if !ok {
			t.Errorf("%s (%s): bilinmeyen adlı argüman geçerli adları sayan bir hata vermedi: %v\n  %s", k, name, err, example)
			continue
		}
		tried++
		var wants [][]string
		if tn.Kind == codes.KindFunction {
			wants = append(wants, namedArgNames(tn.Signature))
		}
		for _, u := range tn.Members {
			wants = append(wants, namedArgNames(u.Signature))
		}
		if !slices.ContainsFunc(wants, func(b []string) bool { return slices.Equal(b, received) }) {
			t.Errorf("%s (%s): yerleşik %v adlı argümanlarını kabul ediyor, tanımda %v", k, name, received, wants)
		}
	}
	if tried < 70 {
		t.Errorf("yalnızca %d kod denendi", tried)
	}
}

// Üye tanımlarındaki tip listeleri, üyeyi gerçekten destekleyen tiplerle aynıdır: her
// tipten bir örnek değer üzerinde her üye okunur (metotlar çağrılmaz, bağlı değer olarak
// döner); okuma hatasızsa (sözlükte: yok dönmezse) tip üyeyi destekler.
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
		{codes.TypeBuiltinFunction, `yazdır`}, {codes.TypeTensor, `tensör([[1.0, 2.0]], türev_izle: doğru)`},
		{codes.TypeOptimizer, `AdamW([tensör([1.0], türev_izle: doğru)])`}, {codes.TypeOptimizer, `SGD([tensör([1.0], türev_izle: doğru)])`},
		{codes.TypeDataset, `veri_kümesi("tests/data/numbers")`},
		{codes.TypeSampler, `veri_kümesi("tests/data/numbers").örnekleyici(toplu: 1, bağlam: 2)`},
		{codes.TypeSequentialReader, `veri_kümesi("tests/data/numbers").sıralı(toplu: 1, bağlam: 2)`},
		{codes.TypeTokenizer, `parçalayıcı_yükle("tests/data/tokenizer.json")`},
		{codes.TypePath, `yol("tests/yerlesikler_test.tr")`},
		{codes.TypeLineReader, `dosya_satırları("tests/data/tokenizer.json")`},
		{codes.TypeProgress, `ilerleme(3)`},
		{codes.TypeLogger, `günlük("` + filepath.ToSlash(filepath.Join(t.TempDir(), "u.jsonl")) + `")`},
		{codes.TypeCheckpoint, `kontrol_noktası("` + filepath.ToSlash(t.TempDir()) + `", {w: tensör([1.0], türev_izle: doğru)})`},
	}
	t.Cleanup(logging.CloseAll)
	var setup []string
	for i, o := range examples {
		setup = append(setup, "değişken u_"+string(rune('a'+i))+" = "+o.setup)
	}
	if _, err := runSame(t, m, strings.Join(setup, "\n")); err != nil {
		t.Fatal(err)
	}
	for _, k := range codes.Codes() {
		tn, _ := codes.DefinitionOf(k)
		if tn.Kind != codes.KindMethod && tn.Kind != codes.KindProperty {
			continue
		}
		var real []codes.Code
		for i, o := range examples {
			v, err := runSame(t, m, "u_"+string(rune('a'+i))+"."+codes.Name(k))
			if err == nil && !(o.typ == codes.TypeDict && v == interp.Null) {
				real = append(real, o.typ)
			}
		}
		slices.Sort(real)
		real = slices.Compact(real) // aynı tipten birden çok örnek (AdamW, SGD: İyileştirici)
		def := tn.Types()
		slices.Sort(def)
		if !slices.Equal(real, def) {
			t.Errorf("%s (%s): üyeyi destekleyen tipler %v, tanımda %v", k, codes.Name(k), codes.Names(real...), codes.Names(def...))
		}
	}
}
