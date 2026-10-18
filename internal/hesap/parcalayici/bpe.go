package parcalayici

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

// birlesme bir çift için kural sırası ve sonuç token kimliği.
type birlesme struct {
	sira int32
	yeni int32
}

func ciftAnahtar(a, b int32) uint64 { return uint64(uint32(a))<<32 | uint64(uint32(b)) }

// birlesmeTablosu çift -> birleştirme eşlemesi için açık adresli, doğrusal
// yoklamalı bir karma tablodur; genel amaçlı eşlemeden belirgin biçimde
// hızlıdır. Boş yuva anahtar = bosAnahtar ile işaretlenir (kimlikler negatif
// olmadığından gerçek bir anahtar bu değeri alamaz).
type birlesmeTablosu struct {
	yuvalar []tabloYuvasi
	maske   uint64
	kaydir  uint
}

// tabloYuvasi anahtarı ve değeri yan yana tutar (tek önbellek satırı erişimi).
type tabloYuvasi struct {
	anahtar uint64
	deger   birlesme
}

const bosAnahtar = ^uint64(0)

func yeniBirlesmeTablosu(m map[uint64]birlesme) *birlesmeTablosu {
	boy, kaydir := uint64(16), uint(60)
	for boy < uint64(len(m))*2 {
		boy <<= 1
		kaydir--
	}
	t := &birlesmeTablosu{
		yuvalar: make([]tabloYuvasi, boy),
		maske:   boy - 1,
		kaydir:  kaydir,
	}
	for i := range t.yuvalar {
		t.yuvalar[i].anahtar = bosAnahtar
	}
	for k, v := range m {
		i := t.yuva(k)
		for t.yuvalar[i].anahtar != bosAnahtar {
			i = (i + 1) & t.maske
		}
		t.yuvalar[i] = tabloYuvasi{k, v}
	}
	return t
}

func (t *birlesmeTablosu) yuva(k uint64) uint64 {
	return (k * 0x9E3779B97F4A7C15) >> t.kaydir
}

// bul çiftin birleştirmesini verir; yoksa sira = -1.
func (t *birlesmeTablosu) bul(a, b int32) birlesme {
	k := ciftAnahtar(a, b)
	i := t.yuva(k)
	for {
		y := &t.yuvalar[i]
		ak := y.anahtar
		if ak == k {
			return y.deger
		}
		if ak == bosAnahtar {
			return birlesme{-1, -1}
		}
		i = (i + 1) & t.maske
	}
}

// birlestirKisa doğrudan tarama ile birleştirir.
// Her komşu çiftin birleştirmesi bir kez aranıp dizide tutulur; bir
// birleştirmeden sonra yalnızca değişen iki komşu çift yeniden aranır.
func (p *bpeModel) birlestirKisa(sem []int32) []int32 {
	if len(sem) < 2 {
		return sem
	}
	t := p.tablo
	var ciftDizi [kisaEsik]birlesme
	var cift []birlesme
	if len(sem)-1 <= kisaEsik {
		cift = ciftDizi[:len(sem)-1]
	} else {
		cift = make([]birlesme, len(sem)-1)
	}
	for k := range cift {
		cift[k] = t.bul(sem[k], sem[k+1])
	}
	for len(cift) > 0 {
		konum := -1
		enIyi := int32(0)
		for k, m := range cift {
			if m.sira >= 0 && (konum < 0 || m.sira < enIyi) {
				enIyi, konum = m.sira, k
			}
		}
		if konum < 0 {
			break
		}
		sem[konum] = cift[konum].yeni
		copy(sem[konum+1:], sem[konum+2:])
		sem = sem[:len(sem)-1]
		copy(cift[konum:], cift[konum+1:])
		cift = cift[:len(cift)-1]
		if konum > 0 {
			cift[konum-1] = t.bul(sem[konum-1], sem[konum])
		}
		if konum < len(cift) {
			cift[konum] = t.bul(sem[konum], sem[konum+1])
		}
	}
	return sem
}

// adayBirlesme yığındaki bir birleştirme adayı.
type adayBirlesme struct {
	konum int32 // soldaki sembolün dizini
	sira  int32
	yeni  int32
}

type adayYigini []adayBirlesme

func (h adayYigini) Len() int { return len(h) }
func (h adayYigini) Less(i, j int) bool {
	if h[i].sira != h[j].sira {
		return h[i].sira < h[j].sira
	}
	return h[i].konum < h[j].konum
}
func (h adayYigini) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *adayYigini) Push(x any)   { *h = append(*h, x.(adayBirlesme)) }
func (h *adayYigini) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// birlestirUzun bağlı liste ve öncelik kuyruğu ile birleştirir (O(n log n)).
func (p *bpeModel) birlestirUzun(sem []int32) []int32 {
	n := len(sem)
	onceki := make([]int32, n)
	sonraki := make([]int32, n)
	canli := make([]bool, n)
	for i := range sem {
		onceki[i] = int32(i - 1)
		sonraki[i] = int32(i + 1)
		canli[i] = true
	}
	sonraki[n-1] = -1
	h := make(adayYigini, 0, n)
	for i := 0; i+1 < n; i++ {
		if m := p.tablo.bul(sem[i], sem[i+1]); m.sira >= 0 {
			h = append(h, adayBirlesme{int32(i), m.sira, m.yeni})
		}
	}
	heap.Init(&h)
	for h.Len() > 0 {
		a := heap.Pop(&h).(adayBirlesme)
		i := a.konum
		if !canli[i] || sonraki[i] < 0 {
			continue
		}
		j := sonraki[i]
		m := p.tablo.bul(sem[i], sem[j])
		if m.sira < 0 || m.yeni != a.yeni {
			continue // aday bayatlamış
		}
		sem[i] = a.yeni
		canli[j] = false
		sonraki[i] = sonraki[j]
		if sonraki[j] >= 0 {
			onceki[sonraki[j]] = i
		}
		if o := onceki[i]; o >= 0 {
			if m := p.tablo.bul(sem[o], sem[i]); m.sira >= 0 {
				heap.Push(&h, adayBirlesme{o, m.sira, m.yeni})
			}
		}
		if s := sonraki[i]; s >= 0 {
			if m := p.tablo.bul(sem[i], sem[s]); m.sira >= 0 {
				heap.Push(&h, adayBirlesme{i, m.sira, m.yeni})
			}
		}
	}
	out := sem[:0]
	for i := int32(0); i >= 0; i = sonraki[i] {
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
	onbellekParcaSayisi  = 64
	onbellekAzamiUzunluk = 256 // bundan uzun parçalar önbelleğe alınmaz
)

type onbellekParcasi struct {
	mu sync.RWMutex
	m  map[string][]int32
	_  [40]byte // komşu parçaların kilitleri aynı önbellek satırını paylaşmasın
}

type onbellek struct {
	tohum       maphash.Seed
	parcaSiniri int
	parcalar    [onbellekParcaSayisi]onbellekParcasi
}

func yeniOnbellek(kapasite int) *onbellek {
	if kapasite <= 0 {
		return nil
	}
	o := &onbellek{tohum: maphash.MakeSeed(), parcaSiniri: (kapasite + onbellekParcaSayisi - 1) / onbellekParcaSayisi}
	for i := range o.parcalar {
		o.parcalar[i].m = make(map[string][]int32)
	}
	return o
}

func (o *onbellek) parca(k string) *onbellekParcasi {
	return &o.parcalar[maphash.String(o.tohum, k)%onbellekParcaSayisi]
}

func (o *onbellek) al(k string) ([]int32, bool) {
	pp := o.parca(k)
	pp.mu.RLock()
	v, ok := pp.m[k]
	pp.mu.RUnlock()
	return v, ok
}

// koy sonucu (kopyalayarak) önbelleğe ekler; önbellek doluysa bir şey yapmaz.
func (o *onbellek) koy(k string, v []int32) {
	pp := o.parca(k)
	pp.mu.RLock()
	dolu := len(pp.m) >= o.parcaSiniri
	pp.mu.RUnlock()
	if dolu {
		return
	}
	kopya := append([]int32(nil), v...)
	pp.mu.Lock()
	if len(pp.m) < o.parcaSiniri {
		pp.m[strings.Clone(k)] = kopya
	}
	pp.mu.Unlock()
}
