package interp

import "testing"

type testAnahtari struct{}

// Görev değeri yalnızca kendi VM görünümünde görünür; yeni görevler boş başlar.
func TestGorevDegeri(t *testing.T) {
	v := New()
	if v.TaskValue(testAnahtari{}) != nil {
		t.Fatal("başlangıçta değer olmamalı")
	}
	v.SetTaskValue(testAnahtari{}, 7)
	if v.TaskValue(testAnahtari{}) != 7 {
		t.Fatalf("değer: %v", v.TaskValue(testAnahtari{}))
	}
	if g := v.NewTaskVM(); g.TaskValue(testAnahtari{}) != nil {
		t.Errorf("yeni görev değeri devralmamalı: %v", g.TaskValue(testAnahtari{}))
	}
	g := v.NewTaskVM()
	g.SetTaskValue(testAnahtari{}, 9)
	if v.TaskValue(testAnahtari{}) != 7 {
		t.Error("görevin değeri ana görünüme sızdı")
	}
	v.SetTaskValue(testAnahtari{}, nil)
	if v.TaskValue(testAnahtari{}) != nil {
		t.Error("nil değer silmeli")
	}
}
