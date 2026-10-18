package veri

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
)

// Durum, bir örnekleyicinin konumudur; eğitimi kaldığı yerden sürdürmek
// için kaydedilip OrnekleyiciSecenekleri.Baslangic olarak geri verilebilir.
type Durum struct {
	Donem   int   // dönem (epoch) numarası, 0'dan başlar
	Sira    int   // dönemin karıştırılmış parça sırasındaki konum
	Pencere int64 // parçadaki sonraki pencerenin sırası (SiraliOkuyucu'da token konumu)
}

// OrnekleyiciSecenekleri, rastgele pencere örneklemenin ayarlarıdır.
type OrnekleyiciSecenekleri struct {
	B, T  int    // toplu iş boyutu ve bağlam uzunluğu; satırlar T+1 tokendır
	Tohum uint64 // aynı tohum ve aynı küme → aynı toplu işler
	// Baslangic verilirse örnekleme bu konumdan sürer.
	Baslangic *Durum
	// IleriBak, önbellek etkinse şimdikilerden sonra kaç parçanın arka
	// planda hazırlanacağıdır. 0 → varsayılan 3, negatif → hiç (yalnızca
	// şimdiki parça). Önbellek kotası en az (1 + IleriBak) × en büyük
	// parça boyutu olmalıdır; değilse örnekleyici kurulmaz.
	IleriBak int
}

// Ornekleyici, kümeden tohumlu ve tekrar üretilebilir rastgele pencereler
// üretir. Her dönemde parça sırası yeniden karıştırılır. Bir parçanın
// içinde, rastgele bir başlangıç kayması seçilir ve parça bu kaymadan
// itibaren üst üste binmeyen T+1 uzunluklu pencerelere bölünür; pencereler
// karıştırılmış sırayla verilir. Böylece her dönemde her token (parça
// sonundaki T'den kısa artık dışında) tam bir kez görülür ve hiçbir pencere
// parça (dosya) sınırını aşmaz.
//
// Ornekleyici eşzamanlı kullanım için değildir; tek bir üretici gorutin
// (ör. OnYukleyici) tarafından sürülmelidir.
type Ornekleyici struct {
	k     *Kume
	s     OrnekleyiciSecenekleri
	satir int64

	donem   int
	sira    int
	pencere int64

	sirala   []int // donem'in parça sırası
	yuklu    bool  // (donem, sira) parçası hazır mı
	parca    *Parca
	n        int64 // parçadaki pencere sayısı
	ofset    int64
	perm     []uint32
	kapali   bool
	sonDonem int // sirala'nın ait olduğu dönem
}

// Ornekleyici, kümeden rastgele toplu iş üreten bir örnekleyici kurar.
func (k *Kume) Ornekleyici(s OrnekleyiciSecenekleri) (*Ornekleyici, error) {
	if s.B <= 0 || s.T <= 0 {
		return nil, fmt.Errorf("veri: B ve T pozitif olmalı (B=%d, T=%d)", s.B, s.T)
	}
	if s.IleriBak == 0 {
		s.IleriBak = 3
	}
	satir := int64(s.T) + 1
	uygun := false
	for _, p := range k.parcalar {
		if p.Token >= satir {
			uygun = true
		}
		if p.Token/satir > math.MaxUint32 {
			return nil, fmt.Errorf("veri: %s: T=%d için pencere sayısı çok büyük", p.Ad, s.T)
		}
	}
	if !uygun {
		return nil, fmt.Errorf("veri: hiçbir parça T+1=%d tokendan uzun değil", satir)
	}
	if err := k.onbellekKotaDenetle(1+max(s.IleriBak, 0), satir); err != nil {
		return nil, err
	}
	o := &Ornekleyici{k: k, s: s, satir: satir, sonDonem: -1}
	if b := s.Baslangic; b != nil {
		if b.Donem < 0 || b.Sira < 0 || b.Sira >= len(k.parcalar) || b.Pencere < 0 {
			return nil, fmt.Errorf("veri: geçersiz başlangıç durumu %+v", *b)
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

// yukle, (donem, sira) parçasını açar ve pencere sırasını kurar.
func (o *Ornekleyici) yukle(ctx context.Context) error {
	o.siraHazirla()
	idx := o.sirala[o.sira]
	pb := o.k.parcalar[idx]
	o.ipucuVer()
	o.n = pb.Token / o.satir
	if o.n == 0 {
		o.yuklu = true
		return nil
	}
	p, err := o.k.ParcaAc(ctx, idx)
	if err != nil {
		return err
	}
	o.parca = p
	r := o.rastgele(2, uint64(o.donem), uint64(idx))
	o.ofset = r.Int64N(pb.Token - o.n*o.satir + 1)
	if int64(cap(o.perm)) < o.n {
		o.perm = make([]uint32, o.n)
	}
	o.perm = o.perm[:o.n]
	for i := range o.perm {
		o.perm[i] = uint32(i)
	}
	r.Shuffle(len(o.perm), func(i, j int) { o.perm[i], o.perm[j] = o.perm[j], o.perm[i] })
	o.yuklu = true
	return nil
}

// ipucuVer, önbelleğe şimdiki ve sonraki parçaları bildirir.
func (o *Ornekleyici) ipucuVer() {
	if o.k.sec.Onbellek == nil {
		return
	}
	var siralar []int
	donem, sira := o.donem, o.sira
	sirala := o.sirala
	for adim := 0; len(siralar) < 1+max(o.s.IleriBak, 0) && adim < 2*len(o.k.parcalar); adim++ {
		if sira >= len(sirala) {
			donem++
			sira = 0
			sirala = o.donemSirasi(donem, nil)
		}
		idx := sirala[sira]
		if o.k.parcalar[idx].Token >= o.satir {
			siralar = append(siralar, idx)
		}
		sira++
	}
	o.k.onbellekHazirla(siralar)
}

// ilerle, şimdiki parçayı kapatıp sıradakine geçer.
func (o *Ornekleyici) ilerle() error {
	var err error
	if o.parca != nil {
		err = o.parca.Kapat()
		o.parca = nil
	}
	o.yuklu = false
	o.pencere = 0
	o.sira++
	if o.sira >= len(o.k.parcalar) {
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
		bas := o.ofset + int64(o.perm[o.pencere])*o.satir
		if err := o.parca.Oku(ti.Satir(r), bas); err != nil {
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

// DonemPencereSayisi, bir dönemde üretilen pencere (satır) sayısıdır.
func (o *Ornekleyici) DonemPencereSayisi() int64 {
	var t int64
	for _, p := range o.k.parcalar {
		t += p.Token / o.satir
	}
	return t
}

// Kapat, açık parçayı kapatır. Birden çok kez çağrılabilir.
func (o *Ornekleyici) Kapat() error {
	o.kapali = true
	if o.parca != nil {
		err := o.parca.Kapat()
		o.parca = nil
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sıralı okuma (doğrulama)
// ---------------------------------------------------------------------------

// SiraliSecenekleri, sıralı okumanın ayarlarıdır.
type SiraliSecenekleri struct {
	B, T int
	// Adim, ardışık pencere başlangıçları arasındaki token sayısıdır.
	// Varsayılan T: her token (ilk hariç) tam bir kez hedef olur.
	Adim int
}

// SiraliOkuyucu, parçaları ad sırasıyla, karıştırmadan ve pencereleri
// sırayla okur. Pencereler parça sınırını aşmaz. Son toplu iş B'den az
// satırlı olabilir (ti.B buna göre ayarlanır); ardından io.EOF döner.
type SiraliOkuyucu struct {
	k      *Kume
	s      SiraliSecenekleri
	satir  int64
	sira   int
	bas    int64
	parca  *Parca
	kapali bool
}

// Sirali, kümeden sıralı okuyan bir okuyucu kurar.
func (k *Kume) Sirali(s SiraliSecenekleri) (*SiraliOkuyucu, error) {
	if s.B <= 0 || s.T <= 0 {
		return nil, fmt.Errorf("veri: B ve T pozitif olmalı (B=%d, T=%d)", s.B, s.T)
	}
	if s.Adim == 0 {
		s.Adim = s.T
	}
	if s.Adim < 0 {
		return nil, fmt.Errorf("veri: adım negatif olamaz")
	}
	if err := k.onbellekKotaDenetle(1, int64(s.T)+1); err != nil {
		return nil, err
	}
	return &SiraliOkuyucu{k: k, s: s, satir: int64(s.T) + 1}, nil
}

// PencereSayisi, okuyucunun baştan sona üreteceği pencere sayısıdır.
func (so *SiraliOkuyucu) PencereSayisi() int64 {
	var t int64
	for _, p := range so.k.parcalar {
		if p.Token >= so.satir {
			t += (p.Token-so.satir)/int64(so.s.Adim) + 1
		}
	}
	return t
}

// Doldur, ti'yi sonraki en çok B pencereyle doldurur; bitince io.EOF döner.
func (so *SiraliOkuyucu) Doldur(ctx context.Context, ti *TopluIs) error {
	if so.kapali {
		return ErrKapali
	}
	ti.hazirla(so.s.B, so.s.T)
	r := 0
	for r < so.s.B {
		if so.parca == nil {
			if so.sira >= len(so.k.parcalar) {
				break
			}
			if so.k.parcalar[so.sira].Token < so.satir {
				so.sira++
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			p, err := so.k.ParcaAc(ctx, so.sira)
			if err != nil {
				return err
			}
			so.parca, so.bas = p, 0
		}
		if so.bas+so.satir > so.parca.Uzunluk() {
			err := so.parca.Kapat()
			so.parca = nil
			so.sira++
			so.bas = 0
			if err != nil {
				return err
			}
			continue
		}
		if err := so.parca.Oku(ti.Satir(r), so.bas); err != nil {
			return err
		}
		so.bas += int64(so.s.Adim)
		r++
	}
	if r == 0 {
		return io.EOF
	}
	ti.B = r
	ti.Veri = ti.Veri[:r*int(so.satir)]
	ti.Durum = Durum{Sira: so.sira, Pencere: so.bas}
	return nil
}

// Basa, okuyucuyu başa sarar.
func (so *SiraliOkuyucu) Basa() error {
	var err error
	if so.parca != nil {
		err = so.parca.Kapat()
		so.parca = nil
	}
	so.sira, so.bas = 0, 0
	return err
}

// Kapat, açık parçayı kapatır.
func (so *SiraliOkuyucu) Kapat() error {
	so.kapali = true
	return so.Basa()
}
