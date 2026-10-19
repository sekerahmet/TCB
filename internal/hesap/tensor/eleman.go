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

var binopAd = [...]string{"add", "sub", "mul", "div", "atan2", "lt", "gt", "le", "ge"}

func binary(a, b *Tensor, op BinOp) *Tensor { return binaryTur(a, b, op, sonucTuru(a.tur, b.tur)) }

// binaryTur binary gibidir; sonucun türü dt'dir (hesap en az float32'de yapılıp dt'ye
// bir kez yuvarlanır).
func binaryTur(a, b *Tensor, op BinOp, dt DType) *Tensor {
	be := sameDevice(binopAd[op], a, b)
	outShape := broadcastShapes(a.shape, b.shape)
	out := emptyOf(binopIslem[op], be, outShape, dt)
	be.Binary(op, out.storage, outShape, a.view(), b.view())
	var kaydet []*Tensor // geri yayılımın okuduğu girdiler
	switch op {
	case OpMul:
		kaydet = []*Tensor{a, b}
	case OpDiv:
		kaydet = []*Tensor{b, out}
	}
	return track(out, binopAd[op], []*Tensor{a, b}, func() {
		g := out.grad
		sh := out.shape
		yok := View{}
		switch op {
		case OpAdd, OpSub:
			if a.requiresGrad {
				be.BinaryGrad(GradPlus, a.accGrad(), a.shape, sh, g, yok, nil)
			}
			if b.requiresGrad {
				m := GradPlus
				if op == OpSub {
					m = GradMinus
				}
				be.BinaryGrad(m, b.accGrad(), b.shape, sh, g, yok, nil)
			}
		case OpMul:
			if a.requiresGrad {
				be.BinaryGrad(GradMul, a.accGrad(), a.shape, sh, g, b.view(), nil)
			}
			if b.requiresGrad {
				be.BinaryGrad(GradMul, b.accGrad(), b.shape, sh, g, a.view(), nil)
			}
		case OpDiv:
			if a.requiresGrad {
				be.BinaryGrad(GradDiv, a.accGrad(), a.shape, sh, g, b.view(), nil)
			}
			if b.requiresGrad {
				be.BinaryGrad(GradDivB, b.accGrad(), b.shape, sh, g, b.view(), out.storage)
			}
		}
	}, kaydet...)
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

var unopAd = map[UnOp]string{
	OpExp: "exp", OpLog: "log", OpSqrt: "sqrt", OpPow: "pow", OpTanh: "tanh", OpRelu: "relu",
	OpGelu: "gelu", OpSigmoid: "sigmoid", OpAddScalar: "add_scalar", OpMulScalar: "mul_scalar",
	OpRSubScalar: "rsub_scalar", OpRDivScalar: "rdiv_scalar", OpSin: "sin", OpCos: "cos",
	OpSilu: "silu", OpTan: "tan", OpAsin: "asin", OpAcos: "acos", OpAtan: "atan", OpSinh: "sinh",
	OpCosh: "cosh", OpLog1p: "log1p", OpLog2: "log2", OpLog10: "log10", OpExpm1: "expm1",
	OpFloor: "floor", OpCeil: "ceil", OpSign: "sign", OpAbs: "abs",
}

// turevsizTekli türevi her yerde 0 olan (parçalı sabit) tekli işlemlerdir: geri yayılım
// girdinin gradyanını (sıfır olarak) ayırır, arka uca hesap göndermez.
func turevsizTekli(op UnOp) bool { return op == OpFloor || op == OpCeil || op == OpSign }

func unary(t *Tensor, op UnOp, p float32) *Tensor {
	out := emptyOf(isElemanBazli, t.be, t.shape, t.tur)
	t.be.Unary(op, p, out.storage, t.view())
	var kaydet []*Tensor
	switch {
	case op == OpAddScalar || op == OpMulScalar || op == OpRSubScalar || turevsizTekli(op):
		// Doğrusal ya da parçalı sabit: geri yayılım girdiyi okumaz.
	default:
		kaydet = []*Tensor{t, out}
	}
	return track(out, unopAd[op], []*Tensor{t}, func() {
		if turevsizTekli(op) {
			t.accGrad() // gradyan 0'dır; yaprakta sıfır gradyan görünür
			return
		}
		t.be.UnaryGrad(op, p, t.accGrad(), out.grad, t.view(), out.storage)
	}, kaydet...)
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
	return binaryTur(t, fromStorage(t.be, t.be.FromHost([]float32{s}, Float32), nil), OpDiv, t.tur)
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
