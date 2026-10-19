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
type simAlloc struct {
	base2 uint64
	data  []byte
}

// bzBellek benzetimin cihaz belleği.
type simMemory struct {
	regions []*simAlloc // tabana göre sıralı
	next    uint64
}

func (m *simMemory) allocate(n uint64) uint64 {
	if m.next == 0 {
		m.next = 1 << 40
	}
	base2 := m.next
	m.next += (n + 4096 + 255) &^ 255 // bölgeler arasında boşluk: taşmalar boşluğa düşer
	m.regions = append(m.regions, &simAlloc{base2: base2, data: make([]byte, n)})
	return base2
}

func (m *simMemory) release(base2 uint64) bool {
	i := sort.Search(len(m.regions), func(i int) bool { return m.regions[i].base2 >= base2 })
	if i == len(m.regions) || m.regions[i].base2 != base2 {
		return false
	}
	m.regions = append(m.regions[:i], m.regions[i+1:]...)
	return true
}

// dilim adresteki n baytı verir; bölge dışıysa hata.
func (m *simMemory) slice(addr uint64, n int) ([]byte, error) {
	i := sort.Search(len(m.regions), func(i int) bool { return m.regions[i].base2 > addr }) - 1
	if i >= 0 {
		b := m.regions[i]
		if o := addr - b.base2; o+uint64(n) <= uint64(len(b.data)) {
			return b.data[o : o+uint64(n)], nil
		}
	}
	return nil, fmt.Errorf("ayrılmış bölge dışında %d baytlık genel bellek erişimi: 0x%x", n, addr)
}

// Yürütme durumları.
const (
	dRunning = iota
	dAtBarrier
	dDone
)

type simJob struct {
	y     []uint64
	pc    int32
	state uint8
	no    int32 // bloktaki doğrusal sıra
	tid   [3]uint32
	// cp.async: bekleyen kopyalar ve açık grubun numarası.
	cp      []simCpJob
	cpGroup int32
}

type simBlock struct {
	is      *simFunc
	param   [][]byte
	shared  []byte
	mem     *simMemory
	special [12]uint32 // tid'ler iş parçacığına göre doldurulur
	budget  *int64
	errJob  string
	// Paylaşımlı bellek yarış dedektörü: her 4 baytlık sözcük için, iki engel arasındaki
	// (dönem) son yazan ve okuyan iş parçacığı (-1 yok, -2 birden çok okuyan). Aynı dönemde
	// bir iş parçacığının yazdığı sözcüğü başkasının okuması ya da yazması, ya da birinin
	// okuduğunu başkasının yazması yarıştır: donanımda sonuç zamanlamaya bağlıdır.
	raceWriter []int32
	raceReader []int32
	raceEpoch  []uint32
	epoch      uint32
}

// yaris paylaşımlı bellek erişimini yarış dedektörüne bildirir.
func (b *simBlock) race(t *simJob, addr, n int, writing bool) error {
	for w := addr / 4; w <= (addr+n-1)/4; w++ {
		if b.raceEpoch[w] != b.epoch {
			b.raceEpoch[w], b.raceWriter[w], b.raceReader[w] = b.epoch, -1, -1
		}
		yz, ok := b.raceWriter[w], b.raceReader[w]
		if yz >= 0 && yz != t.no {
			return fmt.Errorf("paylaşımlı bellek yarışı: %d. bayt aynı engel aralığında %d. iş parçacığınca yazıldı, %d. iş parçacığınca da %s",
				4*w, yz, t.no, map[bool]string{true: "yazılıyor", false: "okunuyor"}[writing])
		}
		if writing {
			if ok == -2 || (ok >= 0 && ok != t.no) {
				return fmt.Errorf("paylaşımlı bellek yarışı: %d. bayt aynı engel aralığında başka bir iş parçacığınca okundu, %d. iş parçacığınca yazılıyor", 4*w, t.no)
			}
			b.raceWriter[w] = t.no
			continue
		}
		switch {
		case ok == -1:
			b.raceReader[w] = t.no
		case ok != t.no:
			b.raceReader[w] = -2
		}
	}
	return nil
}

// bzBaslat bir çekirdeği bütün ızgarada yürütür.
func simLaunch(is *simFunc, mem *simMemory, grid, block [3]uint32, dynamic int, param [][]byte) error {
	if len(param) != len(is.param) {
		return fmt.Errorf("%s: %d parametre bekleniyordu, %d verildi", is.ad, len(is.param), len(param))
	}
	for i, p := range is.param {
		if len(param[i]) != p.byteN {
			return fmt.Errorf("%s: %s parametresi %d bayt olmalı, %d verildi", is.ad, p.ad, p.byteN, len(param[i]))
		}
	}
	n := int(block[0] * block[1] * block[2])
	if n == 0 || n > 1024 {
		return fmt.Errorf("%s: blok başına %d iş parçacığı (1..1024 olmalı)", is.ad, n)
	}
	if grid[1] > 65535 || grid[2] > 65535 {
		return fmt.Errorf("%s: ızgaranın y/z boyutu 65535'i aşıyor (%v)", is.ad, grid)
	}
	budget := int64(4e9)
	sharedBytes := is.sharedBytes + dynamic
	b := &simBlock{is: is, param: param, shared: make([]byte, sharedBytes), mem: mem, budget: &budget}
	word := (sharedBytes + 3) / 4
	b.raceWriter, b.raceReader, b.raceEpoch = make([]int32, word), make([]int32, word), make([]uint32, word)
	b.special[3], b.special[4], b.special[5] = block[0], block[1], block[2]
	b.special[9], b.special[10], b.special[11] = grid[0], grid[1], grid[2]
	jobs := make([]simJob, n)
	reg := make([]uint64, n*is.regCount)
	for i := range jobs {
		jobs[i].y = reg[i*is.regCount : (i+1)*is.regCount]
		jobs[i].no = int32(i)
		jobs[i].tid = [3]uint32{uint32(i) % block[0], uint32(i) / block[0] % block[1], uint32(i) / (block[0] * block[1])}
	}
	for z := uint32(0); z < grid[2]; z++ {
		for y := uint32(0); y < grid[1]; y++ {
			for x := uint32(0); x < grid[0]; x++ {
				b.special[6], b.special[7], b.special[8] = x, y, z
				clear(b.shared)
				clear(reg)
				if err := b.execute(jobs); err != nil {
					return fmt.Errorf("%s, blok (%d,%d,%d), %s: %v", is.ad, x, y, z, b.errJob, err)
				}
			}
		}
	}
	return nil
}

func (b *simBlock) execute(jobs []simJob) error {
	for i := range jobs {
		jobs[i].pc, jobs[i].state = 0, dRunning
		jobs[i].cp, jobs[i].cpGroup = jobs[i].cp[:0], 0
	}
	b.epoch++
	for {
		atBarrier, finished := 0, 0
		for i := range jobs {
			t := &jobs[i]
			if t.state == dRunning {
				if err := b.run(t); err != nil {
					b.errJob = fmt.Sprintf("iş parçacığı %v, komut %d (%s)", t.tid, t.pc, b.is.instructions[t.pc].row)
					return err
				}
			}
			switch t.state {
			case dAtBarrier:
				atBarrier++
			case dDone:
				finished++
			}
		}
		if atBarrier == 0 {
			return nil
		}
		if finished > 0 {
			b.errJob = "engel"
			return fmt.Errorf("dağınık engel: %d iş parçacığı bitmişken %d tanesi bar.sync'te bekliyor", finished, atBarrier)
		}
		for i := range jobs {
			jobs[i].state = dRunning
		}
		b.epoch++
	}
}

func (b *simBlock) read(t *simJob, o *simTerm) uint64 {
	switch o.variant {
	case tmReg:
		return t.y[o.y]
	case tmConst:
		return o.s
	}
	if o.y < 3 {
		return uint64(t.tid[o.y])
	}
	return uint64(b.special[o.y])
}

// calistir iş parçacığını bir engele ya da sona kadar yürütür.
func (b *simBlock) run(t *simJob) error {
	inst := b.is.instructions
	for {
		if int(t.pc) >= len(inst) {
			return b.finish(t)
		}
		*b.budget--
		if *b.budget < 0 {
			return fmt.Errorf("komut bütçesi aşıldı (sonsuz döngü?)")
		}
		k := &inst[t.pc]
		if k.cond >= 0 && (t.y[k.cond] != 0) == k.condNot {
			t.pc++
			continue
		}
		switch k.op {
		case simBra:
			t.pc = k.target
			continue
		case simBar:
			t.pc++
			t.state = dAtBarrier
			return nil
		case simRet:
			return b.finish(t)
		case simCp, simCpGroup, simCpWait:
			if err := b.cpRun(t, k); err != nil {
				return err
			}
		case simLd:
			if err := b.load(t, k); err != nil {
				return err
			}
		case simSt:
			if err := b.store(t, k); err != nil {
				return err
			}
		default:
			if err := b.compute(t, k); err != nil {
				return err
			}
		}
		t.pc++
	}
}

// turBayt bir türün bellekteki baytı.
func dtypeBytes(t simType) int {
	switch t {
	case simU16, simF16:
		return 2
	case simU64, simS64, simF64:
		return 8
	}
	return 4
}

func (b *simBlock) addressSlice(t *simJob, k *simInst, n int, writing bool) ([]byte, error) {
	switch k.space {
	case spaceParam:
		p := b.param[k.param]
		if k.offset < 0 || int(k.offset)+n > len(p) {
			return nil, fmt.Errorf("parametre dışı okuma")
		}
		return p[k.offset : int(k.offset)+n], nil
	case spaceShared:
		addr := int64(uint32(b.read(t, &k.base2))) + k.offset
		if addr%int64(n) != 0 {
			return nil, fmt.Errorf("hizasız paylaşımlı bellek erişimi %d", addr)
		}
		if addr < 0 || int(addr)+n > len(b.shared) {
			return nil, fmt.Errorf("paylaşımlı bellek dışı erişim %d (boyut %d)", addr, len(b.shared))
		}
		if err := b.race(t, int(addr), n, writing); err != nil {
			return nil, err
		}
		return b.shared[addr : int(addr)+n], nil
	}
	addr := b.read(t, &k.base2) + uint64(k.offset)
	if addr%uint64(n) != 0 {
		return nil, fmt.Errorf("hizasız genel bellek erişimi 0x%x (%d bayt)", addr, n)
	}
	return b.mem.slice(addr, n)
}

// bitir iş parçacığını bitirir; bekleyen eşzamansız kopyası kalmışsa hatadır.
func (b *simBlock) finish(t *simJob) error {
	if len(t.cp) > 0 {
		return fmt.Errorf("iş parçacığı %d cp.async kopyası beklenmeden bitti", len(t.cp))
	}
	t.state = dDone
	return nil
}

func (b *simBlock) load(t *simJob, k *simInst) error {
	if k.vec > 0 {
		return b.vecLoad(t, k)
	}
	n := dtypeBytes(k.dtype)
	d, err := b.addressSlice(t, k, n, false)
	if err != nil {
		return err
	}
	var v uint64
	switch n {
	case 2:
		v = uint64(binary.LittleEndian.Uint16(d))
	case 4:
		v = uint64(binary.LittleEndian.Uint32(d))
		if k.dtype == simS32 {
			v = uint64(uint32(v))
		}
	default:
		v = binary.LittleEndian.Uint64(d)
	}
	t.y[k.d] = v
	return nil
}

func (b *simBlock) store(t *simJob, k *simInst) error {
	if k.vec > 0 {
		return b.vecStore(t, k)
	}
	if k.space == spaceParam {
		return fmt.Errorf("parametre alanına yazılamaz")
	}
	n := dtypeBytes(k.dtype)
	d, err := b.addressSlice(t, k, n, true)
	if err != nil {
		return err
	}
	v := b.read(t, &k.o[0])
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
func width(t simType, v uint64) uint64 {
	switch t {
	case simU16, simF16:
		return v & 0xFFFF
	case simU32, simS32, simF32:
		return v & 0xFFFFFFFF
	case simPred:
		if v != 0 {
			return 1
		}
		return 0
	}
	return v
}

func (b *simBlock) compute(t *simJob, k *simInst) error {
	a := b.read(t, &k.o[0])
	c := b.read(t, &k.o[1])
	e := b.read(t, &k.o[2])
	var r uint64
	var err error
	switch {
	case k.op == simCvt:
		r, err = convert(k, a)
		if err == nil {
			t.y[k.d] = width(k.dtype, r)
		}
		return err
	case k.dtype == simF32:
		r, err = computeF32(k, a, c, e)
	case k.dtype == simF64:
		r, err = computeF64(k, a, c, e)
	default:
		r, err = computeInt(k, a, c, e)
	}
	if err != nil {
		return err
	}
	if k.op == simSetp {
		r = width(simPred, r)
	} else if k.op != simMulWide && k.op != simMadWide {
		r = width(k.dtype, r)
	}
	t.y[k.d] = r
	return nil
}

func computeInt(k *simInst, a, b, c uint64) (uint64, error) {
	signed := k.dtype == simS32 || k.dtype == simS64
	gen64 := k.dtype == simU64 || k.dtype == simS64
	sa, sb := int64(int32(a)), int64(int32(b))
	if gen64 {
		sa, sb = int64(a), int64(b)
	}
	switch k.op {
	case simMov, simCvta:
		return a, nil
	case simAdd:
		return a + b, nil
	case simSub:
		return a - b, nil
	case simMul:
		return a * b, nil
	case simMad:
		return a*b + c, nil
	case simMulWide, simMadWide:
		var r uint64
		if signed {
			r = uint64(int64(int32(a)) * int64(int32(b)))
		} else {
			r = uint64(uint32(a)) * uint64(uint32(b))
		}
		if k.op == simMadWide {
			r += c
		}
		return r, nil
	case simDiv, simRem:
		if width(k.dtype, b) == 0 {
			return 0, fmt.Errorf("tam sayı sıfıra bölme")
		}
		if signed {
			if k.op == simDiv {
				return uint64(sa / sb), nil
			}
			return uint64(sa % sb), nil
		}
		ua, ub := width(k.dtype, a), width(k.dtype, b)
		if k.op == simDiv {
			return ua / ub, nil
		}
		return ua % ub, nil
	case simMin, simMax:
		small := width(k.dtype, a) < width(k.dtype, b)
		if signed {
			small = sa < sb
		}
		if small == (k.op == simMin) {
			return a, nil
		}
		return b, nil
	}
	return computeBits(k, a, b, c, sa, signed)
}

// hesaplaBit tam sayı türlerinde işaret, kaydırma, mantık, seçme ve karşılaştırma.
func computeBits(k *simInst, a, b, c uint64, sa int64, signed bool) (uint64, error) {
	switch k.op {
	case simNeg:
		return -a, nil
	case simAbs:
		if sa < 0 {
			return uint64(-sa), nil
		}
		return a, nil
	case simShl:
		return a << (b & 63), nil
	case simShr:
		if signed {
			return uint64(sa >> (b & 63)), nil
		}
		return width(k.dtype, a) >> (b & 63), nil
	case simAnd:
		return a & b, nil
	case simOr:
		return a | b, nil
	case simXor:
		return a ^ b, nil
	case simNot:
		if k.dtype == simPred {
			return a ^ 1, nil
		}
		return ^a, nil
	case simSelp:
		if c != 0 {
			return a, nil
		}
		return b, nil
	case simSetp:
		sb := int64(int32(b))
		if k.dtype == simU64 || k.dtype == simS64 {
			sb = int64(b)
		}
		return compareInt(k, a, b, sa, sb, signed), nil
	case simCvt:
		return convert(k, a)
	}
	return 0, fmt.Errorf("benzetimde tam sayı türü için desteklenmeyen işlem")
}

func compareInt(k *simInst, a, b uint64, sa, sb int64, signed bool) uint64 {
	var small, equal bool
	if signed {
		small, equal = sa < sb, sa == sb
	} else {
		ua, ub := width(k.dtype, a), width(k.dtype, b)
		small, equal = ua < ub, ua == ub
	}
	var r bool
	switch k.mode2 {
	case kEq:
		r = equal
	case kNe:
		r = !equal
	case kLt:
		r = small
	case kLe:
		r = small || equal
	case kGt:
		r = !small && !equal
	case kGe:
		r = !small
	}
	if r {
		return 1
	}
	return 0
}
