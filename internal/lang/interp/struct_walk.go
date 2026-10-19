package interp

import "sort"

// EachField passes the fields of a struct instance to fn in a fixed order: first the fields of
// the struct definition in definition order, then fields not in the definition (only Go code
// can add those) in name order. TR code cannot assign to a field outside the definition (7.5).
// Engines (e.g. the builtin that collects parameters) walk objects in this order; it does not
// depend on map iteration order.
func (si *StructInstanceVal) EachField(fn func(name string, value Value)) {
	defined := make(map[string]bool, len(si.Def.Fields))
	for _, f := range si.Def.Fields {
		defined[f.Name] = true
		if v, ok := si.Fields[f.Name]; ok {
			fn(f.Name, v)
		}
	}
	var extra []string
	for name := range si.Fields {
		if !defined[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		fn(name, si.Fields[name])
	}
}
