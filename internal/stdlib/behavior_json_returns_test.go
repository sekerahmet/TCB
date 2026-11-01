package stdlib

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// jsonProbe örneğin başına eklenir: json_record(x) x'i olduğu gibi döndürür, x bir JSON değeri
// değilse tipinin adını kaydeder. JSON değeri kendi çevirisine eşittir (x == TR_JSON(x)); JSON olmayan
// değerle karşılaştırma çalışma hatasıdır (H2660, H2705), JSON'a giremeyen değerin çevirisi de (H2654).
const jsonProbe = `TR_var json_mismatches: TR_List[TR_String] = []
TR_func json_record(x: TR_JSON) -> TR_JSON {
    TR_var kod = TR_error_code { TR_var e = x == TR_JSON(x) }
    TR_if kod != TR_none { json_mismatches.TR_append(TR_String(TR_Type(x))) }
    TR_return x
}`

// Kural (3.7a, 1.2): denetleyici checker.JSONReturns'teki yerleşiklerin çağrısını TR_JSON sayar;
// motorun TR_Type'ı JSON'un türünü verdiği için TestSingleReturnsMatchEngine onları sınayamaz. Bu
// test tanım örneğindeki her çağrıyı json_record ile sarar: dönen değer bir JSON değeridir. Tip
// çağrısı TR_JSON(x)'i tipin örneği çağırmaz; onu tests/tipler_test.tr sınar.
func TestJSONReturnsAreJSONValues(t *testing.T) {
	chdirTemp(t)
	verified := 0
	for k := range checker.JSONReturns {
		def, _ := codes.DefinitionOf(k)
		if def.Kind == codes.KindType {
			continue // tip çağrısı (TR_JSON(x)): tipin örneği onu çağırmaz; tests/tipler_test.tr sınar
		}
		if got := probeJSONReturns(t, k); got != "" {
			t.Errorf("%s: dönüşü TR_JSON olmalı, örnekte %s", k, got)
		}
		verified++
	}
	if verified == 0 {
		t.Error("hiçbir JSON dönüşü doğrulanmadı")
	}
}

// probeJSONReturns k'nın örneğini, k'nın her çağrısı json_record ile sarılmış olarak koşar; boş sonuç
// dönüşlerin hepsinin JSON değeri olduğudur, değilse uyuşmazlığın ya da sınanamamanın tarifi.
func probeJSONReturns(t *testing.T, k codes.Code) string {
	prog, _ := exampleProgram(t, k)
	if prog == nil {
		return "unparsed example"
	}
	found := calls(prog, func(c *ast.CallExpression) bool {
		f, ok := c.Function.(*ast.Identifier)
		return ok && f.Token.Code == k
	})
	if len(found) == 0 {
		return "no call in the example"
	}
	for _, c := range found {
		orig := *c
		wrap := parseCoded(t, "json_record(0)").Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
		wrap.Arguments[0] = &orig
		*c = *wrap
	}
	probe := parseCoded(t, jsonProbe)
	tail := parseCoded(t, fmt.Sprintf(`TR_print("%s", json_mismatches)`, returnMarker))
	prog.Statements = append(append(probe.Statements, prog.Statements...), tail.Statements...)
	out := runExampleOutput(t, prog)
	i := strings.LastIndex(out, returnMarker+" ")
	if i < 0 {
		return "example did not finish: " + out
	}
	if rest := strings.TrimSpace(strings.SplitN(out[i+len(returnMarker)+1:], "\n", 2)[0]); rest != "[]" {
		return rest
	}
	return ""
}
