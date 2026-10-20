package interp

import (
	"fmt"
	"strings"
)

// writeContainer bir kabı (Liste, Sözlük, yapı nesnesi) yazdırır. yol, şu an yazdırılmakta olan
// dış kaplardır: kendini (doğrudan ya da dolaylı) içeren bir kap yeniden görülünce sonsuz
// özyineleme yerine "[...]" / "{...}" / "Ad{...}" yazılır.
func writeContainer(v Value, filePath []Value) string {
	for _, u := range filePath {
		if u == v {
			switch x := v.(type) {
			case *ListVal:
				return "[...]"
			case *StructInstanceVal:
				return x.Def.Name + "{...}"
			}
			return "{...}"
		}
	}
	filePath = append(filePath, v)
	var sb strings.Builder
	switch x := v.(type) {
	case *ListVal:
		sb.WriteByte('[')
		for i, el := range x.Elements {
			if i > 0 {
				sb.WriteString(", ")
			}
			if el.Type() == ValString {
				fmt.Fprintf(&sb, "%q", el.Inspect())
			} else {
				sb.WriteString(writeItem(el, filePath))
			}
		}
		sb.WriteByte(']')
	case *DictVal:
		sb.WriteByte('{')
		for i, k := range x.Order {
			if i > 0 {
				sb.WriteString(", ")
			}
			if kv, ok := x.KeyVals[k]; ok {
				sb.WriteString(kv.Inspect())
			} else {
				fmt.Fprintf(&sb, "%q", k)
			}
			sb.WriteString(": ")
			val := x.Pairs[k]
			if val.Type() == ValString {
				fmt.Fprintf(&sb, "%q", val.Inspect())
			} else {
				sb.WriteString(writeItem(val, filePath))
			}
		}
		sb.WriteByte('}')
	case *StructInstanceVal:
		sb.WriteString(x.Def.Name)
		sb.WriteByte('{')
		first := true
		for _, f := range x.Def.Fields {
			if val, ok := x.Fields[f.Name]; ok {
				if !first {
					sb.WriteString(", ")
				}
				first = false
				sb.WriteString(f.Name)
				sb.WriteString(": ")
				sb.WriteString(writeItem(val, filePath))
			}
		}
		sb.WriteByte('}')
	}
	return sb.String()
}

// writeItem bir kabın elemanını yazar; eleman da bir kapsa döngü denetimi sürer.
func writeItem(v Value, filePath []Value) string {
	switch v.(type) {
	case *ListVal, *DictVal, *StructInstanceVal:
		return writeContainer(v, filePath)
	}
	return v.Inspect()
}
