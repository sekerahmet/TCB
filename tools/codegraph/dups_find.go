package main

import (
	"sort"
	"strconv"
)

// Bu dosya tekrar adaylarını bulur. Yöntem: düğüm dizisinde uzunluğu minNodes olan her pencerenin
// katlanan (Karp-Rabin) özeti hesaplanır, özeti eşit pencereler gerçekten eşit mi diye dizilerle
// karşılaştırılır; sola doğru genişleyemeyen (kendinden önceki düğüm de eşleşmeyen) ve aynı
// dosyada çakışmayan eşleşmeler sağa doğru en uzun eşleşmeye kadar uzatılır. Aynı yerde başlayan
// eşleşmeler birleştirilerek üç ve daha çok konumlu kümeler kurulur; daha büyük bir kümenin
// içinde kalan küçük kümeler atılır. Gerekçe ve dupl/CPD karşılaştırması README'dedir.

// dupLoc bir tekrar konumudur.
type dupLoc struct {
	File     string `json:"file"`
	From     int    `json:"from"` // başlangıç satırı
	To       int    `json:"to"`   // bitiş satırı
	Func     string `json:"func,omitempty"`
	Nodes    int    `json:"nodes"`
	Changed  bool   `json:"changed,omitempty"` // --changed: konum değişen satırlara dokunuyor
	start    int    // düğüm dizisinde başlangıç
	fileIdx  int
	lineFrom int
}

// dupCluster birbirinin tekrarı olan konumlardır.
type dupCluster struct {
	Nodes int      `json:"nodes"` // en kısa konumun düğüm sayısı
	Locs  []dupLoc `json:"locations"`
	Diff  dupDiff  `json:"diff"`
}

// maxGroup bir özet grubunda en çok bakılan konum sayısıdır; fazlası kalıp koddur (örneğin boş
// gövdeler) ve çiftler üretilmez.
const maxGroup = 60

// pairMatch iki konumun en uzun eşleşmesidir.
type pairMatch struct{ a, b, length int }

// isSentinel dizideki değerin dosya ayracı olup olmadığı.
func isSentinel(v uint32) bool { return v >= sentinelBase }

// windowHashes her pencerenin özetini verir; ayraç içeren pencerelerin özeti 0'dır (kullanılmaz).
func (co *dupCorpus) windowHashes(w int) []uint64 {
	const base = 1099511628211
	n := len(co.seq)
	if n < w {
		return nil
	}
	hashes := make([]uint64, n-w+1)
	var pow uint64 = 1
	for i := 0; i < w-1; i++ {
		pow *= base
	}
	var h uint64
	sentinels := 0
	for i := 0; i < n; i++ {
		h = h*base + uint64(co.seq[i])
		if isSentinel(co.seq[i]) {
			sentinels++
		}
		if i >= w {
			old := co.seq[i-w]
			h -= uint64(old) * pow * base
			if isSentinel(old) {
				sentinels--
			}
		}
		if i >= w-1 {
			if sentinels == 0 {
				hashes[i-w+1] = h
			}
		}
	}
	return hashes
}

// equalWindows iki konumdaki w düğümlük pencereler eşit mi.
func (co *dupCorpus) equalWindows(a, b, w int) bool {
	for k := 0; k < w; k++ {
		if co.seq[a+k] != co.seq[b+k] {
			return false
		}
	}
	return true
}

// extend a ve b'den sağa doğru eşleşen düğüm sayısıdır (ayraçta durur).
func (co *dupCorpus) extend(a, b int) int {
	n := 0
	for a+n < len(co.seq) && b+n < len(co.seq) && co.seq[a+n] == co.seq[b+n] && !isSentinel(co.seq[a+n]) {
		n++
	}
	return n
}

// matches bütün sola genişlemeyen eşleşmeleri verir; skipped, kalıp kod diye atlanan grup sayısıdır.
func (co *dupCorpus) matches(w int) (pairs []pairMatch, skipped int) {
	hashes := co.windowHashes(w)
	type entry struct {
		h   uint64
		pos int32
	}
	var list []entry
	for i, h := range hashes {
		if h != 0 {
			list = append(list, entry{h, int32(i)})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].h != list[j].h {
			return list[i].h < list[j].h
		}
		return list[i].pos < list[j].pos
	})
	for i := 0; i < len(list); {
		j := i
		for j < len(list) && list[j].h == list[i].h {
			j++
		}
		if j-i >= 2 {
			group := make([]int, 0, j-i)
			for _, e := range list[i:j] {
				group = append(group, int(e.pos))
			}
			ps, sk := co.groupPairs(group, w)
			pairs = append(pairs, ps...)
			skipped += sk
		}
		i = j
	}
	return pairs, skipped
}

// groupPairs özeti eşit konumları gerçek eşitliğe göre bölüp sola genişlemeyen çiftleri çıkarır.
func (co *dupCorpus) groupPairs(group []int, w int) (pairs []pairMatch, skipped int) {
	var classes [][]int
outer:
	for _, p := range group {
		for k, c := range classes {
			if co.equalWindows(c[0], p, w) {
				classes[k] = append(c, p)
				continue outer
			}
		}
		classes = append(classes, []int{p})
	}
	for _, c := range classes {
		if len(c) < 2 {
			continue
		}
		if len(c) > maxGroup {
			skipped++
			continue
		}
		for x := 0; x < len(c); x++ {
			for y := x + 1; y < len(c); y++ {
				a, b := c[x], c[y]
				if a > 0 && b > 0 && co.seq[a-1] == co.seq[b-1] && !isSentinel(co.seq[a-1]) {
					continue // bir önceki düğümden de eşleşiyor: o çift raporlanır
				}
				l := co.extend(a, b)
				if co.file[a] == co.file[b] && b < a+l {
					l = b - a // aynı dosyada çakışma: ilk kopyanın uzunluğuyla sınırla
				}
				if l >= w {
					pairs = append(pairs, pairMatch{a, b, l})
				}
			}
		}
	}
	return pairs, skipped
}

// dupGroups eşleşmeleri başlangıç konumuna göre birleştirip kümeler kurar.
func (co *dupCorpus) dupGroups(pairs []pairMatch) []*dupCluster {
	parent := map[int]int{}
	length := map[int]int{}
	var find func(x int) int
	find = func(x int) int {
		if p, ok := parent[x]; !ok || p == x {
			parent[x] = x
			return x
		}
		parent[x] = find(parent[x])
		return parent[x]
	}
	for _, p := range pairs {
		ra, rb := find(p.a), find(p.b)
		parent[ra] = rb
		length[p.a] = max(length[p.a], p.length)
		length[p.b] = max(length[p.b], p.length)
	}
	byRoot := map[int][]int{}
	for x := range parent {
		r := find(x)
		byRoot[r] = append(byRoot[r], x)
	}
	var out []*dupCluster
	for _, members := range byRoot {
		sort.Ints(members)
		if c := co.clusterOf(members, length); c != nil {
			out = append(out, c)
		}
	}
	return out
}

// clusterOf başlangıç konumlarından kümeyi (satır aralıklarıyla) kurar.
func (co *dupCorpus) clusterOf(starts []int, length map[int]int) *dupCluster {
	c := &dupCluster{}
	for _, s := range starts {
		l := length[s]
		loc := dupLoc{File: co.files[co.file[s]], Nodes: l, start: s, fileIdx: int(co.file[s]), Func: co.funcAt(s)}
		loc.From, loc.To = co.lineSpan(s, l)
		c.Locs = append(c.Locs, loc)
		if c.Nodes == 0 || l < c.Nodes {
			c.Nodes = l
		}
	}
	c.Locs = dropOverlapping(c.Locs)
	if len(c.Locs) < 2 {
		return nil // konumlar aynı yerin parçalarıydı
	}
	c.Nodes = 0
	for _, l := range c.Locs {
		if c.Nodes == 0 || l.Nodes < c.Nodes {
			c.Nodes = l.Nodes
		}
	}
	c.Diff = co.diffOf(c)
	return c
}

// dropOverlapping aynı dosyada satırları önceki bir konumla çakışan konumları atar (uzun olan kalır).
func dropOverlapping(locs []dupLoc) []dupLoc {
	sort.SliceStable(locs, func(i, j int) bool { return locs[i].Nodes > locs[j].Nodes })
	var kept []dupLoc
next:
	for _, l := range locs {
		for _, k := range kept {
			if k.File == l.File && l.From <= k.To && k.From <= l.To {
				continue next
			}
		}
		kept = append(kept, l)
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].File != kept[j].File {
			return kept[i].File < kept[j].File
		}
		return kept[i].From < kept[j].From
	})
	return kept
}

// lineSpan [start, start+length) düğümlerinin kapsadığı satır aralığıdır.
func (co *dupCorpus) lineSpan(start, length int) (from, to int) {
	for i := start; i < start+length; i++ {
		if l := int(co.line[i]); l > 0 && (from == 0 || l < from) {
			from = l
		}
		if l := int(co.last[i]); l > to {
			to = l
		}
	}
	return from, to
}

// funcAt konumu içeren işlevin adıdır (yoksa boş).
func (co *dupCorpus) funcAt(pos int) string {
	i := sort.Search(len(co.funcs), func(i int) bool { return co.funcs[i].to > pos })
	for ; i < len(co.funcs) && co.funcs[i].from <= pos; i++ {
		if pos < co.funcs[i].to {
			return co.funcs[i].name
		}
	}
	return ""
}

// findDups kümeleri bulur: büyükten küçüğe sıralı, daha büyük bir kümenin içinde kalanlar atılmış.
func (co *dupCorpus) findDups(minNodes int) (clusters []*dupCluster, skippedGroups int) {
	pairs, skipped := co.matches(minNodes)
	groups := co.dupGroups(pairs)
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Nodes != groups[j].Nodes {
			return groups[i].Nodes > groups[j].Nodes
		}
		return groups[i].Locs[0].File+strconv.Itoa(groups[i].Locs[0].From) < groups[j].Locs[0].File+strconv.Itoa(groups[j].Locs[0].From)
	})
	return dropSubsumed(groups), skipped
}

// dropSubsumed her konumu, önce gelen (daha büyük) bir kümenin bir konumunun içinde kalan kümeleri atar.
func dropSubsumed(sorted []*dupCluster) []*dupCluster {
	byFile := map[string][][2]int{}
	var kept []*dupCluster
	for _, c := range sorted {
		inside := true
		for _, l := range c.Locs {
			if !coveredBy(byFile[l.File], l.From, l.To) {
				inside = false
				break
			}
		}
		if inside {
			continue
		}
		kept = append(kept, c)
		for _, l := range c.Locs {
			byFile[l.File] = append(byFile[l.File], [2]int{l.From, l.To})
		}
	}
	return kept
}

func coveredBy(spans [][2]int, from, to int) bool {
	for _, s := range spans {
		if s[0] <= from && to <= s[1] {
			return true
		}
	}
	return false
}
