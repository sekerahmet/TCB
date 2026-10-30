package stdlib

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// returnProbe örneğin başına eklenir: return_record(x, T) x'i olduğu gibi döndürür, tipi T değilse
// (öğe tipli T[...] de T sayılır) tipin adını kaydeder; son satır kayıtları yazar. %[1]s beklenen
// dönüş tipidir (tanımın yazdığı gibi): çalışma anı parametre tipini zorlamadığı için kayıt her
// değeri görür.
const returnProbe = `TR_var return_mismatches: TR_List[TR_String] = []
TR_func return_record(x: %[1]s, t: TR_Type) -> %[1]s {
    TR_var s = TR_String(TR_Type(x))
    TR_var e = TR_String(t)
    TR_if s != e TR_and TR_not s.TR_starts_with(e + "[") { return_mismatches.TR_append(s) }
    TR_return x
}`

const returnMarker = "RETURN_MISMATCHES"

// Kural (1.2, 13.9; 4a2'nin ön şartı): denetleyici tek tipli dönüşü (checker.SingleReturn) çağrının
// tipi sayar; bu yüzden her tek tipli yerleşiğin tanım örneğindeki çağrıları çalışırken o tipte
// değer döndürür. Örneğin her çağrısı return_record ile sarılıp koşulur. Sınanamayan (örnekte çağrısı
// olmayan ya da örneği sona ermeyen) ve uyuşmayan kodlar checker.ReturnsNotVerified'dadır:
// denetleyici onların dönüşünü kullanmaz; doğrulanan kod listeden çıkar.
func TestSingleReturnsMatchEngine(t *testing.T) {
	chdirTemp(t)
	verified := 0
	for _, k := range codes.Codes() {
		ret, ok := checker.SingleReturn(k)
		def, _ := codes.DefinitionOf(k)
		if !ok || def.Example == "" {
			continue
		}
		got := probeReturns(t, k, ret)
		_, listed := checker.ReturnsNotVerified[k]
		switch {
		case got == "" && listed:
			t.Errorf("%s: dönüşü (%s) doğrulandı; checker.ReturnsNotVerified'dan çıkarın", k, ret)
		case got == "":
			verified++
		case !listed:
			t.Errorf("%s: tanımın dönüşü %s, örnekte %s", k, ret, got)
		}
	}
	if verified < 50 {
		t.Errorf("yalnız %d kodun dönüşü doğrulandı", verified)
	}
}

// probeReturns k'nın örneğini, k'nın her çağrısı return_record ile sarılmış olarak koşar; boş
// sonuç dönüşlerin hepsinin ret tipinde olduğudur, değilse uyuşmazlığın ya da sınanamamanın tarifi.
func probeReturns(t *testing.T, k, ret codes.Code) string {
	prog, _ := exampleProgram(t, k)
	if prog == nil {
		return "unparsed example"
	}
	found := calls(prog, func(c *ast.CallExpression) bool {
		switch f := c.Function.(type) {
		case *ast.Identifier:
			return f.Token.Code == k
		case *ast.MemberExpression:
			return f.NameToken.Code == k
		}
		return false
	})
	if len(found) == 0 {
		return "no call in the example"
	}
	for _, c := range found {
		orig := *c
		wrap := parseCoded(t, "return_record(0, "+codes.Spelling(ret)+")").Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
		wrap.Arguments[0] = &orig
		*c = *wrap
	}
	probe := parseCoded(t, fmt.Sprintf(returnProbe, codes.Spelling(ret)))
	tail := parseCoded(t, `TR_print("`+returnMarker+`", return_mismatches)`)
	prog.Statements = append(append(probe.Statements, prog.Statements...), tail.Statements...)
	out := runExampleOutput(t, prog)
	i := strings.LastIndex(out, returnMarker+" ")
	if i < 0 {
		return "example did not finish"
	}
	rest := strings.TrimSpace(strings.SplitN(out[i+len(returnMarker)+1:], "\n", 2)[0])
	if rest == "[]" {
		return ""
	}
	return rest
}

func parseCoded(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.NewBody(src, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("ayrıştırılamadı: %v\n%s", p.Errors(), src)
	}
	return prog
}

// runExampleOutput runExample gibidir; çıktıyı döndürür, çalışma hatası çıktıyı kesmez.
func runExampleOutput(t *testing.T, prog *ast.Program) string {
	t.Helper()
	var out bytes.Buffer
	m := interp.New()
	m.Out, m.Err = &out, &out
	m.Stdin = strings.NewReader("Ada\n")
	m.SourceFile = filepath.Join(repoRoot(t), "ornek.tr")
	done := make(chan struct{})
	go func() {
		m.Run(prog)
		m.WaitAsync()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("örnek 20 saniyede bitmedi")
	}
	return out.String()
}
