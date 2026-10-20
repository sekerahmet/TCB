package shape

import "testing"

func check(t *testing.T, s Shape) {
	t.Helper()
	if err := Describe(s); err != nil {
		t.Log(err)
	}
}

func TestDescribe(t *testing.T) { check(t, Square{2}) }

func TestSquare_Area(t *testing.T) {
	if (Square{3}).Area() != 9 {
		t.Fatal("area")
	}
}

func TestTwiceName(t *testing.T) {}
