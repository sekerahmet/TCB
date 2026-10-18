package vm

import (
	"fmt"
	"strings"
)

// kapYaz bir kabı (Liste, Sözlük, yapı nesnesi) yazdırır. yol, şu an yazdırılmakta olan
// dış kaplardır: kendini (doğrudan ya da dolaylı) içeren bir kap yeniden görülünce sonsuz
// özyineleme yerine "[...]" / "{...}" / "Ad{...}" yazılır.
func kapYaz(v Value, yol []Value) string {
	for _, u := range yol {
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
	yol = append(yol, v)
	var sb strings.Builder
	switch x := v.(type) {
	case *ListVal:
		sb.WriteByte('[')
		for i, el := range x.Elements {
			if i > 0 {
				sb.WriteString(", ")
			}
			if el.Type() == VAL_STRING {
				fmt.Fprintf(&sb, "%q", el.Inspect())
			} else {
				sb.WriteString(ogeYaz(el, yol))
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
			if val.Type() == VAL_STRING {
				fmt.Fprintf(&sb, "%q", val.Inspect())
			} else {
				sb.WriteString(ogeYaz(val, yol))
			}
		}
		sb.WriteByte('}')
	case *StructInstanceVal:
		sb.WriteString(x.Def.Name)
		sb.WriteByte('{')
		ilk := true
		for _, f := range x.Def.Fields {
			if val, ok := x.Fields[f.Name]; ok {
				if !ilk {
					sb.WriteString(", ")
				}
				ilk = false
				sb.WriteString(f.Name)
				sb.WriteString(": ")
				sb.WriteString(ogeYaz(val, yol))
			}
		}
		sb.WriteByte('}')
	}
	return sb.String()
}

// ogeYaz bir kabın elemanını yazar; eleman da bir kapsa döngü denetimi sürer.
func ogeYaz(v Value, yol []Value) string {
	switch v.(type) {
	case *ListVal, *DictVal, *StructInstanceVal:
		return kapYaz(v, yol)
	}
	return v.Inspect()
}
