package codeindex

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// Bağlı bildirimler (deneme 7 R2). Bir parametre aynı adla başka bir çağrıya iletiliyorsa (`f(ad: ad)` ya da konumlu
// `f(..., ad, ...)`, çağrılanın aynı adlı parametresi varsa) ya da aynı adlı alana atanıyorsa (`kendi.ad = ad`), iki
// bildirim aynı kavramı taşır: biri yeniden adlandırılınca öbürünün de değiştirilmesi önerilir, ama zorunlu değildir
// (zorunlu olan çağrı yerindeki adlı argümandır, o kesin değişikliktir). Bu bağ `forwards` kenarıdır; yeniden
// adlandırma bağlı kümenin bütününü ayrı bir grupta verir.

// forwardArgs çağrıda sahibin parametresini aynı adla ileten argümanları çağrılanın parametresine bağlar.
func (x *indexer) forwardArgs(f *file, o owner, sc *scope, c *ast.CallExpression) {
	forward := func(v *ast.Identifier, name string) {
		p := sc.params[name]
		if p == "" || v.Value != name || sc.bindCount[name] > 0 {
			return // parametre değil ya da gövdede yeniden bağlanmış
		}
		to, conf, _ := x.namedArgTarget(f, o, sc, c, name)
		if to != "" && to != p {
			x.addEdge(p, to, EdgeForwards, conf, tokenPos(f, v.Token.Line, v.Token.Column, v.Value))
		}
	}
	for _, a := range c.NamedArgs {
		if v, ok := a.Value.(*ast.Identifier); ok {
			forward(v, a.Name)
		}
	}
	for _, a := range c.Arguments {
		if v, ok := a.(*ast.Identifier); ok {
			forward(v, v.Value) // konumlu: çağrılanın aynı adlı parametresine
		}
	}
}

// storeParam `kendi.ad = ad` atamasında parametreyi aynı adlı alana bağlar.
func (x *indexer) storeParam(f *file, o owner, sc *scope, s *ast.AssignStatement) {
	m, ok := s.Target.(*ast.MemberExpression)
	v, ok2 := s.Value.(*ast.Identifier)
	if !ok || !ok2 || m.Property != v.Value || sc.params[v.Value] == "" {
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

// Sözlük anahtarı (deneme 7 R3). `kendi.ayar = {..., bakış_sayısı: bakış_sayısı, ...}` anahtarı bir ad değil, verinin
// biçimidir (kaydedilen ayar dosyası onu okur): yeniden adlandırılan parametreyle aynı yazılsa da değiştirilmesi bir veri
// biçimi kararıdır. Metin eşleşmesi böyle bir anahtarsa ayrı rol ve eşleşme türüyle (sözlük anahtarı; veri biçimi,
// kendiliğinden değişmez) olası grupta kalır.
const RoleDictKey Role = "dict_key" // sözlük değişmezinin tırnaksız anahtarı

const MatchDataFormat = "data_format" // anahtar verinin biçimidir: kendiliğinden değişmez

// lines dosyanın Türkçe gösterim satırlarıdır (yoksa nil).
func (s *lineSource) lines(file string) []string {
	s.at(Pos{File: file, Line: 1})
	return s.cache[file]
}

// isDictKey p'de başlayan word bir sözlük değişmezinin anahtarı mı: ardından ':' gelir (':=' değil) ve onu saran en
// yakın açık parantez '{'dir. Çağrının adlı argümanı da `ad:` yazılır ama '(' içindedir; dizindeki adlı argümanlar
// zaten kesin değişikliktir, metin taramasına gelmez.
func isDictKey(lines []string, p Pos, word string) bool {
	if p.Line < 1 || p.Line > len(lines) || p.Col < 1 {
		return false
	}
	row := []rune(lines[p.Line-1])
	end := p.Col - 1 + len([]rune(word))
	if end > len(row) {
		return false
	}
	rest := strings.TrimLeft(string(row[end:]), " \t")
	if !strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, ":=") {
		return false
	}
	depth := 0 // geriye doğru kapanmış parantezler
	for l := p.Line - 1; l >= 0 && l >= p.Line-40; l-- {
		r := []rune(lines[l])
		from := len(r) - 1
		if l == p.Line-1 {
			from = p.Col - 2
		}
		for i := from; i >= 0; i-- {
			switch r[i] {
			case ')', '}', ']':
				depth++
			case '(', '[':
				if depth == 0 {
					return false
				}
				depth--
			case '{':
				if depth == 0 {
					return true
				}
				depth--
			}
		}
	}
	return false
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

// textChange bir metin eşleşmesini olası değişikliğe çevirir; TR kodundaki sözlük anahtarı kendi rolünü alır.
func textChange(h TextHit, lines *lineSource) Change {
	ch := Change{Pos: h.Pos, Where: h.Where, Match: h.Match, Word: h.Word, Context: h.Context, Replacement: h.Suggestion, Check: h.Check}
	if strings.HasSuffix(h.Pos.File, ".tr") && h.Where == WhereCode && isDictKey(lines.lines(h.Pos.File), h.Pos, h.Word) {
		ch.Role, ch.Match = RoleDictKey, MatchDataFormat
	}
	return ch
}
