package gpu

import (
	"fmt"
	"math"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// PTX benzetimi: warp düzeyi komutlar
// ============================================================================
//
// mma.sync ve shfl.sync bir warp'ın 32 iş parçacığının hepsinin aynı komuta gelmesini ister:
// her iş parçacığı yalnızca kendi parçasını tutar, sonuç bütün şeridin yazmaçlarından
// hesaplanır. Yürütücü bu komutlarda iş parçacığını durdurur (dAtWarp); bloğun bütün iş
// parçacıkları bir engele, bir warp komutuna ya da sona varınca, bütün şeridi aynı warp
// komutunda olan warp'lar birlikte yürütülür. Şeridin bir kısmı başka yerdeyse (dağınık warp)
// hatadır: donanımda .sync komutları bütün şeridin katılımını ister.
//
// Desteklenen biçimler:
//   mma.sync.aligned.m16n8k16.row.col.f32.{bf16,f16}.{bf16,f16}.f32 {d0..d3}, {a0..a3}, {b0,b1}, {c0..c3}
//   shfl.sync.bfly.b32 d, a, şerit_maskesi, 31, -1
//   cvt.rn.{bf16x2,f16x2}.f32 d, üst, alt

// simMmaArgs mma.sync işlenenleri.
type simMmaArgs struct {
	d    [4]int32
	a, c [4]simTerm
	b    [2]simTerm
	f16  bool // girdiler f16 (değilse bf16)
}

// parseWarp warp düzeyi komutları ve paketli dönüşümü çözer; komut bunlardan değilse ok
// yanlıştır.
func (a *simParser) parseWarp(k *simInst, op, args string) (ok bool, err error) {
	switch {
	case strings.HasPrefix(op, "mma."):
		return true, a.parseMma(k, op, args)
	case strings.HasPrefix(op, "shfl."):
		return true, a.parseShfl(k, op, args)
	case op == "cvt.rn.bf16x2.f32" || op == "cvt.rn.f16x2.f32":
		k.op, k.dtype = simCvtPack, simU32
		if op == "cvt.rn.f16x2.f32" {
			k.mode2 = 1
		}
		t := splitTerms(args)
		if len(t) != 3 {
			return true, fmt.Errorf("%s üç işlenen ister", op)
		}
		if err := a.targetReg(k, t[0]); err != nil {
			return true, err
		}
		for i, s := range t[1:] {
			if k.o[i], err = a.term(s, simF32); err != nil {
				return true, err
			}
		}
		return true, nil
	}
	return false, nil
}

func splitTerms(args string) []string {
	if args == "" {
		return nil
	}
	t := strings.Split(args, ",")
	for i := range t {
		t[i] = strings.TrimSpace(t[i])
	}
	return t
}

func (a *simParser) parseMma(k *simInst, op, args string) error {
	m := &simMmaArgs{}
	switch op {
	case "mma.sync.aligned.m16n8k16.row.col.f32.bf16.bf16.f32":
	case "mma.sync.aligned.m16n8k16.row.col.f32.f16.f16.f32":
		m.f16 = true
	default:
		return fmt.Errorf("benzetimin bilmediği mma biçimi %s", op)
	}
	var groups [][]string
	for len(groups) < 4 {
		i, j := strings.IndexByte(args, '{'), strings.IndexByte(args, '}')
		if i < 0 || j < i {
			return fmt.Errorf("mma dört süslü parantezli işlenen ister")
		}
		groups = append(groups, splitTerms(args[i+1:j]))
		args = args[j+1:]
	}
	if len(groups[0]) != 4 || len(groups[1]) != 4 || len(groups[2]) != 2 || len(groups[3]) != 4 {
		return fmt.Errorf("mma işlenen sayıları 4, 4, 2, 4 olmalı")
	}
	for i, s := range groups[0] {
		y, ok := a.reg[s]
		if !ok {
			return fmt.Errorf("tanımsız hedef yazmaç %s", s)
		}
		m.d[i] = y
	}
	var err error
	for i, s := range groups[1] {
		if m.a[i], err = a.term(s, simU32); err != nil {
			return err
		}
	}
	for i, s := range groups[2] {
		if m.b[i], err = a.term(s, simU32); err != nil {
			return err
		}
	}
	for i, s := range groups[3] {
		if m.c[i], err = a.term(s, simF32); err != nil {
			return err
		}
	}
	k.op, k.mma = simMma, m
	return nil
}

func (a *simParser) parseShfl(k *simInst, op, args string) error {
	if op != "shfl.sync.bfly.b32" {
		return fmt.Errorf("benzetimin bilmediği shfl biçimi %s", op)
	}
	t := splitTerms(args)
	if len(t) != 5 || t[3] != "31" || t[4] != "-1" {
		return fmt.Errorf("shfl.sync.bfly.b32 d, a, maske, 31, -1 biçiminde olmalı")
	}
	if err := a.targetReg(k, t[0]); err != nil {
		return err
	}
	var err error
	if k.o[0], err = a.term(t[1], simU32); err != nil {
		return err
	}
	if k.o[1], err = a.term(t[2], simU32); err != nil {
		return err
	}
	k.op, k.dtype = simShfl, simU32
	return nil
}

// packPair iki float32'yi (en yakına-çifte) 16 bitlik çifte paketler: üst yarı hi, alt yarı lo
// (cvt.rn.bf16x2.f32 d, hi, lo).
func packPair(hi, lo uint64, f16 bool) uint64 {
	dt := tensor.BFloat16
	if f16 {
		dt = tensor.Float16
	}
	return uint64(tensor.Bits16(dt, f32(hi)))<<16 | uint64(tensor.Bits16(dt, f32(lo)))
}

// half16 16 bitlik bir girdiyi float64'e açar.
func half16(h uint16, f16 bool) float64 {
	if f16 {
		return float64(tensor.FromBits16(tensor.Float16, h))
	}
	return float64(tensor.FromBits16(tensor.BFloat16, h))
}

// warpStep bütün şeridi aynı warp komutunda bekleyen warp'ları yürütür.
func (b *simBlock) warpStep(jobs []simJob) error {
	if len(jobs)%32 != 0 {
		return fmt.Errorf("warp komutu 32'nin katı olmayan blokta (%d iş parçacığı)", len(jobs))
	}
	for w := 0; w < len(jobs); w += 32 {
		lanes := jobs[w : w+32]
		waiting := 0
		for i := range lanes {
			if lanes[i].state == dAtWarp {
				waiting++
			}
		}
		if waiting == 0 {
			continue
		}
		pc := lanes[0].pc
		for i := range lanes {
			if lanes[i].state != dAtWarp || lanes[i].pc != pc {
				b.errJob = fmt.Sprintf("warp %d", w/32)
				return fmt.Errorf("dağınık warp: şeridin bir kısmı warp komutunda (%s), bir kısmı başka yerde", b.is.instructions[lanes[0].pc].row)
			}
		}
		k := &b.is.instructions[pc]
		switch k.op {
		case simMma:
			b.mmaStep(lanes, k.mma)
		case simShfl:
			b.shflStep(lanes, k)
		}
		for i := range lanes {
			lanes[i].pc++
			lanes[i].state = dRunning
		}
	}
	return nil
}

// mmaStep m16n8k16 (A satır öncelikli 16×16, B sütun öncelikli 16×8, C/D 16×8 float32). Şerit
// l için g = l/4, t = l%4; A: a0 (g, 2t..2t+1), a1 (g+8, 2t..), a2 (g, 2t+8..), a3 (g+8, 2t+8..);
// B: b0 (k = 2t..2t+1, n = g), b1 (k = 2t+8.., n = g); C/D: c0 (g, 2t), c1 (g, 2t+1), c2 (g+8, 2t),
// c3 (g+8, 2t+1). 16 bitlik çiftte alt yarı küçük sütundur. Toplam float64'te tam yapılıp bir kez
// yuvarlanır (donanım birikimi en az float32 hassasiyetindedir).
func (b *simBlock) mmaStep(lanes []simJob, m *simMmaArgs) {
	var A [16][16]float64
	var B [16][8]float64
	var C [16][8]float64
	for l := range lanes {
		t := &lanes[l]
		g, q := l/4, l%4
		for i, ofs := range [4][2]int{{0, 0}, {8, 0}, {0, 8}, {8, 8}} {
			v := b.read(t, &m.a[i])
			A[g+ofs[0]][2*q+ofs[1]] = half16(uint16(v), m.f16)
			A[g+ofs[0]][2*q+ofs[1]+1] = half16(uint16(v>>16), m.f16)
		}
		for i := range 2 {
			v := b.read(t, &m.b[i])
			B[2*q+8*i][g] = half16(uint16(v), m.f16)
			B[2*q+8*i+1][g] = half16(uint16(v>>16), m.f16)
		}
		for i := range 4 {
			C[g+8*(i/2)][2*q+i%2] = float64(f32(b.read(t, &m.c[i])))
		}
	}
	for l := range lanes {
		g, q := l/4, l%4
		for i := range 4 {
			r, c := g+8*(i/2), 2*q+i%2
			s := C[r][c]
			for kk := range 16 {
				s += A[r][kk] * B[kk][c]
			}
			lanes[l].y[m.d[i]] = f32b(float32(s))
		}
	}
}

// shflStep shfl.sync.bfly: şerit l, l ^ maske şeridinin değerini alır.
func (b *simBlock) shflStep(lanes []simJob, k *simInst) {
	var v [32]uint64
	for l := range lanes {
		v[l] = b.read(&lanes[l], &k.o[0])
	}
	for l := range lanes {
		src := l ^ int(b.read(&lanes[l], &k.o[1])&31)
		lanes[l].y[k.d] = v[src] & math.MaxUint32
	}
}
