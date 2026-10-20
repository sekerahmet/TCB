package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// PathValuer is an external value carrying a file path (later the standard library's Yol
// type). Builtins that take paths read the argument with PathArg, so Metin and PathValuer are
// accepted the same way everywhere and builtins need no change when a new path type arrives.
type PathValuer interface {
	// FilePath is the path the value points to (unresolved; may be relative, see ResolvePath).
	FilePath() string
}

// PathArg reads the argument of a builtin that takes a path: Metin or a PathValuer. If the
// value is not a path it returns ok=false (the caller gives its own type error). If the path
// contains a tab or a newline it returns an explanatory error: these almost never occur in file
// names and usually mean '\t' was taken as an escape in a Windows path like
// "C:\tokenizer.json".
func PathArg(fn string, d Value) (filePath string, ok bool, err error) {
	switch x := d.(type) {
	case *StringVal:
		filePath = x.Val
	case PathValuer:
		filePath = x.FilePath()
	default:
		return "", false, nil
	}
	return filePath, true, CheckPath(fn, filePath)
}

// CheckPath looks for control characters (tab, newline) in a path that may have come from an
// escape sequence; if it finds one it returns an error suggesting a raw string.
func CheckPath(fn, filePath string) error {
	if !strings.ContainsAny(filePath, "\t\n\r") {
		return nil
	}
	ne := messages.Text(messages.H2487)
	if strings.Contains(filePath, "\t") {
		ne = messages.Text(messages.H2488)
	}
	visible := strings.NewReplacer("\t", `\t`, "\n", `\n`, "\r", `\r`).Replace(filePath)
	return messages.New(messages.H2489, fn, ne, visible)
}
