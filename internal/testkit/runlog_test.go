package testkit

import (
	"slices"
	"testing"
)

// Kural: ValidLossMetrics, devam kaydından sonra adımı ondan büyük kayıtları düşürür; kaybı olmayan
// kaydı sayar; yarım satırı yalnızca skipBroken ile atlar.
func TestValidLossMetrics(t *testing.T) {
	dir := t.TempDir()
	log := WriteFile(t, dir, "metrics.jsonl",
		`{"adım":1,"kayıp":0.5}`+"\n"+`{"adım":2,"kayıp":0.4}`+"\n"+`{"adım":3,"kayıp":0.3}`+"\n"+
			`{"adım":2,"tür":"devam"}`+"\n"+`{"adım":9,"tür":"başka"}`+"\n"+`{"adım":3,"kayıp":0.31}`+"\n"+`{"adım":4,"ka`)
	want := []string{"1:0.5", "2:0.4", "3:0.31"}
	if got := ValidLossMetrics(t, log, true); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Kural: AssertSameRunFiles aynı dosyalarda ve aynı geçerli kayıp dizisinde sessizdir.
func TestAssertSameRunFiles(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, d := range []string{a, b} {
		WriteFile(t, d, "son.trt", "x")
		WriteFile(t, d, "metrics.jsonl", `{"adım":1,"kayıp":0.5}`+"\n")
	}
	WriteFile(t, b, "metrics.jsonl", `{"adım":1,"kayıp":0.5}`+"\n"+`{"adım`)
	AssertSameRunFiles(t, a, b, []string{"son.trt"}, true)
	// Katı kipte yarım satır hatadır.
	if !failsFatally(func(tb testing.TB) { AssertSameRunFiles(tb, a, b, []string{"son.trt"}, false) }) {
		t.Error("katı kipte yarım satır düşmeliydi")
	}
}

// fatalRecorder Fatalf'i panikle yakalanabilir kılan sahte testing.TB'dir.
type fatalRecorder struct{ testing.TB }

func (fatalRecorder) Helper()               {}
func (fatalRecorder) Fatalf(string, ...any) { panic("fatal") }
func (fatalRecorder) Fatal(...any)          { panic("fatal") }
func (fatalRecorder) Errorf(string, ...any) { panic("fatal") }

// failsFatally f'nin test düşürüp düşürmediğini söyler.
func failsFatally(f func(testing.TB)) (failed bool) {
	defer func() { failed = recover() != nil }()
	f(fatalRecorder{})
	return false
}
