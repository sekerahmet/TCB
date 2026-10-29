package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

// shown cevap Claude Code'a düzenlemeden sonra verilen bağlam mı; metnini verir.
func shown(t *testing.T, out string) (string, bool) {
	t.Helper()
	if out == "" {
		return "", false
	}
	var d struct {
		HookSpecificOutput struct{ HookEventName, AdditionalContext string }
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("cevap JSON değil: %v %q", err, out)
	}
	h := d.HookSpecificOutput
	return h.AdditionalContext, h.HookEventName == "PostToolUse"
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

// Kural (codegraph kullanım kuralı): bir oturumda izlenen bir dosyanın ilk düzenlemesinden sonra
// tests-for cevabı ajana bağlam olarak gider; düzenleme durmaz (Claude Code'un PostToolUse cevabı).
// Aynı dosyanın sonraki düzenlemesinde bir şey söylenmez; başka dosya ve başka oturum yeniden görür.
func TestFirstEditShowsTheGuardingTestsOnce(t *testing.T) {
	stamps := t.TempDir()
	asked := 0
	_, out := hook(t, stamps, "s1", "a.go", &asked)
	if note, ok := shown(t, out); !ok || !strings.Contains(note, "tests-for a.go") || !strings.Contains(note, "tests guarding it") {
		t.Fatalf("ilk düzenlemeden sonra cevap bağlam olarak verilmeli: %q", out)
	}
	if _, held := held(t, out); held {
		t.Fatalf("düzenleme durdurulmamalı: %q", out)
	}
	if _, out = hook(t, stamps, "s1", "a.go", &asked); out != "" {
		t.Fatalf("ikinci düzenlemede bir şey söylenmemeli: %q", out)
	}
	if _, out = hook(t, stamps, "s1", "b.go", &asked); out == "" {
		t.Fatal("başka dosya görülmeli")
	}
	if _, out = hook(t, stamps, "s2", "a.go", &asked); out == "" {
		t.Fatal("başka oturum yeniden görmeli")
	}
	if asked != 3 {
		t.Fatalf("codegraph %d kez soruldu, beklenen 3", asked)
	}
}

// Kural: kancanın gösterdiği ilk maxLines satır, doğrudan testi olmayan uzun listeli bir Go dosyasında da koşu
// satırlarını ve boyut satırını içerir (2026-10-29: compiler_assign.go'nun 496 testlik cevabının başında ne
// düzenlenen paketin testi ne de compiler.assignTo'nun 80/80 satırı vardı). Gerçek codegraph'a sorar, bu yüzden
// kısa kipte atlanır. Graf yoksa (CI'ın taze çekimi) önce kurulur: codegraph yalnız var olan grafı tazeler.
func TestFirstEditShowsTheRunLineAndTheSizeLine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds codegraph and the graph")
	}
	root := reporoot.Root()
	if _, err := os.Stat(filepath.Join(root, ".codegraph", "graph.json")); os.IsNotExist(err) {
		build := exec.Command("go", "run", "./tools/codegraph", "build")
		build.Dir = root
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("codegraph build: %v\n%s", err, out)
		}
	}
	got := testsFor(target{root, "internal/lang/interp/compiler_assign.go"})
	lines := strings.Split(got, "\n")
	if len(lines) > maxLines+2 { // bayat uyarısı ve "... more lines" satırı
		t.Fatalf("%d lines shown, at most %d:\n%s", len(lines), maxLines, got)
	}
	if !strings.Contains(got, "\nrun: go test -short ./internal/lang/interp/\n") || !strings.Contains(got, "TestLayoutFuncSize|") ||
		!strings.Contains(got, "\n  size (TestLayoutFileSize, TestLayoutFuncSize): file ") {
		t.Fatalf("the first %d lines must hold the run lines and the size line:\n%s", maxLines, got)
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

// shell kancayı bir kabuk aracının (Bash, PowerShell) komutuyla ana oturumda koşar; dosya aranmaz.
func shell(t *testing.T, tool, command string, background bool) string {
	t.Helper()
	return shellIn(t, `C:\Users\u\.claude\projects\C--Turkish-Code-Base\s.jsonl`, tool, command, background)
}

// shellIn shell'dir, dökümü transcript'e yazılan oturumda.
func shellIn(t *testing.T, transcript, tool, command string, background bool) string {
	t.Helper()
	in, err := json.Marshal(map[string]any{"session_id": "s", "transcript_path": transcript, "tool_name": tool,
		"tool_input": map[string]any{"command": command, "run_in_background": background}})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	never := func(string) (target, bool) { t.Fatal("dosya aranmamalı"); return target{}, false }
	if code := run(strings.NewReader(string(in)), &out, &errOut, t.TempDir(), never, nil); code != 0 || errOut.Len() > 0 {
		t.Fatalf("kanca çalışamadı: çıkış %d, %s", code, errOut.String())
	}
	return out.String()
}

// Kural (uzun denetimler arka planda): dakikalar süren denetimler (ön denetim ve tam denetim,
// bütün ağacın ya da bir üst klasörün go test'i) ön planda çağrılınca durur ve arka planda
// koşulması istenir; ileti komutun hiçbir parçasının çalışmadığını söyler. Arka planda koşan,
// saniyelik hedefli test, dar alt ağaç (./internal/lang/...) ve komutu yalnızca anan metin
// (tırnak, heredoc, PowerShell here-string, grep) geçer.
func TestLongCommandsRunInTheBackground(t *testing.T) {
	long := []string{
		"bash tools/full_check.sh",
		`cd "/c/Turkish Code Base" && bash tools/full_check.sh --kip=on 2>&1 | tail -8`,
		"./tools/full_check.sh --kip=zorlama",
		"go test ./... -count=1",
		"go test -short ./internal/... ./cmd/...",
		"python fix.py && go build ./... && go test ./... -count=1",
		"git add x && go run ./tools/codegraph dups --changed main\nTR_LANG=en bash tools/full_check.sh",
		"x=$(bash tools/full_check.sh --kip=on)",
	}
	text := []string{
		"go test ./cmd/tr/e2e -run TestDocLinks -count=1",
		"go test ./internal/lang/... -count=1",
		"grep -n dups_check tools/full_check.sh",
		`git commit -m "tools/full_check.sh; bash tools/full_check.sh runs"`,
		"git commit -q -F - <<'EOF'\nbash tools/full_check.sh is slow\ngo test ./...\nEOF\ngit log -1",
		"git commit -m @'\nbash tools/full_check.sh\n'@",
		`echo 'go test ./...'`,
	}
	for _, tool := range []string{"Bash", "PowerShell"} {
		for _, c := range long {
			if reason, ok := held(t, shell(t, tool, c, false)); !ok || !strings.Contains(reason, "run_in_background") || !strings.HasPrefix(reason, nothingRan) {
				t.Errorf("%s: ön plandaki uzun komut durmalı, gerekçe arka planı söylemeli: %q -> %q", tool, c, reason)
			}
			if out := shell(t, tool, c, true); out != "" {
				t.Errorf("%s: arka plandaki komut geçmeli: %q -> %q", tool, c, out)
			}
		}
		for _, c := range text {
			if out := shell(t, tool, c, false); out != "" {
				t.Errorf("%s: komutu yalnızca anan metin geçmeli: %q -> %q", tool, c, out)
			}
		}
	}
}

// Kural (docs/development.md, tam denetim; tools/edithook/README.md): yan oturum tam denetimi ve bütün ağacın testini
// (`...` desenli go test) arka planda da koşmaz; yalnız dokunduğu paketlerin testlerini ve bir
// maddenin sonunda ön denetimi (`--kip=on`, arka planda) koşar, tam denetim birleştirmede ana
// oturumdadır. Yan oturum, dökümü bir çalışma ağacının klasörüne yazılan oturumdur.
func TestSideSessionsRunOnlyTheirPackagesTests(t *testing.T) {
	side := `C:\Users\u\.claude\projects\C--Turkish-Code-Base--claude-worktrees-ceviri-c395b3\s.jsonl`
	wide := []string{
		"bash tools/full_check.sh",
		"go test ./internal/... ./tools/... ./cmd/...",
		"go test ./internal/lang/... -count=1",
	}
	for _, tool := range []string{"Bash", "PowerShell"} {
		for _, c := range wide {
			for _, background := range []bool{false, true} {
				if reason, ok := held(t, shellIn(t, side, tool, c, background)); !ok || !strings.Contains(reason, "main session") {
					t.Errorf("%s: yan oturumda geniş koşu (arka plan %v) durmalı, gerekçe ana oturumu söylemeli: %q -> %q",
						tool, background, c, reason)
				}
			}
		}
		if out := shellIn(t, side, tool, "go test ./internal/lang/hint ./internal/lang/interp -run TestSuggest -count=1", true); out != "" {
			t.Errorf("%s: yan oturumda dokunulan paketlerin testi geçmeli: %q", tool, out)
		}
		// Ön denetim maddenin sonunda yan oturumun da işidir; dakikalar sürdüğü için arka planda.
		precheck := "bash tools/full_check.sh --kip=on --taban=abc123"
		if out := shellIn(t, side, tool, precheck, true); out != "" {
			t.Errorf("%s: yan oturumda arka plandaki ön denetim geçmeli: %q", tool, out)
		}
		if reason, ok := held(t, shellIn(t, side, tool, precheck, false)); !ok || !strings.Contains(reason, "run_in_background") {
			t.Errorf("%s: yan oturumda ön planda ön denetim arka plana yönlendirilmeli: %q", tool, reason)
		}
	}
}

// Kanca Claude Code'a projenin ayarıyla bağlanır (.claude/settings.json, izlenir, yan oturumların
// çalışma ağaçlarında da): düzenleme araçlarının ve kabuk araçlarının hepsinde bu araç çalışır,
// kabuk araçlarında `go run`'ı yalnızca uzun komut adayları için çağıran ön süzgeçle. Ayar
// bozulur ya da araç taşınırsa kural sessizce kalkardı.
func TestSettingsRunTheHookOnEveryEditAndShellTool(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path(".claude/settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Hooks struct {
			PreToolUse, PostToolUse []struct {
				Matcher string
				Hooks   []struct{ Type, Command string }
			}
		}
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("ayar JSON değil: %v", err)
	}
	// Düzenlemeler düzenlemeden sonra (PostToolUse) görülür, kabuk komutları çalışmadan önce
	// (PreToolUse) durdurulabilir.
	commandOf := map[string]string{}
	for event, entries := range map[string][]struct {
		Matcher string
		Hooks   []struct{ Type, Command string }
	}{"PreToolUse": s.Hooks.PreToolUse, "PostToolUse": s.Hooks.PostToolUse} {
		for _, e := range entries {
			for _, h := range e.Hooks {
				if h.Type == "command" && strings.Contains(h.Command, "go run ./tools/edithook") {
					for _, tool := range strings.Split(e.Matcher, "|") {
						commandOf[event+" "+tool] = h.Command
					}
				}
			}
		}
	}
	for _, tool := range []string{"PostToolUse Edit", "PostToolUse Write", "PostToolUse MultiEdit", "PostToolUse NotebookEdit", "PreToolUse Bash", "PreToolUse PowerShell"} {
		if _, ok := commandOf[tool]; !ok {
			t.Errorf("kanca %s aracında çalışmıyor (.claude/settings.json, `go run ./tools/edithook`)", tool)
		}
	}
	for _, tool := range []string{"Bash", "PowerShell"} {
		for _, candidate := range []string{"full_check.sh", "go test"} {
			if c := commandOf["PreToolUse "+tool]; c != "" && !strings.Contains(c, candidate) {
				t.Errorf("%s kancasının ön süzgeci %q adayını kanca aracına geçirmiyor: %q", tool, candidate, c)
			}
		}
	}
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
