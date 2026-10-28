package codeindex

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Bağlı bildirimler (deneme 7 R2). Bir parametre aynı adla başka bir çağrıya iletiliyorsa (`f(ad: ad)` ya da konumlu
// `f(..., ad, ...)`, çağrılanın o sıradaki parametresinin adı da `ad` ise) ya da aynı adlı alana atanıyorsa
// (`kendi.ad = ad`), iki bildirim aynı kavramı taşır: biri yeniden adlandırılınca öbürünün de değiştirilmesi önerilir,
// ama zorunlu değildir (zorunlu olan çağrı yerindeki adlı argümandır, o kesin değişikliktir). Bu bağ `forwards`
// kenarıdır; yeniden adlandırma bağlı kümenin bütününü ayrı bir grupta verir.

// forwardArgs çağrıda sahibin parametresini aynı adla ileten argümanları çağrılanın parametresine bağlar: adlı
// argüman `f(ad: ad)` aynı adlı parametreye, konumlu argüman yalnızca çağrılanın aynı sıradaki parametresi aynı adı
// taşıyorsa (`g(b, a)` için `f(a) { g(a, 1) }` ilk parametreye gider, adı `b`dir: bağ yoktur; bağımsız denetim grup 6,
// R2).
func (x *indexer) forwardArgs(f *file, o owner, sc *scope, c *ast.CallExpression) {
	def, conf, called := "", Confidence(""), false
	link := func(v *ast.Identifier, to func() string) {
		p := sc.forwardedParam(v.Value)
		if p == "" {
			return
		}
		if !called {
			def, conf, _ = x.callee(f, o, sc, c)
			called = true
		}
		if t := to(); t != "" && t != p {
			x.addEdge(p, t, EdgeForwards, conf, tokenPos(f, v.Token.Line, v.Token.Column, v.Value))
		}
	}
	for _, a := range c.NamedArgs {
		if v, ok := a.Value.(*ast.Identifier); ok && sameName(v.Value, a.Name) {
			link(v, func() string { return x.calleeParam(def, a.Name) })
		}
	}
	for i, a := range c.Arguments {
		if v, ok := a.(*ast.Identifier); ok {
			link(v, func() string {
				if t := x.calleeParamAt(def, i); t != "" && sameName(shortName(x.nodes[t]), v.Value) {
					return t
				}
				return ""
			})
		}
	}
}

// sameName iki ad aynı kavramın adı mı: aynı yazılmış ya da aynı İngilizce adlı iki kimlik (kimlik TR_<sınıf>_<İngilizce
// ad> kuralıyla verilir, 17.2): alan TR_mem_num_heads ile parametre TR_arg_num_heads, her dilde aynı adı taşır.
func sameName(a, b string) bool {
	if a == b {
		return true
	}
	ka, errA := codes.Parse(a)
	kb, errB := codes.Parse(b)
	return errA == nil && errB == nil && strings.TrimPrefix(a, codes.IdentityPrefix+string(ka.Class())+"_") ==
		strings.TrimPrefix(b, codes.IdentityPrefix+string(kb.Class())+"_")
}

// forwardedParam name gövdede yeniden bağlanmamış bir parametre ise onun düğümüdür, değilse "": döngü değişkeni ya da
// yeniden tanım parametreyi gölgeler, o ad artık parametre değildir. İletim ve alana atama aynı kuralı kullanır.
func (sc *scope) forwardedParam(name string) string {
	if sc.bindCount[name] > 0 {
		return ""
	}
	return sc.params[name]
}

// storeParam `kendi.ad = ad` atamasında parametreyi aynı adlı alana bağlar.
func (x *indexer) storeParam(f *file, o owner, sc *scope, s *ast.AssignStatement) {
	m, ok := s.Target.(*ast.MemberExpression)
	v, ok2 := s.Value.(*ast.Identifier)
	if !ok || !ok2 || !sameName(m.Property, v.Value) || sc.forwardedParam(v.Value) == "" {
		return
	}
	structID, conf, basis := x.receiver(f, o, sc, m.Object)
	if basis != BasisSelf {
		return
	}
	if fid, kind := x.structMember(structID, m.Property); kind == KindField {
		x.addEdge(sc.params[v.Value], fid, EdgeForwards, conf, tokenPos(f, v.Token.Line, v.Token.Column, v.Value))
	}
}

// linkedTo düğümün `forwards` bağlarıyla (iki yönde, geçişli) ulaşılan bildirimleridir, düğümün kendisi hariç; kimliğe
// göre sıralı.
func (r *Reader) linkedTo(id string) []*Node {
	adj := map[string][]string{}
	for _, e := range r.Index.Edges {
		if e.Kind == EdgeForwards {
			adj[e.From] = append(adj[e.From], e.To)
			adj[e.To] = append(adj[e.To], e.From)
		}
	}
	seen := map[string]bool{id: true}
	queue := []string{id}
	var out []*Node
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
				if n := r.byID[next]; n != nil {
					out = append(out, n)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// addLinked hedeflerin bağlı bildirimlerini sonuca ekler (hedefler hariç, her biri bir kez) ve bildirim yerlerini metin
// taramasından çıkarır: onlar bu gruptadır.
func (r *Reader) addLinked(res *Result, targets []*Node, skip map[string]bool) {
	isTarget := map[string]bool{}
	for _, n := range targets {
		isTarget[n.ID] = true
	}
	for _, n := range targets {
		for _, l := range r.linkedTo(n.ID) {
			if isTarget[l.ID] || slices.ContainsFunc(res.Linked, func(m Node) bool { return m.ID == l.ID }) {
				continue
			}
			res.Linked = append(res.Linked, r.display(*l))
			if l.Pos != nil {
				skip[l.Pos.File+":"+strconv.Itoa(l.Pos.Line)+":"+strconv.Itoa(l.Pos.Col)] = true
			}
		}
	}
}

// textChange bir metin eşleşmesini olası değişikliğe çevirir. Sözlük anahtarı bir metindir (3.7:
// {"baş": baş}): metin sabiti olarak olası grupta durur, kendiliğinden değişmez.
func textChange(h TextHit) Change {
	return Change{Pos: h.Pos, Where: h.Where, Match: h.Match, Word: h.Word, Context: h.Context, Replacement: h.Suggestion, Check: h.Check}
}
