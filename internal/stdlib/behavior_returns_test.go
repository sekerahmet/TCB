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
// değeri görür. return_record_on metot çağrısını alıcısıyla sarar: dönüş yalnız alıcı types'tan biriyse
// kaydedilir, o çağrılar return_hits'te sayılır (TestNoValueBuiltinsGiveNone, örneğin alıcısı).
const returnProbe = `TR_var return_mismatches: TR_List[TR_String] = []
TR_var return_hits = 0
TR_func return_record(x: %[1]s, t: TR_Type) -> %[1]s {
    TR_var s = TR_String(TR_Type(x))
    TR_var e = TR_String(t)
    TR_if s != e TR_and TR_not s.TR_starts_with(e + "[") { return_mismatches.TR_append(s) }
    TR_return x
}
TR_func return_record_on(r: TR_Any, f: TR_Function, types: TR_List[TR_Type], t: TR_Type) -> TR_Any {
    TR_var x = f(r)
    TR_if types.TR_contains(TR_Type(r)) {
        return_hits = return_hits + 1
        TR_return return_record(x, t)
    }
    TR_return x
}`

const (
	returnMarker = "RETURN_MISMATCHES"
	hitsMarker   = "RETURN_HITS"
)

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
		_, json := checker.JSONReturns[k] // TR_Type JSON'un türünü verir: TestJSONReturnsAreJSONValues
		if !ok || def.Example == "" || json {
			continue
		}
		got := probeReturns(t, k, ret, nil)
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

// Kural (13.10): tanımda dönüşü TR_NoneType olan yerleşik çalışırken TR_none verir, "başarılı" anlamında
// TR_true ya da alıcıyı döndürmez. Örnek denetlenmeden koşar: değer yerindeki çağrıyı denetleyici keser.
func TestNoValueBuiltinsGiveNone(t *testing.T) {
	chdirTemp(t)
	verified := 0
	for _, k := range codes.Codes() {
		def, _ := codes.DefinitionOf(k)
		receivers, ok := givesNoValue(def)
		if def.Example == "" || k.IsLibrary() || !ok {
			continue
		}
		switch got := probeReturns(t, k, codes.TypeNoneType, receivers); got {
		case "":
			verified++
		case "no call in the example", "example did not finish", "no call on a receiver without a value":
		default:
			t.Errorf("%s: tanımın dönüşü TR_NoneType, örnekte %s", k, got)
		}
	}
	if verified < 12 {
		t.Errorf("yalnız %d kodun dönüşü doğrulandı", verified)
	}
}

// givesNoValue tanımın dönüşü TR_NoneType mı. Metotta dönüşü TR_NoneType olan alıcı tipleri de döner:
// her alıcıda öyleyse nil (her çağrı sınanır), değilse yalnız o alıcılardaki çağrılar sınanır (örneğin
// alıcısı: TR_step İyileştirici'de değer vermez, MaskeAkışı'nda Metin verir).
func givesNoValue(def codes.Definition) ([]codes.Code, bool) {
	none := func(r codes.TypeExpr) bool {
		return len(r.Types) == 1 && r.Types[0] == codes.TypeNoneType && !r.Optional
	}
	if def.Kind != codes.KindMethod {
		return nil, none(def.Signature.Return)
	}
	var receivers []codes.Code
	for _, u := range def.Members {
		if none(u.Signature.Return) {
			receivers = append(receivers, u.Type)
		}
	}
	if len(receivers) == len(def.Members) {
		return nil, len(receivers) > 0
	}
	return receivers, len(receivers) > 0
}

// probeReturns k'nın örneğini, k'nın her çağrısı return_record ile sarılmış olarak koşar; boş
// sonuç dönüşlerin hepsinin ret tipinde olduğudur, değilse uyuşmazlığın ya da sınanamamanın tarifi.
// receivers boş değilse metot çağrısı return_record_on ile sarılır: yalnız bu tiplerdeki alıcının
// dönüşü sınanır; örnekte böyle bir alıcı yoksa sınanamamıştır.
func probeReturns(t *testing.T, k, ret codes.Code, receivers []codes.Code) string {
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
		case *ast.IndexExpression: // tip argümanlı çağrı: TR_from_bytes[TR_Int](...)
			left, ok := f.Left.(*ast.Identifier)
			return ok && left.Token.Code == k
		}
		return false
	})
	if len(found) == 0 {
		return "no call in the example"
	}
	for _, c := range found {
		if m, ok := c.Function.(*ast.MemberExpression); ok && receivers != nil {
			*c = *wrapOnReceiver(t, k, ret, receivers, m, c)
			continue
		}
		orig := *c
		wrap := parseCoded(t, "return_record(0, "+codes.Spelling(ret)+")").Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
		wrap.Arguments[0] = &orig
		*c = *wrap
	}
	param := codes.Spelling(ret)
	if ret == codes.TypeNoneType {
		param = codes.Spelling(codes.TypeAny) // TR_NoneType parametre tipi yazılmaz (3.5)
	}
	probe := parseCoded(t, fmt.Sprintf(returnProbe, param))
	tail := parseCoded(t, `TR_print("`+hitsMarker+`", return_hits)`+"\n"+`TR_print("`+returnMarker+`", return_mismatches)`)
	prog.Statements = append(append(probe.Statements, prog.Statements...), tail.Statements...)
	out := runExampleOutput(t, prog)
	i := strings.LastIndex(out, returnMarker+" ")
	if i < 0 {
		return "example did not finish"
	}
	rest := strings.TrimSpace(strings.SplitN(out[i+len(returnMarker)+1:], "\n", 2)[0])
	if receivers != nil && strings.Contains(out, hitsMarker+" 0\n") {
		return "no call on a receiver without a value"
	}
	if rest == "[]" {
		return ""
	}
	return rest
}

// wrapOnReceiver o.k(a) çağrısını return_record_on(o, (return_probe_r) => return_probe_r.k(a), [tipler], ret)
// yapar: alıcı ve argümanlar bir kez ve aynı sırayla değerlendirilir.
func wrapOnReceiver(t *testing.T, k, ret codes.Code, receivers []codes.Code, m *ast.MemberExpression, c *ast.CallExpression) *ast.CallExpression {
	t.Helper()
	types := make([]string, len(receivers))
	for i, r := range receivers {
		types[i] = codes.Spelling(r)
	}
	src := "return_record_on(0, (return_probe_r) => return_probe_r." + codes.Spelling(k) + "(), [" + strings.Join(types, ", ") + "], " + codes.Spelling(ret) + ")"
	wrap := parseCoded(t, src).Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	wrap.Arguments[0] = m.Object
	inner := calls(wrap.Arguments[1], func(x *ast.CallExpression) bool {
		im, ok := x.Function.(*ast.MemberExpression)
		return ok && im.NameToken.Code == k
	})
	if len(inner) != 1 {
		t.Fatalf("%s: sarmalayıcıda çağrı bulunamadı", k)
	}
	inner[0].Arguments, inner[0].NamedArgs = c.Arguments, c.NamedArgs
	inner[0].Function.(*ast.MemberExpression).Safe = m.Safe
	return wrap
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
