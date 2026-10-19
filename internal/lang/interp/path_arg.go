package interp

import (
	"fmt"
	"strings"
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
func PathArg(fn string, d Value) (yol string, ok bool, err error) {
	switch x := d.(type) {
	case *StringVal:
		yol = x.Val
	case PathValuer:
		yol = x.FilePath()
	default:
		return "", false, nil
	}
	return yol, true, CheckPath(fn, yol)
}

// CheckPath looks for control characters (tab, newline) in a path that may have come from an
// escape sequence; if it finds one it returns an error suggesting a raw string.
func CheckPath(fn, yol string) error {
	if !strings.ContainsAny(yol, "\t\n\r") {
		return nil
	}
	ne := "satır sonu"
	if strings.Contains(yol, "\t") {
		ne = "sekme"
	}
	gorunen := strings.NewReplacer("\t", `\t`, "\n", `\n`, "\r", `\r`).Replace(yol)
	return fmt.Errorf("%s: dosya yolunda %s karakteri var; metindeki ters bölü bir kaçış dizisi sayıldı (\"\\t\" sekme, \"\\n\" satır sonudur). "+
		"Windows yolu için ham metin kullanın: `%s` (ya da ters bölüleri ikileyin veya / yazın)", fn, ne, gorunen)
}
