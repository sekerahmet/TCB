package ml

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// q4kWeight sonlu küçük ölçekli rastgele bir q4_k ağırlığı [N, K] kurar (ölçeklerin üst baytı 0x1C: fp16 ~0,004).
func q4kWeight(seed int64, N, K int) *tensor.Tensor {
	r := rand.New(rand.NewSource(seed))
	raw := make([]byte, N*K/256*144)
	r.Read(raw)
	for b := 0; b < len(raw); b += 144 {
		raw[b+1], raw[b+3] = 0x1C|raw[b+1]&0x83, 0x1C|raw[b+3]&0x83
	}
	return tensor.FromQuantized(tensor.Q4K, raw, N, K)
}

func wantMessage(t *testing.T, err error, code messages.Code) {
	t.Helper()
	var me *messages.Error
	if !errors.As(err, &me) || me.Code != code {
		t.Fatalf("ileti %s beklendi, %v geldi", code, err)
	}
}

// TestQuantizedActivationsBlockPicksTheIntPath: TR_quantized_activations bloğunda (görev durumu açık) `@` nicemli
// ağırlıkla tensor.MatMulActQuantKernel'in yolundan gider: sonuç onunla bit bit aynı, bloğun dışındakiyle
// (float yolu) farklıdır. Blok dışında yol değişmez (15.53).
func TestQuantizedActivationsBlockPicksTheIntPath(t *testing.T) {
	r := rand.New(rand.NewSource(53))
	for _, M := range []int{1, 5} {
		x := tensor.Randn(r, M, 512)
		w := q4kWeight(int64(M), 7, 512).T()
		m := interp.New()
		outside, err := matmulDType(m, x, w, tensor.Float32)
		if err != nil {
			t.Fatal(err)
		}
		plain := tensor.MatMul(x, w).Data()
		m.SetTaskValue(actQuantKey{}, true)
		inside, err := matmulDType(m, x, w, tensor.Float32)
		if err != nil {
			t.Fatal(err)
		}
		r, k := tensor.MatMulActQuantKernel(x, w, tensor.Float32)
		k.RunLocked()
		want, got, out := r.Data(), inside.Data(), outside.Data()
		same := true
		for i := range want {
			if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
				t.Fatalf("M=%d [%d]: blokta %v, nicemli yol %v", M, i, got[i], want[i])
			}
			if math.Float32bits(out[i]) != math.Float32bits(plain[i]) {
				t.Fatalf("M=%d [%d]: blok dışında %v, float yolu %v", M, i, out[i], plain[i])
			}
			same = same && math.Float32bits(got[i]) == math.Float32bits(out[i])
		}
		if same {
			t.Fatalf("M=%d: blokta da float yolu kullanıldı", M)
		}
	}
}

// TestQuantizedActivationsRefusesOtherWeights: blokta q8 ağırlık hatadır (H6723), sessizce float yoluna düşmez.
func TestQuantizedActivationsRefusesOtherWeights(t *testing.T) {
	m := interp.New()
	m.SetTaskValue(actQuantKey{}, true)
	w := tensor.FromQuantized(tensor.Q8, make([]byte, 2*34), 2, 32).T()
	_, err := matmulDType(m, tensor.Ones(1, 32), w, tensor.Float32)
	wantMessage(t, err, messages.H6723)
}

// TestQuantizedActivationsArguments: blok yalnız bir blok alır (H6725, H6726); sorgu argüman almaz.
func TestQuantizedActivationsArguments(t *testing.T) {
	m := interp.New()
	block := m.Builtin(codes.FnQuantizedActivations).Fn
	_, err := block(m, nil, nil)
	wantMessage(t, err, messages.H6725)
	_, err = block(m, []interp.Value{&interp.IntVal{Val: 3}}, nil)
	wantMessage(t, err, messages.H6726)
	v, err := m.Builtin(codes.FnActivationsQuantized).Fn(m, nil, nil)
	if err != nil || v != interp.False {
		t.Fatalf("blok dışında sorgu: %v, %v", v, err)
	}
}
