package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, text string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	dir := t.TempDir()
	write(t, dir, "docs/spec/15_x.md", "**15.3** Üç: gövde (bkz. 15.4).\n**15.4** Dört: gövde.\n**15.9** Dokuz: gövde.\n")
	write(t, dir, "docs/a.md", "dil tanımı 15.9'daki, **15.3**'ün ve **15.4**'te; sürüm **15.0**.\n")
	return dir
}

// Kural: --apply olmadan hiçbir dosya değişmez; plan her anmayı dosya:satır ile yazar.
func TestPlanDoesNotWriteWithoutApply(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "moves.txt", "# 15.9 öne alınır\n15.9 -> 15.2\n15.3 15.4\n15.4 15.3\n")
	var out, errb bytes.Buffer
	if code := run([]string{"--root", dir, "--map", filepath.Join(dir, "moves.txt")}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "docs/a.md:1  15.9 → 15.2  [renumber]") || !strings.Contains(out.String(), "docs/a.md:1  daki → deki  [suffix]") {
		t.Errorf("plan:\n%s", out.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "docs", "a.md"))
	if !strings.Contains(string(b), "15.9'daki") {
		t.Errorf("file written without --apply: %s", b)
	}
}

// Kural: --apply özgün konumlardan tek geçişte yazar; swap zincirlenmez, ekler yeni okunuşa uyar, 15.0'a dokunulmaz.
func TestApplyRenumbersSuffixesAndSkipsNonItems(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "moves.txt", "15.9 15.2\n15.3 15.4\n15.4 15.3\n")
	var out, errb bytes.Buffer
	if code := run([]string{"--root", dir, "--map", filepath.Join(dir, "moves.txt"), "--apply"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "docs", "a.md"))
	want := "dil tanımı 15.2'deki, **15.4**'ün ve **15.3**'te; sürüm **15.0**.\n"
	if string(b) != want {
		t.Errorf("got  %q\nwant %q", b, want)
	}
	s, _ := os.ReadFile(filepath.Join(dir, "docs", "spec", "15_x.md"))
	if !strings.Contains(string(s), "**15.4** Üç") || !strings.Contains(string(s), "(bkz. 15.3)") || !strings.Contains(string(s), "**15.2** Dokuz") {
		t.Errorf("spec:\n%s", s)
	}
}

// Kural: çakışan eşleme ve olmayan eski numara reddedilir, hiçbir şey yazılmaz; kullanım hatası 2 verir.
func TestRejectsBadMappingsAndUsage(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "bad.txt", "15.3 15.9\n")
	var out, errb bytes.Buffer
	if code := run([]string{"--root", dir, "--map", filepath.Join(dir, "bad.txt"), "--apply"}, &out, &errb); code != 1 {
		t.Errorf("collision: exit %d", code)
	}
	write(t, dir, "bad.txt", "15.77 15.1\n")
	if code := run([]string{"--root", dir, "--map", filepath.Join(dir, "bad.txt")}, &out, &errb); code != 1 {
		t.Errorf("unknown item: exit %d", code)
	}
	if code := run([]string{"--root", dir}, &out, &errb); code != 2 {
		t.Errorf("no --map: exit %d", code)
	}
}

// Kural: --suffixes yalnızca ses uyumu bozuk ekleri düzeltir.
func TestSuffixesOnlyFixesBrokenHarmony(t *testing.T) {
	dir := fixture(t)
	write(t, dir, "docs/b.md", "dil tanımı 15.9'taki ve **15.3**'in\n")
	var out, errb bytes.Buffer
	if code := run([]string{"--root", dir, "--suffixes", "--apply"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "docs", "b.md"))
	if string(b) != "dil tanımı 15.9'daki ve **15.3**'ün\n" {
		t.Errorf("got %q", b)
	}
}
