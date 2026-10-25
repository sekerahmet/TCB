// edithook, Claude Code'un düzenleme kancasıdır (PreToolUse: Edit, Write, MultiEdit,
// NotebookEdit; .claude/settings.json). Bir oturum depoda izlenen bir dosyayı ilk kez
// düzenlemeden önce o dosyanın `codegraph tests-for` çıktısını ajana gösterir ve düzenlemeyi bir
// kez durdurur; ajan dosyayı koruyan testleri ve kuralları okuyup düzenlemeyi yineleyince geçer.
// Kural CLAUDE.md'dedir ("Kesin kurallar": dokunmadan önce tests-for); kanca onu ajanın
// dikkatine bırakmaz.
//
// Kanca girdisi standart girişten JSON gelir (session_id, tool_input.file_path ya da
// notebook_path). Depo dışındaki, izlenmeyen (yeni ya da scratch) dosyalar ve oturumda daha önce
// gösterilmiş dosyalar sessizce geçer. İşaretler geçici klasörde oturum başınadır. Durdurma
// çıkış koduyla değil Claude Code'un JSON kararıyla (permissionDecision "deny") yapılır: kanca
// `go run` ile çalışır ve `go run` programın 2 çıkış kodunu 1'e çevirir. Kancanın kendisi
// çalışamazsa (bozuk girdi, git yok) çıkış 1'dir: Claude Code düzenlemeyi durdurmaz.
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// maxLines tests-for çıktısından gösterilen en çok satırdır; uzun listenin başı (önce doğrudan
// testler, sonra kurallar) yeter, gerisi elle sorulur.
const maxLines = 60

type hookInput struct {
	SessionID string `json:"session_id"`
	ToolInput struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"tool_input"`
}

// target düzenlenecek dosyanın deposu ve depodaki yoludur (eğik çizgili).
type target struct{ root, rel string }

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr, filepath.Join(os.TempDir(), "tr-edithook"), locate, testsFor))
}

// run kancanın kararıdır. Çıkış 0'da out boşsa düzenleme geçer, doluysa out durdurma kararıdır
// (gerekçesi ajana gider); çıkış 1 kancanın çalışamadığıdır (errOut).
func run(in io.Reader, out, errOut io.Writer, stamps string, find func(string) (target, bool), ask func(target) string) int {
	var h hookInput
	if err := json.NewDecoder(in).Decode(&h); err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	path := h.ToolInput.FilePath
	if path == "" {
		path = h.ToolInput.NotebookPath
	}
	if path == "" {
		return 0
	}
	t, ok := find(path)
	if !ok {
		return 0
	}
	stamp := filepath.Join(stamps, safeName.ReplaceAllString(h.SessionID, "_"), key(t))
	if _, err := os.Stat(stamp); err == nil {
		return 0
	}
	report := ask(t)
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	reason := fmt.Sprintf("First edit of %s in this session: read the tests and rules that guard it "+
		"(CLAUDE.md, hard rules: codegraph), write the change by them, run the fast ones before "+
		"committing, then repeat the edit. A wrong, ambiguous or missing answer is reported to the "+
		"main session with the command, the expected and the actual answer (tools/codegraph/gaps.md)."+
		"\n\n$ go run ./tools/codegraph tests-for %s\n%s", t.rel, t.rel, report)
	return deny(out, errOut, reason)
}

// deny düzenlemeyi Claude Code'un PreToolUse kararıyla durdurur; gerekçe ajana gider.
func deny(out, errOut io.Writer, reason string) int {
	type decision struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	}
	err := json.NewEncoder(out).Encode(struct {
		HookSpecificOutput decision `json:"hookSpecificOutput"`
	}{decision{"PreToolUse", "deny", reason}})
	if err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	return 0
}

var safeName = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// key dosyanın işaret adıdır: aynı yol farklı çalışma ağaçlarında ayrı sayılır.
func key(t target) string {
	sum := sha256.Sum256([]byte(t.root + "\x00" + t.rel))
	return hex.EncodeToString(sum[:8])
}

// locate dosyanın git deposunu bulur; dosya depoda izlenmiyorsa (yeni, scratch, depo dışı) false.
func locate(path string) (target, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return target{}, false
	}
	top, err := exec.Command("git", "-C", filepath.Dir(abs), "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return target{}, false
	}
	root := filepath.Clean(strings.TrimSpace(string(top)))
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return target{}, false
	}
	rel = filepath.ToSlash(rel)
	if exec.Command("git", "-C", root, "ls-files", "--error-unmatch", "--", rel).Run() != nil {
		return target{}, false
	}
	return target{root, rel}, true
}

// testsFor codegraph'a dosyayı koruyan testleri ve kuralları sorar; cevabın ilk maxLines satırı.
func testsFor(t target) string {
	cmd := exec.Command("go", "run", "./tools/codegraph", "tests-for", t.rel)
	cmd.Dir = t.root
	b, err := cmd.CombinedOutput()
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], fmt.Sprintf("... (%d more lines: run the command)", len(lines)-maxLines))
	}
	text := strings.Join(lines, "\n")
	if err != nil {
		text += fmt.Sprintf("\n(codegraph: %v; an ambiguous or missing answer is not evidence: check with grep, "+
			"CLAUDE.md on reading codegraph answers)", err)
	}
	return text
}
