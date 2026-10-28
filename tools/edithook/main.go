// edithook, Claude Code'un düzenleme kancasıdır (PostToolUse: Edit, Write, MultiEdit,
// NotebookEdit; .claude/settings.json). Bir oturum depoda izlenen bir dosyayı ilk kez
// düzenleyince o dosyanın `codegraph tests-for` cevabının başını ajana bağlam olarak verir:
// düzenleme durmaz (durdurup yinelemek her dosyada bir tur alıyor, kararı değiştirmiyordu), ajan
// dosyayı koruyan testleri görür ve commit'ten önce hızlı olanları koşar.
//
// Kabuk araçlarında (Bash, PowerShell; PreToolUse) ikinci kural: dakikalar süren denetimler (ön
// denetim ve tam denetim `tools/full_check.sh`, bütün ağacın ya da bir üst klasörün `go test`'i:
// `./...`, `./internal/...`) ön planda çağrılınca durur ve arka planda koşulması istenir; dar alt
// ağaç (`./internal/lang/...`) geçer. Durdurma bütün komutu durdurur, iletisi bunu söyler: aynı
// komuttaki önceki adımlar da çalışmamıştır. Yazılı kural ve hafıza bunu tutmadı, kanca tutar. Ayar dosyası
// `go run`'ı yalnızca komut bu adayları anıyorsa çağırır (her kabuk çağrısına derleme bedeli
// binmez); asıl karar burada, tırnak ve heredoc metni sayılmadan verilir.
//
// Kanca girdisi standart girişten JSON gelir (session_id, tool_name, tool_input.file_path ya da
// notebook_path; kabukta tool_input.command ve run_in_background). Depo dışındaki, izlenmeyen (yeni ya da scratch) dosyalar ve oturumda daha önce
// gösterilmiş dosyalar sessizce geçer. İşaretler geçici klasörde oturum başınadır. Bağlam ve
// durdurma çıkış koduyla değil Claude Code'un JSON cevabıyla (additionalContext, permissionDecision "deny") verilir: kanca
// `go run` ile çalışır ve `go run` programın 2 çıkış kodunu 1'e çevirir. Kancanın kendisi
// çalışamazsa (bozuk girdi, git yok) çıkış 1'dir: Claude Code düzenlemeyi durdurmaz.
//
// `go run ./tools/edithook codegraph <args>` aynı aracın üçüncü işidir: ön denetim codegraph'ı
// kancayla aynı yoldan koşar (codegraph.go: derlenmeyen ağaçta son derlenen ikili, bayat uyarısıyla).
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
	"slices"
	"strings"
)

// maxLines tests-for çıktısından gösterilen en çok satırdır; uzun listenin başı (önce dosyanın
// kendi testleri ve doğrudan testler) yeter, gerisi elle sorulur.
const maxLines = 30

type hookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	ToolName       string `json:"tool_name"`
	ToolInput      struct {
		FilePath        string `json:"file_path"`
		NotebookPath    string `json:"notebook_path"`
		Command         string `json:"command"`
		RunInBackground bool   `json:"run_in_background"`
	} `json:"tool_input"`
}

// target düzenlenecek dosyanın deposu ve depodaki yoludur (eğik çizgili).
type target struct{ root, rel string }

func main() {
	if code, ok := mode(os.Args[1:], os.Stdout, os.Stderr); ok {
		os.Exit(code)
	}
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr, filepath.Join(os.TempDir(), "tr-edithook"), locate, testsFor))
}

// run kancanın cevabıdır. Çıkış 0'da out boşsa söylenecek bir şey yoktur; doluysa kabukta durdurma
// kararı, düzenlemede ajana giden bağlamdır. Çıkış 1 kancanın çalışamadığıdır (errOut).
func run(in io.Reader, out, errOut io.Writer, stamps string, find func(string) (target, bool), ask func(target) string) int {
	var h hookInput
	if err := json.NewDecoder(in).Decode(&h); err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	if h.ToolName == "Bash" || h.ToolName == "PowerShell" {
		side := sideSession(h.TranscriptPath)
		switch c := longCommand(h.ToolInput.Command, side); {
		case c == "":
		case side && c != precheck:
			return deny(out, errOut, nothingRan+c+" is the main session's job: a side session runs only the tests of the packages "+
				"it touched (go test ./internal/x -run TestY, the tests-for answer), in the background; the main session "+
				"runs the full check when it merges (CLAUDE.md, side sessions).")
		case !h.ToolInput.RunInBackground:
			return deny(out, errOut, nothingRan+c+" takes minutes: run it with run_in_background: true and go on; "+
				"the notification says when it ends (CLAUDE.md, hard rules: long checks run in the background).")
		}
		return 0
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
	note := fmt.Sprintf("First edit of %s in this session (the edit is done): these tests and rules guard it; "+
		"keep the change by them and run the fast ones before committing. A wrong, ambiguous or missing "+
		"answer is reported to the main session with the command, the expected and the actual answer "+
		"(tools/codegraph/gaps.md).\n\n$ go run ./tools/codegraph tests-for %s\n%s", t.rel, t.rel, report)
	return context(out, errOut, note)
}

// nothingRan kabuk durdurma iletisinin başıdır: kanca komutun tamamını durdurur, ajan aynı komuttaki
// önceki adımların (bir düzeltme, bir derleme) çalıştığını sanmasın.
const nothingRan = "Nothing in this command ran, its earlier steps included (the hook stops the whole command): "

// sideSession oturum bir yan çalışma ağacında mı başladı: Claude Code dökümü oturumun başladığı
// klasörün adından kurulan klasöre yazar (C--Turkish-Code-Base--claude-worktrees-<ad>); ana
// oturumun ve ajanlarının dökümü depo kökünün klasöründedir. Oturumun o anki klasörü (cwd) ölçüt
// değildir: ana oturum birleştirmede bir çalışma ağacına geçip tam denetimi orada koşar.
func sideSession(transcript string) bool {
	return strings.Contains(transcript, "-claude-worktrees-")
}

// precheck ön denetimdir (`full_check.sh --kip=on`): yan oturum da bir maddenin sonunda koşar, arka
// planda; tam denetim ana oturumundur.
const precheck = "tools/full_check.sh --kip=on"

// longCommand komut dakikalar süren bir denetimi çağırıyorsa onun adını verir (ön denetimse
// precheck), yoksa "". Yan oturumda (side) her alt ağaç deseni geniştir: yan oturum yalnız dokunduğu
// paketleri koşar.
func longCommand(command string, side bool) string {
	pattern := wide
	if side {
		pattern = subtree
	}
	for _, words := range commands(command) {
		for len(words) > 0 && (strings.Contains(words[0], "=") || slices.Contains([]string{"bash", "sh", "time", "nohup", "env"}, words[0])) {
			words = words[1:] // ortam ataması (TR_DIL=en) ve sarmalayan komut
		}
		switch {
		case len(words) == 0:
		case strings.TrimPrefix(words[0], "./") == "tools/full_check.sh":
			if slices.Contains(words[1:], "--kip=on") {
				return precheck
			}
			return "tools/full_check.sh"
		case len(words) > 1 && words[0] == "go" && words[1] == "test" && slices.ContainsFunc(words[2:], pattern):
			return "go test ./..."
		}
	}
	return ""
}

// wide go test'in desen argümanı bütün ağaç ya da bir üst klasör mü (./..., ./internal/...,
// cmd/...): bunlar dakikalar sürer. Daha dar alt ağaç (./internal/lang/...) saniyeler sürer, geçer.
func wide(a string) bool {
	p, ok := strings.CutSuffix(strings.TrimPrefix(a, "./"), "...")
	return ok && strings.Count(strings.Trim(p, "/"), "/") == 0
}

// subtree go test'in desen argümanı herhangi bir alt ağaç mı (yan oturumda hepsi geniştir).
func subtree(a string) bool { return strings.HasSuffix(a, "...") }

// commands kabuk metnini komutlara ve sözcüklere böler: ayırıcılar &&, ||, ;, |, &, (, ) ve satır
// sonudur. Tırnağın içi sözcüğün metnidir; heredoc gövdesi (<<EOF ... EOF) ve PowerShell
// here-string'i (@' ... '@) metindir, komut sayılmaz. Ayırıcılar ASCII olduğundan bayt bayt gezmek
// UTF-8 metni bozmaz.
func commands(s string) [][]string {
	var out [][]string
	var words, heredocs []string
	var word strings.Builder
	inWord := false
	endWord := func() {
		if inWord {
			words, inWord = append(words, word.String()), false
			word.Reset()
		}
	}
	endCommand := func() {
		if endWord(); len(words) > 0 {
			out, words = append(out, words), nil
		}
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '@' && i+2 < len(s) && (s[i+1] == '\'' || s[i+1] == '"') && (s[i+2] == '\n' || s[i+2] == '\r'):
			end := strings.Index(s[i+2:], "\n"+s[i+1:i+2]+"@")
			if end < 0 {
				i = len(s)
				continue
			}
			i, inWord = i+2+end+2, true
		case c == '\'' || c == '"':
			j := i + 1
			for j < len(s) && s[j] != c {
				if c == '"' && s[j] == '\\' {
					j++
				}
				j++
			}
			j = min(j, len(s))
			word.WriteString(s[i+1 : j])
			i, inWord = j, true
		case c == '<' && i+1 < len(s) && s[i+1] == '<':
			j := i + 2
			for j < len(s) && (s[j] == '-' || s[j] == ' ') {
				j++
			}
			k := j
			for k < len(s) && !strings.ContainsRune(" \t\r\n;&|<>()", rune(s[k])) {
				k++
			}
			if tag := strings.Trim(s[j:k], `'"`); tag != "" {
				heredocs = append(heredocs, tag)
			}
			i = k - 1
		case c == '\n':
			endCommand()
			for _, tag := range heredocs {
				for i < len(s) {
					start, end := i+1, len(s)
					if n := strings.IndexByte(s[min(start, len(s)):], '\n'); n >= 0 {
						end = start + n
					}
					line := strings.TrimSpace(s[min(start, len(s)):end])
					i = end
					if line == tag {
						break
					}
				}
			}
			heredocs = nil
		case strings.IndexByte(";&|()", c) >= 0:
			endCommand()
		case c == ' ' || c == '\t' || c == '\r':
			endWord()
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	endCommand()
	return out
}

// context düzenlemeden sonra ajana bağlam verir (Claude Code'un PostToolUse cevabı).
func context(out, errOut io.Writer, text string) int {
	type note struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	}
	err := json.NewEncoder(out).Encode(struct {
		HookSpecificOutput note `json:"hookSpecificOutput"`
	}{note{"PostToolUse", text}})
	if err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	return 0
}

// deny kabuk komutunu Claude Code'un PreToolUse kararıyla durdurur; gerekçe ajana gider.
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
// Depodaki yolu git'in kendisi verir (ls-files --full-name): kökle dosyanın yolunu burada
// karşılaştırmak Windows'un kısa adlarında (RUNNER~1) ya da sembolik bağda yanılır.
func locate(path string) (target, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return target{}, false
	}
	dir, name := filepath.Split(abs)
	top, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return target{}, false
	}
	rel, err := exec.Command("git", "--literal-pathspecs", "-C", dir, "ls-files", "--full-name", "--error-unmatch", "--", name).Output()
	if err != nil {
		return target{}, false
	}
	return target{filepath.Clean(strings.TrimSpace(string(top))), strings.TrimSpace(string(rel))}, true
}

// testsFor codegraph'a dosyayı koruyan testleri ve kuralları sorar; cevabın ilk maxLines satırı.
// Araç bu ağaçla derlenmiyorsa cevap son derlenen sürümündür ve başında bunu söyler (codegraph.go).
func testsFor(t target) string {
	cmd, stale, err := codegraph(t.root, "tests-for", t.rel)
	var b []byte
	if err == nil {
		b, err = cmd.CombinedOutput()
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], fmt.Sprintf("... (%d more lines: run the command)", len(lines)-maxLines))
	}
	text := strings.Join(lines, "\n")
	if stale != "" {
		text = stale + "\n" + text
	}
	if err != nil {
		text += fmt.Sprintf("\n(codegraph: %v; an ambiguous or missing answer is not evidence: check with grep, "+
			"CLAUDE.md on reading codegraph answers)", err)
	}
	return text
}
