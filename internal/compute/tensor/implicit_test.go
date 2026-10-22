package tensor

import "testing"

// implicitDiff f'nin çalışması boyunca CPU'daki (yön, üst işlem, tür) kaydına eklenen çağrı
// ve elemanı verir (sayaçlar süreç geneli: t.Parallel kullanılmaz).
func implicitDiff(back bool, parent OpKind, k ImplicitKind, f func()) (call, elems int64) {
	find := func() ImplicitRecord {
		for _, r := range ImplicitSnapshot() {
			if r.Device == KindCPU && r.Backward == back && r.Parent == parent && r.Kind == k {
				return r
			}
		}
		return ImplicitRecord{}
	}
	before := find()
	f()
	after := find()
	return after.Calls - before.Calls, after.Elems - before.Elems
}

// Yerinde yazma kendi işlem türüyle sayılır ve adımlı hedefe yazdığı kopya ona bağlanır.
func TestImplicitCopyBelongsToAssign(t *testing.T) {
	dst := Zeros(4, 6)
	src := Ones(4, 2)
	if call, _, _, _ := counterDiff(false, OpKindAssign, func() { dst.Slice(1, 2, 4).CopyFrom(src) }); call != 1 {
		t.Fatalf("yerinde yazma çağrısı %d, beklenen 1", call)
	}
	call, elems := implicitDiff(false, OpKindAssign, ImplicitStrided, func() { dst.Slice(1, 2, 4).CopyFrom(src) })
	if call != 1 || elems != 8 {
		t.Errorf("yerinde yazmanın adımlı kopyası %d çağrı %d eleman, beklenen 1 ve 8", call, elems)
	}
}

// Geri yayılımdaki gradyan biriktirme, gradyanı yazan işlemin geri yönüne bağlanır: w iki
// kez kullanıldığından ikinci gradyan birincisinin üstüne eklenir.
func TestImplicitAccumulateBelongsToBackward(t *testing.T) {
	w := New([]float32{1, 2, 3, 4}, 2, 2).SetRequiresGrad(true)
	loss := Add(w, w).Reshape(-1).SumAll()
	call, elems := implicitDiff(true, OpKindAdd, ImplicitAccumulate, loss.Backward)
	if call != 1 || elems != 4 {
		t.Errorf("toplamanın geri yönünde ekleme %d çağrı %d eleman, beklenen 1 ve 4", call, elems)
	}
}
