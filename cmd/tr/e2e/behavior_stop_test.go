package e2e

import (
	"testing"
)

// Durdurma isteğinin süreçteki yolu (1.5-1.7, 1.9, 1.15): durdurmayı üstlenen program kendi
// sınırında biter ve isteğin sinyaliyle ya da TR_exit'in koduyla çıkar; program biterken gelen ilk
// istek ertelenen işleri kesmez; durdurmanın ertelediği işlerde beklemeler bırakılmaz.

// handledLoop durdurmayı üstlenen ve istek gelene kadar kısa uykularla dönen programdır; after
// döngüden sonra çalışır.
func handledLoop(after string) string {
	return trProgram(
		"TR_handle_stop()",
		`TR_defer { TR_write_file("ertele.txt", "çalıştı") }`,
		`TR_write_file("hazir.txt", "1")`,
		"TR_for {",
		"    TR_if TR_stop_requested() { TR_break }",
		"    TR_try { TR_sleep(0.05) } TR_catch e { }",
		"}",
		after,
	)
}

// Durdurmayı üstlenen program kendi sınırında biter: döngüden çıkar, temizliğini yapar, ertelenen
// işler çalışır; sona geldiği için süreç isteğin sinyaliyle biter (1.6, 1.9).
func TestHandledStopEndsAtTheProgramBoundaryWithTheSignal(t *testing.T) {
	s := startInterruptible(t, handledLoop(`TR_write_file("temiz.txt", "1")`))
	s.waitCond("program hazır", func() bool { return s.fileExists("hazir.txt") })
	sendInterrupt(t, s.cmd)
	if !s.endedByInterrupt() {
		t.Fatalf("sona gelen üstlenmiş program isteğin sinyaliyle bitmeliydi; standart hata: %s", s.stderr.String())
	}
	for _, name := range []string{"temiz.txt", "ertele.txt"} {
		if !s.fileExists(name) {
			t.Errorf("%s yok: program kendi sınırında temizlik yapmalıydı", name)
		}
	}
}

// Durdurmayı üstlenen program TR_exit(kod) ile biterse süreç o kodla çıkar (1.9).
func TestHandledStopThenExitCode(t *testing.T) {
	s := startInterruptible(t, handledLoop("TR_exit(5)"))
	s.waitCond("program hazır", func() bool { return s.fileExists("hazir.txt") })
	sendInterrupt(t, s.cmd)
	if code := s.exitCode(); code != 5 {
		t.Fatalf("çıkış kodu %d, 5 bekleniyordu; standart hata: %s", code, s.stderr.String())
	}
	if !s.fileExists("ertele.txt") {
		t.Error("ertelenen iş çalışmalıydı")
	}
}

// Program biterken gelen ilk istek ertelenen işleri kesmez: iş bitince süreç isteğin sinyaliyle
// biter (1.15).
func TestFirstInterruptDoesNotCutTheOutermostDefers(t *testing.T) {
	s := startInterruptible(t, trProgram(
		"TR_defer {",
		`    TR_write_file("hazir.txt", "1")`,
		"    TR_sleep(1.5)",
		`    TR_write_file("ertele_bitti.txt", "1")`,
		"}",
		`TR_print("ana bitti")`,
	))
	s.waitCond("ertele başladı", func() bool { return s.fileExists("hazir.txt") })
	sendInterrupt(t, s.cmd)
	if !s.endedByInterrupt() {
		t.Fatalf("süreç isteğin sinyaliyle bitmeliydi; standart hata: %s", s.stderr.String())
	}
	if !s.fileExists("ertele_bitti.txt") {
		t.Errorf("ilk istek ertelenen işi kesmemeliydi; standart hata: %s", s.stderr.String())
	}
}

// Durdurmanın kendi başlattığı ertelenen işlerde beklemeler bırakılmaz: temizlik bekleyebilir (1.5).
func TestWaitsInDefersAfterAStopAreNotAbandoned(t *testing.T) {
	s := startInterruptible(t, trProgram(
		`TR_defer { TR_sleep(0.5); TR_write_file("ertele_bitti.txt", "1") }`,
		`TR_write_file("hazir.txt", "1")`,
		"TR_sleep(600)",
	))
	s.waitCond("program hazır", func() bool { return s.fileExists("hazir.txt") })
	sendInterrupt(t, s.cmd)
	if !s.endedByInterrupt() {
		t.Fatalf("süreç isteğin sinyaliyle bitmeliydi; standart hata: %s", s.stderr.String())
	}
	if !s.fileExists("ertele_bitti.txt") {
		t.Error("ertelenen işteki bekleme bırakılmamalı, iş tamamlanmalıydı")
	}
}
