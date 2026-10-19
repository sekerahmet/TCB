package ast

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// sozcukTipi sözcük birimi alanlarının tipidir (Dump onları atlar).
var sozcukTipi = reflect.TypeOf(lexer.Token{})

// Dump writes a node's meaning (without positions) as text. Tokens (lexer.Token fields) are
// skipped because they carry line/column; every other field is written. If two sources give
// the same Dump output the programs mean the same thing; the formatter uses this for its
// "meaning unchanged" check.
func Dump(n Node) string {
	var sb strings.Builder
	dok(&sb, reflect.ValueOf(n), 0)
	return sb.String()
}

// String returns the program's position-independent dump (see Dump).
func (p *Program) String() string { return Dump(p) }

func dok(sb *strings.Builder, v reflect.Value, girinti int) {
	bosluk := strings.Repeat("  ", girinti)
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			sb.WriteString("∅") // dilden bağımsız boş değer imi (döküm yalnızca karşılaştırılır)
			return
		}
		dok(sb, v.Elem(), girinti)
	case reflect.Struct:
		sb.WriteString(v.Type().Name() + " {\n")
		for i := 0; i < v.NumField(); i++ {
			alan := v.Type().Field(i)
			if alan.Type == sozcukTipi || !alan.IsExported() {
				continue
			}
			sb.WriteString(bosluk + "  " + alan.Name + ": ")
			dok(sb, v.Field(i), girinti+1)
			sb.WriteString("\n")
		}
		sb.WriteString(bosluk + "}")
	case reflect.Slice:
		sb.WriteString("[\n")
		for i := 0; i < v.Len(); i++ {
			sb.WriteString(bosluk + "  ")
			dok(sb, v.Index(i), girinti+1)
			sb.WriteString("\n")
		}
		sb.WriteString(bosluk + "]")
	case reflect.Map:
		anahtarlar := v.MapKeys()
		sort.Slice(anahtarlar, func(i, j int) bool {
			return fmt.Sprint(anahtarlar[i].Interface()) < fmt.Sprint(anahtarlar[j].Interface())
		})
		sb.WriteString("{\n")
		for _, k := range anahtarlar {
			fmt.Fprintf(sb, "%s  %v: ", bosluk, k.Interface())
			dok(sb, v.MapIndex(k), girinti+1)
			sb.WriteString("\n")
		}
		sb.WriteString(bosluk + "}")
	case reflect.String:
		fmt.Fprintf(sb, "%q", v.String())
	default:
		fmt.Fprintf(sb, "%v", v.Interface())
	}
}
