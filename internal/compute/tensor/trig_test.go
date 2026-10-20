package tensor

import (
	"math"
	"testing"
)

func init() {
	registerGrad("trig",
		unarySpec("sin", (*Tensor).Sin, math.Sin, -4, 4, 0),
		unarySpec("cos", (*Tensor).Cos, math.Cos, -4, 4, 0))
}

// TestGradTrig sinüs ve kosinüsün gradyanı float64 başvuruya ve bf16 toleransına uyar.
func TestGradTrig(t *testing.T) { runGradGroup(t, "trig") }
