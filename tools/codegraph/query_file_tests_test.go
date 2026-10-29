package main

import (
	"slices"
	"strings"
	"testing"
)

// fileOrderFiles: p.Target'ı iki test 2 adımda sınar; biri başka paketten (adı alfabede önde), biri aynı paketten.
var fileOrderFiles = map[string]string{
	"p/x.go":         "package p\n\nfunc Target() int { return 1 }\n",
	"p/wrap.go":      "package p\n\nfunc Wrap() int { return Target() }\n",
	"p/wrap_test.go": "package p\n\nimport \"testing\"\n\nfunc TestLocal(t *testing.T) { _ = Wrap() }\n",
	"a/a_test.go":    "package a\n\nimport (\n\t\"testing\"\n\n\t\"example.com/audit/p\"\n)\n\nfunc TestAway(t *testing.T) { _ = p.Wrap() }\n",
}

// testsForItems tests-for'u fikstürde koşar ve heading'i içeren grubun öğelerinin adlarını verir.
func testsForItems(t *testing.T, root, target, heading string) []string {
	t.Helper()
	var out, errb strings.Builder
	if code := run([]string{"tests-for", target, "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for %s: %d %s", target, code, errb.String())
	}
	items, ok := groupItems(strings.Split(out.String(), "\n"), heading)
	if !ok {
		t.Fatalf("tests-for %s: no group %q:\n%s", target, heading, out.String())
	}
	var names []string
	for _, it := range items {
		names = append(names, strings.Fields(it)[0])
	}
	return names
}

// Kural: dosya düzeyindeki tests-for bildirim düzeyiyle aynı sıralama kuralını (testRank) kullanır: eşit uzaklıkta
// hedefle aynı klasördeki test önce, sonra ada göre (DENETİM 2026-10-29: `tests-for assign.go`'da düzenlenen paketin
// ilk testi 140. satırdaydı). Koşulan küme değişmez: dosyanın grupları changed'in seçimidir.
func TestAFileOrdersItsTestsLikeItsDeclarations(t *testing.T) {
	root := auditModule(t, fileOrderFiles)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	want := []string{"p.TestLocal", "a.TestAway"}
	if got := testsForItems(t, root, "p.Target", "<- tests, indirect ("); !slices.Equal(got, want) {
		t.Errorf("declaration: indirect tests %v, want %v (same folder first at equal steps)", got, want)
	}
	if got := testsForItems(t, root, "p/x.go", "<- tests, indirect ("); !slices.Equal(got, want) {
		t.Errorf("file: indirect tests %v, want %v (the declaration's order)", got, want)
	}
}
