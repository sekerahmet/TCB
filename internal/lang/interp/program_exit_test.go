package interp

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// runEnd kodu v'de çalıştırır (en çok 30 sn) ve sürecin nasıl biteceğini verir.
func runEnd(t *testing.T, v *VM, code string) (End, error) {
	t.Helper()
	prog := mustParseProgram(t, code)
	type outcome struct{ err error }
	done := make(chan outcome, 1)
	go func() {
		_, err := v.Run(prog)
		done <- outcome{err}
	}()
	select {
	case o := <-done:
		return v.End(o.err), o.err
	case <-time.After(30 * time.Second):
		t.Fatal("program 30 sn içinde bitmedi")
		return End{}, nil
	}
}

func traceText(v *VM) string {
	traces, _ := v.GlobalEnv.Get("izler")
	if traces == nil {
		return ""
	}
	return traces.Inspect()
}

// TR_exit(kod): kod 0-125 arasında bir Tam'dır, dışındaki değer çalışma hatasıdır (1.10, 1.11).
func TestExitCodeAndRange(t *testing.T) {
	for _, tc := range []struct {
		code   string
		want   int
		failed bool
	}{
		{"TR_exit(0)", 0, false},
		{"TR_exit(4)", 4, false},
		{"TR_exit(125)", 125, false},
		{"TR_exit(126)", 1, true},
		{"TR_exit(-1)", 1, true},
		{`TR_exit("a")`, 1, true},
	} {
		v := New()
		end, err := runEnd(t, v, "# TR 4\n"+tc.code+"\n")
		if end.Code != tc.want || end.Signal != 0 {
			t.Errorf("%s: çıkış %+v, kod %d bekleniyordu", tc.code, end, tc.want)
		}
		if tc.failed != (len(end.Errors) == 1) {
			t.Errorf("%s: hata yazılması %v bekleniyordu: %+v", tc.code, tc.failed, end.Errors)
		}
		if _, isExit := IsExitRequest(err); isExit == tc.failed {
			t.Errorf("%s: çıkış isteği %v bekleniyordu (aralık dışı kod çıkış değil, çalışma hatasıdır): %v", tc.code, !tc.failed, err)
		}
	}
}

// TR_exit nazik çıkıştır: TR_catch yakalamaz, ertele blokları çalışır; bir ertele bloğunda
// çağrılan TR_exit süren çıkışın kodunu değiştirir, kalan ertelenen işler yine çalışır (1.10, 1.13).
func TestExitSkipsCatchRunsDefersAndLastCallWins(t *testing.T) {
	v := New()
	end, _ := runEnd(t, v, `# TR 4
TR_var izler = []
TR_defer { izler.TR_append("dış ertele"); TR_exit(9) }
TR_defer { izler.TR_append("iç ertele") }
TR_try { TR_exit(3) } TR_catch e { izler.TR_append("yakalandı") }
izler.TR_append("sürdü")
`)
	if end.Code != 9 || end.Signal != 0 || len(end.Errors) != 0 {
		t.Errorf("son TR_exit çağrısının kodu 9 bekleniyordu: %+v", end)
	}
	text := traceText(v)
	for _, s := range []string{"iç ertele", "dış ertele"} {
		if !strings.Contains(text, s) {
			t.Errorf("%q çalışmalıydı: %s", s, text)
		}
	}
	for _, s := range []string{"yakalandı", "sürdü"} {
		if strings.Contains(text, s) {
			t.Errorf("%q olmamalıydı: %s", s, text)
		}
	}
}

// Bir eşzamanlı görevden çağrılan TR_exit bütün programı bitirir; ana görev beklemesini bırakır (1.10).
func TestExitFromTaskEndsTheProgram(t *testing.T) {
	v := New()
	end, _ := runEnd(t, v, `# TR 4
TR_var izler = []
TR_async { TR_exit(6) }
TR_sleep(60)
izler.TR_append("sürdü")
`)
	if end.Code != 6 || len(end.Errors) != 0 {
		t.Errorf("görevin kodu 6 bekleniyordu: %+v", end)
	}
	if strings.Contains(traceText(v), "sürdü") {
		t.Error("TR_exit'ten sonra ana program sürmemeli")
	}
}

// Biterken bir ertelenen iş hata verirse hata yazılır ve çıkış kodu 1 olur (1.14); bir görevde
// yakalanmayan hata da çıkışı 1 yapar (1.9).
func TestDeferredAndTaskErrorsMakeTheExitOne(t *testing.T) {
	v := New()
	end, _ := runEnd(t, v, "# TR 4\nTR_defer { TR_throw \"ertele hatası\" }\nTR_exit(3)\n")
	if end.Code != 1 || len(end.Errors) != 1 || end.Message != "" {
		t.Errorf("ertelenen işin hatası çıkışı 1 yapmalı: %+v", end)
	}

	v = New()
	var failure strings.Builder
	v.Err = &failure
	end, _ = runEnd(t, v, "# TR 4\nTR_async { TR_throw \"görev hatası\" }\nTR_wait_all()\n")
	if end.Code != 1 || !strings.Contains(failure.String(), "görev hatası") {
		t.Errorf("görevdeki yakalanmayan hata çıkışı 1 yapmalı: %+v %q", end, failure.String())
	}

	v = New()
	end, _ = runEnd(t, v, "# TR 4\nTR_print(\"tamam\")\n")
	if end.Code != 0 || end.Signal != 0 || len(end.Errors) != 0 {
		t.Errorf("hatasız program 0 ile bitmeli: %+v", end)
	}
}

// Durdurmayı üstlenmemiş programa istek geldiyse süreç isteğin sinyaliyle biter; TR_exit'in kodu
// ve ertelenen işin hatası bunu değiştirmez, hata yine yazılır (1.13, 1.14).
func TestUnhandledStopKeepsTheSignal(t *testing.T) {
	v := New()
	stopAfter(v, 2)
	end, _ := runEnd(t, v, `# TR 4
TR_defer { TR_exit(5) }
TR_defer { TR_throw "ertele hatası" }
hazır()
TR_sleep(60)
`)
	if end.Signal != 2 || len(end.Errors) != 1 {
		t.Errorf("isteğin sinyali ve ertelenen işin hatası bekleniyordu: %+v", end)
	}
}

// Durdurmayı üstlenen program sona gelirse sinyalle, TR_exit ile bitirirse o kodla, ertelenen
// işi hata verirse 1 ile biter (1.9).
func TestHandledProgramExitDecisions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		code       string
		wantSignal int
		wantCode   int
	}{
		{"sona gelir", "", 15, 0},
		{"TR_exit", "TR_exit(5)", 0, 5},
		{"ertelenen iş hatası", "TR_defer { TR_throw \"x\" }", 0, 1},
	} {
		v := New()
		stopAfter(v, 15)
		end, _ := runEnd(t, v, `# TR 4
TR_handle_stop()
hazır()
TR_try { TR_sleep(60) } TR_catch e { TR_print("durdu") }
`+tc.code+"\n")
		if end.Signal != tc.wantSignal || end.Code != tc.wantCode {
			t.Errorf("%s: %+v, sinyal %d kod %d bekleniyordu", tc.name, end, tc.wantSignal, tc.wantCode)
		}
	}
}

// Bir testin içinden (çağırdığı fonksiyon ya da başlattığı görev dahil) TR_exit süreci bitirmez:
// o test başarısız sayılır, öbür testler sürer (1.12).
func TestExitInATestFailsOnlyThatTest(t *testing.T) {
	v := New()
	prog := mustParseProgram(t, `# TR 4
TR_test "çıkış çağırır" { TR_exit(3) }
TR_test "görevden çağırır" { TR_async { TR_exit(4) } }
TR_test "sürer" { TR_print("tamam") }
`)
	if _, err := v.Run(prog); err != nil {
		t.Fatal(err)
	}
	var tests []*ast.TestStatement
	for _, st := range prog.Statements {
		if ts, ok := st.(*ast.TestStatement); ok {
			tests = append(tests, ts)
		}
	}
	if len(tests) != 3 {
		t.Fatalf("3 test bekleniyordu: %d", len(tests))
	}
	for i := 0; i < 2; i++ {
		if err := v.RunTest(tests[i]); err == nil {
			t.Errorf("%s: TR_exit testi düşürmeli", tests[i].Name)
		}
	}
	if err := v.RunTest(tests[2]); err != nil {
		t.Errorf("öbür test sürmeli: %v", err)
	}
	if end := v.End(nil); end.Code != 0 || end.Signal != 0 {
		t.Errorf("testteki TR_exit sürecin çıkışını değiştirmemeli: %+v", end)
	}
}

// splitProgramErrors durdurma ve çıkış isteklerini hata saymaz; ertelenen işlerin hatalarını ayırır.
func TestSplitProgramErrors(t *testing.T) {
	failure := errors.New("hata")
	later := errors.New("ertele hatası")
	inTest := &ExitRequest{Code: 3, inTest: true}
	cases := []struct {
		name         string
		err          error
		wantMain     error
		wantDeferred int
	}{
		{"hata yok", nil, nil, 0},
		{"durdurma", ErrInterrupted, nil, 0},
		{"çıkış", &ExitRequest{Code: 3}, nil, 0},
		{"testteki çıkış bir hatadır", inTest, inTest, 0},
		{"hata", failure, failure, 0},
		{"durdurma ve ertele hatası", &deferError{mainSrc: ErrInterrupted, extras: []error{later}}, nil, 1},
		{"hata ve ertele hatası", &deferError{mainSrc: failure, extras: []error{later}}, failure, 1},
	}
	for _, tc := range cases {
		deferred, main := splitProgramErrors(tc.err)
		if main != tc.wantMain || len(deferred) != tc.wantDeferred {
			t.Errorf("%s: ana=%v ertelenen=%d, ana=%v ertelenen=%d bekleniyordu", tc.name, main, len(deferred), tc.wantMain, tc.wantDeferred)
		}
	}
}

// Yakalanmayan hatayla biten programda öbür görevler 1.5'teki gibi durur ve beklenir, sonra en
// dıştaki ertelenen işler çalışır (1.9): görevin ertelenen işi ana programınkinden önce biter.
func TestUncaughtErrorStopsTasksBeforeTheOutermostDefers(t *testing.T) {
	v := New()
	end, err := runEnd(t, v, `# TR 4
TR_var izler = []
TR_defer { izler.TR_append("ana ertele") }
TR_async {
    TR_defer { izler.TR_append("görev ertele") }
    TR_sleep(60)
}
TR_sleep(0.1)
TR_throw "ana hata"
`)
	if err == nil || end.Code != 1 || len(end.Errors) != 1 {
		t.Fatalf("hata ve çıkış 1 bekleniyordu: %v %+v", err, end)
	}
	text := traceText(v)
	task, main := strings.Index(text, "görev ertele"), strings.Index(text, "ana ertele")
	if task < 0 || main < 0 || task > main {
		t.Errorf("görev durdurulup ertelenen işi bitmeli, sonra ana program ertelenen işleri çalışmalı: %s", text)
	}
}

// Görevin ve ertelenen işin hatası TR_exit'in kodundan önce gelir (çıkış 1); ertelenen işten
// çağrılan TR_exit yalnız programın kendi yakalanmayan hatasının 1'ini değiştirir (1.9, 1.13).
func TestErrorsComeBeforeTheExitCode(t *testing.T) {
	for _, tc := range []struct {
		name       string
		code       string
		wantCode   int
		wantErrors int
	}{
		{"görevin hatası, sonra TR_exit", "TR_async { TR_throw \"görev hatası\" }\nTR_wait_all()\nTR_exit(3)", 1, 0},
		{"ertelenen işin hatası, sonra TR_exit", "TR_defer { TR_throw \"ertele hatası\" }\nTR_exit(3)", 1, 1},
		{"ertelenen işten TR_exit programın kendi hatasının kodunu değiştirir", "TR_defer { TR_exit(7) }\nTR_throw \"ana hata\"", 7, 1},
		{"ertelenen iş hatası TR_exit'i geçer", "TR_defer { TR_exit(7) }\nTR_defer { TR_throw \"ertele hatası\" }\nTR_throw \"ana hata\"", 1, 2},
		{"görev hatası TR_exit'i geçer", "TR_async { TR_throw \"görev hatası\" }\nTR_wait_all()\nTR_defer { TR_exit(7) }\nTR_throw \"ana hata\"", 1, 1},
	} {
		v := New()
		var failure strings.Builder
		v.Err = &failure
		end, _ := runEnd(t, v, "# TR 4\n"+tc.code+"\n")
		if end.Code != tc.wantCode || end.Signal != 0 || len(end.Errors) != tc.wantErrors {
			t.Errorf("%s: %+v, kod %d ve %d hata bekleniyordu", tc.name, end, tc.wantCode, tc.wantErrors)
		}
	}
}
