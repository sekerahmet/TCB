package turkish

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
	if m == nil || source == "" {
		return ""
	}
	lineNo, _ := strconv.Atoi(m[1])
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	if lineNo < 1 || lineNo > len(lines) {
		return ""
	}
	line := strings.TrimRight(lines[lineNo-1], "\r")
	if strings.TrimSpace(line) == "" {
		return ""
	}
	number := strconv.Itoa(lineNo)
	pad := strings.Repeat(" ", len(number))
	var sb strings.Builder
	fmt.Fprintf(&sb, " %s | %s\n", number, line)

	if m[2] == "" {
		return sb.String()
	}
	col, _ := strconv.Atoi(m[2])
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
