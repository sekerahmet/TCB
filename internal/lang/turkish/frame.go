package turkish

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

var quotedNameRe = regexp.MustCompile(`'([^'\s]+)'`)

// ErrorFrame shows the source line of an error and, when the column is known, underlines the
// faulty part with ^^^ (as long as the quoted name in the message if that name is at the
// position):
//
//	12 | yazdır(toplm + 1)
//	   |        ^^^^^
//
// The position is read from the error (messages.PosError), never from its text, so it works in
// every writing language and a user text such as "Satır 3 yanlış" is not taken for a position.
// Returns "" when the error has no position.
func ErrorFrame(source string, err error) string {
	if err == nil {
		return ""
	}
	line, col, ok := messages.Pos(err)
	if !ok {
		return ""
	}
	return codeFrameAt(source, line, col, err.Error())
}

// LineFrame shows line n of source without a marker (for a position known only as a line,
// such as the 'aktar' line of a module error). Returns "" when the line is blank or missing.
func LineFrame(source string, n int) string { return codeFrameAt(source, n, 0, "") }

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
