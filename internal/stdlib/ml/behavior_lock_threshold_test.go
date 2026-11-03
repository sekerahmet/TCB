package ml

import (
	"fmt"
	"io"
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Ölçüm (10.2, S-kilit): kilidi bırakma eşiğinin (2^20 öğe; tensor.unlockedElemWork,
// unlockedStepWork, unlockedSampleWork) dayanağı. Her işlem ve boy için aynı yorumlayıcıda iki
// süre yazılır: doğrudan (kilit altında) ve bırakarak (released ya da stepOn: izinler, Blocking,
// başka görev canlıyken yarı işçi). Bırakmanın bedeli oranlarıdır; kazancı yalnız işin kendisi
// uzunsa vardır (kilit yerleşikler arasında zaten devredilir). Eşiğin altında bırakarak yol
// doğrudan yolla aynıdır (oran ≈ 1). Sınır konmaz; sayılar okunur:
// go test ./internal/stdlib/ml -run TestUnlockThresholdCost -v
func TestUnlockThresholdCost(t *testing.T) {
	if testing.Short() {
		t.Skip("ölçüm: -short'ta koşmaz")
	}
	for _, other := range []bool{false, true} {
		measureInVM(t, other, func(y *interp.VM) {
			rng := rand.New(rand.NewSource(1))
			for _, op := range thresholdOps(y) {
				for n := 1 << 16; n <= 1<<22; n <<= 2 {
					direct, viaRelease := op.run(tensor.Randn(rng, n), tensor.Randn(rng, n))
					d, r := median(9, direct), median(9, viaRelease)
					t.Logf("eşik ölçümü: %s n=2^%d, başka görev %v: doğrudan %s, bırakarak %s, oran %.3f",
						op.name, bitLen(n), other, us(d), us(r), float64(r)/float64(d))
				}
			}
		})
	}
}

type thresholdOp struct {
	name string
	// run iki çağrı verir: doğrudan ve bırakarak (aynı iş).
	run func(a, b *tensor.Tensor) (direct, viaRelease func())
}

func thresholdOps(y *interp.VM) []thresholdOp {
	pair := func(f func() *tensor.Tensor) (func(), func()) {
		return func() { f().Drop() }, func() { released(y, f)().Drop() }
	}
	return []thresholdOp{
		{"TR_exp", func(a, _ *tensor.Tensor) (func(), func()) { return pair(a.Exp) }},
		{"+", func(a, b *tensor.Tensor) (func(), func()) {
			return pair(func() *tensor.Tensor { return tensor.Add(a, b) })
		}},
		{"TR_sum", func(a, _ *tensor.Tensor) (func(), func()) { return pair(a.SumAll) }},
		{".TR_step()", func(a, _ *tensor.Tensor) (func(), func()) {
			p := a.Clone().SetRequiresGrad(true)
			p.SumAll().Backward()
			opt := tensor.NewAdamW([]*tensor.Tensor{p}, 1e-3, 0)
			o := &Optimizer{opt: opt, adamw: opt, params: []*tensor.Tensor{p}}
			return opt.Step, func() { stepOn(y, o) }
		}},
	}
}

// measureInVM f'yi bir programın içinden, kilit tutulurken çalıştırır; other doğruysa yanında
// 1 ms'de bir uyanan bir görev canlıdır.
func measureInVM(t *testing.T, other bool, f func(y *interp.VM)) {
	t.Helper()
	m := interp.New()
	m.Out = io.Discard
	var flag interp.Value = interp.False
	if other {
		flag = interp.True
	}
	m.GlobalEnv.Define("başka", flag, true)
	m.GlobalEnv.Define("ölç", &interp.BuiltinVal{Name: "ölç", Fn: func(y *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		f(y)
		return interp.Null, nil
	}}, true)
	_, err := m.Run(trrun.Parse(t, `TR_var bitti = TR_false
TR_if başka { TR_async { TR_for TR_not bitti { TR_sleep(0.001) } } }
TR_sleep(0.01)
ölç()
bitti = TR_true
TR_wait_all()
`))
	m.WaitAsync()
	if err != nil {
		t.Fatal(err)
	}
}

// median f'nin bir çağrısının süresidir: f önce ısınır, sonra her ölçümde toplam ≥ 20 ms olana
// dek tekrarlanır (Windows'ta saat ~0,5 ms adımlıdır) ve süre çağrı sayısına bölünür; k
// ölçümün ortancası verilir.
func median(k int, f func()) time.Duration {
	f()
	ds := make([]time.Duration, k)
	for i := range ds {
		start, n := time.Now(), 0
		for time.Since(start) < 20*time.Millisecond {
			f()
			n++
		}
		ds[i] = time.Since(start) / time.Duration(n)
	}
	slices.Sort(ds)
	return ds[k/2]
}

func us(d time.Duration) string { return fmt.Sprintf("%.1f µs", float64(d)/float64(time.Microsecond)) }

func bitLen(n int) int {
	k := 0
	for n > 1 {
		n >>= 1
		k++
	}
	return k
}
