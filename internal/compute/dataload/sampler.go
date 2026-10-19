package dataload

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"sync"
)

// Durum, bir örnekleyicinin konumudur; eğitimi kaldığı yerden sürdürmek
// için kaydedilip OrnekleyiciSecenekleri.Baslangic olarak geri verilebilir.
// Durum yalnızca aynı küme, aynı tohum ve aynı GrupBoyu ile anlamlıdır.
type State struct {
	Epoch  int   // dönem (epoch) numarası, 0'dan başlar
	Pos    int   // Ornekleyici: dönemdeki grup numarası; SiraliOkuyucu: parça sırası
	Window int64 // Ornekleyici: grubun karışık pencere sırasındaki konum; SiraliOkuyucu: token konumu
}

// VarsayilanGrupBoyu, bir grupta birlikte karıştırılan parça sayısıdır.
const DefaultGroupSize = 4

// VarsayilanIleriBak, önbellekte şimdiki grubun ötesinde hazırlanan parça
// sayısıdır.
const DefaultLookahead = 4

// OrnekleyiciSecenekleri, rastgele pencere örneklemenin ayarlarıdır.
type SamplerOptions struct {
	B, T int    // toplu iş boyutu ve bağlam uzunluğu; satırlar T+1 tokendır
	Seed uint64 // aynı tohum ve aynı küme → aynı toplu işler
	// GrupBoyu, karıştırılmış parça sırasında ardışık kaç parçanın tek bir
	// grup olarak birlikte açılıp pencerelerinin tek permütasyonda
	// karıştırılacağıdır. 0 → VarsayilanGrupBoyu; 1 → her parça ayrı (bir
	// toplu işin tüm satırları çoğunlukla aynı parçadan gelir).
	GroupSize int
	// Baslangic verilirse örnekleme bu konumdan sürer.
	Start *State
	// IleriBak, önbellek etkinse şimdiki grubun parçalarından sonra kaç
	// parçanın arka planda hazırlanacağıdır. 0 → VarsayilanIleriBak, negatif →
	// hiç. Önbellek kotası en az (GrupBoyu + IleriBak) × en büyük parça
	// boyutu olmalıdır; değilse örnekleyici kurulmaz.
	Lookahead int
}

// Ornekleyici, kümeden tohumlu ve tekrar üretilebilir rastgele pencereler
// üretir.
//
// Her dönemde parça sırası yeniden karıştırılır ve bu sıra ardışık
// GrupBoyu parçalık gruplara bölünür. Bir grubun parçaları birlikte açılır;
// her parçada rastgele bir başlangıç kayması seçilir ve parça bu kaymadan
// itibaren üst üste binmeyen T+1 uzunluklu pencerelere bölünür. Grubun tüm
// pencereleri tek bir permütasyonda karıştırılır, böylece bir toplu işin
// satırları gruptaki farklı parçalardan gelir. Her dönemde her token (parça
// sonundaki T'den kısa artık dışında) tam bir kez görülür ve hiçbir pencere
// parça (dosya) sınırını aşmaz.
//
// Permütasyon belleği grup başına 4 × (gruptaki pencere sayısı) bayttır
// (pencere sayısı 2^32'yi aşarsa 8 bayt); ör. 4 parça × 250M token, T=1024
// için yaklaşık 4 MB.
//
// Ornekleyici tek bir üretici (ör. OnYukleyici) tarafından sürülmelidir.
// Tüm yöntemler kilitle korunur: Kapat, süren bir Doldur'un bitmesini
// bekler ve ardından parçaları kapatır; sonraki Doldur ErrKapali döner.
type Sampler struct {
	mu     sync.Mutex
	k      *Dataset
	s      SamplerOptions
	row    int64
	groupN int // dönem başına grup sayısı

	epoch  int
	order  int // grup numarası
	window int64

	sortFn    []int // donem'in parça sırası
	lastEpoch int   // sirala'nın ait olduğu dönem
	loaded    bool  // (donem, sira) grubu hazır mı

	group  []groupShard
	limit  []int64 // grup pencerelerinin parça sınırları; len(grup)+1
	n      int64   // gruptaki toplam pencere sayısı
	perm32 []uint32
	perm64 []uint64

	closed bool
}

type groupShard struct {
	shard  *Shard
	offset int64
}

// Ornekleyici, kümeden rastgele toplu iş üreten bir örnekleyici kurar.
func (k *Dataset) Sampler(s SamplerOptions) (*Sampler, error) {
	if s.B <= 0 || s.T <= 0 {
		return nil, fmt.Errorf("veri: B ve T pozitif olmalı (B=%d, T=%d)", s.B, s.T)
	}
	if s.GroupSize < 0 {
		return nil, fmt.Errorf("veri: grup boyu negatif olamaz")
	}
	if s.GroupSize == 0 {
		s.GroupSize = DefaultGroupSize
	}
	s.GroupSize = min(s.GroupSize, len(k.shards))
	if s.Lookahead == 0 {
		s.Lookahead = DefaultLookahead
	}
	row := int64(s.T) + 1
	fits := false
	for _, p := range k.shards {
		if p.Token >= row {
			fits = true
		}
	}
	if !fits {
		return nil, fmt.Errorf("veri: hiçbir veri dosyası T+1=%d parçadan uzun değil", row)
	}
	if err := k.checkCacheQuota(s.GroupSize+max(s.Lookahead, 0), row); err != nil {
		return nil, err
	}
	o := &Sampler{k: k, s: s, row: row, lastEpoch: -1}
	o.groupN = (len(k.shards) + s.GroupSize - 1) / s.GroupSize
	if b := s.Start; b != nil {
		if b.Epoch < 0 || b.Pos < 0 || b.Pos >= o.groupN || b.Window < 0 {
			return nil, fmt.Errorf("veri: geçersiz başlangıç durumu %+v (grup sayısı %d)", *b, o.groupN)
		}
		o.epoch, o.order, o.window = b.Epoch, b.Pos, b.Window
	}
	return o, nil
}

// karistir, 64 bitlik değerleri tek bir tohuma katlar (splitmix64 adımları).
func shuffle(values ...uint64) uint64 {
	h := uint64(0x6A09E667F3BCC909)
	for _, d := range values {
		h ^= d
		h += 0x9E3779B97F4A7C15
		z := h
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		h = z ^ (z >> 31)
	}
	return h
}

func (o *Sampler) random(label, epoch, ek uint64) *rand.Rand {
	return rand.New(rand.NewPCG(shuffle(o.s.Seed, label, epoch, ek), shuffle(label, o.s.Seed, ek, epoch)))
}

// donemSirasi, verilen dönemin karıştırılmış parça sırasını hesaplar.
func (o *Sampler) epochOrder(epoch int, dst []int) []int {
	n := len(o.k.shards)
	if cap(dst) < n {
		dst = make([]int, n)
	}
	dst = dst[:n]
	for i := range dst {
		dst[i] = i
	}
	r := o.random(1, uint64(epoch), 0)
	r.Shuffle(n, func(i, j int) { dst[i], dst[j] = dst[j], dst[i] })
	return dst
}

func (o *Sampler) prepareOrder() {
	if o.lastEpoch != o.epoch {
		o.sortFn = o.epochOrder(o.epoch, o.sortFn)
		o.lastEpoch = o.epoch
	}
}

// grupParcalari, verilen sıralamada g'inci grubun parça sıralarıdır.
func (o *Sampler) groupShards(sortFn []int, g int) []int {
	begin := g * o.s.GroupSize
	return sortFn[begin:min(begin+o.s.GroupSize, len(sortFn))]
}

// yukle, (donem, sira) grubunun parçalarını açar ve karışık pencere
// sırasını kurar.
func (o *Sampler) load(ctx context.Context) error {
	o.prepareOrder()
	o.hint()
	r := o.random(2, uint64(o.epoch), uint64(o.order))
	o.group = o.group[:0]
	o.limit = append(o.limit[:0], 0)
	for _, idx := range o.groupShards(o.sortFn, o.order) {
		pb := o.k.shards[idx]
		n := pb.Token / o.row
		if n == 0 {
			continue
		}
		offset := r.Int64N(pb.Token - n*o.row + 1)
		p, err := o.k.OpenShard(ctx, idx)
		if err != nil {
			o.closeGroup()
			return err
		}
		o.group = append(o.group, groupShard{shard: p, offset: offset})
		o.limit = append(o.limit, o.limit[len(o.limit)-1]+n)
	}
	o.n = o.limit[len(o.limit)-1]
	if o.n <= math.MaxUint32 {
		o.perm64 = nil
		if int64(cap(o.perm32)) < o.n {
			o.perm32 = make([]uint32, o.n)
		}
		o.perm32 = o.perm32[:o.n]
		for i := range o.perm32 {
			o.perm32[i] = uint32(i)
		}
		r.Shuffle(len(o.perm32), func(i, j int) { o.perm32[i], o.perm32[j] = o.perm32[j], o.perm32[i] })
	} else {
		o.perm32 = nil
		o.perm64 = make([]uint64, o.n)
		for i := range o.perm64 {
			o.perm64[i] = uint64(i)
		}
		r.Shuffle(len(o.perm64), func(i, j int) { o.perm64[i], o.perm64[j] = o.perm64[j], o.perm64[i] })
	}
	o.loaded = true
	return nil
}

// konum, gruptaki i'inci karışık pencerenin parçasını ve başlangıcını verir.
func (o *Sampler) pos(i int64) (*Shard, int64) {
	var e int64
	if o.perm64 != nil {
		e = int64(o.perm64[i])
	} else {
		e = int64(o.perm32[i])
	}
	j := sort.Search(len(o.group), func(j int) bool { return o.limit[j+1] > e })
	g := o.group[j]
	return g.shard, g.offset + (e-o.limit[j])*o.row
}

// ipucuVer, önbelleğe şimdiki grubun parçalarını ve sonraki IleriBak
// parçayı (dönem sınırını aşarak) bildirir.
func (o *Sampler) hint() {
	if o.k.pick.Cache == nil {
		return
	}
	var orders []int
	for _, idx := range o.groupShards(o.sortFn, o.order) {
		if o.k.shards[idx].Token >= o.row {
			orders = append(orders, idx)
		}
	}
	epoch, pos := o.epoch, min((o.order+1)*o.s.GroupSize, len(o.sortFn))
	sortFn := o.sortFn
	ahead := 0
	for step := 0; ahead < max(o.s.Lookahead, 0) && step < len(o.k.shards); step++ {
		if pos >= len(sortFn) {
			epoch++
			pos = 0
			sortFn = o.epochOrder(epoch, nil)
		}
		idx := sortFn[pos]
		if o.k.shards[idx].Token >= o.row {
			orders = append(orders, idx)
			ahead++
		}
		pos++
	}
	o.k.prepareCache(orders)
}

func (o *Sampler) closeGroup() error {
	var errs []error
	for _, g := range o.group {
		errs = append(errs, g.shard.Close())
	}
	o.group = o.group[:0]
	return errors.Join(errs...)
}

// ilerle, şimdiki grubu kapatıp sıradakine geçer.
func (o *Sampler) advance() error {
	err := o.closeGroup()
	o.loaded = false
	o.window = 0
	o.order++
	if o.order >= o.groupN {
		o.order = 0
		o.epoch++
	}
	return err
}

// Doldur, ti'yi B adet rastgele pencereyle doldurur. Bir hata olursa
// örnekleyicinin konumu toplu işin ortasında kalabilir.
func (o *Sampler) Fill(ctx context.Context, ti *Batch) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return ErrClosed
	}
	ti.prepare(o.s.B, o.s.T)
	for r := 0; r < o.s.B; r++ {
		for {
			if !o.loaded {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := o.load(ctx); err != nil {
					return err
				}
			}
			if o.window < o.n {
				break
			}
			if err := o.advance(); err != nil {
				return err
			}
		}
		p, begin := o.pos(o.window)
		if err := p.Read(ti.Row(r), begin); err != nil {
			return err
		}
		o.window++
	}
	ti.State = o.state()
	return nil
}

// Sonraki, Doldur'un bağlamsız kısa yoludur.
func (o *Sampler) Next(ti *Batch) error { return o.Fill(context.Background(), ti) }

// Durum, sonraki pencerenin konumudur.
func (o *Sampler) State() State {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state()
}

func (o *Sampler) state() State { return State{Epoch: o.epoch, Pos: o.order, Window: o.window} }

// Donem, şimdiki dönem numarasıdır.
func (o *Sampler) Epoch() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.epoch
}

// GrupSayisi, bir dönemdeki grup sayısıdır.
func (o *Sampler) GroupCount() int { return o.groupN }

// DonemPencereSayisi, bir dönemde üretilen pencere (satır) sayısıdır.
func (o *Sampler) EpochWindows() int64 {
	var t int64
	for _, p := range o.k.shards {
		t += p.Token / o.row
	}
	return t
}

// Kapat, açık parçaları kapatır. Birden çok kez çağrılabilir.
func (o *Sampler) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	o.loaded = false
	return o.closeGroup()
}
