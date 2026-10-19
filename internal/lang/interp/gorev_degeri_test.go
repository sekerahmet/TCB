package interp

import "testing"

type testAnahtari struct{}

// Görev değeri yalnızca kendi VM görünümünde görünür; yeni görevler boş başlar.
func TestGorevDegeri(t *testing.T) {
	v := New()
	if v.GorevDegeri(testAnahtari{}) != nil {
		t.Fatal("başlangıçta değer olmamalı")
	}
	v.GorevDegeriAyarla(testAnahtari{}, 7)
	if v.GorevDegeri(testAnahtari{}) != 7 {
		t.Fatalf("değer: %v", v.GorevDegeri(testAnahtari{}))
	}
	if g := v.NewTaskVM(); g.GorevDegeri(testAnahtari{}) != nil {
		t.Errorf("yeni görev değeri devralmamalı: %v", g.GorevDegeri(testAnahtari{}))
	}
	g := v.NewTaskVM()
	g.GorevDegeriAyarla(testAnahtari{}, 9)
	if v.GorevDegeri(testAnahtari{}) != 7 {
		t.Error("görevin değeri ana görünüme sızdı")
	}
	v.GorevDegeriAyarla(testAnahtari{}, nil)
	if v.GorevDegeri(testAnahtari{}) != nil {
		t.Error("nil değer silmeli")
	}
}
