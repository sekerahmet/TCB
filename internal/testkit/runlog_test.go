package testkit

import (
	"fmt"
	"slices"
	"testing"
)

// Kural: ValidLossMetrics, devam kaydından sonra adımı ondan büyük kayıtları düşürür; kaybı olmayan
// kaydı sayar; yarım satırı yalnızca skipBroken ile atlar.
func TestValidLossMetrics(t *testing.T) {
	dir := t.TempDir()
	head := `{"step":1,"loss":0.5}` + "\n" + `{"step":2,"loss":0.4}` + "\n"
	log := WriteFile(t, dir, "metrics.jsonl",
		head+`{"step":3,"loss":0.3}`+"\n"+resume(2, len(head))+`{"step":9,"kind":"başka"}`+"\n"+`{"step":3,"loss":0.31}`+"\n"+`{"step":4,"ka`)
	want := []string{"1:0.5", "2:0.4", "3:0.31"}
	if got := ValidLossMetrics(t, log, true); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// resume, adımı ve offset'iyle bir devam kaydının satırıdır (19.6).
func resume(step, offset int) string {
	return fmt.Sprintf(`{"kind":"resume","step":%d,"offset":%d}`, step, offset) + "\n"
}

// Kural: offset ile devam kaydı arasındaki her kayıt, adımı olsun olmasın, geçersizdir (19.6): adımsız
// ve devamınkinden küçük adımlı ara kayıt düşer, adımı devamınkine eşit olan önceki kayıt kalır.
func TestValidLossMetricsDropsEveryRecordAfterTheOffset(t *testing.T) {
	head := `{"step":1,"loss":0.5}` + "\n" + `{"step":2,"loss":0.4}` + "\n"
	log := WriteFile(t, t.TempDir(), "metrics.jsonl",
		head+`{"loss":0.9}`+"\n"+`{"step":1,"loss":0.8}`+"\n"+resume(2, len(head))+`{"step":3,"loss":0.3}`+"\n")
	if got, want := ValidLossMetrics(t, log, false), []string{"1:0.5", "2:0.4", "3:0.3"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Kural: ardışık devam kayıtlarının her biri kuralını kendinden önceki kayıtlara uygular: aynı noktaya
// iki geri dönüş ve daha eski bir noktaya (daha küçük offset'e) dönüş.
func TestValidLossMetricsAppliesEachResume(t *testing.T) {
	one := `{"step":1,"loss":0.5}` + "\n"
	two := one + `{"step":2,"loss":0.4}` + "\n"
	same := two + `{"step":3,"loss":0.3}` + "\n" + `{"loss":0.9}` + "\n"
	same += resume(2, len(two)) + `{"step":3,"loss":0.31}` + "\n" + `{"loss":0.91}` + "\n"
	same += resume(2, len(two)) + `{"step":3,"loss":0.32}` + "\n"
	if got, want := ValidLossMetrics(t, WriteFile(t, t.TempDir(), "a.jsonl", same), false), []string{"1:0.5", "2:0.4", "3:0.32"}; !slices.Equal(got, want) {
		t.Errorf("same point: got %v, want %v", got, want)
	}
	older := two + `{"step":3,"loss":0.3}` + "\n" + resume(2, len(two)) + `{"step":3,"loss":0.31}` + "\n"
	older += resume(1, len(one)) + `{"step":2,"loss":0.41}` + "\n"
	if got, want := ValidLossMetrics(t, WriteFile(t, t.TempDir(), "b.jsonl", older), false), []string{"1:0.5", "2:0.41"}; !slices.Equal(got, want) {
		t.Errorf("older point: got %v, want %v", got, want)
	}
}

// Kural: offset devam satırının başından büyükse (günlük kesilmiş, 20.6) aralık boştur, yalnız adım
// kuralı uygulanır; offset devam satırının başına eşitse de aralık boştur; offset 0 her şeyi düşürür.
func TestValidLossMetricsOffsetPastTheResume(t *testing.T) {
	head := `{"step":1,"loss":0.5}` + "\n" + `{"step":2,"loss":0.4}` + "\n" + `{"step":3,"loss":0.3}` + "\n"
	for _, c := range []struct {
		offset int
		want   []string
	}{
		{1 << 20, []string{"1:0.5", "2:0.4", "4:0.2"}},
		{len(head), []string{"1:0.5", "2:0.4", "4:0.2"}},
		{0, []string{"4:0.2"}},
	} {
		log := WriteFile(t, t.TempDir(), "metrics.jsonl", head+resume(2, c.offset)+`{"step":4,"loss":0.2}`+"\n")
		if got := ValidLossMetrics(t, log, false); !slices.Equal(got, c.want) {
			t.Errorf("offset %d: got %v, want %v", c.offset, got, c.want)
		}
	}
}

// Kural: satırın ortasına düşen, eksi, tam sayı olmayan offset ve offset'siz devam kaydı bozuk günlüktür,
// skipBroken'da da test düşer.
func TestValidLossMetricsRejectsABadOffset(t *testing.T) {
	head := `{"step":1,"loss":0.5}` + "\n"
	for _, line := range []string{
		resume(1, 5),
		resume(1, -1),
		`{"kind":"resume","step":1,"offset":2.5}` + "\n",
		`{"kind":"resume","step":1}` + "\n",
	} {
		log := WriteFile(t, t.TempDir(), "metrics.jsonl", head+line)
		if !failsFatally(func(tb testing.TB) { ValidLossMetrics(tb, log, true) }) {
			t.Errorf("%q should have failed the test", line)
		}
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
