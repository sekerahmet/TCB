package codeindex

import (
	"os"
	"path/filepath"
	"testing"
)

const inferSource = `yapı S {
    a
    fonksiyon kurucu() {
        kendi.a = 1
    }
}

fonksiyon yap() {
    sonuç S()
}

fonksiyon yap2() {
    değişken s = yap()
    sonuç s
}

fonksiyon karışık(b) {
    eğer b {
        sonuç S()
    }
    sonuç 3
}

değişken g = yap2()
yazdır(g.a)

fonksiyon kullan() {
    değişken t = yap()
    değişken u = karışık(doğru)
    sonuç t.a + u.a
}
`

// Kural: tür çıkarımı tek tür ya da bilinmezdir (birleşim yok). Yapıcı, bütün `sonuç`'ları aynı yapıyı veren işlevin
// dönüşü ve tek bağlamalı yerel ya da üst düzey değişken türü taşır; `x.alan` o yapının alanına inferred_type dayanağıyla
// bağlanır. `sonuç`'ları farklı tür veren işlevin dönüşü bilinmez kalır (deneme 7 R4; kod-grafigi-40).
func TestInferredTypesBindMemberAccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), []byte(inferSource), 0o644); err != nil {
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
	for pos, want := range map[string]Basis{"m.tr:25:10": BasisInferredType, "m.tr:30:13": BasisInferredType, "m.tr:30:19": BasisNameOnly} {
		if got[pos] != want {
			t.Errorf("%s: basis %q, want %q (all: %v)", pos, got[pos], want, got)
		}
	}
}
