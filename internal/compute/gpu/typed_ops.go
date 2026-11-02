package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Cinsli işlemler (tensor.TypedBackend): bugün GPU'da çalışan yolların cinsli hâli
// ============================================================================
//
// Bu dilimde GPU'da yalnız şunlar vardır (2a, adım 8): karşılaştırma (hesap ondalıkları,
// "int64", mantıkta == ve !=) → TR_Bool; mantık tensöründe &, |, ^ ve ~; TR_where (koşul TR_Bool,
// dallar aynı türde, her tür); "int64" ve mantıktan "float32"ye dönüşüm (maske ve kimlik
// tensörünün ondalık çekirdeğe girişi). Öbürleri ok=false döner: çağıran 15.48'in hassasiyet
// hatasını verir (genel çekirdekler 2b'de). Çekirdekler cihaz içidir: TR_replay kaydında da
// çalışır.

// gpuTypedOK işlemin bu dilimde GPU'da olup olmadığı.
func gpuTypedOK(op tensor.TypedOp, out tensor.DType, ins []tensor.DType) bool {
	switch op.Kind {
	case tensor.TCompare:
		a, b := ins[0], ins[1]
		switch {
		case a.IsFloat() && b.IsFloat():
			return true
		case a == tensor.Int64 && b == tensor.Int64:
			return true
		case a == tensor.Bool && b == tensor.Bool:
			c := tensor.CmpOp(op.Sub)
			return c == tensor.CmpEq || c == tensor.CmpNe
		}
	case tensor.TLogic:
		l := tensor.LogicOp(op.Sub)
		return out == tensor.Bool && ins[0] == tensor.Bool && ins[1] == tensor.Bool && l <= tensor.LogicXor
	case tensor.TNot:
		return ins[0] == tensor.Bool
	case tensor.TWhere:
		return ins[0] == tensor.Bool && ins[1] == out && ins[2] == out
	case tensor.TConvert:
		return out == tensor.Float32 && (ins[0] == tensor.Int64 || ins[0] == tensor.Bool)
	}
	return false
}

// Typed (tensor.TypedBackend) cinsli işlemi cihazda yapar.
func (g *backend) Typed(op tensor.TypedOp, out tensor.Storage, outShape []int, ins []tensor.View) (*tensor.ElemFault, bool) {
	g.enter()
	defer g.exit()
	o := g.storageOf(out)
	dts := make([]tensor.DType, len(ins))
	for i, v := range ins {
		dts[i] = g.storageOf(v.S).dtype
	}
	if !gpuTypedOK(op, o.dtype, dts) {
		return nil, false
	}
	n := tensor.NumelOf(outShape)
	if n == 0 {
		return nil, true
	}
	checkSize(n)
	nd, meta := broadcastMeta(messages.H4055, outShape, ins...)
	key := fmt.Sprintf("cinsli:%d:%d:%d:%v:%s", op.Kind, op.Sub, nd, dts, o.dtype)
	k := g.takeKernel(key, typedKernel(op, nd, dts, o.dtype))
	p := g.param()
	for _, v := range ins {
		p = p.readV(v)
	}
	iz, bl := plain(n, sameBlock)
	g.launch(k, iz, bl, p.write(o, 0).u32(n).arr((len(ins)+1)*nd, meta...))
	return nil, true
}

// rawSuffix bir öğenin ham yükleme/saklama sonekidir (u8, u16, u32, u64).
func rawSuffix(dt tensor.DType) string {
	return map[int]string{1: "u8", 2: "u16", 4: "u32", 8: "u64"}[dtypeSize(dt)]
}

// rawLoad öğeyi dönüştürmeden yükler (mantık ve 8 bitlik öğe 32 bitlik yazmaca).
func (k *emitter) rawLoad(dt tensor.DType, addr reg) reg {
	var v reg
	switch dtypeSize(dt) {
	case 8:
		v = k.rd()
	case 2:
		v = k.h()
	default:
		v = k.r()
	}
	k.e("ld.global."+rawSuffix(dt), v, mem(addr, 0))
	return v
}

// cmpMode karşılaştırmanın PTX kipi: ondalıkta != sırasız (NaN'lı karşılaştırma doğru), öbürleri
// sıralı (NaN'lı karşılaştırma yanlış; 15.5, IEEE 754).
func cmpMode(c tensor.CmpOp, float bool) string {
	modes := [...]string{"eq", "ne", "lt", "gt", "le", "ge"}
	if float && c == tensor.CmpNe {
		return "neu"
	}
	return modes[c]
}

// typedKernel tek iş parçacığı bir öğe: girdiler yayınlamalı (meta: şekil, girdi adımları),
// çıktı bitişik. Çekirdeğin adı PTX adıdır: türler alt çizgiyle ayrılır (`%v`'nin köşeli
// parantezi ve boşluğu ptxas'ta sözdizimi hatasıdır).
func typedKernel(op tensor.TypedOp, nd int, ins []tensor.DType, out tensor.DType) func() *emitter {
	return func() *emitter {
		name := fmt.Sprintf("cinsli_%d_%d_%d", op.Kind, op.Sub, nd)
		for _, dt := range append(ins[:len(ins):len(ins)], out) {
			name += "_" + dt.String()
		}
		k := newEmitter(name)
		ptrs := make([]reg, len(ins))
		for i := range ins {
			ptrs[i] = k.pointer(fmt.Sprintf("g%d", i))
		}
		dst := k.written("cikti")
		n := k.u32p("n")
		k.arr("meta", (len(ins)+1)*nd)
		i := k.globalIndex()
		last := k.bounded(i, n)
		ofs := k.tripleOffsets(i, nd, len(ins))
		addr := func(j int) reg { return k.addressS(ptrs[j], ofs[j], dtypeSize(ins[j])) }
		to := k.address(dst, i, dtypeSize(out))
		switch op.Kind {
		case tensor.TCompare:
			k.typedCompare(tensor.CmpOp(op.Sub), ins, addr(0), addr(1), to)
		case tensor.TLogic, tensor.TNot:
			a := k.rawLoad(tensor.Bool, addr(0))
			r := k.r()
			if op.Kind == tensor.TNot {
				k.e("xor.b32", r, a, 1)
			} else {
				b := k.rawLoad(tensor.Bool, addr(1))
				k.e([...]string{"and.b32", "or.b32", "xor.b32"}[op.Sub], r, a, b)
			}
			k.e("st.global.u8", mem(to, 0), r)
		case tensor.TWhere:
			c := k.rawLoad(tensor.Bool, addr(0))
			p := k.p()
			k.e("setp.ne.u32", p, c, 0)
			a, b := k.rawLoad(out, addr(1)), k.rawLoad(out, addr(2))
			bitsN := map[int]string{1: "b32", 2: "b16", 4: "b32", 8: "b64"}[dtypeSize(out)]
			r := map[string]func() reg{"b16": k.h, "b32": k.r, "b64": k.rd}[bitsN]()
			k.e("selp."+bitsN, r, a, b, p)
			k.e("st.global."+rawSuffix(out), mem(to, 0), r)
		case tensor.TConvert:
			v := k.f()
			if ins[0] == tensor.Int64 {
				k.e("cvt.rn.f32.s64", v, k.rawLoad(ins[0], addr(0)))
			} else {
				k.e("cvt.rn.f32.u32", v, k.rawLoad(ins[0], addr(0)))
			}
			k.store(tensor.Float32, to, v)
		}
		k.put(last)
		return k
	}
}

// typedCompare a op b'yi yazar (1 ya da 0 bayt).
func (k *emitter) typedCompare(c tensor.CmpOp, ins []tensor.DType, aAddr, bAddr, to reg) {
	p := k.p()
	switch {
	case ins[0].IsFloat():
		a, b := k.load(ins[0], aAddr), k.load(ins[1], bAddr)
		k.e("setp."+cmpMode(c, true)+".f32", p, a, b)
	case ins[0] == tensor.Int64:
		a, b := k.rawLoad(ins[0], aAddr), k.rawLoad(ins[1], bAddr)
		k.e("setp."+cmpMode(c, false)+".s64", p, a, b)
	default:
		a, b := k.rawLoad(ins[0], aAddr), k.rawLoad(ins[1], bAddr)
		k.e("setp."+cmpMode(c, false)+".u32", p, a, b)
	}
	r := k.r()
	k.e("selp.u32", r, 1, 0, p)
	k.e("st.global.u8", mem(to, 0), r)
}
