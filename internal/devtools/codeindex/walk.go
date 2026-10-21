package codeindex

import (
	"reflect"
	"sort"
)

const astPkg = "github.com/sekerahmet/TCB/internal/lang/ast"

// walkAST bir TR sözdizimi ağacını (ast paketinin işaretçi düğümleri) derinlemesine gezer. Her
// ast düğümü için fn çağrılır, false dönerse çocuklarına inilmez. Düğüm türleri yansıma ile
// gezilir: ast'ye eklenen yeni bir düğüm türü elle eklenmeden de görülür.
func walkAST(root any, fn func(n any) bool) {
	var rec func(v reflect.Value)
	rec = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Interface:
			if !v.IsNil() {
				rec(v.Elem())
			}
		case reflect.Ptr:
			if v.IsNil() {
				return
			}
			if v.Type().Elem().PkgPath() == astPkg && !fn(v.Interface()) {
				return
			}
			rec(v.Elem())
		case reflect.Struct:
			t := v.Type()
			for i := 0; i < v.NumField(); i++ {
				if t.Field(i).IsExported() && t.Field(i).Type.Name() != "Token" {
					rec(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				rec(v.Index(i))
			}
		case reflect.Map:
			keys := v.MapKeys()
			sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
			for _, k := range keys {
				rec(v.MapIndex(k))
			}
		}
	}
	rec(reflect.ValueOf(root))
}
