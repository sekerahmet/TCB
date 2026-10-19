package gpu

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ============================================================================
// PTX benzetimi: ayrıştırıcı
// ============================================================================
//
// Benzetim, yazıcının (ptx.go) ürettiği PTX altkümesini satır satır çözer ve her iş
// parçacığını sırayla yürütür. Amaç donanımsız sınamadır: çekirdeklerin mantık hataları,
// sınır dışı bellek erişimleri ve dağınık engeller (bar.sync) burada yakalanır. Altküme
// dışındaki bir komut açık hatadır; böylece yazıcı benzetimin bilmediği bir komut
// üretemez.

// bzIslem komut ailesi.
type bzIslem uint8

const (
	bzMov bzIslem = iota
	bzAdd
	bzSub
	bzMul
	bzMulWide
	bzMad
	bzMadWide
	bzFma
	bzDiv
	bzRem
	bzNeg
	bzAbs
	bzMin
	bzMax
	bzSqrt
	bzRcp
	bzEx2
	bzLg2
	bzSin
	bzCos
	bzShl
	bzShr
	bzAnd
	bzOr
	bzXor
	bzNot
	bzSetp
	bzSelp
	bzCvt
	bzCvta
	bzLd
	bzSt
	bzBra
	bzBar
	bzRet
	bzCp      // cp.async (genelden paylaşımlıya eşzamansız kopya)
	bzCpGrup  // cp.async.commit_group
	bzCpBekle // cp.async.wait_group
)

var bzIslemAdlari = map[string]bzIslem{
	"mov": bzMov, "add": bzAdd, "sub": bzSub, "mul": bzMul, "mad": bzMad, "fma": bzFma,
	"div": bzDiv, "rem": bzRem, "neg": bzNeg, "abs": bzAbs, "min": bzMin, "max": bzMax,
	"sqrt": bzSqrt, "rcp": bzRcp, "ex2": bzEx2, "lg2": bzLg2, "sin": bzSin, "cos": bzCos,
	"shl": bzShl, "shr": bzShr, "and": bzAnd, "or": bzOr, "xor": bzXor, "not": bzNot,
	"setp": bzSetp, "selp": bzSelp, "cvt": bzCvt, "cvta": bzCvta, "ld": bzLd, "st": bzSt,
	"bra": bzBra, "bar": bzBar, "ret": bzRet, "cp": bzCp,
}

// bzTur işlenen türü.
type bzTur uint8

const (
	bzYok bzTur = iota
	bzU16
	bzU32
	bzS32
	bzU64
	bzS64
	bzF16
	bzF32
	bzF64
	bzPred
)

var bzTurAdlari = map[string]bzTur{
	"b16": bzU16, "u16": bzU16, "s16": bzU16, "f16": bzF16,
	"b32": bzU32, "u32": bzU32, "s32": bzS32, "f32": bzF32,
	"b64": bzU64, "u64": bzU64, "s64": bzS64, "f64": bzF64, "pred": bzPred,
}

// Karşılaştırma kipleri (setp).
const (
	kEq = iota
	kNe
	kLt
	kLe
	kGt
	kGe
	kEqu
	kNeu
	kLtu
	kLeu
	kGtu
	kGeu
	kNum
	kNan
)

var bzKarsilastirma = map[string]int{
	"eq": kEq, "ne": kNe, "lt": kLt, "le": kLe, "gt": kGt, "ge": kGe, "equ": kEqu, "neu": kNeu,
	"ltu": kLtu, "leu": kLeu, "gtu": kGtu, "geu": kGeu, "num": kNum, "nan": kNan,
}

// Bellek alanları.
const (
	alanParam = iota
	alanGenel
	alanOrtak
)

// Terim çeşitleri.
const (
	tmYazmac = iota
	tmSabit
	tmOzel
)

// Özel yazmaçlar.
var bzOzeller = map[string]int{
	"%tid.x": 0, "%tid.y": 1, "%tid.z": 2, "%ntid.x": 3, "%ntid.y": 4, "%ntid.z": 5,
	"%ctaid.x": 6, "%ctaid.y": 7, "%ctaid.z": 8, "%nctaid.x": 9, "%nctaid.y": 10, "%nctaid.z": 11,
}

type bzTerim struct {
	cesit uint8
	y     int32  // yazmaç yuvası ya da özel yazmaç numarası
	s     uint64 // sabit (bitler)
}

type bzKomut struct {
	islem    bzIslem
	tur      bzTur // işlem türü (cvt'de hedef türü)
	kaynak   bzTur // cvt kaynak türü
	kip      int   // setp karşılaştırması; cvt yuvarlama; mul/mad: 1 = .lo
	kos      int32 // koşul yüklemi yuvası (-1 yok)
	kosDegil bool
	d        int32 // hedef yazmaç (-1 yok)
	o        [3]bzTerim
	alan     int     // ld/st bellek alanı
	taban    bzTerim // ld/st adres tabanı (yazmaç ya da sembol sabiti)
	ofset    int64
	param    int32 // ld.param: parametre sırası
	hedef    int32 // bra hedefi
	satir    string
	// Vektörel erişim (.v2/.v4): vek eleman sayısı; ld hedefleri vy, st kaynakları vo.
	vek uint8
	vy  [4]int32
	vo  [4]bzTerim
	// cp.async: kaynak (genel bellek) adresi, kopya boyu (o[0]) ve kaynak boyu (o[1]).
	taban2 bzTerim
	ofset2 int64
}

type bzIslev struct {
	ad        string
	param     []paramTuru
	yazmacSay int
	ortakBayt int
	komutlar  []bzKomut
}

// bzAyristirici tek bir modülün ayrıştırma durumu.
type bzAyristirici struct {
	is       *bzIslev
	yazmac   map[string]int32
	ortak    map[string]int // sembol → paylaşımlı bellek ofseti
	paramNo  map[string]int32
	etiket   map[string]int32
	bekleyen []struct {
		komut int
		ad    string
	}
}

// bzAyristir PTX modülünü çözer (tek giriş noktası).
func bzAyristir(metin string) (*bzIslev, error) {
	a := &bzAyristirici{is: &bzIslev{}, yazmac: map[string]int32{}, ortak: map[string]int{},
		paramNo: map[string]int32{}, etiket: map[string]int32{}}
	satirlar := strings.Split(metin, "\n")
	govde := false
	for no, s := range satirlar {
		s = strings.TrimSpace(s)
		if i := strings.Index(s, "//"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
		if s == "" {
			continue
		}
		if err := a.satir(s, &govde); err != nil {
			return nil, fmt.Errorf("benzetim: PTX satır %d (%q): %v", no+1, s, err)
		}
	}
	for _, b := range a.bekleyen {
		h, ok := a.etiket[b.ad]
		if !ok {
			return nil, fmt.Errorf("benzetim: tanımsız etiket %s", b.ad)
		}
		a.is.komutlar[b.komut].hedef = h
	}
	if a.is.ad == "" {
		return nil, fmt.Errorf("benzetim: giriş noktası (.entry) yok")
	}
	a.is.yazmacSay = len(a.yazmac)
	return a.is, nil
}

func (a *bzAyristirici) satir(s string, govde *bool) error {
	switch {
	case strings.HasPrefix(s, ".version"), strings.HasPrefix(s, ".target"), strings.HasPrefix(s, ".address_size"):
		return nil
	case strings.HasPrefix(s, ".visible .entry "):
		ad := strings.TrimSuffix(strings.TrimPrefix(s, ".visible .entry "), "(")
		a.is.ad = strings.TrimSpace(ad)
		return nil
	case strings.HasPrefix(s, ".param "):
		return a.paramBildir(strings.TrimSuffix(s, ","))
	case s == ")":
		return nil
	case s == "{":
		*govde = true
		return nil
	case s == "}":
		*govde = false
		return nil
	case strings.HasPrefix(s, ".reg "):
		return a.yazmacBildir(strings.TrimSuffix(s, ";"))
	case strings.HasPrefix(s, ".shared "):
		return a.ortakBildir(strings.TrimSuffix(s, ";"))
	case strings.HasSuffix(s, ":"):
		a.etiket[strings.TrimSuffix(s, ":")] = int32(len(a.is.komutlar))
		return nil
	}
	if !*govde {
		return fmt.Errorf("gövde dışında komut")
	}
	return a.komutCoz(strings.TrimSuffix(s, ";"))
}

func (a *bzAyristirici) paramBildir(s string) error {
	f := strings.Fields(s)
	ad := f[len(f)-1]
	bayt := 0
	switch {
	case strings.Contains(ad, "["):
		i := strings.Index(ad, "[")
		n, err := strconv.Atoi(strings.TrimSuffix(ad[i+1:], "]"))
		if err != nil {
			return err
		}
		ad, bayt = ad[:i], n
	case f[1] == ".u64":
		bayt = 8
	case f[1] == ".u32", f[1] == ".f32", f[1] == ".s32":
		bayt = 4
	default:
		return fmt.Errorf("bilinmeyen parametre türü %s", f[1])
	}
	a.paramNo[ad] = int32(len(a.is.param))
	a.is.param = append(a.is.param, paramTuru{ad: ad, bayt: bayt})
	return nil
}

func (a *bzAyristirici) yazmacBildir(s string) error {
	f := strings.Fields(s) // .reg .f32 %f<12>
	if len(f) != 3 {
		return fmt.Errorf("yazmaç bildirimi")
	}
	i := strings.Index(f[2], "<")
	n, err := strconv.Atoi(strings.TrimSuffix(f[2][i+1:], ">"))
	if err != nil {
		return err
	}
	for k := 0; k < n; k++ {
		a.yazmac[f[2][:i]+strconv.Itoa(k)] = int32(len(a.yazmac))
	}
	return nil
}

func (a *bzAyristirici) ortakBildir(s string) error {
	f := strings.Fields(s) // .shared .align 16 .b8 ad[4096]
	son := f[len(f)-1]
	i := strings.Index(son, "[")
	n, err := strconv.Atoi(strings.TrimSuffix(son[i+1:], "]"))
	if err != nil {
		return err
	}
	a.is.ortakBayt = (a.is.ortakBayt + 15) &^ 15
	a.ortak[son[:i]] = a.is.ortakBayt
	a.is.ortakBayt += n
	return nil
}

func (a *bzAyristirici) komutCoz(s string) error {
	k := bzKomut{kos: -1, d: -1, satir: s}
	if strings.HasPrefix(s, "@") {
		sp := strings.IndexByte(s, ' ')
		p := s[1:sp]
		if strings.HasPrefix(p, "!") {
			k.kosDegil = true
			p = p[1:]
		}
		y, ok := a.yazmac[p]
		if !ok {
			return fmt.Errorf("tanımsız yüklem %s", p)
		}
		k.kos = y
		s = strings.TrimSpace(s[sp+1:])
	}
	op, args := s, ""
	if sp := strings.IndexByte(s, ' '); sp >= 0 {
		op, args = s[:sp], strings.TrimSpace(s[sp+1:])
	}
	// Vektör işleneni ({a, b, c, d}) tek terim olarak yer tutucuyla değiştirilir.
	var vekListe []string
	if i := strings.IndexByte(args, '{'); i >= 0 {
		j := strings.IndexByte(args, '}')
		if j < i {
			return fmt.Errorf("kapanmamış vektör işleneni")
		}
		for _, v := range strings.Split(args[i+1:j], ",") {
			vekListe = append(vekListe, strings.TrimSpace(v))
		}
		args = args[:i] + "@V" + args[j+1:]
	}
	parca := strings.Split(op, ".")
	isl, ok := bzIslemAdlari[parca[0]]
	if !ok {
		return fmt.Errorf("benzetimin bilmediği komut %s", parca[0])
	}
	k.islem = isl
	if err := a.ekler(&k, parca[1:]); err != nil {
		return err
	}
	var terimler []string
	if args != "" {
		terimler = strings.Split(args, ",")
		for i := range terimler {
			terimler[i] = strings.TrimSpace(terimler[i])
		}
	}
	if err := a.vektor(&k, terimler, vekListe); err != nil {
		return err
	}
	if err := a.terimler(&k, terimler); err != nil {
		return err
	}
	a.is.komutlar = append(a.is.komutlar, k)
	return nil
}

// ekler komutun nokta eklerini (tür, kip, alan) çözer.
func (a *bzAyristirici) ekler(k *bzKomut, ek []string) error {
	var turler []bzTur
	for _, e := range ek {
		if t, ok := bzTurAdlari[e]; ok {
			turler = append(turler, t)
			continue
		}
		switch e {
		case "lo":
			k.kip = 1
		case "wide":
			if k.islem == bzMul {
				k.islem = bzMulWide
			} else {
				k.islem = bzMadWide
			}
		case "rn", "approx", "uni", "sync", "to", "async", "ca", "cg":
		case "v2":
			k.vek = 2
		case "v4":
			k.vek = 4
		case "commit_group":
			k.islem = bzCpGrup
		case "wait_group":
			k.islem = bzCpBekle
		case "rni":
			k.kip = 1
		case "rzi":
			k.kip = 2
		case "rmi":
			k.kip = 3
		case "rpi":
			k.kip = 4
		case "param":
			k.alan = alanParam
		case "global":
			k.alan = alanGenel
		case "shared":
			k.alan = alanOrtak
		default:
			kp, ok := bzKarsilastirma[e]
			if !ok || k.islem != bzSetp {
				return fmt.Errorf("benzetimin bilmediği ek .%s", e)
			}
			k.kip = kp
		}
	}
	if len(turler) > 0 {
		k.tur = turler[0]
	}
	if len(turler) > 1 {
		k.kaynak = turler[1]
	}
	if (k.islem == bzMul || k.islem == bzMad) && k.tur != bzF32 && k.tur != bzF64 && k.kip != 1 {
		return fmt.Errorf("tam sayı çarpımı .lo ya da .wide ister")
	}
	return nil
}

func (a *bzAyristirici) terimler(k *bzKomut, t []string) error {
	switch k.islem {
	case bzBra:
		if len(t) != 1 {
			return fmt.Errorf("bra tek etiket ister")
		}
		a.bekleyen = append(a.bekleyen, struct {
			komut int
			ad    string
		}{len(a.is.komutlar), t[0]})
		return nil
	case bzBar, bzRet, bzCpGrup:
		return nil
	case bzCpBekle:
		if len(t) != 1 {
			return fmt.Errorf("cp.async.wait_group tek sayı ister")
		}
		v, err := a.terim(t[0], bzU32)
		k.o[0] = v
		return err
	case bzCp:
		return a.cpTerimleri(k, t)
	case bzSt:
		if len(t) != 2 {
			return fmt.Errorf("st iki işlenen ister")
		}
		if err := a.bellek(k, t[0]); err != nil {
			return err
		}
		if k.vek > 0 {
			return nil // kaynaklar vektörde (vo)
		}
		v, err := a.terim(t[1], k.tur)
		k.o[0] = v
		return err
	case bzLd:
		if len(t) != 2 {
			return fmt.Errorf("ld iki işlenen ister")
		}
		if k.vek == 0 {
			if err := a.hedefYazmac(k, t[0]); err != nil {
				return err
			}
		}
		return a.bellek(k, t[1])
	}
	if len(t) < 2 || len(t) > 4 {
		return fmt.Errorf("işlenen sayısı %d", len(t))
	}
	if err := a.hedefYazmac(k, t[0]); err != nil {
		return err
	}
	kTur := k.tur
	if k.islem == bzCvt {
		kTur = k.kaynak
	}
	for i, s := range t[1:] {
		v, err := a.terim(s, kTur)
		if err != nil {
			return err
		}
		k.o[i] = v
	}
	return nil
}

func (a *bzAyristirici) hedefYazmac(k *bzKomut, s string) error {
	y, ok := a.yazmac[s]
	if !ok {
		return fmt.Errorf("tanımsız hedef yazmaç %s", s)
	}
	k.d = y
	return nil
}

// terim bir kaynak işleneni çözer: yazmaç, özel yazmaç, paylaşımlı bellek sembolü ya da
// sabit (ondalık tam sayı, 0f/0d onaltılık kayan nokta).
func (a *bzAyristirici) terim(s string, tur bzTur) (bzTerim, error) {
	if y, ok := a.yazmac[s]; ok {
		return bzTerim{cesit: tmYazmac, y: y}, nil
	}
	if o, ok := bzOzeller[s]; ok {
		return bzTerim{cesit: tmOzel, y: int32(o)}, nil
	}
	if o, ok := a.ortak[s]; ok {
		return bzTerim{cesit: tmSabit, s: uint64(o)}, nil
	}
	switch {
	case strings.HasPrefix(s, "0f") && len(s) == 10:
		v, err := strconv.ParseUint(s[2:], 16, 32)
		if tur == bzF64 {
			v = math.Float64bits(float64(math.Float32frombits(uint32(v))))
		}
		return bzTerim{cesit: tmSabit, s: v}, err
	case strings.HasPrefix(s, "0d") && len(s) == 18:
		v, err := strconv.ParseUint(s[2:], 16, 64)
		return bzTerim{cesit: tmSabit, s: v}, err
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return bzTerim{}, fmt.Errorf("çözülemeyen işlenen %q", s)
	}
	if tur == bzF32 || tur == bzF64 {
		return bzTerim{}, fmt.Errorf("kayan noktalı komutta tam sayı sabiti %q (0f biçimi gerekir)", s)
	}
	return bzTerim{cesit: tmSabit, s: uint64(n)}, nil
}

// bellek [taban+ofset] işlenenini çözer.
func (a *bzAyristirici) bellek(k *bzKomut, s string) error {
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return fmt.Errorf("bellek işleneni bekleniyordu: %s", s)
	}
	s = s[1 : len(s)-1]
	taban := s
	if i := strings.IndexByte(s, '+'); i >= 0 {
		taban = s[:i]
		n, err := strconv.ParseInt(s[i+1:], 10, 64)
		if err != nil {
			return err
		}
		k.ofset = n
	}
	if k.alan == alanParam {
		p, ok := a.paramNo[taban]
		if !ok {
			return fmt.Errorf("tanımsız parametre %s", taban)
		}
		k.param = p
		return nil
	}
	t, err := a.terim(taban, bzU64)
	k.taban = t
	return err
}
