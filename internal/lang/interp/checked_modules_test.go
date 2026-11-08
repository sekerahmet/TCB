package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Modülün denetlenmiş ağacı, çalışma anı aynı metni okursa kullanılır: denetleyicinin çağrıya
// yazdığı boş listenin öğe tipi (ast.CallExpression.ListElem, 15.18 ve 13) modülde de çalışır.
// Metin bayt bayt karşılaştırılır; dosya denetimden sonra değişirse saklanan ağaç düşer ve modül
// yeniden ayrıştırılır.
func TestACheckedModuleKeepsItsTreeOnlyForTheSameText(t *testing.T) {
	text := "# TR 4\nTR_func f() -> TR_String? {\n    TR_var l = [TR_ones([2]) TR_for x TR_in [1] TR_if x > 5]\n    TR_return TR_error_code { TR_sum(l) }\n}\n"
	dir := t.TempDir()
	modPath := testkit.WriteFile(t, dir, "toplam.tr", text)
	main := "# TR 4\nTR_import \"toplam\" TR_as tp\n"
	mainPath := testkit.WriteFile(t, dir, "ana.tr", main)
	if err := CheckImportedModules(mainPath, mustParseProgram(t, main)); err != nil {
		t.Fatal(err)
	}
	r, ok := checkedModuleSource(modPath, text)
	if !ok {
		t.Fatal("denetlenen modülün ağacı saklanmalı")
	}
	var elem ast.ListElem
	ast.Walk(r.Program, func(n any) bool {
		if c, ok := n.(*ast.CallExpression); ok && c.ListElem != ast.ListElemUnknown {
			elem = c.ListElem
		}
		return true
	})
	if elem != ast.ListElemTensor {
		t.Errorf("denetlenmiş ağaçta TR_sum'ın öğe tipi TR_Tensor olmalı: %v", elem)
	}
	// Bir bayt farklı metin saklanan ağacı kullanmaz ve onu düşürür.
	if _, ok := checkedModuleSource(modPath, text+" "); ok {
		t.Error("değişen metin saklanan ağacı kullanmamalı")
	}
	if _, ok := checkedModuleSource(modPath, text); ok {
		t.Error("metin değişince saklanan ağaç düşmeli")
	}
}
