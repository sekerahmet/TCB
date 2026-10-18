package veri

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
)

// Durum, bir örnekleyicinin konumudur; eğitimi kaldığı yerden sürdürmek
// için kaydedilip OrnekleyiciSecenekleri.Baslangic olarak geri verilebilir.
// Durum yalnızca aynı küme, aynı tohum ve aynı GrupBoyu ile anlamlıdır.
type Durum struct {
	Donem   int   // dönem (epoch) numarası, 0'dan başlar
	Sira    int   // Ornekleyici: dönemdeki grup numarası; SiraliOkuyucu: parça sırası
	Pencere int64 // Ornekleyici: grubun karışık pencere sırasındaki konum; SiraliOkuyucu: token konumu
}

// VarsayilanGrupBoyu, bir grupta birlikte karıştırılan parça sayısıdır.
const VarsayilanGrupBoyu = 4

// OrnekleyiciSecenekleri, rastgele pencere örneklemenin ayarlarıdır.
type OrnekleyiciSecenekleri struct {
	B, T  int    // toplu iş boyutu ve bağlam uzunluğu; satırlar T+1 tokendır
	Tohum uint64 // aynı tohum ve aynı küme → aynı toplu işler
	// GrupBoyu, karıştırılmış parça sırasında ardışık kaç parçanın tek bir
	// grup olarak birlikte açılıp pencerelerinin tek permütasyonda
	// karıştırılacağıdır. 0 → VarsayilanGrupBoyu; 1 → her parça ayrı (bir
	// toplu işin tüm satırları çoğunlukla aynı parçadan gelir).
	GrupBoyu int
	// Baslangic verilirse örnekleme bu konumdan sürer.
	Baslangic *Durum
	// IleriBak, önbellek etkinse şimdiki grubun parçalarından sonra kaç
	// parçanın arka planda hazırlanacağıdır. 0 → varsayılan 3, negatif →
	// hiç. Önbellek kotası en az (GrupBoyu + IleriBak) × en büyük parça
	// boyutu olmalıdır; değilse örnekleyici kurulmaz.
	IleriBak int
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
// Ornekleyici tek bir üretici (ör. OnYukleyici) tarafından sürülmelidir;
// Doldur ve Kapat yine de kilitle korunur.
type Ornekleyici struct {
	k     *Kume
	s     OrnekleyiciSecenekleri
	satir int64
	grupS int // dönem başına grup sayısı

	donem   int
	sira    int // grup numarası
	pencere int64

	sirala   []int // donem'in parça sırası
	sonDonem int   // sirala'nın ait olduğu dönem
	yuklu    bool  // (donem, sira) grubu hazır mı

	grup   []grupParcasi
	sinir  []int64 // grup pencerelerinin parça sınırları; len(grup)+1
	n      int64   // gruptaki toplam pencere sayısı
	perm32 []uint32
	perm64 []uint64

	kapali bool
}

type grupParcasi struct {
	parca *Parca
	ofset int64
}

// Ornekleyici, kümeden rastgele toplu iş üreten bir örnekleyici kurar.
func (k *Kume) Ornekleyici(s OrnekleyiciSecenekleri) (*Ornekleyici, error) {
	if s.B <= 0 || s.T <= 0 {
		return nil, fmt.Errorf("veri: B ve T pozitif olmalı (B=%d, T=%d)", s.B, s.T)
	}
	if s.GrupBoyu < 0 {
		return nil, fmt.Errorf("veri: grup boyu negatif olamaz")
	}
	if s.GrupBoyu == 0 {
		s.GrupBoyu = VarsayilanGrupBoyu
	}
	s.GrupBoyu = min(s.GrupBoyu, len(k.parcalar))
	if s.IleriBak == 0 {
		s.IleriBak = 3
	}
	satir := int64(s.T) + 1
	uygun := false
	for _, p := range k.parcalar {
		if p.Token >= satir {
			uygun = true
		}
	}
	if !uygun {
		return nil, fmt.Errorf("veri: hiçbir parça T+1=%d tokendan uzun değil", satir)
	}
	if err := k.onbellekKotaDenetle(s.GrupBoyu+max(s.IleriBak, 0), satir); err != nil {
		return nil, err
	}
	o := &Ornekleyici{k: k, s: s, satir: satir, sonDonem: -1}
	o.grupS = (len(k.parcalar) + s.GrupBoyu - 1) / s.GrupBoyu
	if b := s.Baslangic; b != nil {
		if b.Donem < 0 || b.Sira < 0 || b.Sira >= o.grupS || b.Pencere < 0 {
			return nil, fmt.Errorf("veri: geçersiz başlangıç durumu %+v (grup sayısı %d)", *b, o.grupS)
		}
		o.donem, o.sira, o.pencere = b.Donem, b.Sira, b.Pencere
	}
	return o, nil
}

// karistir, 64 bitlik değerleri tek bir tohuma katlar (splitmix64 adımları).
func karistir(degerler ...uint64) uint64 {
	h := uint64(0x6A09E667F3BCC909)
	for _, d := range degerler {
		h ^= d
		h += 0x9E3779B97F4A7C15
		z := h
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		h = z ^ (z >> 31)
	}
	return h
}

func (o *Ornekleyici) rastgele(etiket, donem, ek uint64) *rand.Rand {
	return rand.New(rand.NewPCG(karistir(o.s.Tohum, etiket, donem, ek), karistir(etiket, o.s.Tohum, ek, donem)))
}

// donemSirasi, verilen dönemin karıştırılmış parça sırasını hesaplar.
func (o *Ornekleyici) donemSirasi(donem int, dst []int) []int {
	n := len(o.k.parcalar)
	if cap(dst) < n {
		dst = make([]int, n)
	}
	dst = dst[:n]
	for i := range dst {
		dst[i] = i
	}
	r := o.rastgele(1, uint64(donem), 0)
	r.Shuffle(n, func(i, j int) { dst[i], dst[j] = dst[j], dst[i] })
	return dst
}

func (o *Ornekleyici) siraHazirla() {
	if o.sonDonem != o.donem {
		o.sirala = o.donemSirasi(o.donem, o.sirala)
		o.sonDonem = o.donem
	}
}

// grupParcalari, verilen sıralamada g'inci grubun parça sıralarıdır.
func (o *Ornekleyici) grupParcalari(sirala []int, g int) []int {
	bas := g * o.s.GrupBoyu
	return sirala[bas:min(bas+o.s.GrupBoyu, len(sirala))]
}

// yukle, (donem, sira) grubunun parçalarını açar ve karışık pencere
// sırasını kurar.
func (o *Ornekleyici) yukle(ctx context.Context) error {
	o.siraHazirla()
	o.ipucuVer()
	r := o.rastgele(2, uint64(o.donem), uint64(o.sira))
	o.grup = o.grup[:0]
	o.sinir = append(o.sinir[:0], 0)
	for _, idx := range o.grupParcalari(o.sirala, o.sira) {
		pb := o.k.parcalar[idx]
		n := pb.Token / o.satir
		if n == 0 {
			continue
		}
		ofset := r.Int64N(pb.Token - n*o.satir + 1)
		p, err := o.k.ParcaAc(ctx, idx)
		if err != nil {
			o.grubuKapat()
			return err
		}
		o.grup = append(o.grup, grupParcasi{parca: p, ofset: ofset})
		o.sinir = append(o.sinir, o.sinir[len(o.sinir)-1]+n)
	}
	o.n = o.sinir[len(o.sinir)-1]
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
	o.yuklu = true
	return nil
}

// konum, gruptaki i'inci karışık pencerenin parçasını ve başlangıcını verir.
func (o *Ornekleyici) konum(i int64) (*Parca, int64) {
	var e int64
	if o.perm64 != nil {
		e = int64(o.perm64[i])
	} else {
		e = int64(o.perm32[i])
	}
	j := sort.Search(len(o.grup), func(j int) bool { return o.sinir[j+1] > e })
	g := o.grup[j]
	return g.parca, g.ofset + (e-o.sinir[j])*o.satir
}

// ipucuVer, önbelleğe şimdiki grubun parçalarını ve sonraki IleriBak
// parçayı (dönem sınırını aşarak) bildirir.
func (o *Ornekleyici) ipucuVer() {
	if o.k.sec.Onbellek == nil {
		return
	}
	var siralar []int
	for _, idx := range o.grupParcalari(o.sirala, o.sira) {
		if o.k.parcalar[idx].Token >= o.satir {
			siralar = append(siralar, idx)
		}
	}
	donem, konum := o.donem, min((o.sira+1)*o.s.GrupBoyu, len(o.sirala))
	sirala := o.sirala
	ileri := 0
	for adim := 0; ileri < max(o.s.IleriBak, 0) && adim < len(o.k.parcalar); adim++ {
		if konum >= len(sirala) {
			donem++
			konum = 0
			sirala = o.donemSirasi(donem, nil)
		}
		idx := sirala[konum]
		if o.k.parcalar[idx].Token >= o.satir {
			siralar = append(siralar, idx)
			ileri++
		}
		konum++
	}
	o.k.onbellekHazirla(siralar)
}

func (o *Ornekleyici) grubuKapat() error {
	var hatalar []error
	for _, g := range o.grup {
		hatalar = append(hatalar, g.parca.Kapat())
	}
	o.grup = o.grup[:0]
	return errors.Join(hatalar...)
}

// ilerle, şimdiki grubu kapatıp sıradakine geçer.
func (o *Ornekleyici) ilerle() error {
	err := o.grubuKapat()
	o.yuklu = false
	o.pencere = 0
	o.sira++
	if o.sira >= o.grupS {
		o.sira = 0
		o.donem++
	}
	return err
}

// Doldur, ti'yi B adet rastgele pencereyle doldurur. Bir hata olursa
// örnekleyicinin konumu toplu işin ortasında kalabilir.
func (o *Ornekleyici) Doldur(ctx context.Context, ti *TopluIs) error {
	if o.kapali {
		return ErrKapali
	}
	ti.hazirla(o.s.B, o.s.T)
	for r := 0; r < o.s.B; r++ {
		for {
			if !o.yuklu {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := o.yukle(ctx); err != nil {
					return err
				}
			}
			if o.pencere < o.n {
				break
			}
			if err := o.ilerle(); err != nil {
				return err
			}
		}
		p, bas := o.konum(o.pencere)
		if err := p.Oku(ti.Satir(r), bas); err != nil {
			return err
		}
		o.pencere++
	}
	ti.Durum = o.Durum()
	return nil
}

// Sonraki, Doldur'un bağlamsız kısa yoludur.
func (o *Ornekleyici) Sonraki(ti *TopluIs) error { return o.Doldur(context.Background(), ti) }

// Durum, sonraki pencerenin konumudur.
func (o *Ornekleyici) Durum() Durum { return Durum{Donem: o.donem, Sira: o.sira, Pencere: o.pencere} }

// Donem, şimdiki dönem numarasıdır.
func (o *Ornekleyici) Donem() int { return o.donem }

// GrupSayisi, bir dönemdeki grup sayısıdır.
func (o *Ornekleyici) GrupSayisi() int { return o.grupS }

// DonemPencereSayisi, bir dönemde üretilen pencere (satır) sayısıdır.
func (o *Ornekleyici) DonemPencereSayisi() int64 {
	var t int64
	for _, p := range o.k.parcalar {
		t += p.Token / o.satir
	}
	return t
}

// Kapat, açık parçaları kapatır. Birden çok kez çağrılabilir.
func (o *Ornekleyici) Kapat() error {
	o.kapali = true
	o.yuklu = false
	return o.grubuKapat()
}
