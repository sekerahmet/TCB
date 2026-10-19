package tensor

import "fmt"

// Eleman bazlı işlemler (cihazdan bağımsız katman). Hesap arka uçta yapılır.

// ============================================================================
// Yayınlama (broadcasting)
// ============================================================================

// broadcastShapes iki şeklin yayınlanmış ortak şeklini verir: eksenler sağdan
// hizalanır; eşit olan ya da biri 1 olan eksenler uyumludur.
func broadcastShapes(a, b []int) []int {
	nd := len(a)
	if len(b) > nd {
		nd = len(b)
	}
	out := make([]int, nd)
	for i := 0; i < nd; i++ {
		da, db := 1, 1
		if j := i - (nd - len(a)); j >= 0 {
			da = a[j]
		}
		if j := i - (nd - len(b)); j >= 0 {
			db = b[j]
		}
		switch {
		case da == db:
			out[i] = da
		case da == 1:
			out[i] = db
		case db == 1:
			out[i] = da
		default:
			panic(fmt.Sprintf("tensor: %v ve %v şekilleri yayınlanamaz", a, b))
		}
	}
	return out
}

// broadcastStrides, shape/strides ile tanımlı tensörün outShape'e yayınlanmış
// adımlarını verir: eklenen ya da 1'den genişletilen eksenlerin adımı 0'dır.
func broadcastStrides(strides, shape, outShape []int) []int {
	nd := len(outShape)
	st := make([]int, nd)
	for i := 0; i < nd; i++ {
		j := i - (nd - len(shape))
		if j < 0 || shape[j] == 1 {
			st[i] = 0
		} else {
			st[i] = strides[j]
		}
	}
	return st
}

// isSuffixShape b'nin, out şeklinin son eksenleriyle aynı olup olmadığı
// (ör. (B,T,C) + (C) sapma toplaması).
func isSuffixShape(b, out []int) bool {
	if len(b) > len(out) {
		return false
	}
	off := len(out) - len(b)
	for i, d := range b {
		if out[off+i] != d {
			return false
		}
	}
	return true
}

// ============================================================================
// İkili işlemler
// ============================================================================

var binopName = [...]string{"add", "sub", "mul", "div", "atan2", "lt", "gt", "le", "ge"}

func binary(a, b *Tensor, op BinOp) *Tensor {
	return binaryKind(a, b, op, resultDType(a.dtyp, b.dtyp))
}

// binaryKind binary gibidir; sonucun türü dt'dir (hesap en az float32'de yapılıp dt'ye
// bir kez yuvarlanır).
func binaryKind(a, b *Tensor, op BinOp, dt DType) *Tensor {
	be := sameDevice(binopName[op], a, b)
	outShape := broadcastShapes(a.shape, b.shape)
	sy := counterStart(be, OpKindAdd+OpKind(op)).start()
	out := emptyOf(binopOp[op], be, outShape, dt)
	be.Binary(op, out.storage, outShape, a.view(), b.view())
	var save []*Tensor // geri yayılımın okuduğu girdiler
	switch op {
	case OpMul:
		save = []*Tensor{a, b}
	case OpDiv:
		save = []*Tensor{b}
		if b.requiresGrad {
			save = append(save, out) // paydanın gradyanı çıktıyı okur
		}
	}
	r := track(out, binopName[op], []*Tensor{a, b}, func() {
		g := out.grad
		sh := out.shape
		switch op {
		case OpAdd, OpSub:
			// b önce: a, b ile aynı tensör olsa da gradyanı sonra devralabilsin.
			if b.requiresGrad {
				m := GradPlus
				if op == OpSub {
					m = GradMinus
				}
				passBinaryGrad(b, out, m, false)
			}
			if a.requiresGrad {
				passBinaryGrad(a, out, GradPlus, true)
			}
		case OpMul:
			if a.requiresGrad && !writeBinaryGrad(a, out, OpMul, b) {
				be.BinaryGrad(GradMul, a.accGrad(), a.shape, sh, g, b.view(), nil)
			}
			if b.requiresGrad && !writeBinaryGrad(b, out, OpMul, a) {
				be.BinaryGrad(GradMul, b.accGrad(), b.shape, sh, g, a.view(), nil)
			}
		case OpDiv:
			if a.requiresGrad && !writeBinaryGrad(a, out, OpDiv, b) {
				be.BinaryGrad(GradDiv, a.accGrad(), a.shape, sh, g, b.view(), nil)
			}
			if b.requiresGrad {
				be.BinaryGrad(GradDivB, b.accGrad(), b.shape, sh, g, b.view(), out.storage)
			}
		}
	}, save...)
	sy.finish()
	return r
}

// passBinaryGrad toplama/çıkarma geri yönünde out'un gradyanını t'ye aktarır (m:
// GradPlus ya da GradMinus). t yayınlanmışsa indirgenir. Aynı şekilli t'de gradyan
// henüz yoksa sıfırlanmadan yazılır; take doğruysa ve tür uyuyorsa depolama kopyasız
// devredilir (out'tan alınır, handOffGrad gibi). Gradyan varsa üzerine eklenir.
func passBinaryGrad(t, out *Tensor, m GradMode, take bool) {
	be, g := t.be, out.grad
	switch {
	case !sameShape(t.shape, out.shape):
		be.BinaryGrad(m, t.accGrad(), t.shape, out.shape, g, View{}, nil)
	case t.grad != nil:
		be.AddTo(t.grad, g, gradSign(m))
	case take && m == GradPlus && g.DType() == t.gradDType():
		t.grad = g
		out.grad = nil
	case m == GradPlus:
		c, _ := t.freshGrad(true)
		be.Copy(c, 0, out.gradView())
	default:
		be.AddTo(t.accGrad(), g, gradSign(m))
	}
}

// writeBinaryGrad çarpma/bölme geri yönünde t'nin (birinci işlenen ya da çarpmada
// herhangi biri) gradyanı henüz yoksa ve t yayınlanmamışsa gradyanı tek ikili işlemle
// sıfırlamadan yazar: dt = g op o (hesap float32, gradyan türüne bir kez yuvarlanır).
// Yazamazsa yanlış döner; çağıran genel yolla (indirgeme ile) ekler.
func writeBinaryGrad(t, out *Tensor, op BinOp, o *Tensor) bool {
	if t.grad != nil || !sameShape(t.shape, out.shape) {
		return false
	}
	c, _ := t.freshGrad(true)
	t.be.Binary(op, c, out.shape, out.gradView(), o.view())
	return true
}

func gradSign(m GradMode) float32 {
	if m == GradMinus {
		return -1
	}
	return 1
}

// Add eleman bazlı toplama (yayınlamalı).
func Add(a, b *Tensor) *Tensor { return binary(a, b, OpAdd) }

// Sub eleman bazlı çıkarma (yayınlamalı).
func Sub(a, b *Tensor) *Tensor { return binary(a, b, OpSub) }

// Mul eleman bazlı çarpma (yayınlamalı).
func Mul(a, b *Tensor) *Tensor { return binary(a, b, OpMul) }

// Div eleman bazlı bölme (yayınlamalı).
func Div(a, b *Tensor) *Tensor { return binary(a, b, OpDiv) }

// Add yöntem biçimi.
func (t *Tensor) Add(o *Tensor) *Tensor { return Add(t, o) }

// Sub yöntem biçimi.
func (t *Tensor) Sub(o *Tensor) *Tensor { return Sub(t, o) }

// Mul yöntem biçimi.
func (t *Tensor) Mul(o *Tensor) *Tensor { return Mul(t, o) }

// Div yöntem biçimi.
func (t *Tensor) Div(o *Tensor) *Tensor { return Div(t, o) }

// ============================================================================
// Tekli işlemler ve skalerle işlemler
// ============================================================================

var unopName = map[UnOp]string{
	OpExp: "exp", OpLog: "log", OpSqrt: "sqrt", OpPow: "pow", OpTanh: "tanh", OpRelu: "relu",
	OpGelu: "gelu", OpSigmoid: "sigmoid", OpAddScalar: "add_scalar", OpMulScalar: "mul_scalar",
	OpRSubScalar: "rsub_scalar", OpRDivScalar: "rdiv_scalar", OpSin: "sin", OpCos: "cos",
	OpSilu: "silu", OpTan: "tan", OpAsin: "asin", OpAcos: "acos", OpAtan: "atan", OpSinh: "sinh",
	OpCosh: "cosh", OpLog1p: "log1p", OpLog2: "log2", OpLog10: "log10", OpExpm1: "expm1",
	OpFloor: "floor", OpCeil: "ceil", OpSign: "sign", OpAbs: "abs",
}

// unaryNoGrad türevi her yerde 0 olan (parçalı sabit) tekli işlemlerdir: geri yayılım
// girdinin gradyanını (sıfır olarak) ayırır, arka uca hesap göndermez.
func unaryNoGrad(op UnOp) bool { return op == OpFloor || op == OpCeil || op == OpSign }

func unary(t *Tensor, op UnOp, p float32) *Tensor {
	sy := counterStart(t.be, OpKindExp+OpKind(op)).start()
	out := emptyOf(isElementwise, t.be, t.shape, t.dtyp)
	t.be.Unary(op, p, out.storage, t.view())
	var save []*Tensor
	switch {
	case op == OpAddScalar || op == OpMulScalar || op == OpRSubScalar || unaryNoGrad(op):
		// Doğrusal ya da parçalı sabit: geri yayılım girdiyi okumaz.
	default:
		save = []*Tensor{t, out}
	}
	r := track(out, unopName[op], []*Tensor{t}, func() {
		if unaryNoGrad(op) {
			t.accGrad() // gradyan 0'dır; yaprakta sıfır gradyan görünür
			return
		}
		us, writer := t.be.(UnaryGradSetter)
		if gx, fresh := t.freshGrad(writer); fresh {
			us.UnaryGradSet(op, p, gx, out.grad, t.view(), out.storage)
			return
		}
		t.be.UnaryGrad(op, p, t.accGrad(), out.grad, t.view(), out.storage)
	}, save...)
	sy.finish()
	return r
}

// AddScalar her elemana s ekler.
func (t *Tensor) AddScalar(s float32) *Tensor { return unary(t, OpAddScalar, s) }

// SubScalar her elemandan s çıkarır.
func (t *Tensor) SubScalar(s float32) *Tensor { return unary(t, OpAddScalar, -s) }

// MulScalar her elemanı s ile çarpar.
func (t *Tensor) MulScalar(s float32) *Tensor { return unary(t, OpMulScalar, s) }

// DivScalar her elemanı s'ye böler. 1/s ile çarpma değil gerçek bölmedir (yuvarlama
// farkı olmaz) ve sonuç t'nin türündedir (16 bitlik tensör 16 bitlik kalır; diğer
// skaler işlemlerle tutarlı).
func (t *Tensor) DivScalar(s float32) *Tensor {
	return binaryKind(t, fromStorage(t.be, t.be.FromHost([]float32{s}, Float32), nil), OpDiv, t.dtyp)
}

// RSubScalar s - x hesaplar.
func (t *Tensor) RSubScalar(s float32) *Tensor { return unary(t, OpRSubScalar, s) }

// RDivScalar s / x hesaplar.
func (t *Tensor) RDivScalar(s float32) *Tensor { return unary(t, OpRDivScalar, s) }

// Neg işaret değiştirir.
func (t *Tensor) Neg() *Tensor { return unary(t, OpMulScalar, -1) }

// Exp e^x.
func (t *Tensor) Exp() *Tensor { return unary(t, OpExp, 0) }

// Log doğal logaritma.
func (t *Tensor) Log() *Tensor { return unary(t, OpLog, 0) }

// Sqrt karekök.
func (t *Tensor) Sqrt() *Tensor { return unary(t, OpSqrt, 0) }

// Pow x^p (p skaler).
func (t *Tensor) Pow(p float32) *Tensor { return unary(t, OpPow, p) }

// Square x².
func (t *Tensor) Square() *Tensor { return unary(t, OpPow, 2) }

// Tanh hiperbolik tanjant.
func (t *Tensor) Tanh() *Tensor { return unary(t, OpTanh, 0) }

// Relu max(0, x).
func (t *Tensor) Relu() *Tensor { return unary(t, OpRelu, 0) }

// Sigmoid 1 / (1 + e^-x) (sayısal kararlı).
func (t *Tensor) Sigmoid() *Tensor { return unary(t, OpSigmoid, 0) }

// Gelu tanh yaklaşımlı Gauss hata doğrusal birimi:
// 0.5·x·(1 + tanh(√(2/π)·(x + 0.044715·x³))).
func (t *Tensor) Gelu() *Tensor { return unary(t, OpGelu, 0) }
