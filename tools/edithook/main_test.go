package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// hook kancayı verilen oturum ve dosya için bir kez koşar; find her dosyayı depoda izlenen sayar.
func hook(t *testing.T, stamps, session, file string, asked *int) (int, string) {
	t.Helper()
	var out, errOut strings.Builder
	in := strings.NewReader(`{"session_id":"` + session + `","tool_name":"Edit","tool_input":{"file_path":"` + file + `"}}`)
	find := func(p string) (target, bool) { return target{"/repo", p}, true }
	ask := func(target) string { *asked++; return "tests guarding it" }
	code := run(in, &out, &errOut, stamps, find, ask)
	if code != 0 || errOut.Len() > 0 {
		t.Fatalf("kanca çalışamadı: çıkış %d, %s", code, errOut.String())
	}
	return code, out.String()
}

// held karar Claude Code'un durdurma kararı mı; gerekçesini verir.
func held(t *testing.T, out string) (string, bool) {
	t.Helper()
	if out == "" {
		return "", false
	}
	var d struct {
		HookSpecificOutput struct {
			HookEventName, PermissionDecision, PermissionDecisionReason string
		}
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("karar JSON değil: %v %q", err, out)
	}
	h := d.HookSpecificOutput
	return h.PermissionDecisionReason, h.HookEventName == "PreToolUse" && h.PermissionDecision == "deny"
}

// Kural (CLAUDE.md, Kesin kurallar): bir oturumda izlenen bir dosyanın ilk düzenlemesi bir kez
// durur (Claude Code'un "deny" kararı) ve tests-for cevabı gerekçe olarak ajana gider; aynı dosyanın sonraki düzenlemesi geçer, başka oturum
// yeniden görür.
func TestFirstEditIsHeldOnceThenPasses(t *testing.T) {
	stamps := t.TempDir()
	asked := 0
	_, out := hook(t, stamps, "s1", "a.go", &asked)
	if reason, ok := held(t, out); !ok || !strings.Contains(reason, "tests-for a.go") || !strings.Contains(reason, "tests guarding it") {
		t.Fatalf("ilk düzenleme durmalı, gerekçede cevap olmalı: %q", out)
	}
	if _, out = hook(t, stamps, "s1", "a.go", &asked); out != "" {
		t.Fatalf("ikinci düzenleme geçmeli: %q", out)
	}
	if _, out = hook(t, stamps, "s1", "b.go", &asked); out == "" {
		t.Fatal("başka dosya durmalı")
	}
	if _, out = hook(t, stamps, "s2", "a.go", &asked); out == "" {
		t.Fatal("başka oturum durmalı")
	}
	if asked != 3 {
		t.Fatalf("codegraph %d kez soruldu, beklenen 3", asked)
	}
}

// İzlenmeyen ya da depo dışındaki dosya ve yolu olmayan girdi sessizce geçer; bozuk girdi kancanın
// hatasıdır (çıkış 1: Claude Code düzenlemeyi durdurmaz).
func TestUntrackedFilesAndBadInputPass(t *testing.T) {
	never := func(target) string { t.Fatal("codegraph sorulmamalı"); return "" }
	var out, errOut strings.Builder
	in := strings.NewReader(`{"session_id":"s","tool_input":{"file_path":"scratch/x.tr"}}`)
	if code := run(in, &out, &errOut, t.TempDir(), func(string) (target, bool) { return target{}, false }, never); code != 0 || out.Len() > 0 {
		t.Fatalf("izlenmeyen dosya: çıkış %d, %q", code, out.String())
	}
	if code := run(strings.NewReader(`{"session_id":"s","tool_input":{}}`), &out, &errOut, t.TempDir(), locate, never); code != 0 || out.Len() > 0 {
		t.Fatalf("yolsuz girdi: çıkış %d, %q", code, out.String())
	}
	if code := run(strings.NewReader(`{`), &out, &errOut, t.TempDir(), locate, never); code != 1 || out.Len() > 0 {
		t.Fatalf("bozuk girdi: çıkış %d, %q", code, out.String())
	}
}

// Kanca Claude Code'a projenin ayarıyla bağlanır (.claude/settings.json, izlenir): düzenleme
// araçlarının hepsinde bu araç çalışır. Ayar bozulur ya da araç taşınırsa kural sessizce kalkardı.
func TestSettingsRunTheHookOnEveryEditTool(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path(".claude/settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string
				Hooks   []struct{ Type, Command string }
			}
		}
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("ayar JSON değil: %v", err)
	}
	for _, e := range s.Hooks.PreToolUse {
		for _, h := range e.Hooks {
			if h.Type != "command" || !strings.Contains(h.Command, "go run ./tools/edithook") {
				continue
			}
			for _, tool := range []string{"Edit", "Write", "MultiEdit", "NotebookEdit"} {
				if !slices.Contains(strings.Split(e.Matcher, "|"), tool) {
					t.Errorf("kanca %s aracında çalışmıyor: matcher %q", tool, e.Matcher)
				}
			}
			return
		}
	}
	t.Fatal(".claude/settings.json'da PreToolUse kancası `go run ./tools/edithook` yok")
}

// locate yalnızca git'in izlediği dosyayı depo yoluyla verir; yeni dosya ve depo dışı geçer.
func TestLocateTrackedFilesOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"dir/a.go", "dir/new.go"} {
		if err := os.WriteFile(filepath.Join(repo, f), []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "dir/a.go"}} {
		if b, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
	}
	if got, ok := locate(filepath.Join(repo, "dir", "a.go")); !ok || got.rel != "dir/a.go" {
		t.Fatalf("izlenen dosya: %+v %v", got, ok)
	}
	if _, ok := locate(filepath.Join(repo, "dir", "new.go")); ok {
		t.Fatal("izlenmeyen dosya geçmeli")
	}
	if _, ok := locate(filepath.Join(t.TempDir(), "x.go")); ok {
		t.Fatal("depo dışı dosya geçmeli")
	}
}
