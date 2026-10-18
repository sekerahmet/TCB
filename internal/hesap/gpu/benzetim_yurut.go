package gpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// ============================================================================
// PTX benzetimi: yürütücü
// ============================================================================
//
// Bir bloğun iş parçacıkları sırayla, engelden (bar.sync) engele yürütülür: her iş
// parçacığı bir engele ya da sona varana kadar çalışır; hepsi engeldeyse birlikte
// bırakılır. Bazı iş parçacıkları bitmişken diğerlerinin engelde beklemesi (dağınık
// engel) donanımda tanımsızdır; benzetim bunu hata sayar. Genel bellek erişimleri
// ayrılmış bölgelerle ve hizayla denetlenir.

// bzAyirma benzetimde ayrılmış bir cihaz belleği bölgesi.
type bzAyirma struct {
	taban uint64
	veri  []byte
}

// bzBellek benzetimin cihaz belleği.
type bzBellek struct {
	bolgeler []*bzAyirma // tabana göre sıralı
	sonraki  uint64
}

func (m *bzBellek) ayir(n uint64) uint64 {
	if m.sonraki == 0 {
		m.sonraki = 1 << 40
	}
	taban := m.sonraki
	m.sonraki += (n + 4096 + 255) &^ 255 // bölgeler arasında boşluk: taşmalar boşluğa düşer
	m.bolgeler = append(m.bolgeler, &bzAyirma{taban: taban, veri: make([]byte, n)})
	return taban
}

func (m *bzBellek) birak(taban uint64) bool {
	i := sort.Search(len(m.bolgeler), func(i int) bool { return m.bolgeler[i].taban >= taban })
	if i == len(m.bolgeler) || m.bolgeler[i].taban != taban {
		return false
	}
	m.bolgeler = append(m.bolgeler[:i], m.bolgeler[i+1:]...)
	return true
}

// dilim adresteki n baytı verir; bölge dışıysa hata.
func (m *bzBellek) dilim(adr uint64, n int) ([]byte, error) {
	i := sort.Search(len(m.bolgeler), func(i int) bool { return m.bolgeler[i].taban > adr }) - 1
	if i >= 0 {
		b := m.bolgeler[i]
		if o := adr - b.taban; o+uint64(n) <= uint64(len(b.veri)) {
			return b.veri[o : o+uint64(n)], nil
		}
	}
	return nil, fmt.Errorf("ayrılmış bölge dışında %d baytlık genel bellek erişimi: 0x%x", n, adr)
}

// Yürütme durumları.
const (
	dCalisiyor = iota
	dEngelde
	dBitti
)

type bzIs struct {
	y     []uint64
	pc    int32
	durum uint8
	no    int32 // bloktaki doğrusal sıra
	tid   [3]uint32
}

type bzBlok struct {
	is     *bzIslev
	param  [][]byte
	ortak  []byte
	bel    *bzBellek
	ozel   [12]uint32 // tid'ler iş parçacığına göre doldurulur
	butce  *int64
	hataIs string
	// Paylaşımlı bellek yarış dedektörü: her 4 baytlık sözcük için, iki engel arasındaki
	// (dönem) son yazan ve okuyan iş parçacığı (-1 yok, -2 birden çok okuyan). Aynı dönemde
	// bir iş parçacığının yazdığı sözcüğü başkasının okuması ya da yazması, ya da birinin
	// okuduğunu başkasının yazması yarıştır: donanımda sonuç zamanlamaya bağlıdır.
	yarisYazan  []int32
	yarisOkuyan []int32
	yarisDonem  []uint32
	donem       uint32
}

// yaris paylaşımlı bellek erişimini yarış dedektörüne bildirir.
func (b *bzBlok) yaris(t *bzIs, adr, n int, yazma bool) error {
	for w := adr / 4; w <= (adr+n-1)/4; w++ {
		if b.yarisDonem[w] != b.donem {
			b.yarisDonem[w], b.yarisYazan[w], b.yarisOkuyan[w] = b.donem, -1, -1
		}
		yz, ok := b.yarisYazan[w], b.yarisOkuyan[w]
		if yz >= 0 && yz != t.no {
			return fmt.Errorf("paylaşımlı bellek yarışı: %d. bayt aynı engel aralığında %d. iş parçacığınca yazıldı, %d. iş parçacığınca da %s",
				4*w, yz, t.no, map[bool]string{true: "yazılıyor", false: "okunuyor"}[yazma])
		}
		if yazma {
			if ok == -2 || (ok >= 0 && ok != t.no) {
				return fmt.Errorf("paylaşımlı bellek yarışı: %d. bayt aynı engel aralığında başka bir iş parçacığınca okundu, %d. iş parçacığınca yazılıyor", 4*w, t.no)
			}
			b.yarisYazan[w] = t.no
			continue
		}
		switch {
		case ok == -1:
			b.yarisOkuyan[w] = t.no
		case ok != t.no:
			b.yarisOkuyan[w] = -2
		}
	}
	return nil
}

// bzBaslat bir çekirdeği bütün ızgarada yürütür.
func bzBaslat(is *bzIslev, bel *bzBellek, izgara, blok [3]uint32, param [][]byte) error {
	if len(param) != len(is.param) {
		return fmt.Errorf("%s: %d parametre bekleniyordu, %d verildi", is.ad, len(is.param), len(param))
	}
	for i, p := range is.param {
		if len(param[i]) != p.bayt {
			return fmt.Errorf("%s: %s parametresi %d bayt olmalı, %d verildi", is.ad, p.ad, p.bayt, len(param[i]))
		}
	}
	n := int(blok[0] * blok[1] * blok[2])
	if n == 0 || n > 1024 {
		return fmt.Errorf("%s: blok başına %d iş parçacığı (1..1024 olmalı)", is.ad, n)
	}
	if izgara[1] > 65535 || izgara[2] > 65535 {
		return fmt.Errorf("%s: ızgaranın y/z boyutu 65535'i aşıyor (%v)", is.ad, izgara)
	}
	butce := int64(4e9)
	b := &bzBlok{is: is, param: param, ortak: make([]byte, is.ortakBayt), bel: bel, butce: &butce}
	sozcuk := (is.ortakBayt + 3) / 4
	b.yarisYazan, b.yarisOkuyan, b.yarisDonem = make([]int32, sozcuk), make([]int32, sozcuk), make([]uint32, sozcuk)
	b.ozel[3], b.ozel[4], b.ozel[5] = blok[0], blok[1], blok[2]
	b.ozel[9], b.ozel[10], b.ozel[11] = izgara[0], izgara[1], izgara[2]
	isler := make([]bzIs, n)
	yazmac := make([]uint64, n*is.yazmacSay)
	for i := range isler {
		isler[i].y = yazmac[i*is.yazmacSay : (i+1)*is.yazmacSay]
		isler[i].no = int32(i)
		isler[i].tid = [3]uint32{uint32(i) % blok[0], uint32(i) / blok[0] % blok[1], uint32(i) / (blok[0] * blok[1])}
	}
	for z := uint32(0); z < izgara[2]; z++ {
		for y := uint32(0); y < izgara[1]; y++ {
			for x := uint32(0); x < izgara[0]; x++ {
				b.ozel[6], b.ozel[7], b.ozel[8] = x, y, z
				clear(b.ortak)
				clear(yazmac)
				if err := b.yurut(isler); err != nil {
					return fmt.Errorf("%s, blok (%d,%d,%d), %s: %v", is.ad, x, y, z, b.hataIs, err)
				}
			}
		}
	}
	return nil
}

func (b *bzBlok) yurut(isler []bzIs) error {
	for i := range isler {
		isler[i].pc, isler[i].durum = 0, dCalisiyor
	}
	b.donem++
	for {
		engelde, biten := 0, 0
		for i := range isler {
			t := &isler[i]
			if t.durum == dCalisiyor {
				if err := b.calistir(t); err != nil {
					b.hataIs = fmt.Sprintf("iş parçacığı %v, komut %d (%s)", t.tid, t.pc, b.is.komutlar[t.pc].satir)
					return err
				}
			}
			switch t.durum {
			case dEngelde:
				engelde++
			case dBitti:
				biten++
			}
		}
		if engelde == 0 {
			return nil
		}
		if biten > 0 {
			b.hataIs = "engel"
			return fmt.Errorf("dağınık engel: %d iş parçacığı bitmişken %d tanesi bar.sync'te bekliyor", biten, engelde)
		}
		for i := range isler {
			isler[i].durum = dCalisiyor
		}
		b.donem++
	}
}

func (b *bzBlok) oku(t *bzIs, o *bzTerim) uint64 {
	switch o.cesit {
	case tmYazmac:
		return t.y[o.y]
	case tmSabit:
		return o.s
	}
	if o.y < 3 {
		return uint64(t.tid[o.y])
	}
	return uint64(b.ozel[o.y])
}

// calistir iş parçacığını bir engele ya da sona kadar yürütür.
func (b *bzBlok) calistir(t *bzIs) error {
	kom := b.is.komutlar
	for {
		if int(t.pc) >= len(kom) {
			t.durum = dBitti
			return nil
		}
		*b.butce--
		if *b.butce < 0 {
			return fmt.Errorf("komut bütçesi aşıldı (sonsuz döngü?)")
		}
		k := &kom[t.pc]
		if k.kos >= 0 && (t.y[k.kos] != 0) == k.kosDegil {
			t.pc++
			continue
		}
		switch k.islem {
		case bzBra:
			t.pc = k.hedef
			continue
		case bzBar:
			t.pc++
			t.durum = dEngelde
			return nil
		case bzRet:
			t.durum = dBitti
			return nil
		case bzLd:
			if err := b.yukle(t, k); err != nil {
				return err
			}
		case bzSt:
			if err := b.sakla(t, k); err != nil {
				return err
			}
		default:
			if err := b.hesapla(t, k); err != nil {
				return err
			}
		}
		t.pc++
	}
}

// turBayt bir türün bellekteki baytı.
func turBayt(t bzTur) int {
	switch t {
	case bzU16, bzF16:
		return 2
	case bzU64, bzS64, bzF64:
		return 8
	}
	return 4
}

func (b *bzBlok) adresDilim(t *bzIs, k *bzKomut, n int, yazma bool) ([]byte, error) {
	switch k.alan {
	case alanParam:
		p := b.param[k.param]
		if k.ofset < 0 || int(k.ofset)+n > len(p) {
			return nil, fmt.Errorf("parametre dışı okuma")
		}
		return p[k.ofset : int(k.ofset)+n], nil
	case alanOrtak:
		adr := int64(uint32(b.oku(t, &k.taban))) + k.ofset
		if adr%int64(n) != 0 {
			return nil, fmt.Errorf("hizasız paylaşımlı bellek erişimi %d", adr)
		}
		if adr < 0 || int(adr)+n > len(b.ortak) {
			return nil, fmt.Errorf("paylaşımlı bellek dışı erişim %d (boyut %d)", adr, len(b.ortak))
		}
		if err := b.yaris(t, int(adr), n, yazma); err != nil {
			return nil, err
		}
		return b.ortak[adr : int(adr)+n], nil
	}
	adr := b.oku(t, &k.taban) + uint64(k.ofset)
	if adr%uint64(n) != 0 {
		return nil, fmt.Errorf("hizasız genel bellek erişimi 0x%x (%d bayt)", adr, n)
	}
	return b.bel.dilim(adr, n)
}

func (b *bzBlok) yukle(t *bzIs, k *bzKomut) error {
	n := turBayt(k.tur)
	d, err := b.adresDilim(t, k, n, false)
	if err != nil {
		return err
	}
	var v uint64
	switch n {
	case 2:
		v = uint64(binary.LittleEndian.Uint16(d))
	case 4:
		v = uint64(binary.LittleEndian.Uint32(d))
		if k.tur == bzS32 {
			v = uint64(uint32(v))
		}
	default:
		v = binary.LittleEndian.Uint64(d)
	}
	t.y[k.d] = v
	return nil
}

func (b *bzBlok) sakla(t *bzIs, k *bzKomut) error {
	if k.alan == alanParam {
		return fmt.Errorf("parametre alanına yazılamaz")
	}
	n := turBayt(k.tur)
	d, err := b.adresDilim(t, k, n, true)
	if err != nil {
		return err
	}
	v := b.oku(t, &k.o[0])
	switch n {
	case 2:
		binary.LittleEndian.PutUint16(d, uint16(v))
	case 4:
		binary.LittleEndian.PutUint32(d, uint32(v))
	default:
		binary.LittleEndian.PutUint64(d, v)
	}
	return nil
}

func f32(v uint64) float32  { return math.Float32frombits(uint32(v)) }
func f32b(f float32) uint64 { return uint64(math.Float32bits(f)) }
func f64(v uint64) float64  { return math.Float64frombits(v) }
func f64b(f float64) uint64 { return math.Float64bits(f) }

// genislik sonucu türün genişliğine kırpar.
func genislik(t bzTur, v uint64) uint64 {
	switch t {
	case bzU16, bzF16:
		return v & 0xFFFF
	case bzU32, bzS32, bzF32:
		return v & 0xFFFFFFFF
	case bzPred:
		if v != 0 {
			return 1
		}
		return 0
	}
	return v
}

func (b *bzBlok) hesapla(t *bzIs, k *bzKomut) error {
	a := b.oku(t, &k.o[0])
	c := b.oku(t, &k.o[1])
	e := b.oku(t, &k.o[2])
	var r uint64
	var err error
	switch {
	case k.islem == bzCvt:
		r, err = donustur(k, a)
		if err == nil {
			t.y[k.d] = genislik(k.tur, r)
		}
		return err
	case k.tur == bzF32:
		r, err = hesaplaF32(k, a, c, e)
	case k.tur == bzF64:
		r, err = hesaplaF64(k, a, c, e)
	default:
		r, err = hesaplaTam(k, a, c, e)
	}
	if err != nil {
		return err
	}
	if k.islem == bzSetp {
		r = genislik(bzPred, r)
	} else if k.islem != bzMulWide && k.islem != bzMadWide {
		r = genislik(k.tur, r)
	}
	t.y[k.d] = r
	return nil
}

func hesaplaTam(k *bzKomut, a, b, c uint64) (uint64, error) {
	isaretli := k.tur == bzS32 || k.tur == bzS64
	gen64 := k.tur == bzU64 || k.tur == bzS64
	sa, sb := int64(int32(a)), int64(int32(b))
	if gen64 {
		sa, sb = int64(a), int64(b)
	}
	switch k.islem {
	case bzMov, bzCvta:
		return a, nil
	case bzAdd:
		return a + b, nil
	case bzSub:
		return a - b, nil
	case bzMul:
		return a * b, nil
	case bzMad:
		return a*b + c, nil
	case bzMulWide, bzMadWide:
		var r uint64
		if isaretli {
			r = uint64(int64(int32(a)) * int64(int32(b)))
		} else {
			r = uint64(uint32(a)) * uint64(uint32(b))
		}
		if k.islem == bzMadWide {
			r += c
		}
		return r, nil
	case bzDiv, bzRem:
		if genislik(k.tur, b) == 0 {
			return 0, fmt.Errorf("tam sayı sıfıra bölme")
		}
		if isaretli {
			if k.islem == bzDiv {
				return uint64(sa / sb), nil
			}
			return uint64(sa % sb), nil
		}
		ua, ub := genislik(k.tur, a), genislik(k.tur, b)
		if k.islem == bzDiv {
			return ua / ub, nil
		}
		return ua % ub, nil
	case bzMin, bzMax:
		kucuk := genislik(k.tur, a) < genislik(k.tur, b)
		if isaretli {
			kucuk = sa < sb
		}
		if kucuk == (k.islem == bzMin) {
			return a, nil
		}
		return b, nil
	}
	return hesaplaBit(k, a, b, c, sa, isaretli)
}

// hesaplaBit tam sayı türlerinde işaret, kaydırma, mantık, seçme ve karşılaştırma.
func hesaplaBit(k *bzKomut, a, b, c uint64, sa int64, isaretli bool) (uint64, error) {
	switch k.islem {
	case bzNeg:
		return -a, nil
	case bzAbs:
		if sa < 0 {
			return uint64(-sa), nil
		}
		return a, nil
	case bzShl:
		return a << (b & 63), nil
	case bzShr:
		if isaretli {
			return uint64(sa >> (b & 63)), nil
		}
		return genislik(k.tur, a) >> (b & 63), nil
	case bzAnd:
		return a & b, nil
	case bzOr:
		return a | b, nil
	case bzXor:
		return a ^ b, nil
	case bzNot:
		if k.tur == bzPred {
			return a ^ 1, nil
		}
		return ^a, nil
	case bzSelp:
		if c != 0 {
			return a, nil
		}
		return b, nil
	case bzSetp:
		sb := int64(int32(b))
		if k.tur == bzU64 || k.tur == bzS64 {
			sb = int64(b)
		}
		return karsilastirTam(k, a, b, sa, sb, isaretli), nil
	case bzCvt:
		return donustur(k, a)
	}
	return 0, fmt.Errorf("benzetimde tam sayı türü için desteklenmeyen işlem")
}

func karsilastirTam(k *bzKomut, a, b uint64, sa, sb int64, isaretli bool) uint64 {
	var kucuk, esit bool
	if isaretli {
		kucuk, esit = sa < sb, sa == sb
	} else {
		ua, ub := genislik(k.tur, a), genislik(k.tur, b)
		kucuk, esit = ua < ub, ua == ub
	}
	var r bool
	switch k.kip {
	case kEq:
		r = esit
	case kNe:
		r = !esit
	case kLt:
		r = kucuk
	case kLe:
		r = kucuk || esit
	case kGt:
		r = !kucuk && !esit
	case kGe:
		r = !kucuk
	}
	if r {
		return 1
	}
	return 0
}
