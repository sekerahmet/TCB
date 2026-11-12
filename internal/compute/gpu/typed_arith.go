package gpu

import "github.com/sekerahmet/TCB/internal/compute/tensor"

// ============================================================================
// "int64" aritmetiği kartta (15.3, 15.6; T1 dilim 2b)
// ============================================================================
//
// İşlemcinin intArith, mulInt, powInt ve unaryAll'u (tensor/cpu_typed_ops.go) ile aynı anlam:
// sonuç int64'e sığmalıdır (taşma), // aşağı yuvarlar, % bölenin işaretini alır, sıfıra bölme
// ve eksi üs hatadır; MinInt64 % -1 = 0'dır, hata değildir. Hatalı öğe yazılmaz; anahtarı
// (düz konum << 4 | çeşit) bayrağın yuvasına tek atom.global.min.u64 ile yazılır
// (fault_slot.go): en küçük düz konum kazanır (15.6). Ayrıntıyı (işlenenler) konak işlemcinin
// aynı işlemiyle yeniden hesaplar (tensor.FaultFromSlot).

// int64Arith a op b'yi yazar ya da hatayı bayrağa yazar.
func (k *emitter) int64Arith(op tensor.ArithOp, aAddr, bAddr, to, i, flag reg) {
	a, b := k.rawLoad(tensor.Int64, aAddr), k.rawLoad(tensor.Int64, bAddr)
	r, bad, kind := k.rd(), k.p(), k.r()
	k.e("mov.u32", kind, int(tensor.FaultOverflow))
	switch op {
	case tensor.ArithAdd, tensor.ArithSub:
		// Taşma: toplamada işlenenler aynı işaretli, sonuç başka işaretli ((a^r)&(b^r) < 0);
		// çıkarmada işlenenler ayrı işaretli, sonuç a'dan başka işaretli ((a^b)&(a^r) < 0).
		x, y := k.rd(), k.rd()
		if op == tensor.ArithAdd {
			k.e("add.s64", r, a, b)
			k.e("xor.b64", x, a, r)
			k.e("xor.b64", y, b, r)
		} else {
			k.e("sub.s64", r, a, b)
			k.e("xor.b64", x, a, b)
			k.e("xor.b64", y, a, r)
		}
		k.e("and.b64", x, x, y)
		k.e("setp.lt.s64", bad, x, 0)
	case tensor.ArithMul:
		k.mulChecked(r, bad, a, b)
	case tensor.ArithFloorDiv, tensor.ArithMod:
		k.floorDivMod(op == tensor.ArithMod, r, bad, kind, a, b)
	default:
		k.powChecked(r, bad, kind, a, b)
	}
	k.storeOrFault(bad, kind, r, to, i, flag)
}

// mulChecked r = a·b'nin alt 64 biti; bad: 128 bitlik çarpım int64'e sığmıyor (üst yarı alt
// yarının işaret uzantısı değil).
func (k *emitter) mulChecked(r, bad, a, b reg) {
	hi, sign := k.rd(), k.rd()
	k.e("mul.lo.s64", r, a, b)
	k.e("mul.hi.s64", hi, a, b)
	k.e("shr.s64", sign, r, 63)
	k.e("setp.ne.s64", bad, hi, sign)
}

// isMinInt64 x == MinInt64 yüklemi: sıfırdan başka ve eksisi kendisi.
func (k *emitter) isMinInt64(x reg) reg {
	n := k.rd()
	k.e("neg.s64", n, x)
	p, nz := k.p(), k.p()
	k.e("setp.eq.s64", p, n, x)
	k.e("setp.ne.s64", nz, x, 0)
	k.e("and.pred", p, p, nz)
	return p
}

// floorDivMod a // b (mod yanlışsa) ya da a % b: bölüm aşağı, kalan bölenin işaretinde. b = 0
// sıfıra bölmedir; MinInt64 // -1 taşmadır, MinInt64 % -1 = 0. Bu iki durumda bölen 1 alınır
// (kartın div.s64'ü onlarda tanımsız), sonuç ve hata ayrıca seçilir.
func (k *emitter) floorDivMod(mod bool, r, bad, kind, a, b reg) {
	zero, minus1, special, safe := k.p(), k.p(), k.p(), k.p()
	k.e("setp.eq.s64", zero, b, 0)
	k.e("setp.eq.s64", minus1, b, -1)
	k.e("and.pred", special, k.isMinInt64(a), minus1)
	k.e("or.pred", safe, zero, special)
	d, one := k.rd(), k.rd()
	k.e("mov.s64", one, 1)
	k.e("selp.b64", d, one, b, safe)
	q, rem := k.rd(), k.rd()
	k.e("div.s64", q, a, d)
	k.e("rem.s64", rem, a, d)
	// Aşağı yuvarlama: kalan sıfır değil ve işareti bölenin işaretinden başkaysa q−1, rem+d.
	nz, diff := k.p(), k.p()
	s := k.rd()
	k.e("setp.ne.s64", nz, rem, 0)
	k.e("xor.b64", s, rem, d)
	k.e("setp.lt.s64", diff, s, 0)
	k.e("and.pred", nz, nz, diff)
	k.ifCase(nz, "sub.s64", q, q, 1)
	k.ifCase(nz, "add.s64", rem, rem, d)
	if mod {
		k.e("mov.b64", r, rem)
		k.e("mov.pred", bad, zero)
	} else {
		k.e("mov.b64", r, q)
		k.e("or.pred", bad, zero, special)
	}
	k.e("selp.u32", kind, int(tensor.FaultDivZero), int(tensor.FaultOverflow), zero)
}

// powChecked a ** b: üs eksiyse eksi üs hatası; kare alarak üs, her çarpım taşmaya denetlenir.
// Taban yalnız üssün kalan biti varken karelenir: gereken her kare sonucun mutlak değerinden
// büyük değildir, kare taşarsa sonuç da taşar (işlemcinin tek tek çarpımıyla aynı hata) [hesap].
func (k *emitter) powChecked(r, bad, kind, a, b reg) {
	neg := k.p()
	k.e("setp.lt.s64", neg, b, 0)
	base, e, t := k.rd(), k.rd(), k.rd()
	k.e("mov.s64", r, 1)
	k.e("mov.b64", base, a)
	k.e("selp.b64", e, r, b, neg) // eksi üste döngü koşmaz (e = 1, sonra bad)
	ovf, step := k.p(), k.p()
	k.e("setp.ne.s64", ovf, a, a) // yanlış
	loop, skip, done := k.label(), k.label(), k.label()
	k.put(loop)
	end, odd := k.p(), k.p()
	k.e("setp.eq.s64", end, e, 0)
	k.jumpIf(end, done)
	bit := k.rd()
	k.e("and.b64", bit, e, 1)
	k.e("setp.ne.s64", odd, bit, 0)
	k.jumpUnless(odd, skip)
	k.mulChecked(t, step, r, base)
	k.e("mov.b64", r, t)
	k.e("or.pred", ovf, ovf, step)
	k.put(skip)
	k.e("shr.s64", e, e, 1)
	k.e("setp.eq.s64", end, e, 0)
	k.jumpIf(end, done)
	k.mulChecked(t, step, base, base)
	k.e("mov.b64", base, t)
	k.e("or.pred", ovf, ovf, step)
	k.jump(loop)
	k.put(done)
	k.e("or.pred", bad, ovf, neg)
	k.e("selp.u32", kind, int(tensor.FaultNegExp), int(tensor.FaultOverflow), neg)
}

// int64Unary tekli eksi ya da mutlak değer: MinInt64'te taşma.
func (k *emitter) int64Unary(op tensor.TypedKind, aAddr, to, i, flag reg) {
	x := k.rawLoad(tensor.Int64, aAddr)
	bad := k.isMinInt64(x)
	r := k.rd()
	k.e("neg.s64", r, x)
	if op == tensor.TAbs {
		minus := k.p()
		k.e("setp.lt.s64", minus, x, 0)
		k.e("selp.b64", r, r, x, minus)
	}
	kind := k.r()
	k.e("mov.u32", kind, int(tensor.FaultOverflow))
	k.storeOrFault(bad, kind, r, to, i, flag)
}

// storeOrFault bad yanlışsa v'yi yazar, doğruysa hatanın anahtarını bayrağa (hatalı öğe yazılmaz).
func (k *emitter) storeOrFault(bad, kind, v, to, i, flag reg) {
	ok, done := k.label(), k.label()
	k.jumpUnless(bad, ok)
	k.writeFault(flag, i, kind)
	k.jump(done)
	k.put(ok)
	k.e("st.global.u64", mem(to, 0), v)
	k.put(done)
}
