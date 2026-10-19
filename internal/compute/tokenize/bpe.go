package tokenize

import (
	"container/heap"
	"hash/maphash"
	"strings"
	"sync"
)

// ============================================================================
// BPE birleştirme
// ============================================================================
//
// Bir ön bölme parçasının baytları önce tek baytlık tokenlara çevrilir, sonra
// birleştirme kuralları öncelik sırasıyla uygulanır: her adımda komşu çiftler
// arasında sırası (rank) EN KÜÇÜK olan kural seçilir; eşitlikte en soldaki çift
// birleşir. Bu, öncelik kuyruğuyla çalışan başvuru uygulamasıyla aynı sonucu
// verir. Kısa parçalarda doğrudan tarama, uzun parçalarda yığın (heap)
// kullanılır.

// mergeRule bir çift için kural sırası ve sonuç token kimliği.
type mergeRule struct {
	order int32
	fresh int32
}

func pairKey(a, b int32) uint64 { return uint64(uint32(a))<<32 | uint64(uint32(b)) }

// mergeTable çift -> birleştirme eşlemesi için açık adresli, doğrusal
// yoklamalı bir karma tablodur; genel amaçlı eşlemeden belirgin biçimde
// hızlıdır. Boş yuva anahtar = bosAnahtar ile işaretlenir (kimlikler negatif
// olmadığından gerçek bir anahtar bu değeri alamaz).
type mergeTable struct {
	slots []tableSlot
	mask  uint64
	shift uint
}

// tableSlot anahtarı ve değeri yan yana tutar (tek önbellek satırı erişimi).
type tableSlot struct {
	key   uint64
	value mergeRule
}

const emptyKey = ^uint64(0)

func newMergeTable(m map[uint64]mergeRule) *mergeTable {
	size, shift := uint64(16), uint(60)
	for size < uint64(len(m))*2 {
		size <<= 1
		shift--
	}
	t := &mergeTable{
		slots: make([]tableSlot, size),
		mask:  size - 1,
		shift: shift,
	}
	for i := range t.slots {
		t.slots[i].key = emptyKey
	}
	for k, v := range m {
		i := t.slot(k)
		for t.slots[i].key != emptyKey {
			i = (i + 1) & t.mask
		}
		t.slots[i] = tableSlot{k, v}
	}
	return t
}

func (t *mergeTable) slot(k uint64) uint64 {
	return (k * 0x9E3779B97F4A7C15) >> t.shift
}

// find çiftin birleştirmesini verir; yoksa sira = -1.
func (t *mergeTable) find(a, b int32) mergeRule {
	k := pairKey(a, b)
	i := t.slot(k)
	for {
		y := &t.slots[i]
		ak := y.key
		if ak == k {
			return y.value
		}
		if ak == emptyKey {
			return mergeRule{-1, -1}
		}
		i = (i + 1) & t.mask
	}
}

// joinShort doğrudan tarama ile birleştirir.
// Her komşu çiftin birleştirmesi bir kez aranıp dizide tutulur; bir
// birleştirmeden sonra yalnızca değişen iki komşu çift yeniden aranır.
func (p *bpeModel) joinShort(sem []int32) []int32 {
	if len(sem) < 2 {
		return sem
	}
	t := p.table
	var pairArr [shortThreshold]mergeRule
	var pair []mergeRule
	if len(sem)-1 <= shortThreshold {
		pair = pairArr[:len(sem)-1]
	} else {
		pair = make([]mergeRule, len(sem)-1)
	}
	for k := range pair {
		pair[k] = t.find(sem[k], sem[k+1])
	}
	for len(pair) > 0 {
		pos := -1
		best := int32(0)
		for k, m := range pair {
			if m.order >= 0 && (pos < 0 || m.order < best) {
				best, pos = m.order, k
			}
		}
		if pos < 0 {
			break
		}
		sem[pos] = pair[pos].fresh
		copy(sem[pos+1:], sem[pos+2:])
		sem = sem[:len(sem)-1]
		copy(pair[pos:], pair[pos+1:])
		pair = pair[:len(pair)-1]
		if pos > 0 {
			pair[pos-1] = t.find(sem[pos-1], sem[pos])
		}
		if pos < len(pair) {
			pair[pos] = t.find(sem[pos], sem[pos+1])
		}
	}
	return sem
}

// candidateMerge yığındaki bir birleştirme adayı.
type candidateMerge struct {
	pos   int32 // soldaki sembolün dizini
	order int32
	fresh int32
}

type candidateHeap []candidateMerge

func (h candidateHeap) Len() int { return len(h) }
func (h candidateHeap) Less(i, j int) bool {
	if h[i].order != h[j].order {
		return h[i].order < h[j].order
	}
	return h[i].pos < h[j].pos
}
func (h candidateHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *candidateHeap) Push(x any)   { *h = append(*h, x.(candidateMerge)) }
func (h *candidateHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// joinLong bağlı liste ve öncelik kuyruğu ile birleştirir (O(n log n)).
func (p *bpeModel) joinLong(sem []int32) []int32 {
	n := len(sem)
	prev := make([]int32, n)
	next := make([]int32, n)
	live := make([]bool, n)
	for i := range sem {
		prev[i] = int32(i - 1)
		next[i] = int32(i + 1)
		live[i] = true
	}
	next[n-1] = -1
	h := make(candidateHeap, 0, n)
	for i := 0; i+1 < n; i++ {
		if m := p.table.find(sem[i], sem[i+1]); m.order >= 0 {
			h = append(h, candidateMerge{int32(i), m.order, m.fresh})
		}
	}
	heap.Init(&h)
	for h.Len() > 0 {
		a := heap.Pop(&h).(candidateMerge)
		i := a.pos
		if !live[i] || next[i] < 0 {
			continue
		}
		j := next[i]
		m := p.table.find(sem[i], sem[j])
		if m.order < 0 || m.fresh != a.fresh {
			continue // aday bayatlamış
		}
		sem[i] = a.fresh
		live[j] = false
		next[i] = next[j]
		if next[j] >= 0 {
			prev[next[j]] = i
		}
		if o := prev[i]; o >= 0 {
			if m := p.table.find(sem[o], sem[i]); m.order >= 0 {
				heap.Push(&h, candidateMerge{o, m.order, m.fresh})
			}
		}
		if s := next[i]; s >= 0 {
			if m := p.table.find(sem[i], sem[s]); m.order >= 0 {
				heap.Push(&h, candidateMerge{i, m.order, m.fresh})
			}
		}
	}
	out := sem[:0]
	for i := int32(0); i >= 0; i = next[i] {
		out = append(out, sem[i])
	}
	return out
}

// ============================================================================
// Sözcük önbelleği
// ============================================================================
//
// Sık geçen parçaların (sözcüklerin) BPE sonucu parçalı (sharded) bir
// eşlemede tutulur; eşzamanlı kodlamada kilit çekişmesi az olsun diye her
// parça kendi kilidine sahiptir. Önbellek dolunca yeni girdi eklenmez.

const (
	cacheShardCount = 64
	cacheMaxLen     = 256 // bundan uzun parçalar önbelleğe alınmaz
)

type cacheShard struct {
	mu sync.RWMutex
	m  map[string][]int32
	_  [40]byte // komşu parçaların kilitleri aynı önbellek satırını paylaşmasın
}

type cache struct {
	seed       maphash.Seed
	pieceLimit int
	pieces     [cacheShardCount]cacheShard
}

func newCache(capacity int) *cache {
	if capacity <= 0 {
		return nil
	}
	o := &cache{seed: maphash.MakeSeed(), pieceLimit: (capacity + cacheShardCount - 1) / cacheShardCount}
	for i := range o.pieces {
		o.pieces[i].m = make(map[string][]int32)
	}
	return o
}

func (o *cache) piece(k string) *cacheShard {
	return &o.pieces[maphash.String(o.seed, k)%cacheShardCount]
}

func (o *cache) al(k string) ([]int32, bool) {
	pp := o.piece(k)
	pp.mu.RLock()
	v, ok := pp.m[k]
	pp.mu.RUnlock()
	return v, ok
}

// put sonucu (kopyalayarak) önbelleğe ekler; önbellek doluysa bir şey yapmaz.
func (o *cache) put(k string, v []int32) {
	pp := o.piece(k)
	pp.mu.RLock()
	full := len(pp.m) >= o.pieceLimit
	pp.mu.RUnlock()
	if full {
		return
	}
	dup := append([]int32(nil), v...)
	pp.mu.Lock()
	if len(pp.m) < o.pieceLimit {
		pp.m[strings.Clone(k)] = dup
	}
	pp.mu.Unlock()
}
