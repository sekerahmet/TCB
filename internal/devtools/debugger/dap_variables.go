package debugger

import (
	"encoding/json"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"sort"
	"strconv"
	"strings"
)

func (s *dapServer) newHandle(h *dapHandle) int {
	s.nextHandle++
	s.handles[s.nextHandle] = h
	return s.nextHandle
}
func (s *dapServer) scopes(msg *dapMessage) {
	var a struct {
		FrameID int `json:"frameId"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	var scopes []interface{}
	if a.FrameID <= 1 && s.curEnv != nil && s.curEnv != s.curVM.GlobalEnv {
		scopes = append(scopes, map[string]interface{}{
			"name": "Yerel", "presentationHint": "locals", "expensive": false,
			"variablesReference": s.newHandle(&dapHandle{scope: s.curEnv}),
		})
	}
	scopes = append(scopes, map[string]interface{}{
		"name": "Genel", "expensive": false,
		"variablesReference": s.newHandle(&dapHandle{scope: s.curVM.GlobalEnv, isGlobal: true}),
	})
	s.respond(msg, map[string]interface{}{"scopes": scopes})
}

// scopeVariables kapsamdaki değişkenleri (yerleşik fonksiyon ve tanımlar hariç) sıralı döndürür.
func (s *dapServer) scopeVariables(h *dapHandle) []varEntry {
	global := s.curVM.GlobalEnv
	seen := map[string]bool{}
	var list []varEntry
	for cur := h.scope; cur != nil; cur = cur.Outer() {
		if !h.isGlobal && cur == global {
			break
		}
		cur.EachLocal(func(k string, v interp.Value) {
			if seen[k] {
				return
			}
			seen[k] = true
			if v.Type() == interp.VAL_BUILTIN || v.Type() == interp.VAL_STRUCT || v.Type() == interp.VAL_FUNC || v.Type() == interp.VAL_TYPE ||
				s.curVM.IsBuiltinConstant(k, v) {
				return
			}
			list = append(list, varEntry{Name: k, Val: v})
		})
		if h.isGlobal {
			break
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// dapChild açılmış bir değerin alt öğesidir. val nil ise öğe yalnızca bilgi satırıdır
// (ör. kısaltılmış tensörün sonundaki "…"); değeri not'tur ve açılamaz.
type dapChild struct {
	name, expr string
	val        interp.Value
	note       string
}

// repeatLimit, gezilebilir dış değerlerde (ör. tensör) panelde gösterilen en fazla öğe
// sayısıdır; fazlası tek bir "…" satırıyla özetlenir.
const repeatLimit = 1000

// children açılabilir bir değerin alt elemanlarını döndürür.
func children(val interp.Value, expr string) []dapChild {
	const limit = 5000
	var out []dapChild
	switch v := val.(type) {
	case interp.Iterable:
		// Dış değerler (ör. tensör): ilk eksen boyunca öğeler (tensörde 1 boyutluda sayılar,
		// N boyutluda (N-1) boyutlu görünümler); çok büyük değerlerde ilk öğeler ve "…".
		n, err := v.IterLen()
		if err != nil {
			break
		}
		for i := 0; i < n && i < repeatLimit; i++ {
			el, err := v.IterItem(i)
			if err != nil {
				break
			}
			out = append(out, dapChild{name: fmt.Sprintf("[%d]", i), expr: fmt.Sprintf("%s[%d]", expr, i), val: el})
		}
		if n > repeatLimit {
			out = append(out, dapChild{name: "…", note: fmt.Sprintf("ilk %d öğe gösteriliyor (toplam %d)", repeatLimit, n)})
		}
	case *interp.ListVal:
		for i, el := range v.Elements {
			if i >= limit {
				break
			}
			out = append(out, dapChild{name: fmt.Sprintf("[%d]", i), expr: fmt.Sprintf("%s[%d]", expr, i), val: el})
		}
	case *interp.DictVal:
		for _, k := range v.Order {
			el, ok := v.Pairs[k]
			if !ok {
				continue
			}
			key := v.KeyValue(k)
			keyExpr := key.Inspect()
			if sv, isStr := key.(*interp.StringVal); isStr {
				keyExpr = trStringLiteral(sv.Val)
			}
			out = append(out, dapChild{name: v.DisplayKey(k), expr: expr + "[" + keyExpr + "]", val: el})
			if len(out) >= limit {
				break
			}
		}
	case *interp.StructInstanceVal:
		seen := map[string]bool{}
		for _, f := range v.Def.Fields {
			if fv, ok := v.Fields[f.Name]; ok {
				seen[f.Name] = true
				out = append(out, dapChild{name: f.Name, expr: expr + "." + f.Name, val: fv})
			}
		}
		var rest []string
		for k := range v.Fields {
			if !seen[k] {
				rest = append(rest, k)
			}
		}
		sort.Strings(rest)
		for _, k := range rest {
			out = append(out, dapChild{name: k, expr: expr + "." + k, val: v.Fields[k]})
		}
	}
	return out
}
func expandable(val interp.Value) bool {
	switch v := val.(type) {
	case *interp.ListVal:
		return len(v.Elements) > 0
	case *interp.DictVal:
		return len(v.Pairs) > 0
	case *interp.StructInstanceVal:
		return len(v.Fields) > 0
	case interp.Iterable:
		n, err := v.IterLen()
		return err == nil && n > 0
	}
	return false
}
func (s *dapServer) refFor(val interp.Value, expr string) int {
	if val == nil || !expandable(val) {
		return 0
	}
	return s.newHandle(&dapHandle{value: val, expr: expr})
}

// display değişkenin yanında gösterilecek kısa metindir.
func display(val interp.Value) string {
	if val == nil {
		return interp.NULL.Inspect()
	}
	if iz, ok := val.(WatchValue); ok {
		// Çok satırlı yazdırma biçimi yerine kısa özet gösterilir.
		return watchDisplay(val, iz)
	}
	var prefix string
	switch v := val.(type) {
	case *interp.ListVal:
		prefix = fmt.Sprintf("(%d) ", len(v.Elements))
	case *interp.DictVal:
		prefix = fmt.Sprintf("{%d} ", len(v.Pairs))
	}
	text := prefix + val.Inspect()
	if r := []rune(text); len(r) > 200 {
		text = string(r[:200]) + "…"
	}
	return text
}
func (s *dapServer) variableEntry(name, expr string, val interp.Value) map[string]interface{} {
	return map[string]interface{}{
		"name": name, "value": display(val), "type": string(val.Type()),
		"evaluateName": expr, "variablesReference": s.refFor(val, expr),
	}
}
func (s *dapServer) variables(msg *dapMessage) {
	var a struct {
		VariablesReference int `json:"variablesReference"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	h := s.handles[a.VariablesReference]
	vars := []interface{}{}
	if h != nil {
		if h.value == nil {
			for _, e := range s.scopeVariables(h) {
				vars = append(vars, s.variableEntry(e.Name, e.Name, e.Val))
			}
		} else {
			for _, c := range children(h.value, h.expr) {
				if c.val == nil {
					vars = append(vars, map[string]interface{}{"name": c.name, "value": c.note, "variablesReference": 0})
					continue
				}
				vars = append(vars, s.variableEntry(c.name, c.expr, c.val))
			}
		}
	}
	s.respond(msg, map[string]interface{}{"variables": vars})
}

// setVariable ✏️ "değeri değiştir": yeni değer bir TR ifadesi olarak hesaplanır.
func (s *dapServer) setVariable(msg *dapMessage) {
	var a struct {
		VariablesReference int    `json:"variablesReference"`
		Name               string `json:"name"`
		Value              string `json:"value"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	h := s.handles[a.VariablesReference]
	if h == nil {
		s.respondErr(msg, "Değişken bulunamadı")
		return
	}
	val, err := s.evalQuiet(s.curVM, s.curEnv, a.Value)
	if err != nil {
		s.respondErr(msg, "Değer hesaplanamadı: %v", err)
		return
	}
	expr := a.Name
	switch {
	case h.value == nil:
		if err := h.scope.Assign(a.Name, val); err != nil {
			s.respondErr(msg, "%v", err)
			return
		}
	default:
		found := false
		for _, c := range children(h.value, h.expr) {
			if c.name != a.Name || c.val == nil {
				continue
			}
			found, expr = true, c.expr
			switch container := h.value.(type) {
			case *interp.ListVal:
				idx, _ := strconv.Atoi(strings.Trim(a.Name, "[]"))
				container.Elements[idx] = val
			case *interp.DictVal:
				for _, k := range container.Order {
					if container.DisplayKey(k) == a.Name {
						container.Pairs[k] = val
					}
				}
			case *interp.StructInstanceVal:
				container.Fields[a.Name] = val
			case interp.IndexAssignable:
				// Dış değerler (ör. tensör satırı ya da elemanı): değer[i] = yeni değer
				idx, _ := strconv.Atoi(strings.Trim(a.Name, "[]"))
				if err := container.SetIndex(interp.NewInt(int64(idx)), val); err != nil {
					s.respondErr(msg, "%v", err)
					return
				}
			default:
				s.respondErr(msg, "Bu değerin elemanları değiştirilemez")
				return
			}
		}
		if !found {
			s.respondErr(msg, "Eleman bulunamadı: %s", a.Name)
			return
		}
	}
	s.respond(msg, map[string]interface{}{
		"value": display(val), "type": string(val.Type()), "variablesReference": s.refFor(val, expr),
	})
}
