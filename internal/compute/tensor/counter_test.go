package tensor

import (
	"testing"
	"time"
)

// counterDiff, f'nin çalışması boyunca (cpu, yön, işlem) kaydında biriken çağrı, ölçüm ve
// baytı verir. Sayaçlar süreç genelidir: bu testler t.Parallel kullanmaz (paralel testler
// sıralı testler bitince koşar, sayaçlara aynı anda yazılmaz).
func counterDiff(back bool, is OpKind, f func()) (call, measurement, byteN int64, dur time.Duration) {
	find := func() CounterRecord {
		for _, k := range CounterSnapshot() {
			if k.Device == KindCPU && k.Backward == back && k.OpKind == is {
				return k
			}
		}
		return CounterRecord{}
	}
	before := find()
	f()
	after := find()
	return after.Calls - before.Calls, after.Measured - before.Measured, after.Bytes - before.Bytes, after.Duration - before.Duration
}

func TestCounterKnownOpCount(t *testing.T) {
	a := New(make([]float32, 16), 4, 4)
	b := New(make([]float32, 16), 4, 4)
	call, _, byteN, _ := counterDiff(false, OpKindAdd, func() {
		for i := 0; i < 7; i++ {
			Add(a, b)
		}
	})
	if call != 7 {
		t.Errorf("toplama çağrısı %d, beklenen 7", call)
	}
	if byteN != 7*16*4 {
		t.Errorf("toplama baytı %d, beklenen %d (7 çıktı × 16 float32)", byteN, 7*16*4)
	}
	call, _, _, _ = counterDiff(false, OpKindMulScalar, func() {
		for i := 0; i < 3; i++ {
			a.MulScalar(0.5)
		}
	})
	if call != 3 {
		t.Errorf("skaler çarpma çağrısı %d, beklenen 3", call)
	}
}

func TestCounterBackwardNodeName(t *testing.T) {
	w := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	x := New([]float32{1, 0, 2, 1, 0, 1}, 3, 2)
	var y *Tensor
	forward, _, _, _ := counterDiff(false, OpKindMatMul, func() { y = MatMul(w, x) })
	if forward != 1 {
		t.Fatalf("ileri matris çarpımı %d, beklenen 1", forward)
	}
	if y.n.opKind != OpKindMatMul {
		t.Fatalf("düğümün işlemi %v, beklenen matris çarpımı", y.n.opKind)
	}
	lossVal := y.Reshape(-1).SumAll()
	back, _, byteN, _ := counterDiff(true, OpKindMatMul, func() { lossVal.Backward() })
	if back != 1 {
		t.Errorf("geri matris çarpımı %d, beklenen 1", back)
	}
	if byteN != 6*4 {
		t.Errorf("geri matris çarpımının ayırdığı gradyan %d bayt, beklenen 24 (w'nin gradyanı)", byteN)
	}
	if g, _, _, _ := counterDiff(true, OpKindView, func() {
		k := MatMul(w, x).Reshape(-1).SumAll()
		k.Backward()
	}); g != 1 {
		t.Errorf("geri görünüm %d, beklenen 1 (yeniden şekillendirme)", g)
	}
}

// Uzun işlemin (ilk çağrı her zaman ölçülür) sonraki bütün çağrıları da ölçülür.
func TestCounterLargeOpMeasuredEachCall(t *testing.T) {
	k := &counters[0][0][OpKindMatMul]
	k.back, k.wt = 0, 0 // işlem türü ilk kez çalışıyormuş gibi (önceki testlerin küçük çarpımları)
	a := New(make([]float32, 128*128), 128, 128)
	call, measurement, _, dur := counterDiff(false, OpKindMatMul, func() {
		for i := 0; i < 3; i++ {
			MatMul(a, a)
		}
	})
	if call != 3 || measurement != 3 {
		t.Errorf("büyük matris çarpımı: %d çağrı, %d ölçüm; beklenen 3 ve 3", call, measurement)
	}
	if dur <= 0 {
		t.Errorf("büyük matris çarpımının süresi %v, sıfırdan büyük olmalı", dur)
	}
}

// Kısa işlemlerde süre örneklenir: her çağrı p = 1/ornekAraligi olasılıkla ölçülür ve ölçülen
// süre 1/p ile ağırlıklanır. Ağırlıkların toplamı çağrı sayısının yansız tahminidir; ölçülen
// çağrı sayısı n·p civarındadır. Uzun işlemde (uzunIslemTik) her çağrı ölçülür.
func TestCounterSampleWeights(t *testing.T) {
	k := &counters[0][0][OpKindOther]
	identity := *k
	*k = counterRecord{no: identity.no}
	defer func() { *k = identity }()
	const n = 64000
	var total, measured int64
	for i := 0; i < n; i++ {
		s := counterStart(cpuBackend{}, OpKindOther).start()
		if s.k != nil {
			total += s.weight
			measured++
		}
		s.finish()
	}
	// Ölçülen çağrı sayısı Binom(n, 1/32): ortalama 2000, standart sapma ~44.
	if measured < n/sampleRange*85/100 || measured > n/sampleRange*115/100 {
		t.Errorf("%d çağrının %d'i ölçüldü; p = 1/%d ile ~%d beklenir", n, measured, sampleRange, n/sampleRange)
	}
	if total < n*85/100 || total > n*115/100 {
		t.Errorf("ağırlıkların toplamı %d; çağrı sayısının (%d) yansız tahmini olmalı", total, n)
	}
	// Uzun işlem: son ölçüm sınırı aşınca her çağrı ölçülür.
	nextMeasure(k, longOpTicks)
	for i := 0; i < 5; i++ {
		s := counterStart(cpuBackend{}, OpKindOther).start()
		if s.k == nil || s.weight != 1 {
			t.Fatalf("uzun işlemin %d. çağrısı ölçülmedi (ağırlık %d)", i+1, s.weight)
		}
		s.begin -= longOpTicks // uzun sürmüş gibi
		s.finish()
	}
}

// Panikle biten işlemin etkin kaydı bir sonraki işleme kadar kalır (ertelemesiz sıcak yol);
// sonraki işlem bitince temizlenir ve sayaçlar doğru sayar.
func TestCounterActiveOpClearedOnNext(t *testing.T) {
	x := New(make([]float32, 6), 2, 3)
	func() {
		defer func() { _ = recover() }()
		CrossEntropy(x, []int{0}) // 2 satır için 1 hedef: panik
	}()
	call, _, _, _ := counterDiff(false, OpKindCrossEntropy, func() {
		CrossEntropy(x, []int{0, 1})
	})
	if call != 1 {
		t.Errorf("çapraz entropi çağrısı %d, beklenen 1", call)
	}
	if is, _, running := ActiveOp(); running {
		t.Fatalf("işlem bittikten sonra etkin işlem %v kaldı", is)
	}
}

func TestCounterOpNames(t *testing.T) {
	seen := map[string]bool{}
	for i := OpKind(0); i < OpKindCount; i++ {
		if i.Code() == "" || i.String() == "" {
			t.Errorf("%d. işlemin adı eksik", i)
		}
		if seen[i.Code()] {
			t.Errorf("%q kodu iki kez", i.Code())
		}
		seen[i.Code()] = true
	}
	if OpKindAdd+OpKind(OpDiv) != OpKindDiv || OpKindAdd+OpKind(OpGe) != OpKindGreaterEq ||
		OpKindExp+OpKind(OpCos) != OpKindCos || OpKindExp+OpKind(OpAbs) != OpKindAbs {
		t.Error("ikili ve tekli işlemlerin sırası BinOp/UnOp sırasıyla aynı olmalı")
	}
}

// Kaynaşık program kendi türüyle sayılır; geri hesabı "diğer"e düşmez (profilde adsız
// kalmaz).
func TestCounterFusedOwnKind(t *testing.T) {
	gu := New([]float32{1, -2, 3, 4, 0.5, -1, 2, 1}, 2, 4).SetRequiresGrad(true)
	var y *Tensor
	if call, _, _, _ := counterDiff(false, OpKindFused, func() { y = runFusedFn(swiglu, []*Tensor{gu})[0] }); call != 1 {
		t.Fatalf("ileri kaynaşık %d, beklenen 1", call)
	}
	if y.n.opKind != OpKindFused {
		t.Fatalf("düğümün işlemi %v, beklenen kaynaşık", y.n.opKind)
	}
	lossVal := y.Reshape(-1).SumAll()
	var other int64
	back, _, _, _ := counterDiff(true, OpKindFused, func() {
		other, _, _, _ = counterDiff(true, OpKindOther, lossVal.Backward)
	})
	if back != 1 || other != 0 {
		t.Errorf("geri kaynaşık %d, diğer %d; beklenen 1 ve 0", back, other)
	}
}
