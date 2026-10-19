package ast

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// tokenType sözcük birimi alanlarının tipidir (Dump onları atlar).
var tokenType = reflect.TypeOf(lexer.Token{})

// Dump writes a node's meaning (without positions) as text. Tokens (lexer.Token fields) are
// skipped because they carry line/column; every other field is written. If two sources give
// the same Dump output the programs mean the same thing; the formatter uses this for its
// "meaning unchanged" check.
func Dump(n Node) string {
	var sb strings.Builder
	dump(&sb, reflect.ValueOf(n), 0)
	return sb.String()
}

// String returns the program's position-independent dump (see Dump).
func (p *Program) String() string { return Dump(p) }

func dump(sb *strings.Builder, v reflect.Value, indent int) {
	pad := strings.Repeat("  ", indent)
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			sb.WriteString("∅") // dilden bağımsız boş değer imi (döküm yalnızca karşılaştırılır)
			return
		}
		dump(sb, v.Elem(), indent)
	case reflect.Struct:
		sb.WriteString(v.Type().Name() + " {\n")
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if field.Type == tokenType || !field.IsExported() {
				continue
			}
			sb.WriteString(pad + "  " + field.Name + ": ")
			dump(sb, v.Field(i), indent+1)
			sb.WriteString("\n")
		}
		sb.WriteString(pad + "}")
	case reflect.Slice:
		sb.WriteString("[\n")
		for i := 0; i < v.Len(); i++ {
			sb.WriteString(pad + "  ")
			dump(sb, v.Index(i), indent+1)
			sb.WriteString("\n")
		}
		sb.WriteString(pad + "]")
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
		})
		sb.WriteString("{\n")
		for _, k := range keys {
			fmt.Fprintf(sb, "%s  %v: ", pad, k.Interface())
			dump(sb, v.MapIndex(k), indent+1)
			sb.WriteString("\n")
		}
		sb.WriteString(pad + "}")
	case reflect.String:
		fmt.Fprintf(sb, "%q", v.String())
	default:
		fmt.Fprintf(sb, "%v", v.Interface())
	}
}
