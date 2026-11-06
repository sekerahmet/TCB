package process_test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// Kabul edilmeyen çıkış kodu çalışma hatasıdır: ileti adı, kodu ve standart hatanın sonunu taşır,
// önerdiği TR_exit_codes: yazımı çalışır (21.4).
func TestExitCodeIsARunError(t *testing.T) {
	prog := helperProgram(t)
	_, err := run(t, `TR_run_process(`+prog+`, ["exit", "3", "dosya bulunamadı\n"])`)
	if !messages.Has(err, messages.H7503) {
		t.Fatalf("H7503 bekleniyordu: %v", err)
	}
	for _, want := range []string{"3", "dosya bulunamadı", "TR_exit_codes: [0, 3]"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q yok: %v", want, err)
		}
	}
	if _, err := run(t, `TR_run_process(`+prog+`, ["exit", "3", "x"], TR_exit_codes: [0, 3])`); err != nil {
		t.Errorf("öneri çalışmadı: %v", err)
	}
	got, err := run(t, `TR_error_code { TR_run_process(`+prog+`, ["exit", "1"]) }`)
	if err != nil || got != "H7503" {
		t.Errorf("TR_error_code: %s (%v)", got, err)
	}
	if _, err := run(t, `TR_run_process(`+prog+`, ["exit", "1"], TR_env: {})`); err == nil || strings.Contains(err.Error(), "Standart hata") {
		t.Errorf("standart hata boşsa başlık da olmaz: %v", err)
	}
}

// Kabul edilen kod başarıdır, standart hata atılır; varsayılan yalnız 0'dır (21.4).
func TestAcceptedExitCodes(t *testing.T) {
	prog := helperProgram(t)
	for _, code := range []string{
		`TR_run_process(` + prog + `, ["exit", "0", "uyarı"])`,
		`TR_run_process(` + prog + `, ["exit", "1"], TR_exit_codes: [0, 1])`,
		`TR_run_process(` + prog + `, ["exit", "2"], TR_exit_codes: [2])`,
	} {
		if _, err := run(t, code); err != nil {
			t.Errorf("%s: %v", code, err)
		}
	}
	if _, err := run(t, `TR_run_process(`+prog+`, ["exit", "0"], TR_exit_codes: [1])`); !messages.Has(err, messages.H7503) {
		t.Errorf("listede olmayan 0 da hatadır: %v", err)
	}
}

// Boş liste program başlamadan hatadır (TCB kararı); öğeler tam sayıdır.
func TestExitCodesAreChecked(t *testing.T) {
	prog := helperProgram(t)
	start := time.Now()
	if _, err := run(t, `TR_run_process(`+prog+`, ["sleep", "5"], TR_exit_codes: [])`); !messages.Has(err, messages.H7508) {
		t.Errorf("boş liste: %v", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("program başlamamalıydı: %v", d)
	}
	for _, code := range []string{`TR_exit_codes: 0`, `TR_exit_codes: ["0"]`, `TR_exit_codes: [0.5]`} {
		if _, err := run(t, `TR_run_process(`+prog+`, ["exit", "0"], `+code+`)`); !messages.Has(err, messages.H7506) {
			t.Errorf("%s: H7506 bekleniyordu: %v", code, err)
		}
	}
}

// Gizli değer standart hatadan iletiye geçmez (21.4, 19.4); e.TR_message da süzülmüştür.
func TestStderrInTheMessageIsMasked(t *testing.T) {
	prog := helperProgram(t)
	t.Setenv("TR_DENEME_TOKEN", "hf_cokgizlideger12345")
	_, err := run(t, `TR_var a = TR_env("TR_DENEME_TOKEN")
TR_run_process(`+prog+`, ["exit", "1", "anahtar: hf_cokgizlideger12345 reddedildi"])`)
	if !messages.Has(err, messages.H7503) || strings.Contains(err.Error(), "hf_cok") || !strings.Contains(err.Error(), "*** reddedildi") {
		t.Errorf("%v", err)
	}
}

// Denetim karakteri ve geçersiz UTF-8 iletiye girmez (21.4).
func TestStderrInTheMessageIsPrintable(t *testing.T) {
	_, err := run(t, `TR_run_process(`+helperProgram(t)+`, ["raw-stderr"])`)
	if err == nil {
		t.Fatal("hata bekleniyordu")
	}
	text := err.Error()
	if strings.ContainsAny(text, "\x1b\x07\r") || strings.Contains(text, "\xff") || !strings.Contains(text, "hata") || !strings.Contains(text, "satır") {
		t.Errorf("%q", text)
	}
}

// Sinyalle ölen program hatadır, ileti sinyali söyler; Windows'ta sinyal yoktur (21.4).
func TestKilledBySignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sinyal Unix'te")
	}
	_, err := run(t, `TR_run_process(`+helperProgram(t)+`, ["kill-self"])`)
	if !messages.Has(err, messages.H7504) || !strings.Contains(err.Error(), "SIGKILL") {
		t.Errorf("%v", err)
	}
	if _, err := run(t, `TR_run_process(`+helperProgram(t)+`, ["kill-self"], TR_exit_codes: [0, -1])`); !messages.Has(err, messages.H7504) {
		t.Errorf("sinyal kod listesiyle kabul edilmez: %v", err)
	}
}

// İki çıktıya birden büyük yazan program kilitlenmez; bütün standart çıktı gelir (21.4).
func TestBothOutputsDoNotDeadlock(t *testing.T) {
	prog := helperProgram(t)
	done := make(chan struct{})
	var got string
	var err error
	go func() {
		defer close(done)
		got, _, err = commandLine(t, `TR_run_process(`+prog+`, ["flood"]).TR_length`)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("kilitlendi")
	}
	if err != nil || got != "4194304" {
		t.Errorf("%s (%v)", got, err)
	}
}

// Belleği aşan çıktı programı durdurur, hata çöküş değil iletidir; ileti akışlı çağrıyı önerir (21.1).
func TestRunProcessOutputIsBounded(t *testing.T) {
	restore := process.SetPhysicalMemory(1 << 20)
	defer restore()
	_, err := run(t, `TR_run_process(`+helperProgram(t)+`, ["endless"])`)
	if !messages.Has(err, messages.H7527) || !strings.Contains(err.Error(), "TR_spawn_process") {
		t.Errorf("%v", err)
	}
	if got, err := run(t, `TR_run_process(`+helperProgram(t)+`, ["pattern", "1000"]).TR_length`); err != nil || got != "1000" {
		t.Errorf("sınırın altı: %s (%v)", got, err)
	}
}
