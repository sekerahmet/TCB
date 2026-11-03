package main

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

const (
	oldProg = "tr"
	newProg = codes.ProgramName
)

// skipPrefixes are tracked directory prefixes untouched by the migration tool (S4).
var skipPrefixes = []string{
	"docs/archive/",
	"tools/trlang_migrate/",
}

// skipExactFiles are tracked files untouched by the migration tool: generated outputs
// (rewritten via tools/gen/...), retired message history, and files maintained only by
// the main session or dedicated tests.
var skipExactFiles = map[string]bool{
	"GEMINI_KURALLARI.md":                          true,
	"docs/codes.md":                                true,
	"internal/command_name_test.go":                true,
	"internal/testdata/old_command_allowed.txt":    true,
	"internal/lang/codes/codes_generated.go":       true,
	"internal/lang/messages/codes_generated.go":    true,
	"internal/lang/messages/retired.json":          true,
	"internal/lang/lexer/confusables_generated.go": true,
	"editors/vscode/syntaxes/tr.tmLanguage.json":   true,
	"tools/gen/snippets/data.json":                 true,
	"tools/codegraph/gaps.md":                      true,
}

var skipBinaryExts = map[string]bool{
	".png": true, ".ico": true, ".safetensors": true, ".npy": true,
	".trt": true, ".bin": true, ".pdf": true, ".woff2": true,
}

// skipLineSubstrings skips specific historical or GNU-tr rationale lines (S4).
var skipLineSubstrings = map[string][]string{
	"docs/roadmap.md": {
		"Unix'in",
		"tr profile --go-profile",
	},
	"docs/status.md": {
		"claude/d1-dilim2",
		"Faz 0.4, 2026-10-30",
		"GNU coreutils 8.32",
		"main `85a19c36`",
	},
	"internal/devtools/codeindex/README.md": {
		"<scheme> <package>",
	},
	"tools/codegraph/README.md": {
		"TCB 2026-10-25, item 3",
	},
	"tools/codegraph/test_names.go": {
		"TCB 2026-10-25",
	},
	"tools/codegraph/test_names_test.go": {
		"TCB 2026-10-25",
	},
}

func commandNames() []string {
	var list []string
	for _, k := range codes.Codes() {
		if k.Class() == codes.ClassCommand {
			list = append(list, codes.CommandLineName(k))
		}
	}
	sort.Strings(list)
	return list
}

func shouldSkipFile(rel string) bool {
	if skipExactFiles[rel] {
		return true
	}
	for _, p := range skipPrefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return skipBinaryExts[strings.ToLower(filepath.Ext(rel))]
}

func shouldSkipLine(rel, line string) bool {
	for _, sub := range skipLineSubstrings[rel] {
		if strings.Contains(line, sub) {
			return true
		}
	}
	return false
}

type compiledRules struct {
	cmds       []string
	cmdEachRe  map[string]*regexp.Regexp
	cmdRe      *regexp.Regexp
	fileRe     *regexp.Regexp
	tmplBrace  *regexp.Regexp
	tmplAngle  *regexp.Regexp
	tmplDash   *regexp.Regexp
	exeRe      *regexp.Regexp
	pathOutRe  *regexp.Regexp
	pathBinRe  *regexp.Regexp
	cmdPkgRe   *regexp.Regexp
	cmdsOfRe   *regexp.Regexp
	mainSymRe  *regexp.Regexp
	proseCmdRe *regexp.Regexp
	specRunOld string
	specRunNew string
}

func buildRules() *compiledRules {
	cmds := commandNames()
	alt := strings.Join(cmds, "|")
	each := make(map[string]*regexp.Regexp, len(cmds))
	for _, c := range cmds {
		each[c] = regexp.MustCompile(oldProg + ` ` + regexp.QuoteMeta(c))
	}
	runTR := codes.Source().Name(codes.CmdRun)
	return &compiledRules{
		cmds:       cmds,
		cmdEachRe:  each,
		cmdRe:      regexp.MustCompile(oldProg + ` (` + alt + `)`),
		fileRe:     regexp.MustCompile(oldProg + ` [\p{L}\p{N}_./${}-]+\.` + oldProg),
		tmplBrace:  regexp.MustCompile(oldProg + ` \{[A-Za-z_][A-Za-z0-9_]*\}`),
		tmplAngle:  regexp.MustCompile(oldProg + ` <[^>\r\n]+>`),
		tmplDash:   regexp.MustCompile(oldProg + ` --(?:\s+\{[A-Za-z_][A-Za-z0-9_]*\}|\s+-[\p{L}\p{N}_.-]+|[\p{L}\p{N}_%-]+)`),
		exeRe:      regexp.MustCompile(oldProg + `\.exe`),
		pathOutRe:  regexp.MustCompile(`(-o\s+"?[^\s"'\x60]+/)` + oldProg),
		pathBinRe:  regexp.MustCompile(`(\./|\$[A-Za-z_][A-Za-z0-9_]*/|\$\{[A-Za-z_][A-Za-z0-9_]*\}/|bin/|/content/` + oldProg + `/)` + oldProg),
		cmdPkgRe:   regexp.MustCompile(`cmd/` + oldProg),
		cmdsOfRe:   regexp.MustCompile(`commands of ` + oldProg),
		mainSymRe:  regexp.MustCompile(oldProg + `\.(catchStop|TestRepoTRFilesPassCheck|printError|startsWithPrint|formatCommand|debugCommand)`),
		proseCmdRe: regexp.MustCompile("(`" + oldProg + "`|'" + oldProg + "'|" + oldProg + `) (komut|ikili)`),
		specRunOld: "(`" + oldProg + " " + runTR + "`",
		specRunNew: "(`" + newProg + " " + runTR + "`",
	}
}

func isIdentOrDashRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func isPathOrIdentRune(r rune) bool {
	return isIdentOrDashRune(r) || r == '.' || r == '/'
}

func prevRune(s string, pos int) (rune, bool) {
	if pos <= 0 {
		return 0, false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:pos])
	return r, true
}

func nextRune(s string, pos int) (rune, bool) {
	if pos >= len(s) {
		return 0, false
	}
	r, _ := utf8.DecodeRuneInString(s[pos:])
	return r, true
}

func validBinaryEnd(s string, end int) bool {
	if end == len(s) {
		return true
	}
	if strings.HasPrefix(s[end:], "${ext}") {
		return true
	}
	b := s[end]
	return b == ' ' || b == '\t' || b == '\r' || b == '"' || b == '\'' || b == '`' || b == '$' || b == ')'
}
