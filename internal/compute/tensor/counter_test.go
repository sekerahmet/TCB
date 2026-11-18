package tensor

import (
	"regexp"
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

// Kural (H1 profil payı A-1): CPU'da her çağrının süresi ölçülür; kısa ve uzun çağrılar
// karışınca da toplam süre çağrıların sürelerinin toplamıdır (örnekleme yok: kısa bir
// ölçümden sonraki uzun çağrı ne kat kat yazılır ne atlanır). Gerçek çarpımda ileri ve geri
// yönde her çağrı ölçülür.
func TestCounterMeasuresEveryCPUCall(t *testing.T) {
	k := &counters[0][0][OpKindOther]
	identity := *k
	*k = counterRecord{no: identity.no}
	defer func() { *k = identity }()
	// Kısa, uzun, kısa, uzun, uzun: ilk kısa çağrıdan sonra dört çağrı. Uzun çağrı saati geriye
	// alınarak bilinen tık kadar sürmüş sayılır.
	const long = 10_000_000
	lengths := []int64{0, long, 0, long, long}
	for _, d := range lengths {
		s := counterStart(cpuBackend{}, OpKindOther).start()
		s.begin -= d
		s.finish()
	}
	if k.call != 5 || k.measurement != 5 {
		t.Errorf("%d çağrı, %d ölçüm; beklenen 5 ve 5", k.call, k.measurement)
	}
	// Alt sınır bilinen sürelerin toplamı; üstü, beş çağrının kendi süresi için geniş bir pay.
	if k.dur < 3*long || k.dur > 3*long+long/2 {
		t.Errorf("toplam süre %d tık, beklenen %d tık (üç uzun çağrı) ile biraz fazlası", k.dur, 3*long)
	}

	w := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	x := New([]float32{1, 0, 2, 1, 0, 1}, 3, 2)
	var loss *Tensor
	call, measured, _, _ := counterDiff(false, OpKindMatMul, func() {
		for i := 0; i < 3; i++ {
			loss = MatMul(w, x).Reshape(-1).SumAll()
		}
	})
	if call != 3 || measured != 3 {
		t.Errorf("ileri matris çarpımı: %d çağrı, %d ölçüm; beklenen 3 ve 3", call, measured)
	}
	call, measured, _, _ = counterDiff(true, OpKindMatMul, func() { loss.Backward() })
	if call != 1 || measured != 1 {
		t.Errorf("geri matris çarpımı: %d çağrı, %d ölçüm; beklenen 1 ve 1", call, measured)
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

// Kural: işlem ve örtük iş anahtarları (profil dosyasının sabit anahtarları) Go adları gibi
// İngilizce tanımlayıcıdır (küçük ASCII harf, rakam, alt çizgi); kişiye görünen ad profiler'dadır.
func TestCounterOpNames(t *testing.T) {
	ident := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for k := ImplicitKind(0); k < ImplicitKindCount; k++ {
		if !ident.MatchString(k.Code()) {
			t.Errorf("örtük iş anahtarı %q İngilizce tanımlayıcı değil", k.Code())
		}
	}
	seen := map[string]bool{}
	for i := OpKind(0); i < OpKindCount; i++ {
		if i.Code() == "" || i.String() == "" {
			t.Errorf("%d. işlemin adı eksik", i)
		}
		if !ident.MatchString(i.Code()) {
			t.Errorf("%q İngilizce tanımlayıcı değil", i.Code())
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
