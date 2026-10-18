package ayikla

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"tr_dili/internal/dil/yurut"
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
		cur.EachLocal(func(k string, v yurut.Value) {
			if seen[k] {
				return
			}
			seen[k] = true
			if v.Type() == yurut.VAL_BUILTIN || v.Type() == yurut.VAL_STRUCT || v.Type() == yurut.VAL_FUNC {
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
	val        yurut.Value
	not        string
}

// yinelemeSiniri, gezilebilir dış değerlerde (ör. tensör) panelde gösterilen en fazla öğe
// sayısıdır; fazlası tek bir "…" satırıyla özetlenir.
const yinelemeSiniri = 1000

// children açılabilir bir değerin alt elemanlarını döndürür.
func children(val yurut.Value, expr string) []dapChild {
	const limit = 5000
	var out []dapChild
	switch v := val.(type) {
	case yurut.Yinelenebilir:
		// Dış değerler (ör. tensör): ilk eksen boyunca öğeler (tensörde 1 boyutluda sayılar,
		// N boyutluda (N-1) boyutlu görünümler); çok büyük değerlerde ilk öğeler ve "…".
		n, err := v.YinelemeUzunlugu()
		if err != nil {
			break
		}
		for i := 0; i < n && i < yinelemeSiniri; i++ {
			el, err := v.YinelemeOgesi(i)
			if err != nil {
				break
			}
			out = append(out, dapChild{name: fmt.Sprintf("[%d]", i), expr: fmt.Sprintf("%s[%d]", expr, i), val: el})
		}
		if n > yinelemeSiniri {
			out = append(out, dapChild{name: "…", not: fmt.Sprintf("ilk %d öğe gösteriliyor (toplam %d)", yinelemeSiniri, n)})
		}
	case *yurut.ListVal:
		for i, el := range v.Elements {
			if i >= limit {
				break
			}
			out = append(out, dapChild{name: fmt.Sprintf("[%d]", i), expr: fmt.Sprintf("%s[%d]", expr, i), val: el})
		}
	case *yurut.DictVal:
		for _, k := range v.Order {
			el, ok := v.Pairs[k]
			if !ok {
				continue
			}
			key := v.KeyValue(k)
			keyExpr := key.Inspect()
			if sv, isStr := key.(*yurut.StringVal); isStr {
				keyExpr = trStringLiteral(sv.Val)
			}
			out = append(out, dapChild{name: v.DisplayKey(k), expr: expr + "[" + keyExpr + "]", val: el})
			if len(out) >= limit {
				break
			}
		}
	case *yurut.StructInstanceVal:
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
func expandable(val yurut.Value) bool {
	switch v := val.(type) {
	case *yurut.ListVal:
		return len(v.Elements) > 0
	case *yurut.DictVal:
		return len(v.Pairs) > 0
	case *yurut.StructInstanceVal:
		return len(v.Fields) > 0
	case yurut.Yinelenebilir:
		n, err := v.YinelemeUzunlugu()
		return err == nil && n > 0
	}
	return false
}
func (s *dapServer) refFor(val yurut.Value, expr string) int {
	if val == nil || !expandable(val) {
		return 0
	}
	return s.newHandle(&dapHandle{value: val, expr: expr})
}

// display değişkenin yanında gösterilecek kısa metindir.
func display(val yurut.Value) string {
	if val == nil {
		return "yok"
	}
	if iz, ok := val.(IzlemeDegeri); ok {
		// Çok satırlı yazdırma biçimi yerine kısa özet gösterilir.
		return izlemeGosterimi(val, iz)
	}
	var prefix string
	switch v := val.(type) {
	case *yurut.ListVal:
		prefix = fmt.Sprintf("(%d) ", len(v.Elements))
	case *yurut.DictVal:
		prefix = fmt.Sprintf("{%d} ", len(v.Pairs))
	}
	text := prefix + val.Inspect()
	if r := []rune(text); len(r) > 200 {
		text = string(r[:200]) + "…"
	}
	return text
}
func (s *dapServer) variableEntry(name, expr string, val yurut.Value) map[string]interface{} {
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
					vars = append(vars, map[string]interface{}{"name": c.name, "value": c.not, "variablesReference": 0})
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
			case *yurut.ListVal:
				idx, _ := strconv.Atoi(strings.Trim(a.Name, "[]"))
				container.Elements[idx] = val
			case *yurut.DictVal:
				for _, k := range container.Order {
					if container.DisplayKey(k) == a.Name {
						container.Pairs[k] = val
					}
				}
			case *yurut.StructInstanceVal:
				container.Fields[a.Name] = val
			case yurut.IndeksAtanabilir:
				// Dış değerler (ör. tensör satırı ya da elemanı): değer[i] = yeni değer
				idx, _ := strconv.Atoi(strings.Trim(a.Name, "[]"))
				if err := container.IndekseAta(yurut.NewInt(int64(idx)), val); err != nil {
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
