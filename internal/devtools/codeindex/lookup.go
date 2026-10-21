package codeindex

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// positionQuery "dosya.tr:satır[:sütun]" biçimli sorgudur: imlecin altındaki ad.
var positionQuery = regexp.MustCompile(`^(.+\.tr):(\d+)(?::(\d+))?$`)

// fold Türkçe büyük-küçük harf kuralıyla küçültür (İ→i, I→ı): düz ToLower "I"yı "i" yapardı.
func fold(s string) string { return strings.ToLowerSpecial(unicode.TurkishCase, s) }

// hasUpper sorguda büyük harf var mı: yoksa eşleştirme büyük-küçük harfe bakmaz, varsa tam eşleşir
// (`go doc` ile aynı akıllı harf kuralı).
func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// builtinNames kodların her gömülü dildeki adlarını (ve okuyanın dilindeki adı) toplar: bir
// yerleşik, kimin hangi dilde yazdığına bakılmadan bulunur.
func builtinNames(idx *Index) map[string][]string {
	names := map[string][]string{}
	add := func(name, id string) {
		if name != "" {
			names[name] = append(names[name], id)
		}
	}
	var langs []*codes.Language
	for _, code := range codes.Languages() {
		if l, err := codes.LoadLanguage(code); err == nil {
			langs = append(langs, l)
		}
	}
	for _, n := range idx.Nodes {
		if n.Kind != KindBuiltin {
			continue
		}
		k, err := codes.Parse(n.Name)
		if err != nil {
			continue
		}
		add(n.Name, n.ID)
		add(codes.DisplayName(k), n.ID)
		for _, l := range langs {
			add(l.Name(k), n.ID)
		}
	}
	return names
}

// Resolve bir sorgu metnini düğümlere çözer; ilk eşleşme veren basamaktaki bütün düğümleri döner
// (birden çoksa belirsizdir). Basamaklar: "dosya.tr:satır:sütun" konumu; kimlik; modül yolu ya da
// adı (`katman`, `tr/katman`); kanonik ad; modülle nitelenmiş ad (`katman.Doğrusal.ileri`);
// yerleşik adı (her dilde); nitelenmiş adın sonu (`Doğrusal.ileri`, `ileri`). Sorguda büyük harf
// yoksa büyük-küçük harf Türkçe kuralıyla yok sayılır. kind boş değilse yalnızca o tür.
func (r *Reader) Resolve(q, kind string) []*Node {
	if m := positionQuery.FindStringSubmatch(q); m != nil {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		return r.atQuery(m[1], line, col, kind)
	}
	if n := r.byID[q]; n != nil && (kind == "" || string(n.Kind) == kind) {
		return []*Node{n}
	}
	exact := hasUpper(q)
	same := func(a, b string) bool {
		if exact {
			return a == b
		}
		return fold(a) == fold(b)
	}
	suffix := func(full, tail string) bool {
		if exact {
			return strings.HasSuffix(full, "."+tail)
		}
		return strings.HasSuffix(fold(full), "."+fold(tail))
	}
	tiers := []func(n *Node) bool{
		func(n *Node) bool {
			if n.Kind != KindModule {
				return false
			}
			if same(n.Name, q) {
				return true
			}
			for _, a := range n.Aliases {
				if same(a, q) {
					return true
				}
			}
			return false
		},
		func(n *Node) bool { return n.Kind != KindBuiltin && n.Kind != KindModule && same(n.Name, q) },
		func(n *Node) bool { return n.Qualified != "" && same(n.Qualified, q) },
		func(n *Node) bool { return n.Kind == KindBuiltin && r.builtinHas(n.ID, q, same) },
		func(n *Node) bool { return n.Qualified != "" && suffix(n.Qualified, q) },
	}
	for _, match := range tiers {
		var found []*Node
		for i := range r.Index.Nodes {
			n := &r.Index.Nodes[i]
			if (kind == "" || string(n.Kind) == kind) && match(n) {
				found = append(found, n)
			}
		}
		if len(found) > 0 {
			return found
		}
	}
	return nil
}

func (r *Reader) builtinHas(id, q string, same func(a, b string) bool) bool {
	for name, ids := range r.builtins {
		if !same(name, q) {
			continue
		}
		for _, other := range ids {
			if other == id {
				return true
			}
		}
	}
	return false
}

// atQuery konum sorgusunu düğüme çevirir; sütun verilmediyse satırdaki ilk başvuru alınır.
func (r *Reader) atQuery(file string, line, col int, kind string) []*Node {
	var n *Node
	if col > 0 {
		_, n = r.At(file, line, col)
	} else {
		for _, i := range r.byFile[file] {
			if o := r.Index.Occurrences[i]; o.Pos.Line == line {
				n = r.byID[o.Node]
				break
			}
		}
	}
	if n == nil || (kind != "" && string(n.Kind) != kind) {
		return nil
	}
	return []*Node{n}
}

// Suggest bulunamayan bir ada yakın düğümleri verir: adı (ya da nitelenmiş adın son parçası ya da
// bir yerleşik adı) sorguyu içerenler önce, sonra Türkçe harf kuralıyla küçük düzenleme
// uzaklığındakiler.
func (r *Reader) Suggest(q string, limit int) []*Node {
	lq := fold(q)
	maxD := min(4, 1+len([]rune(lq))/4)
	type scored struct {
		n     *Node
		score int
	}
	var found []scored
	for i := range r.Index.Nodes {
		n := &r.Index.Nodes[i]
		cands := []string{n.Name, n.Qualified}
		if j := strings.LastIndexByte(n.Qualified, '.'); j >= 0 {
			cands = append(cands, n.Qualified[j+1:])
		}
		if n.Kind == KindBuiltin {
			for name, ids := range r.builtins {
				for _, id := range ids {
					if id == n.ID {
						cands = append(cands, name)
					}
				}
			}
		}
		best := -1
		for _, c := range cands {
			lc := fold(c)
			if lc == "" {
				continue
			}
			if strings.Contains(lc, lq) {
				best = 0
				break
			}
			if d := editDistance(lq, lc, maxD); d <= maxD && (best < 0 || d < best) {
				best = d
			}
		}
		if best >= 0 {
			found = append(found, scored{n, best})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.score != b.score {
			return a.score < b.score
		}
		if len(a.n.Name) != len(b.n.Name) {
			return len(a.n.Name) < len(b.n.Name)
		}
		return a.n.ID < b.n.ID
	})
	if len(found) > limit {
		found = found[:limit]
	}
	out := make([]*Node, len(found))
	for i, f := range found {
		out[i] = f.n
	}
	return out
}

// editDistance iki metnin Levenshtein uzaklığıdır; bound'u aşacağı belliyse bound+1 verir.
func editDistance(a, b string, bound int) int {
	ra, rb := []rune(a), []rune(b)
	if d := len(ra) - len(rb); d > bound || -d > bound {
		return bound + 1
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			rowMin = min(rowMin, cur[j])
		}
		if rowMin > bound {
			return bound + 1
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
