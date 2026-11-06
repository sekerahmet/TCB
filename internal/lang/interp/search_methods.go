package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// setupSearchMethods metni satırlarına bölen metodu kaydeder (13.2).
func setupSearchMethods() {
	addMethod(typeString, codes.MemLines, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		var lines []string
		textWork(y, inst, func(s string) { lines = textLines(s) })
		elems := make([]Value, len(lines))
		for i, l := range lines {
			elems[i] = &StringVal{Val: l}
		}
		return &ListVal{Elements: elems}, nil
	})
}

// textLines metnin satırlarıdır (13.2): TR_read_lines'ın kuralı, satır sonu \n ya da \r\n; sondaki satır
// sonundan boş satır çıkmaz, tek \r bölmez. Boş metinde satır yoktur.
func textLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}
