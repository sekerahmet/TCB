package turkish

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

var posRe = regexp.MustCompile(`Satır (\d+)(?:, Sütun (\d+))?`)
var quotedNameRe = regexp.MustCompile(`'([^'\s]+)'`)

// CodeFrame shows the "Satır N, Sütun M" (line, column) position of an error message in
// the source. When the column is known it underlines the faulty part with ^^^, as long as
// the quoted name in the message if that name is at the position. Returns "" when no
// position is found.
//
//	12 | yazdır(toplm + 1)
//	   |        ^^^^^
func CodeFrame(source, message string) string {
	m := posRe.FindStringSubmatch(message)
	if m == nil {
		return ""
	}
	lineNo, _ := strconv.Atoi(m[1])
	col := 0
	if m[2] != "" {
		col, _ = strconv.Atoi(m[2])
	}
	return codeFrameAt(source, lineNo, col, message)
}

// ErrorFrame is CodeFrame for an error: the position is read from the error
// (messages.PosError), not from its text, so it works in every writing language. Errors without
// a position (not yet migrated) fall back to the text.
func ErrorFrame(source string, err error) string {
	if err == nil {
		return ""
	}
	if line, col, ok := messages.Pos(err); ok {
		return codeFrameAt(source, line, col, err.Error())
	}
	return CodeFrame(source, err.Error())
}

// codeFrameAt satırı ve (sütun biliniyorsa) işaretçiyi çizer; message yalnızca altı çizilecek
// tırnaklı adı bulmak için kullanılır.
func codeFrameAt(source string, lineNo, col int, message string) string {
	if source == "" {
		return ""
	}
	line, ok := lineAt(source, lineNo)
	if !ok {
		return ""
	}
	if strings.TrimSpace(line) == "" {
		return ""
	}
	number := strconv.Itoa(lineNo)
	pad := strings.Repeat(" ", len(number))
	var sb strings.Builder
	fmt.Fprintf(&sb, " %s | %s\n", number, line)

	if col == 0 {
		return sb.String()
	}
	runes := []rune(line)
	if col < 1 || col > len(runes)+1 {
		return sb.String()
	}
	length := 1
	if t := quotedNameRe.FindStringSubmatch(message); t != nil {
		name := []rune(t[1])
		if col-1+len(name) <= len(runes) && string(runes[col-1:col-1+len(name)]) == t[1] {
			length = len(name)
		}
	}
	// Sekmeler hizayı bozmasın diye işaretçinin önündeki boşluk kaynak satırdan kopyalanır
	var prefix strings.Builder
	for _, r := range runes[:col-1] {
		if r == '\t' {
			prefix.WriteRune('\t')
		} else {
			prefix.WriteRune(' ')
		}
	}
	fmt.Fprintf(&sb, " %s | %s%s\n", pad, prefix.String(), strings.Repeat("^", length))
	return sb.String()
}

// lineAt returns line n (1-based) of source without its line ending. It scans only up to that
// line and copies nothing, so framing many errors of a large file stays cheap.
func lineAt(source string, n int) (string, bool) {
	if n < 1 {
		return "", false
	}
	for i := 1; i < n; i++ {
		j := strings.IndexByte(source, '\n')
		if j < 0 {
			return "", false
		}
		source = source[j+1:]
	}
	if j := strings.IndexByte(source, '\n'); j >= 0 {
		source = source[:j]
	}
	return strings.TrimRight(source, "\r"), true
}
