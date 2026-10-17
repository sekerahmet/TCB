package vm

import (
	"fmt"
)

func GoToTRValue(raw interface{}) Value {
	switch v := raw.(type) {
	case nil:
		return NULL
	case bool:
		return BoolToVal(v)
	case int:
		return NewInt(int64(v))
	case int64:
		return NewInt(v)
	case float64:
		if v == float64(int64(v)) {
			return NewInt(int64(v))
		}
		return &FloatVal{Val: v}
	case string:
		return &StringVal{Val: v}
	case []interface{}:
		elems := make([]Value, len(v))
		for i, el := range v {
			elems[i] = GoToTRValue(el)
		}
		return &ListVal{Elements: elems}
	case map[string]interface{}:
		d := NewDictVal()
		for k, val := range v {
			d.Set(k, GoToTRValue(val))
		}
		return d
	default:
		return &StringVal{Val: fmt.Sprintf("%v", v)}
	}
}

func TRToGoValue(val Value) interface{} {
	switch v := val.(type) {
	case *NullVal:
		return nil
	case *BoolVal:
		return v.Val
	case *IntVal:
		return v.Val
	case *FloatVal:
		return v.Val
	case *StringVal:
		return v.Val
	case *ListVal:
		res := make([]interface{}, len(v.Elements))
		for i, el := range v.Elements {
			res[i] = TRToGoValue(el)
		}
		return res
	case *DictVal:
		res := make(map[string]interface{})
		for k, el := range v.Pairs {
			// JSON/veritabanı anahtarları metin olmak zorundadır; metin olmayan anahtarlar
			// görünen hâlleriyle yazılır (1 -> "1").
			res[v.DisplayKey(k)] = TRToGoValue(el)
		}
		return res
	case *MatrixVal:
		return v.Data
	default:
		return v.Inspect()
	}
}
