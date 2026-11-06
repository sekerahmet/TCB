package interp

import (
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// setupSearchMethods metni satırlarına bölen ve metinde ya da listede yer bulan metotları kaydeder (13.2, 13.3).
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
	addMethod(typeString, codes.MemIndexOf, signature{1, 1, []*argKind{textArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		part, at := y.textOf(args[0]), -1
		textWork(y, inst, func(s string) {
			if i := strings.Index(s, part); i >= 0 {
				at = utf8.RuneCountInString(s[:i]) // konum kod noktasıdır, bayt değil (3.3)
			}
		})
		if at < 0 {
			return Null, nil
		}
		return NewInt(int64(at)), nil
	})
	addMethod(typeList, codes.MemIndexOf, signature{1, 1, nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		for i, el := range inst.(*ListVal).Elements {
			eq, err := EqualValues(el, args[0]) // .TR_contains'in eşitliği (4.5)
			if err != nil {
				return nil, err
			}
			if eq {
				return NewInt(int64(i)), nil
			}
		}
		return Null, nil
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
