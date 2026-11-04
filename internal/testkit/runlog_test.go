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
		`{"step":1,"loss":0.5}`+"\n"+`{"step":2,"loss":0.4}`+"\n"+`{"step":3,"loss":0.3}`+"\n"+
			`{"step":2,"kind":"resume"}`+"\n"+`{"step":9,"kind":"başka"}`+"\n"+`{"step":3,"loss":0.31}`+"\n"+`{"step":4,"ka`)
	want := []string{"1:0.5", "2:0.4", "3:0.31"}
	if got := ValidLossMetrics(t, log, true); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Kural: ValidLossMetrics tam sayı olmayan adımı 0 diye okumaz, test düşer (19.5); başlangıç kaydında
// adım yoktur, o hata değildir.
func TestValidLossMetricsRejectsAFractionalStep(t *testing.T) {
	dir := t.TempDir()
	log := WriteFile(t, dir, "metrics.jsonl", `{"kind":"start"}`+"\n"+`{"step":3.0,"loss":0.5}`+"\n")
	r := &fatalRecorder{recorder: &recorder{TB: t}}
	ValidLossMetrics(r, log, true)
	if len(r.fatals) != 1 {
		t.Errorf("the fractional step should have failed the test once: %v", r.fatals)
	}
	if got := ValidLossMetrics(t, WriteFile(t, dir, "ok.jsonl", `{"kind":"start"}`+"\n"+`{"step":3,"loss":0.5}`+"\n"), true); !slices.Equal(got, []string{"3:0.5"}) {
		t.Errorf("got %v", got)
	}
}

// Kural: AssertSameRunFiles aynı dosyalarda ve aynı geçerli kayıp dizisinde sessizdir.
func TestAssertSameRunFiles(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, d := range []string{a, b} {
		WriteFile(t, d, "son.trt", "x")
		WriteFile(t, d, "metrics.jsonl", `{"step":1,"loss":0.5}`+"\n")
	}
	WriteFile(t, b, "metrics.jsonl", `{"step":1,"loss":0.5}`+"\n"+`{"step`)
	AssertSameRunFiles(t, a, b, []string{"son.trt"}, true)
	// Katı kipte yarım satır hatadır.
	if !failsFatally(func(tb testing.TB) { AssertSameRunFiles(tb, a, b, []string{"son.trt"}, false) }) {
		t.Error("katı kipte yarım satır düşmeliydi")
	}
}

// panickingTB Fatalf'i panikle yakalanabilir kılan sahte testing.TB'dir.
type panickingTB struct{ testing.TB }

func (panickingTB) Helper()               {}
func (panickingTB) Fatalf(string, ...any) { panic("fatal") }
func (panickingTB) Fatal(...any)          { panic("fatal") }
func (panickingTB) Errorf(string, ...any) { panic("fatal") }

// failsFatally f'nin test düşürüp düşürmediğini söyler.
func failsFatally(f func(testing.TB)) (failed bool) {
	defer func() { failed = recover() != nil }()
	f(panickingTB{})
	return false
}
