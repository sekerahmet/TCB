package tensor

import "testing"

// Bits16/FromBits16: 16 bitlik tensör değerleri bit desenine kayıpsız gidip gelir.
func TestBits16GidisDonus(t *testing.T) {
	for _, dt := range []DType{BFloat16, Float16} {
		x := NewOf(dt, []float32{0.1, -2.5, 65504, 1e-5, 0, 3.14159}, 6)
		for i, v := range x.Data() {
			if g := FromBits16(dt, Bits16(dt, v)); g != v {
				t.Errorf("%s[%d]: %v → %v", dt, i, v, g)
			}
		}
	}
	if Bits16(BFloat16, 1) != 0x3F80 || Bits16(Float16, 1) != 0x3C00 {
		t.Errorf("1.0 bit deseni: %#x %#x", Bits16(BFloat16, 1), Bits16(Float16, 1))
	}
	defer func() {
		if recover() == nil {
			t.Error("float32 için panik bekleniyordu")
		}
	}()
	Bits16(Float32, 1)
}
