package internal_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

const oldCmdName = "tr"

type oldCmdCheck struct {
	re    *regexp.Regexp
	valid func(line string, st, en int) bool
	fix   func(match string) string
}

// Kural (DENETİM istek 76, Ş3–Ş6): TR komut satırı aracının adı 'trlang'dır; izlenen metin
// dosyalarında eski 'tr <komut>', 'tr <yol>.tr', ileti şablonu ('tr {command}'), 'tr.exe', yolla
// çağrılan 'tr' ikilisi ve 'cmd/tr' paket yolu kalmaz. Komut listesi codes.ClassCommand'dan okunur.
func TestNoOldCommandName(t *testing.T) {
	allowed := loadOldCommandAllowed(t)
	checks := buildOldCmdChecks()
	var bad []string
	used := map[string]bool{}
	skip := func(rel string) bool {
		return strings.HasPrefix(rel, "docs/archive/") ||
			strings.HasPrefix(rel, "tools/trlang_migrate/") ||
			rel == "internal/lang/messages/retired.json" ||
			rel == "internal/command_name_test.go" ||
			rel == "internal/testdata/old_command_allowed.txt"
	}
	scanTextLines(t, skip, func(rel string, lineNo int, text string) {
		if markAllowedOldCmd(allowed, rel, text, used) {
			return
		}
		if (rel == "internal/lang/messages/tr.json" || rel == "internal/lang/messages/en.json") && strings.Contains(text, `"`+oldCmdName+`\t`) {
			bad = append(bad, fmt.Sprintf("%s:%d: eski komut adı %q; doğrusu %q", rel, lineNo, oldCmdName, codes.ProgramName))
		}
		for _, c := range checks {
			for _, loc := range c.re.FindAllStringIndex(text, -1) {
				if c.valid(text, loc[0], loc[1]) {
					got := text[loc[0]:loc[1]]
					bad = append(bad, fmt.Sprintf("%s:%d: eski komut adı %q; doğrusu %q", rel, lineNo, got, c.fix(got)))
				}
			}
		}
	})
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%d yerde eski komut adı bulundu (komut adı %q):\n  %s", len(bad), codes.ProgramName, strings.Join(bad, "\n  "))
	}
	verifyOldCmdAllowancesUsed(t, allowed, used)
}

func loadOldCommandAllowed(t *testing.T) map[string]map[string]string {
	t.Helper()
	p := filepath.Join(reporoot.Root(), "internal", "testdata", "old_command_allowed.txt")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[2]) == "" {
			t.Fatalf("old_command_allowed.txt: %q: dosya, sekme, alt dizge, sekme, gerekçe bekleniyordu", line)
		}
		if out[parts[0]] == nil {
			out[parts[0]] = map[string]string{}
		}
		out[parts[0]][parts[1]] = parts[2]
	}
	return out
}

func markAllowedOldCmd(allowed map[string]map[string]string, rel, text string, used map[string]bool) bool {
	for sub := range allowed[rel] {
		if strings.Contains(text, sub) {
			used[rel+"\x00"+sub] = true
			return true
		}
	}
	return false
}

func verifyOldCmdAllowancesUsed(t *testing.T, allowed map[string]map[string]string, used map[string]bool) {
	t.Helper()
	for file, subs := range allowed {
		for sub := range subs {
			if !used[file+"\x00"+sub] {
				t.Errorf("old_command_allowed.txt: %s dosyasında %q artık eşleşmiyor; kaydı silin", file, sub)
			}
		}
	}
}

func buildOldCmdChecks() []oldCmdCheck {
	var cmds []string
	for _, k := range codes.Codes() {
		if k.Class() == codes.ClassCommand {
			cmds = append(cmds, regexp.QuoteMeta(codes.CommandLineName(k)))
		}
	}
	sort.Strings(cmds)
	replacePrefix := func(m string) string { return codes.ProgramName + m[len(oldCmdName):] }
	replaceSuffix := func(m string) string { return m[:len(m)-len(oldCmdName)] + codes.ProgramName }
	plainLeft := func(s string, st, _ int) bool { return oldCmdPlainBefore(s, st) }
	return []oldCmdCheck{
		{
			re:    regexp.MustCompile(oldCmdName + ` (` + strings.Join(cmds, "|") + `)`),
			valid: func(s string, st, en int) bool { return oldCmdPlainBefore(s, st) && oldCmdIdentAfter(s, en) },
			fix:   replacePrefix,
		},
		{
			re:    regexp.MustCompile(oldCmdName + ` [\p{L}\p{N}_./${}-]+\.` + oldCmdName),
			valid: func(s string, st, en int) bool { return oldCmdPlainBefore(s, st) && oldCmdPathAfter(s, en) },
			fix:   replacePrefix,
		},
		{re: regexp.MustCompile(oldCmdName + ` \{[A-Za-z_][A-Za-z0-9_]*\}`), valid: plainLeft, fix: replacePrefix},
		{re: regexp.MustCompile(oldCmdName + ` <[^>\r\n]+>`), valid: plainLeft, fix: replacePrefix},
		{
			re:    regexp.MustCompile(oldCmdName + ` --(?:\s+\{[A-Za-z_][A-Za-z0-9_]*\}|\s+-[\p{L}\p{N}_.-]+|[\p{L}\p{N}_%-]+)`),
			valid: plainLeft,
			fix:   replacePrefix,
		},
		{
			re: regexp.MustCompile(oldCmdName + `\.exe`),
			valid: func(s string, st, en int) bool {
				r, _ := utf8.DecodeLastRuneInString(s[:st])
				return (st == 0 || (!oldCmdWordRune(r) && r != '.')) && oldCmdIdentAfter(s, en)
			},
			fix: func(string) string { return codes.ProgramName + ".exe" },
		},
		{
			re:    regexp.MustCompile(`(-o\s+"?[^\s"'\x60]+/)` + oldCmdName),
			valid: func(s string, _, en int) bool { return oldCmdBinaryEnd(s, en) },
			fix:   replaceSuffix,
		},
		{
			re:    regexp.MustCompile(`(\./|\$[A-Za-z_][A-Za-z0-9_]*/|\$\{[A-Za-z_][A-Za-z0-9_]*\}/|bin/|/content/` + oldCmdName + `/)` + oldCmdName),
			valid: func(s string, st, en int) bool { return oldCmdPathPrefixBefore(s, st) && oldCmdBinaryEnd(s, en) },
			fix:   replaceSuffix,
		},
		{
			re: regexp.MustCompile(`cmd/` + oldCmdName),
			valid: func(s string, st, en int) bool {
				r, _ := utf8.DecodeLastRuneInString(s[:st])
				return (st == 0 || !oldCmdWordRune(r)) && oldCmdIdentAfter(s, en)
			},
			fix: func(string) string { return "cmd/" + codes.ProgramName },
		},
	}
}

func oldCmdWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func oldCmdPlainBefore(s string, st int) bool {
	if st <= 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:st])
	return !oldCmdWordRune(r) && r != '.' && r != '/'
}

func oldCmdPathPrefixBefore(s string, st int) bool {
	if st <= 0 || s[st] == '$' || s[st] == '/' {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:st])
	return !oldCmdWordRune(r)
}

func oldCmdIdentAfter(s string, en int) bool {
	if en >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[en:])
	return !oldCmdWordRune(r)
}

func oldCmdPathAfter(s string, en int) bool {
	if en >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[en:])
	return !oldCmdWordRune(r) && r != '.' && r != '/'
}

func oldCmdBinaryEnd(s string, en int) bool {
	if en == len(s) || strings.HasPrefix(s[en:], "${ext}") {
		return true
	}
	return strings.ContainsRune(" \t\r\"'`$)", rune(s[en]))
}
