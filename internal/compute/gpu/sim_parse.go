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

// simOp komut ailesi.
type simOp uint8

const (
	simMov simOp = iota
	simAdd
	simSub
	simMul
	simMulWide
	simMad
	simMadWide
	simFma
	simDiv
	simRem
	simNeg
	simAbs
	simMin
	simMax
	simSqrt
	simRcp
	simEx2
	simLg2
	simSin
	simCos
	simShl
	simShr
	simAnd
	simOr
	simXor
	simNot
	simSetp
	simSelp
	simCvt
	simCvta
	simLd
	simSt
	simBra
	simBar
	simRet
	simCp      // cp.async (genelden paylaşımlıya eşzamansız kopya)
	simCpGroup // cp.async.commit_group
	simCpWait  // cp.async.wait_group
	simMma     // mma.sync (warp düzeyi matris çarpımı; bkz. sim_warp.go)
	simShfl    // shfl.sync (warp içi değiş tokuş)
	simCvtPack // cvt.rn.bf16x2.f32 / cvt.rn.f16x2.f32 (iki float32'yi 16 bitlik çifte)
)

var simOpNames = map[string]simOp{
	"mov": simMov, "add": simAdd, "sub": simSub, "mul": simMul, "mad": simMad, "fma": simFma,
	"div": simDiv, "rem": simRem, "neg": simNeg, "abs": simAbs, "min": simMin, "max": simMax,
	"sqrt": simSqrt, "rcp": simRcp, "ex2": simEx2, "lg2": simLg2, "sin": simSin, "cos": simCos,
	"shl": simShl, "shr": simShr, "and": simAnd, "or": simOr, "xor": simXor, "not": simNot,
	"setp": simSetp, "selp": simSelp, "cvt": simCvt, "cvta": simCvta, "ld": simLd, "st": simSt,
	"bra": simBra, "bar": simBar, "ret": simRet, "cp": simCp,
}

// simType işlenen türü.
type simType uint8

const (
	simNone simType = iota
	simU16
	simU32
	simS32
	simU64
	simS64
	simF16
	simF32
	simF64
	simPred
)

var simTypeNames = map[string]simType{
	"b16": simU16, "u16": simU16, "s16": simU16, "f16": simF16,
	"b32": simU32, "u32": simU32, "s32": simS32, "f32": simF32,
	"b64": simU64, "u64": simU64, "s64": simS64, "f64": simF64, "pred": simPred,
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

var simCompare = map[string]int{
	"eq": kEq, "ne": kNe, "lt": kLt, "le": kLe, "gt": kGt, "ge": kGe, "equ": kEqu, "neu": kNeu,
	"ltu": kLtu, "leu": kLeu, "gtu": kGtu, "geu": kGeu, "num": kNum, "nan": kNan,
}

// Bellek alanları.
const (
	spaceParam = iota
	spaceGlobal
	spaceShared
)

// Terim çeşitleri.
const (
	tmReg = iota
	tmConst
	tmSpecial
)

// Özel yazmaçlar.
var simSpecials = map[string]int{
	"%tid.x": 0, "%tid.y": 1, "%tid.z": 2, "%ntid.x": 3, "%ntid.y": 4, "%ntid.z": 5,
	"%ctaid.x": 6, "%ctaid.y": 7, "%ctaid.z": 8, "%nctaid.x": 9, "%nctaid.y": 10, "%nctaid.z": 11,
}

type simTerm struct {
	variant uint8
	y       int32  // yazmaç yuvası ya da özel yazmaç numarası
	s       uint64 // sabit (bitler)
}

type simInst struct {
	op      simOp
	dtype   simType // işlem türü (cvt'de hedef türü)
	source  simType // cvt kaynak türü
	mode2   int     // setp karşılaştırması; cvt yuvarlama; mul/mad: 1 = .lo
	cond    int32   // koşul yüklemi yuvası (-1 yok)
	condNot bool
	d       int32 // hedef yazmaç (-1 yok)
	o       [3]simTerm
	space   int     // ld/st bellek alanı
	base2   simTerm // ld/st adres tabanı (yazmaç ya da sembol sabiti)
	offset  int64
	param   int32 // ld.param: parametre sırası
	target  int32 // bra hedefi
	row     string
	// Vektörel erişim (.v2/.v4): vek eleman sayısı; ld hedefleri vy, st kaynakları vo.
	vec uint8
	vy  [4]int32
	vo  [4]simTerm
	// cp.async: kaynak (genel bellek) adresi, kopya boyu (o[0]) ve kaynak boyu (o[1]).
	baseB   simTerm
	offset2 int64
	// mma.sync işlenenleri (yalnızca simMma).
	mma *simMmaArgs
}

type simFunc struct {
	ad          string
	param       []paramType
	regCount    int
	sharedBytes int
	// dynamic: devingen paylaşımlı dizi (.extern .shared) statik dizilerden sonra
	// (ortakBayt'ta) başlar; boyu başlatmada verilir.
	dynamic      bool
	instructions []simInst
}

// simParser tek bir modülün ayrıştırma durumu.
type simParser struct {
	is         *simFunc
	reg        map[string]int32
	shared     map[string]int // sembol → paylaşımlı bellek ofseti
	paramIndex map[string]int32
	label      map[string]int32
	pending    []struct {
		instruction int
		ad          string
	}
}

// simParse PTX modülünü çözer (tek giriş noktası).
func simParse(text string) (*simFunc, error) {
	a := &simParser{is: &simFunc{}, reg: map[string]int32{}, shared: map[string]int{},
		paramIndex: map[string]int32{}, label: map[string]int32{}}
	rows := strings.Split(text, "\n")
	body := false
	for no, s := range rows {
		s = strings.TrimSpace(s)
		if i := strings.Index(s, "//"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
		if s == "" {
			continue
		}
		if err := a.row(s, &body); err != nil {
			return nil, fmt.Errorf("benzetim: PTX satır %d (%q): %v", no+1, s, err)
		}
	}
	for _, b := range a.pending {
		h, ok := a.label[b.ad]
		if !ok {
			return nil, fmt.Errorf("benzetim: tanımsız etiket %s", b.ad)
		}
		a.is.instructions[b.instruction].target = h
	}
	if a.is.ad == "" {
		return nil, fmt.Errorf("benzetim: giriş noktası (.entry) yok")
	}
	a.is.regCount = len(a.reg)
	if a.is.sharedBytes > staticSharedMemLimit {
		return nil, fmt.Errorf("benzetim: %d bayt statik paylaşımlı bellek (sınır %d; üstü devingen diziyle)", a.is.sharedBytes, staticSharedMemLimit)
	}
	return a.is, nil
}

func (a *simParser) row(s string, body *bool) error {
	switch {
	case strings.HasPrefix(s, ".version"), strings.HasPrefix(s, ".target"), strings.HasPrefix(s, ".address_size"):
		return nil
	case strings.HasPrefix(s, ".visible .entry "):
		ad := strings.TrimSuffix(strings.TrimPrefix(s, ".visible .entry "), "(")
		a.is.ad = strings.TrimSpace(ad)
		return nil
	case strings.HasPrefix(s, ".param "):
		return a.declareParam(strings.TrimSuffix(s, ","))
	case s == ")":
		return nil
	case s == "{":
		*body = true
		return nil
	case s == "}":
		*body = false
		return nil
	case strings.HasPrefix(s, ".reg "):
		return a.declareReg(strings.TrimSuffix(s, ";"))
	case strings.HasPrefix(s, ".shared "):
		return a.declareShared(strings.TrimSuffix(s, ";"))
	case strings.HasPrefix(s, ".extern .shared "):
		if *body {
			return fmt.Errorf(".extern .shared modül düzeyinde bildirilmeli (ptxas gövdede kabul etmez)")
		}
		return a.declareDynamic(strings.TrimSuffix(s, ";"))
	case strings.HasSuffix(s, ":"):
		a.label[strings.TrimSuffix(s, ":")] = int32(len(a.is.instructions))
		return nil
	}
	if !*body {
		return fmt.Errorf("gövde dışında komut")
	}
	return a.parseInstruction(strings.TrimSuffix(s, ";"))
}

func (a *simParser) declareParam(s string) error {
	f := strings.Fields(s)
	ad := f[len(f)-1]
	byteN := 0
	switch {
	case strings.Contains(ad, "["):
		i := strings.Index(ad, "[")
		n, err := strconv.Atoi(strings.TrimSuffix(ad[i+1:], "]"))
		if err != nil {
			return err
		}
		ad, byteN = ad[:i], n
	case f[1] == ".u64":
		byteN = 8
	case f[1] == ".u32", f[1] == ".f32", f[1] == ".s32":
		byteN = 4
	default:
		return fmt.Errorf("bilinmeyen parametre türü %s", f[1])
	}
	a.paramIndex[ad] = int32(len(a.is.param))
	a.is.param = append(a.is.param, paramType{ad: ad, byteN: byteN})
	return nil
}

func (a *simParser) declareReg(s string) error {
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
		a.reg[f[2][:i]+strconv.Itoa(k)] = int32(len(a.reg))
	}
	return nil
}

// declareDynamic .extern .shared .align 16 .b8 ad[]: devingen dizi statik dizilerden sonra,
// 16'ya hizalı başlar; ardından statik dizi bildirilemez (tek devingen dizi).
func (a *simParser) declareDynamic(s string) error {
	f := strings.Fields(s)
	last := f[len(f)-1]
	if !strings.HasSuffix(last, "[]") || a.is.dynamic {
		return fmt.Errorf("devingen paylaşımlı dizi bildirimi")
	}
	a.is.dynamic = true
	a.is.sharedBytes = (a.is.sharedBytes + 15) &^ 15
	a.shared[strings.TrimSuffix(last, "[]")] = a.is.sharedBytes
	return nil
}

func (a *simParser) declareShared(s string) error {
	if a.is.dynamic {
		return fmt.Errorf("devingen paylaşımlı diziden sonra statik dizi")
	}
	f := strings.Fields(s) // .shared .align 16 .b8 ad[4096]
	last := f[len(f)-1]
	i := strings.Index(last, "[")
	n, err := strconv.Atoi(strings.TrimSuffix(last[i+1:], "]"))
	if err != nil {
		return err
	}
	a.is.sharedBytes = (a.is.sharedBytes + 15) &^ 15
	a.shared[last[:i]] = a.is.sharedBytes
	a.is.sharedBytes += n
	return nil
}

func (a *simParser) parseInstruction(s string) error {
	k := simInst{cond: -1, d: -1, row: s}
	if strings.HasPrefix(s, "@") {
		sp := strings.IndexByte(s, ' ')
		p := s[1:sp]
		if strings.HasPrefix(p, "!") {
			k.condNot = true
			p = p[1:]
		}
		y, ok := a.reg[p]
		if !ok {
			return fmt.Errorf("tanımsız yüklem %s", p)
		}
		k.cond = y
		s = strings.TrimSpace(s[sp+1:])
	}
	op, args := s, ""
	if sp := strings.IndexByte(s, ' '); sp >= 0 {
		op, args = s[:sp], strings.TrimSpace(s[sp+1:])
	}
	if ok, err := a.parseWarp(&k, op, args); ok {
		if err == nil {
			a.is.instructions = append(a.is.instructions, k)
		}
		return err
	}
	// Vektör işleneni ({a, b, c, d}) tek terim olarak yer tutucuyla değiştirilir.
	var vecList []string
	if i := strings.IndexByte(args, '{'); i >= 0 {
		j := strings.IndexByte(args, '}')
		if j < i {
			return fmt.Errorf("kapanmamış vektör işleneni")
		}
		for _, v := range strings.Split(args[i+1:j], ",") {
			vecList = append(vecList, strings.TrimSpace(v))
		}
		args = args[:i] + "@V" + args[j+1:]
	}
	part := strings.Split(op, ".")
	op2, ok := simOpNames[part[0]]
	if !ok {
		return fmt.Errorf("benzetimin bilmediği komut %s", part[0])
	}
	k.op = op2
	if err := a.extras(&k, part[1:]); err != nil {
		return err
	}
	var terms []string
	if args != "" {
		terms = strings.Split(args, ",")
		for i := range terms {
			terms[i] = strings.TrimSpace(terms[i])
		}
	}
	if err := a.vector(&k, terms, vecList); err != nil {
		return err
	}
	if err := a.terms(&k, terms); err != nil {
		return err
	}
	a.is.instructions = append(a.is.instructions, k)
	return nil
}

func (a *simParser) terms(k *simInst, t []string) error {
	switch k.op {
	case simBra:
		if len(t) != 1 {
			return fmt.Errorf("bra tek etiket ister")
		}
		a.pending = append(a.pending, struct {
			instruction int
			ad          string
		}{len(a.is.instructions), t[0]})
		return nil
	case simBar, simRet, simCpGroup:
		return nil
	case simCpWait:
		if len(t) != 1 {
			return fmt.Errorf("cp.async.wait_group tek sayı ister")
		}
		v, err := a.term(t[0], simU32)
		k.o[0] = v
		return err
	case simCp:
		return a.cpTerms(k, t)
	case simSt:
		if len(t) != 2 {
			return fmt.Errorf("st iki işlenen ister")
		}
		if err := a.memory(k, t[0]); err != nil {
			return err
		}
		if k.vec > 0 {
			return nil // kaynaklar vektörde (vo)
		}
		v, err := a.term(t[1], k.dtype)
		k.o[0] = v
		return err
	case simLd:
		if len(t) != 2 {
			return fmt.Errorf("ld iki işlenen ister")
		}
		if k.vec == 0 {
			if err := a.targetReg(k, t[0]); err != nil {
				return err
			}
		}
		return a.memory(k, t[1])
	}
	if len(t) < 2 || len(t) > 4 {
		return fmt.Errorf("işlenen sayısı %d", len(t))
	}
	if err := a.targetReg(k, t[0]); err != nil {
		return err
	}
	kKind := k.dtype
	if k.op == simCvt {
		kKind = k.source
	}
	for i, s := range t[1:] {
		v, err := a.term(s, kKind)
		if err != nil {
			return err
		}
		k.o[i] = v
	}
	return nil
}

func (a *simParser) targetReg(k *simInst, s string) error {
	y, ok := a.reg[s]
	if !ok {
		return fmt.Errorf("tanımsız hedef yazmaç %s", s)
	}
	k.d = y
	return nil
}

// term bir kaynak işleneni çözer: yazmaç, özel yazmaç, paylaşımlı bellek sembolü ya da
// sabit (ondalık tam sayı, 0f/0d onaltılık kayan nokta).
func (a *simParser) term(s string, dtype simType) (simTerm, error) {
	if y, ok := a.reg[s]; ok {
		return simTerm{variant: tmReg, y: y}, nil
	}
	if o, ok := simSpecials[s]; ok {
		return simTerm{variant: tmSpecial, y: int32(o)}, nil
	}
	if o, ok := a.shared[s]; ok {
		return simTerm{variant: tmConst, s: uint64(o)}, nil
	}
	switch {
	case strings.HasPrefix(s, "0f") && len(s) == 10:
		v, err := strconv.ParseUint(s[2:], 16, 32)
		if dtype == simF64 {
			v = math.Float64bits(float64(math.Float32frombits(uint32(v))))
		}
		return simTerm{variant: tmConst, s: v}, err
	case strings.HasPrefix(s, "0d") && len(s) == 18:
		v, err := strconv.ParseUint(s[2:], 16, 64)
		return simTerm{variant: tmConst, s: v}, err
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return simTerm{}, fmt.Errorf("çözülemeyen işlenen %q", s)
	}
	if dtype == simF32 || dtype == simF64 {
		return simTerm{}, fmt.Errorf("kayan noktalı komutta tam sayı sabiti %q (0f biçimi gerekir)", s)
	}
	return simTerm{variant: tmConst, s: uint64(n)}, nil
}

// memory [taban+ofset] işlenenini çözer.
func (a *simParser) memory(k *simInst, s string) error {
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return fmt.Errorf("bellek işleneni bekleniyordu: %s", s)
	}
	s = s[1 : len(s)-1]
	base2 := s
	if i := strings.IndexByte(s, '+'); i >= 0 {
		base2 = s[:i]
		n, err := strconv.ParseInt(s[i+1:], 10, 64)
		if err != nil {
			return err
		}
		k.offset = n
	}
	if k.space == spaceParam {
		p, ok := a.paramIndex[base2]
		if !ok {
			return fmt.Errorf("tanımsız parametre %s", base2)
		}
		k.param = p
		return nil
	}
	t, err := a.term(base2, simU64)
	k.base2 = t
	return err
}
