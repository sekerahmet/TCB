package main

import (
	"bytes"
	"strings"
	"testing"
)

var refsFiles = map[string]string{
	"docs/spec/15_x.md": "**15.3** Karışık hassasiyet: gövde (bkz. 15.4).\n**15.4** Ortalama: gövde.\n**15.9** Dokuz: gövde.\n",
	"docs/guide.md":     "dil tanımı 15.9'taki ve **15.3**'te, dil tanımı 15.77'de, [15.4](spec/15_x.md)'te.\n",
	"lib/a.go":          "package lib\n\n// dil tanımı 15.3'ün gereği.\nfunc F() {}\n",
}

// Kural: spec-refs olmayan maddeye atıfı hata sayar (çıkış 1), ses uyumu bozuk eki bildirir; geçerli
// atıflar --list'te hedef başlığıyla görünür.
func TestSpecRefsReportsMissingItemsAndSuffixesAndListsTargets(t *testing.T) {
	dir := auditModule(t, refsFiles)
	var out, errb bytes.Buffer
	if code := run([]string{"spec-refs", "--root", dir}, &out, &errb); code != exitFail {
		t.Fatalf("exit %d, want %d: %s", code, exitFail, errb.String())
	}
	got := out.String()
	for _, want := range []string{
		"docs/guide.md:1  error  [missing] no item 15.77",
		"docs/guide.md:1  error  [suffix] suffix 'taki after 15.9",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "15.4'te") || strings.Contains(got, "lib/a.go") {
		t.Errorf("valid references reported:\n%s", got)
	}
	out.Reset()
	if code := run([]string{"spec-refs", "--root", dir, "--list"}, &out, &errb); code != exitOK {
		t.Fatalf("--list exit %d: %s", code, errb.String())
	}
	for _, want := range []string{"docs/guide.md:1  15.4 → Ortalama  (docs/spec/15_x.md:2)", "lib/a.go:3  15.3 → Karışık hassasiyet  (docs/spec/15_x.md:1)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--list lacks %q:\n%s", want, out.String())
		}
	}
}

// Kural: temiz bir depoda spec-refs çıkış 0 verir.
func TestSpecRefsPassesOnCleanReferences(t *testing.T) {
	dir := auditModule(t, map[string]string{
		"docs/spec/15_x.md": refsFiles["docs/spec/15_x.md"],
		"docs/guide.md":     "dil tanımı 15.9'daki ve **15.3**'te.\n",
	})
	var out, errb bytes.Buffer
	if code := run([]string{"spec-refs", "--root", dir}, &out, &errb); code != exitOK {
		t.Fatalf("exit %d: %s%s", code, out.String(), errb.String())
	}
}
