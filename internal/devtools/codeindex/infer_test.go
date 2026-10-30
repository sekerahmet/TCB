package codeindex

import (
	"os"
	"path/filepath"
	"testing"
)

const inferSource = `TR_struct S {
    a: TR_Int
    TR_func TR_init() {
        TR_self.a = 1
    }
}

TR_func yap() {
    TR_return S()
}

TR_func yap2() {
    TR_var s = yap()
    TR_return s
}

TR_func karışık(b: TR_Bool) {
    TR_if b {
        TR_return S()
    }
    TR_return 3
}

TR_var g = yap2()
TR_print(g.a)

TR_func kullan() -> TR_Int {
    TR_var t = yap()
    TR_var u = karışık(TR_true)
    TR_return t.a + u.a
}
`

// Kural: tür çıkarımı tek tür ya da bilinmezdir (birleşim yok). Yapıcı, bütün `sonuç`'ları aynı yapıyı veren işlevin
// dönüşü ve tek bağlamalı yerel ya da üst düzey değişken türü taşır; `x.alan` o yapının alanına inferred_type dayanağıyla
// bağlanır. `sonuç`'ları farklı tür veren işlevin dönüşü bilinmez kalır (deneme 7 R4; kod-grafigi-40).
func TestInferredTypesBindMemberAccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), trText(filepath.Join(dir, "m.tr"), inferSource), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	for id, want := range map[string]string{
		"function:m.tr#yap": "struct:m.tr#S", "function:m.tr#yap2": "struct:m.tr#S",
		"function:m.tr#karışık": "", "variable:m.tr#g": "struct:m.tr#S",
	} {
		if n := r.Node(id); n == nil || n.InferredType != want {
			t.Errorf("%s: inferred type %+v, want %q", id, n, want)
		}
	}
	got := map[string]Basis{} // "satır:sütun" -> dayanak
	for _, oc := range r.Occurrences("field:m.tr#S.a") {
		if oc.Role == RoleRead {
			got[oc.Pos.String()] = oc.Basis
		}
	}
	// g.a (üst düzey değişken), t.a (yerel, yap'ın dönüşü): inferred_type. u.a (karışık'ın dönüşü bilinmez): ad tahmini.
	ga, ta, ua := srcAt(t, inferSource, "TR_print(g.a)", "a", 1), srcAt(t, inferSource, "TR_return t.a + u.a", "a", 1), srcAt(t, inferSource, "TR_return t.a + u.a", "a", 2)
	for pos, want := range map[string]Basis{"m.tr:" + ga: BasisInferredType, "m.tr:" + ta: BasisInferredType, "m.tr:" + ua: BasisNameOnly} {
		if got[pos] != want {
			t.Errorf("%s: basis %q, want %q (all: %v)", pos, got[pos], want, got)
		}
	}
}

// Kural: her bildirimin kullandığı dil yapıları kaydedilir; işlecin sol işleneninin türü yerleşik dönüşünden
// (tip çağrısı: `Metin(x)` → TR_type_String; `Tensör(...)` → TR_type_Tensor) ya da değişmez değerden gelir, bilinmezse boştur. Kod
// grafiği bu türle işleci doğru uygulamaya bağlar (deneme 9 H1; değer dili tasarımı 3. tur).
func TestConstructsRecordOperatorsWithOperandTypes(t *testing.T) {
	dir := t.TempDir()
	src := "TR_func f(x, y) {\n    TR_var a = TR_String(x) == \"1\"\n    TR_var b = TR_Tensor([1.0]) @ y\n    TR_return x + y\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), trText(filepath.Join(dir, "m.tr"), src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{} // işleç -> işlenen türü
	kinds := map[string]bool{}
	for _, c := range idx.Constructs {
		if c.In == "function:m.tr#f" {
			kinds[c.Kind] = true
			if c.Op != "" {
				got[c.Op] = c.Operand
			}
		}
	}
	if got["=="] != "type:TR_type_String" || got["@"] != "type:TR_type_Tensor" || got["+"] != "" {
		t.Errorf("operand types: %v", got)
	}
	if !kinds["CallExpression"] || !kinds["ListLiteral"] || !kinds["InfixExpression"] {
		t.Errorf("constructs: %v", kinds)
	}
}
