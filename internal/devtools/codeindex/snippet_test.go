package codeindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const builtinUseSource = `değişken kn = kontrol_noktası(1, {}, her_dakika: 3)
kn.güncelle(1)

test "kontrol noktası adımı bildirir" {
    değişken k = kontrol_noktası(1, {}, her_dakika: 1)
    k.güncelle(2)
}
`

func builtinUseReader(t *testing.T) *Reader {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tests", "a_test.tr"), []byte(builtinUseSource), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return NewReader(idx)
}

// Kural: üye (Y) ve adlı argüman (x9xx) kodları da yerleşik düğümdür; test bloğunun kullandığı her yerleşik, üye ve
// adlı argüman o ad koduna `tests` bağıdır (`sınayanlar` boş dönmez). Ad koduyla da sorulabilir (`T984`).
func TestMembersAndNamedArgumentsAreBuiltinsTestedByTestBlocks(t *testing.T) {
	r := builtinUseReader(t)
	for _, q := range []string{"TR_arg_every_minutes", "her_dakika", "TR_mem_update", "güncelle"} {
		res := r.TestedBy(q, QueryOptions{})
		if res.Status != StatusOK || len(res.Tests) != 1 || !strings.Contains(res.Tests[0].In.Name, "kontrol noktası adımı bildirir") {
			t.Errorf("sınayanlar %s: %+v", q, res)
		}
	}
	users := r.References("TR_arg_every_minutes", QueryOptions{})
	if users.Status != StatusOK || users.Counts == nil || users.Counts.Total < 2 {
		t.Errorf("kullananlar TR_arg_every_minutes sees the program and the test block: %+v", users)
	}
}

// Kural: aynı ad hem bildirilmiş bir adı (komut satırı seçeneği) hem yerleşik bir adı gösteriyorsa ikisi de aday olur.
func TestADeclaredNameAndABuiltinNameAreBothCandidates(t *testing.T) {
	dir := t.TempDir()
	src := "komut_satırı(her_dakika: 5)\nkontrol_noktası(1, {}, her_dakika: 3)\n"
	if err := os.WriteFile(filepath.Join(dir, "a.tr"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, n := range NewReader(idx).Resolve("her_dakika", "") {
		kinds = append(kinds, string(n.Kind))
	}
	if len(kinds) < 1 {
		t.Fatal("her_dakika resolves to nothing")
	}
	hasBuiltin := false
	for _, k := range kinds {
		hasBuiltin = hasBuiltin || k == string(KindBuiltin)
	}
	if !hasBuiltin {
		t.Errorf("the builtin named argument must be among the candidates: %v", kinds)
	}
}

// Kural: Snippet bir metindeki TR kodunun kullandığı yerleşik, üye ve adlı argüman kodlarını verir (Go test dizeleri
// için); ayrıştırılamayan metin nil verir.
func TestSnippetListsTheCodesAProgramTextUses(t *testing.T) {
	got := Snippet(builtinUseSource)
	var codes []string
	for _, u := range got {
		codes = append(codes, u.Code)
	}
	if !strings.Contains(strings.Join(codes, ","), "TR_fn_checkpoint") || !strings.Contains(strings.Join(codes, ","), "TR_arg_every_minutes") ||
		!strings.Contains(strings.Join(codes, ","), "TR_mem_update") {
		t.Errorf("codes = %v", codes)
	}
	if Snippet("bu bir ((( program değil") != nil {
		t.Error("an unparsable text has no uses")
	}
}
